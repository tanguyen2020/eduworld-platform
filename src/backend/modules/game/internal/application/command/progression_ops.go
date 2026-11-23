package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// GradeQuestExpirer expires the active grade-bound quests of a student that do not serve the new grade (F072 quest
// engine, command.Quests); returns how many ended.
type GradeQuestExpirer interface {
	ExpireForGrade(ctx context.Context, tenant, student uuid.UUID, grade int, at time.Time) (int, error)
}

// SchoolYear is the school year label (YYYY-YYYY) of t: a school year starts on 1 August, calendar Asia/Ho_Chi_Minh
// [Proposed — CONFIG_DEFAULTS has no school-year start; FR-GAM-010 keys the transition by school year].
func SchoolYear(t time.Time) string {
	l := t.In(capZone)
	y := l.Year()
	if l.Month() < time.August {
		y--
	}
	return fmt.Sprintf("%04d-%04d", y, y+1)
}

// ApplyGradeTransition is the consumer game.grade_transition (F074-AC-25, AC-26, FR-GAM-010): when the grade of a
// student changed, XP, level, title and ledger stay; the active learning/daily/weekly quests whose grades do not hold
// the new grade expire without penalty; grade_transition_year records the school year. A repeated event finds nothing
// left to expire and the year already set: no change, no message. An unchanged grade does nothing. Runs in the consumer
// transaction.
func (h Progression) ApplyGradeTransition(ctx context.Context, tenant, student uuid.UUID, grade int, changed bool, at time.Time) (applied bool, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.ApplyGradeTransition")
	defer func() { done(err) }()
	if !changed || grade < 1 || grade > 9 {
		return false, nil
	}
	if err := h.Store.LockStudent(ctx, student); err != nil {
		return false, err
	}
	expired := 0
	if h.Quests != nil {
		if expired, err = h.Quests.ExpireForGrade(ctx, tenant, student, grade, h.now()); err != nil {
			return false, err
		}
	}
	state, err := h.Store.Progress(ctx, tenant, student)
	if err != nil {
		return false, err
	}
	year := SchoolYear(at)
	if state != nil && (state.GradeYear == nil || *state.GradeYear != year) {
		if err := h.Store.SetGradeYear(ctx, tenant, student, year); err != nil {
			return false, err
		}
		applied = true
	}
	return applied || expired > 0, nil
}

// RolloutCurve is one run of the job curve-rollout (F074-AC-21, AC-22): every live profile not yet on the current curve
// version is moved to it in its own tenant transaction — the level never decreases (floor protection), every level the
// new thresholds give is reached in order with its level.reached and its rewards (idempotent per (student, level,
// curve)). A profile already on the version is skipped, so a rerun changes nothing.
func (h Progression) RolloutCurve(ctx context.Context, batch int) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.RolloutCurve")
	defer func() { done(err) }()
	ps, err := authz.SystemPlatformScope("curve-rollout")
	if err != nil {
		return 0, err
	}
	var refs []write.ProfileRef
	var version int
	if err := h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
		c, err := h.Curves.Current(ctx)
		if err != nil || c == nil {
			return err
		}
		version = c.Version
		refs, err = h.Scan.BehindCurve(ctx, c.Version, batch)
		return err
	}); err != nil || version == 0 {
		return 0, err
	}
	var errs []error
	for _, ref := range refs {
		sc, err := authz.EventScope(ref.TenantID)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		now := h.now()
		err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
			if err := h.Store.LockStudent(ctx, ref.StudentID); err != nil {
				return err
			}
			state, err := h.Store.Progress(ctx, ref.TenantID, ref.StudentID)
			if err != nil || state == nil {
				return err
			}
			curve, err := h.Curves.Current(ctx)
			if err != nil || curve == nil || (state.CurveVersion != nil && *state.CurveVersion == curve.Version) {
				return err
			}
			school := state.SchoolID
			levels, err := h.levelUp(ctx, ref.TenantID, ref.StudentID, &school, state, curve, nil, now)
			if err != nil {
				return err
			}
			if err := h.Store.SaveProgress(ctx, ref.TenantID, ref.StudentID, *state); err != nil {
				return err
			}
			h.push(ctx, ref.StudentID, pushes{levels: levels}, now)
			n++
			return nil
		})
		if err != nil {
			errs = append(errs, err)
		}
	}
	return n, errors.Join(errs...)
}

