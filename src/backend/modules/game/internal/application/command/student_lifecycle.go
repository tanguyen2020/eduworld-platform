package command

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Results of the student lifecycle facade operations (StudentLifecycleResult.status).
const (
	LifecycleDone        = "done"
	LifecycleBlocked     = "blocked"
	LifecycleNothingToDo = "nothing_to_do"
)

// Delete reasons of the game profile (ADR-0028: soft-delete only).
const (
	ReasonInactiveCleanup = "inactive_cleanup"
	ReasonUserMerge       = "user_merge"
)

// StudentLifecycle implements the game part of the identity sagas (F016 T024, ADR-0032; F070 T039, OQ F016-2,
// F045-3): a student with an active game session is not cleaned up nor merged (blocked: active_game_session); the
// cleanup soft-deletes the game profile and its saved outfits, the restore brings them back, the anonymization
// overwrites the nickname and drops the custom avatar, the merge keeps one profile (the target's). Sessions and
// login events are history and stay.
type StudentLifecycle struct {
	UoW      write.UnitOfWork
	Sessions write.SessionRepository
	Profiles write.ProfileRepository
	Presets  write.PresetRepository
	// XP moves the XP ledger of the source to the target (F074 T041); nil = no ledger step.
	XP XPMerger
	// Analytics clears the pseudonymous keys of the student in analytics.events_raw/player_days (F080-AC-07); nil = none.
	Analytics AnalyticsAnonymizer
	Now       func() time.Time
}

// AnalyticsAnonymizer is the analytics step of anonymizeUserData (AnalyticsIngest.AnonymizeStudent).
type AnalyticsAnonymizer interface {
	AnonymizeStudent(ctx context.Context, student uuid.UUID) (raw, days int64, err error)
}

// XPMerger is the XP step of mergeUserData (command.Progression.MergeXP).
type XPMerger interface {
	MergeXP(ctx context.Context, tenant, from, into, mergeID uuid.UUID) (bool, error)
}

func (h StudentLifecycle) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func scopeTenant(ctx context.Context, op string) (uuid.UUID, error) {
	s, ok := tenancy.ScopeFrom(ctx)
	if !ok {
		return uuid.Nil, problem.New(problem.Internal, op+" needs a tenant scope")
	}
	tenant, single := s.SessionTenantID()
	if !single {
		return uuid.Nil, problem.New(problem.Internal, op+" needs one tenant")
	}
	return tenant, nil
}

func outcome(changed bool) string {
	if changed {
		return LifecycleDone
	}
	return LifecycleNothingToDo
}

// Cleanup answers cleanupStudentData for the tenant of the caller's scope.
func (h StudentLifecycle) Cleanup(ctx context.Context, studentID uuid.UUID) (status, blockedReason string, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.CleanupStudentData")
	defer func() { done(err) }()
	tenant, err := scopeTenant(ctx, "cleanupStudentData")
	if err != nil {
		return "", "", err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		active, err := h.Sessions.Active(ctx, tenant, studentID)
		if err != nil {
			return err
		}
		if active != nil {
			status, blockedReason = LifecycleBlocked, "active_game_session"
			return nil
		}
		removed, err := h.Profiles.SoftDelete(ctx, tenant, studentID, ReasonInactiveCleanup, h.now())
		status = outcome(removed)
		return err
	})
	return status, blockedReason, err
}

// Restore answers restoreStudentData: the profile removed by the cleanup comes back (its nickname is cleared when
// another student of the tenant took it meanwhile; the student then chooses again).
func (h StudentLifecycle) Restore(ctx context.Context, studentID uuid.UUID) (status string, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.RestoreStudentData")
	defer func() { done(err) }()
	tenant, err := scopeTenant(ctx, "restoreStudentData")
	if err != nil {
		return "", err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		restored, err := h.Profiles.Restore(ctx, tenant, studentID, ReasonInactiveCleanup)
		status = outcome(restored)
		return err
	})
	return status, err
}

// AnonymousNickname is the nickname an anonymized profile keeps: stable per user, unique in practice, no personal
// data ("an_" + 16 hex of SHA-256 of the user id, 19 characters ≤ 32).
func AnonymousNickname(user uuid.UUID) string {
	sum := sha256.Sum256([]byte("game-nickname:" + user.String()))
	return "an_" + hex.EncodeToString(sum[:8])
}

// Anonymize answers anonymizeUserData: the nickname becomes AnonymousNickname, the custom avatar, settings and
// outfit names are removed; level and avatar_created_at stay for statistics. game.nickname_changes is append-only
// and keeps the generated nicknames (words of the approved catalog, never typed by the student).
func (h StudentLifecycle) Anonymize(ctx context.Context, userID uuid.UUID) (status string, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.AnonymizeUserData")
	defer func() { done(err) }()
	tenant, err := scopeTenant(ctx, "anonymizeUserData")
	if err != nil {
		return "", err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		changed, err := h.Profiles.Anonymize(ctx, tenant, userID, AnonymousNickname(userID))
		if err != nil {
			return err
		}
		if h.Analytics != nil {
			raw, days, err := h.Analytics.AnonymizeStudent(ctx, userID)
			if err != nil {
				return err
			}
			changed = changed || raw+days > 0
		}
		status = outcome(changed)
		return nil
	})
	return status, err
}

// Merge answers mergeUserData (idempotent per merge_id: a second call finds nothing left to move): saved outfits
// move to the free slots of the target, then the target keeps its own profile and the source profile is
// soft-deleted, or the target takes the source profile when it has none. An active game session of the source
// blocks the step.
func (h StudentLifecycle) Merge(ctx context.Context, from, into, mergeID uuid.UUID) (status, blockedReason string, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.MergeUserData")
	defer func() { done(err) }()
	tenant, err := scopeTenant(ctx, "mergeUserData")
	if err != nil {
		return "", "", err
	}
	if from == into {
		return "", "", problem.Invalid("target_user_id", problem.SubInvalid, "must differ from user_id")
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		active, err := h.Sessions.Active(ctx, tenant, from)
		if err != nil {
			return err
		}
		if active != nil {
			status, blockedReason = LifecycleBlocked, "active_game_session"
			return nil
		}
		src, err := h.Profiles.Get(ctx, tenant, from)
		if err != nil {
			return err
		}
		if src == nil {
			status = LifecycleNothingToDo
			return nil
		}
		dst, err := h.Profiles.Get(ctx, tenant, into)
		if err != nil {
			return err
		}
		now := h.now()
		if h.XP != nil && mergeID != uuid.Nil {
			if _, err := h.XP.MergeXP(ctx, tenant, from, into, mergeID); err != nil {
				return err
			}
		}
		if err := h.Presets.MoveStudent(ctx, tenant, from, into, ReasonUserMerge, now); err != nil {
			return err
		}
		if dst == nil {
			_, err = h.Profiles.Reassign(ctx, tenant, from, into)
		} else {
			_, err = h.Profiles.SoftDelete(ctx, tenant, from, ReasonUserMerge, now)
		}
		status = LifecycleDone
		return err
	})
	return status, blockedReason, err
}
