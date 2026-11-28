package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/social"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Results of the lifecycle facade operations (StudentLifecycleResult / UserLifecycleResult status).
const (
	LifecycleDone        = "done"
	LifecycleBlocked     = "blocked"
	LifecycleNothingToDo = "nothing_to_do"

	// BlockedOpenModerationCase is the blocked_reason of a cleanup while a moderation case of the student is open
	// (ACCOUNT_PROVISIONING §6b).
	BlockedOpenModerationCase = "open_moderation_case"
)

// Audit actions of the lifecycle steps (ids and counts only, never content — R14).
const (
	AuditStudentCleanedUp = "chat.student.cleaned_up"
	AuditStudentRestored  = "chat.student.restored"
	AuditUserAnonymized   = "chat.user.anonymized"
	AuditUserMerged       = "chat.user.merged"
)

// sanctionCopyNamespace derives the id of the copy of a sanction on the target of a merge (one per merge and sanction).
var sanctionCopyNamespace = uuid.MustParse("6f1d3b2a-8c4e-5a7f-9b0d-1e2c3a4b5d6f")

// UserLifecycle implements the chat part of the identity sagas (F115 T042, OQ F016-2, F045-3; ADR-0028, ADR-0032;
// ACCOUNT_PROVISIONING §6b, §7, §7b; CHAT_SYSTEM §6, §6.1):
//
//   - cleanup (student cleaned up for inactivity): blocked while a moderation case of the student is open; otherwise the
//     active channel memberships are soft-deleted (delete_reason inactive_cleanup). Messages stay (history of the others,
//     evidence); nothing is deleted physically (R21, R24).
//   - restore: the memberships soft-deleted by the cleanup come back in the live persistent channels (gameplay channels
//     are temporary and follow their owner module).
//   - anonymize: the messages of the user in chat.messages are anonymised like persistent-chat-retention (evidence of
//     reports and cases copied first, then body, body_display, quote and revisions overwritten), the quotes of the
//     replies to them are cleared, the projected card (nickname, avatar, title) is cleared and the memberships end.
//     Gameplay messages are not touched: only gameplay-chat-purge removes them, within the gameplay retention (ADR-0033).
//   - merge: the memberships of the source in the live persistent channels move to the target (clubs follow their own
//     flow, gameplay channels their owner module); the live blocks and mutes of the source are carried to the target and
//     its active sanctions copied to the target (a merge never lifts a protection or a sanction, CHILD_SAFETY). Messages
//     stay under the original sender_id (history and evidence integrity).
//
// Every step runs in the TenantScope of the saga step (R8), is idempotent (a repeated call finds nothing left to do,
// R10) and records one audit with ids and counts only (R14).
type UserLifecycle struct {
	UoW         write.UnitOfWork
	AfterCommit write.AfterCommit
	Store       write.LifecycleStore
	Audit       write.Auditor
	Facts       FactsInvalidator // nil: no cache
	// Social carries the blocks and mutes of a merge (social repository, events, relation caches).
	Social Social
	// Safety copies the active sanctions of a merge (sanction.applied, audit, sanction cache).
	Safety Safety
	// EvidenceContext is the number of messages copied around a referenced message (moderation_evidence_context).
	EvidenceContext int
	// Batch is the number of messages anonymised per transaction (default 500).
	Batch int
	Now   func() time.Time
}

func (h UserLifecycle) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

// lifecycleTenant is the session tenant of the saga step (x-context-required: tenant_scope).
func lifecycleTenant(ctx context.Context, op string) (uuid.UUID, error) {
	s, ok := tenancy.ScopeFrom(ctx)
	if !ok {
		return uuid.Nil, problem.New(problem.Unauthenticated, op+" needs a tenant scope")
	}
	tenant, single := s.SessionTenantID()
	if !single {
		return uuid.Nil, problem.New(problem.PermissionDenied, op+" needs one tenant")
	}
	return tenant, nil
}

func lifecycleOutcome(changed bool) string {
	if changed {
		return LifecycleDone
	}
	return LifecycleNothingToDo
}

// invalidateAll drops the cached facts of the channels after the commit.
func (h UserLifecycle) invalidateAll(ctx context.Context, tenant uuid.UUID, channels []uuid.UUID) error {
	if h.Facts == nil || h.AfterCommit == nil || len(channels) == 0 {
		return nil
	}
	list := append([]uuid.UUID(nil), channels...)
	return h.AfterCommit(ctx, func(ctx context.Context) {
		for _, id := range list {
			_ = h.Facts.InvalidateChannel(ctx, tenant, id)
		}
	})
}

func (h UserLifecycle) record(ctx context.Context, action string, user uuid.UUID, meta map[string]any) error {
	if h.Audit == nil {
		return nil
	}
	return h.Audit.Record(ctx, audit.Event{Action: action, TargetType: "student", TargetID: user.String(), Result: audit.ResultSuccess,
		Metadata: meta})
}