// ReconcileXP is one run of the nightly job xp-reconcile (F074-AC-03, SC-003): every live profile whose xp_total differs
// from Σ xp_ledger raises eduworld_game_ledger_mismatch_total{ledger="xp"} and an audit record (warning); nothing is
// corrected (corrections are compensating entries, F075).
func (h Progression) ReconcileXP(ctx context.Context, batch int) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.ReconcileXP")
	defer func() { done(err) }()
	ps, err := authz.SystemPlatformScope("xp-reconcile")
	if err != nil {
		return 0, err
	}
	var after *write.ProfileRef
	var errs []error
	for {
		var page []write.ProfileMismatch
		if err := h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
			var err error
			page, err = h.Scan.Mismatched(ctx, after, batch)
			return err
		}); err != nil {
			return n, err
		}
		for _, m := range page {
			n++
			h.metric(func(m ProgressionMetrics) { m.LedgerMismatch() })
			sc, err := authz.EventScope(m.TenantID)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			mm := m
			if err := h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
				return h.Audit.Record(ctx, audit.Event{Action: "game.xp_ledger.mismatch", TargetType: "game_profile", TargetID: mm.StudentID.String(),
					Result: audit.ResultFailure, Metadata: map[string]any{"ledger": "xp", "cached": mm.Cached, "ledger_total": mm.Ledger}})
			}); err != nil {
				errs = append(errs, err)
			}
		}
		if len(page) < batch || len(page) == 0 {
			break
		}
		last := page[len(page)-1].ProfileRef
		after = &last
	}
	return n, errors.Join(errs...)
}

// MergeXP is the XP step of mergeUserData (F074 T041, OQ F016-2, IDENTITY_MAPPING §7): the ledger of the source user is
// moved to the target by two compensating entries (−total on the source, +total on the target; append-only, nothing
// rewritten), idempotent per merge_id. The target keeps the higher level (floor) and reaches the levels the summed XP
// gives. When the target has no profile the lifecycle reassigns the source profile (and its cache) to it. Runs in the
// transaction of the caller.
func (h Progression) MergeXP(ctx context.Context, tenant, from, into, mergeID uuid.UUID) (bool, error) {
	if err := h.Store.LockStudent(ctx, into); err != nil {
		return false, err
	}
	if err := h.Store.LockStudent(ctx, from); err != nil {
		return false, err
	}
	total, err := h.Store.LedgerTotal(ctx, tenant, from)
	if err != nil || total == 0 {
		return false, err
	}
	now := h.now()
	src, err := h.Store.Progress(ctx, tenant, from)
	if err != nil {
		return false, err
	}
	dst, err := h.Store.Progress(ctx, tenant, into)
	if err != nil {
		return false, err
	}
	post := func(student uuid.UUID, school *uuid.UUID, amount int64) (bool, write.XpEntry, error) {
		e := write.XpEntry{ID: ids.New(), TenantID: tenant, StudentID: student, SchoolID: school, Amount: int(amount), ReasonCode: write.ReasonAdjustment,
			SourceType: "merge", SourceCode: "merge.adjustment", SourceID: mergeID, IdempotencyKey: Key("merge", mergeID.String(), student.String()), At: now}
		ok, err := h.Store.InsertEntry(ctx, e)
		return ok, e, err
	}
	var srcSchool, dstSchool *uuid.UUID
	if src != nil {
		s := src.SchoolID
		srcSchool = &s
	}
	if dst != nil {
		s := dst.SchoolID
		dstSchool = &s
	}
	ok, out, err := post(from, srcSchool, -total)
	if err != nil || !ok {
		return false, err // already merged under this merge_id
	}
	_, in, err := post(into, dstSchool, total)
	if err != nil {
		return false, err
	}
	var levels []map[string]any
	if dst != nil {
		dst.XPTotal += total
		if src != nil && src.Level > dst.Level {
			dst.Level = src.Level
		}
		curve, err := h.Curves.Current(ctx)
		if err != nil {
			return false, err
		}
		entry := in.ID
		if levels, err = h.levelUp(ctx, tenant, into, dstSchool, dst, curve, &entry, now); err != nil {
			return false, err
		}
		if err := h.Store.SaveProgress(ctx, tenant, into, *dst); err != nil {
			return false, err
		}
		if src != nil {
			src.XPTotal = 0
			if err := h.Store.SaveProgress(ctx, tenant, from, *src); err != nil {
				return false, err
			}
		}
	}
	dstTotal, dstLevel, srcLevel := total, 1, 1
	if dst != nil {
		dstTotal, dstLevel = dst.XPTotal, dst.Level
	}
	if src != nil {
		srcLevel = src.Level
		if dst == nil {
			dstLevel = src.Level
		}
	}
	srcTotal := int64(0)
	if dst == nil && src != nil {
		srcTotal = src.XPTotal // the profile moves to the target with its cache
	}
	for _, x := range []struct {
		e     write.XpEntry
		total int64
		level int
	}{{out, srcTotal, srcLevel}, {in, dstTotal, dstLevel}} {
		if err := h.Events.Publish(ctx, write.Event{Type: EventXPAwarded, SubjectType: "student", SubjectID: x.e.StudentID.String(),
			PartitionKey: x.e.StudentID.String(), TenantID: &tenant, OccurredAt: now, Payload: xpAwardedPayload(x.e, x.total, x.level)}); err != nil {
			return false, err
		}
	}
	h.push(ctx, into, pushes{levels: levels}, now)
	return true, nil
}