// Cleanup answers cleanupStudentData (ADR-0032): status and blocked_reason.
func (h UserLifecycle) Cleanup(ctx context.Context, student uuid.UUID) (status, reason string, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.CleanupStudentData")
	defer func() { done(err) }()
	tenant, err := lifecycleTenant(ctx, "cleanupStudentData")
	if err != nil {
		return "", "", err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		open, err := h.Store.OpenCaseOf(ctx, tenant, student)
		if err != nil {
			return err
		}
		if open {
			status, reason = LifecycleBlocked, BlockedOpenModerationCase
			return nil
		}
		channels, err := h.Store.EndMemberships(ctx, tenant, student, write.MemberDeletedCleanup, h.now())
		if err != nil {
			return err
		}
		status = lifecycleOutcome(len(channels) > 0)
		if len(channels) == 0 {
			return nil
		}
		if err := h.record(ctx, AuditStudentCleanedUp, student, map[string]any{"memberships_ended": len(channels)}); err != nil {
			return err
		}
		return h.invalidateAll(ctx, tenant, channels)
	})
	return status, reason, err
}

// Restore answers restoreStudentData (ADR-0032 §5, compensation of a cleanup saga).
func (h UserLifecycle) Restore(ctx context.Context, student uuid.UUID) (status string, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.RestoreStudentData")
	defer func() { done(err) }()
	tenant, err := lifecycleTenant(ctx, "restoreStudentData")
	if err != nil {
		return "", err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		channels, err := h.Store.RestoreMemberships(ctx, tenant, student, write.MemberDeletedCleanup, h.now())
		if err != nil {
			return err
		}
		status = lifecycleOutcome(len(channels) > 0)
		if len(channels) == 0 {
			return nil
		}
		if err := h.record(ctx, AuditStudentRestored, student, map[string]any{"memberships_restored": len(channels)}); err != nil {
			return err
		}
		return h.invalidateAll(ctx, tenant, channels)
	})
	return status, err
}

// Anonymize answers anonymizeUserData (ADR-0028 §8, retention or erasure request): messages in batches (one
// transaction each), then the card and the memberships with the aggregate audit.
func (h UserLifecycle) Anonymize(ctx context.Context, user uuid.UUID) (status string, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.AnonymizeUserData")
	defer func() { done(err) }()
	tenant, err := lifecycleTenant(ctx, "anonymizeUserData")
	if err != nil {
		return "", err
	}
	batch := h.Batch
	if batch <= 0 {
		batch = 500
	}
	anonymized, copied := 0, 0
	for {
		n := 0
		err := h.UoW.Do(ctx, func(ctx context.Context) error {
			at := h.now()
			msgs, err := h.Store.AuthoredMessages(ctx, tenant, user, batch)
			if err != nil || len(msgs) == 0 {
				return err
			}
			c, err := h.Store.CopyEvidence(ctx, tenant, msgs, h.EvidenceContext, at)
			if err != nil {
				return err
			}
			copied += c
			n, err = h.Store.AnonymizeMessages(ctx, tenant, msgs, at)
			return err
		})
		if err != nil {
			return "", err
		}
		anonymized += n
		if n < batch {
			break
		}
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		card, err := h.Store.AnonymizeCard(ctx, tenant, user)
		if err != nil {
			return err
		}
		channels, err := h.Store.EndMemberships(ctx, tenant, user, write.MemberDeletedAnonymized, h.now())
		if err != nil {
			return err
		}
		status = lifecycleOutcome(anonymized > 0 || card || len(channels) > 0)
		if status == LifecycleNothingToDo {
			return nil
		}
		if err := h.record(ctx, AuditUserAnonymized, user, map[string]any{"messages_anonymized": anonymized, "evidence_copied": copied,
			"card_cleared": card, "memberships_ended": len(channels)}); err != nil {
			return err
		}
		return h.invalidateAll(ctx, tenant, channels)
	})
	return status, err
}

// MergeInput is the merge step of the user_merge saga of module identity.
type MergeInput struct {
	MergeID uuid.UUID
	From    uuid.UUID
	Into    uuid.UUID
}

// Merge answers mergeUserData (user_merge saga, platform.user.merge of the System Admin who approved the merge,
// idempotent per merge_id): status and blocked_reason.
func (h UserLifecycle) Merge(ctx context.Context, in MergeInput) (status, reason string, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.MergeUserData")
	defer func() { done(err) }()
	p, ok := authz.PrincipalFrom(ctx)
	if !ok {
		return "", "", problem.New(problem.Unauthenticated, "")
	}
	if err := authz.Require(ctx, p, authz.PermPlatformUserMerge, authz.Resource{}); err != nil {
		return "", "", err
	}
	tenant, err := lifecycleTenant(ctx, "mergeUserData")
	if err != nil {
		return "", "", err
	}
	switch {
	case in.MergeID == uuid.Nil || in.From == uuid.Nil || in.Into == uuid.Nil:
		return "", "", problem.Invalid("merge_id", problem.SubRequired, "merge_id, user_id and target_user_id are required")
	case in.From == in.Into:
		return LifecycleBlocked, "the source and target users are the same", nil
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		now := h.now()
		if err := h.Social.Repo.LockUsers(ctx, in.From, in.Into); err != nil {
			return err
		}
		moved, err := h.Store.MoveMemberships(ctx, tenant, in.From, in.Into, now)
		if err != nil {
			return err
		}
		blocks, mutes, err := h.carryRelations(ctx, tenant, in.From, in.Into, now)
		if err != nil {
			return err
		}
		sanctions, err := h.copySanctions(ctx, tenant, in, now)
		if err != nil {
			return err
		}
		status = lifecycleOutcome(len(moved) > 0 || blocks > 0 || mutes > 0 || sanctions > 0)
		if status == LifecycleNothingToDo {
			return nil
		}
		if err := h.record(ctx, AuditUserMerged, in.From, map[string]any{"merge_id": in.MergeID.String(), "target_user_id": in.Into.String(),
			"memberships_moved": len(moved), "blocks_carried": blocks, "mutes_carried": mutes, "sanctions_copied": sanctions}); err != nil {
			return err
		}
		// The facts of every channel of the target carry its members, cards and blocks: dropped after the commit.
		live, err := h.Store.LiveChannels(ctx, tenant, in.Into)
		if err != nil {
			return err
		}
		return h.invalidateAll(ctx, tenant, append(moved, live...))
	})
	return status, reason, err
}

// carryRelations gives the target the live blocks and mutes of the source, both directions (a student who blocked or
// muted the source keeps that protection against the merged account). A block also ends the friendship and the pending
// requests of the pair, as blockUser does (F095-AC-18).
func (h UserLifecycle) carryRelations(ctx context.Context, tenant, from, into uuid.UUID, now time.Time) (blocks, mutes int, err error) {
	rels, err := h.Store.Relations(ctx, tenant, from)
	if err != nil {
		return 0, 0, err
	}
	s := h.Social
	for _, r := range rels {
		a, b := r.From, r.To
		if a == from {
			a = into
		}
		if b == from {
			b = into
		}
		if a == b {
			continue // the source blocked or muted the target itself
		}
		if r.Kind == "mute" {
			created, err := s.Repo.SetMute(ctx, tenant, a, b, true)
			if err != nil {
				return blocks, mutes, err
			}
			if !created {
				continue
			}
			if err := s.publish(ctx, "user.muted", a, tenant, now, map[string]any{"tenant_id": tenant, "muter_id": a, "muted_id": b,
				"source_version": version(now)}); err != nil {
				return blocks, mutes, err
			}
			mutes++
			if err := s.invalidateRelations(ctx, tenant, a, b); err != nil {
				return blocks, mutes, err
			}
			continue
		}
		created, err := s.Repo.SetBlock(ctx, tenant, a, b, true)
		if err != nil {
			return blocks, mutes, err
		}
		if !created {
			continue
		}
		removed, err := s.Repo.RemoveFriendship(ctx, tenant, social.NewPair(a, b), social.RemovedByBlock)
		if err != nil {
			return blocks, mutes, err
		}
		if removed {
			if err := s.friendRemoved(ctx, tenant, a, b, social.RemovedByBlock, now); err != nil {
				return blocks, mutes, err
			}
		}
		if _, err := s.Repo.CancelPending(ctx, tenant, a, b, now); err != nil {
			return blocks, mutes, err
		}
		if err := s.publish(ctx, "user.blocked", a, tenant, now, map[string]any{"tenant_id": tenant, "blocker_id": a, "blocked_id": b,
			"source_version": version(now)}); err != nil {
			return blocks, mutes, err
		}
		blocks++
		if err := s.invalidateRelations(ctx, tenant, a, b); err != nil {
			return blocks, mutes, err
		}
		if err := s.invalidate(ctx, a, b); err != nil {
			return blocks, mutes, err
		}
	}
	return blocks, mutes, nil
}

// copySanctions copies the active sanctions of the source to the target (same type, scope, case, reason code and end;
// issued by the system; sanction.applied + audit), so a merge never lifts a sanction. The copy of a case hold keeps its
// case: resolving the case releases it with the original.
func (h UserLifecycle) copySanctions(ctx context.Context, tenant uuid.UUID, in MergeInput, now time.Time) (int, error) {
	list, err := h.Store.ActiveSanctions(ctx, tenant, in.From, now)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, s := range list {
		id := uuid.NewSHA1(sanctionCopyNamespace, []byte(in.MergeID.String()+":"+s.ID.String()))
		exists, err := h.Store.SanctionExists(ctx, tenant, id)
		if err != nil {
			return n, err
		}
		if exists {
			continue
		}
		c := write.SafetySanction{ID: id, TenantID: tenant, SchoolID: s.SchoolID, UserID: in.Into, Type: s.Type, Scope: s.Scope,
			ChannelID: s.ChannelID, Reason: s.Reason, ReasonCode: s.ReasonCode, CaseID: s.CaseID, IssuedByKind: "system",
			StartsAt: now, EndsAt: s.EndsAt}
		if err := h.Safety.applySanction(ctx, c); err != nil {
			return n, err
		}
		if err := h.Safety.later(ctx, in.Into, &c); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
