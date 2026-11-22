package command

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/questrules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

func newID() uuid.UUID { return ids.New() }

func setResult(ctx context.Context, v any) error { return idempotency.SetResult(ctx, v) }

// GrantPeriodicQuests is the consumer game.periodic_quests_on_session_started (F072-AC-22, QUEST_SYSTEM §2.4): on the first
// game session of a period the student gets daily_quests_per_day auto_daily quests and, on the first session of the
// week, weekly_quests_per_week auto_weekly quests — assigned to the tenant, of its grade, of an enabled subject, within
// availability, weak objectives first — each with its period_key and expiring at the next reset. One grant row per
// student, kind and period: a later session of the same period grants nothing. Runs in the consumer transaction.
func (h Quests) GrantPeriodicQuests(ctx context.Context, tenant, student uuid.UUID, at time.Time) (err error) {
	ctx, done := observability.UseCase(ctx, "game.command.GrantPeriodicQuests")
	defer func() { done(err) }()
	if at.IsZero() {
		at = h.now()
	}
	if err := h.Store.LockStudent(ctx, student); err != nil {
		return err
	}
	st, err := h.World.Student(ctx, tenant, student)
	if err != nil || st == nil || st.Deleted || st.Status != "active" {
		return err
	}
	rows, err := h.Content.AssignedQuests(ctx, tenant, st.Grade)
	if err != nil {
		return err
	}
	hist, err := h.Content.Histories(ctx, tenant, student)
	if err != nil {
		return err
	}
	f, err := questrules.Facts(ctx, h.World, h.Content, tenant, student, st, hist)
	if err != nil {
		return err
	}
	for _, kind := range []struct {
		name, trigger, recurrence string
		n                         int
	}{{"daily", quest.TriggerAutoDaily, quest.RecurrenceDaily, h.Config.DailyPerDay},
		{"weekly", quest.TriggerAutoWeekly, quest.RecurrenceWeekly, h.Config.WeeklyPerWeek}} {
		if kind.n <= 0 {
			continue
		}
		period := h.Config.Reset.PeriodKey(kind.recurrence, at)
		var cands []quest.Candidate
		byID := map[string]int{}
		for i, r := range rows {
			v := r.Version
			if v.StartTrigger.Kind != kind.trigger {
				continue
			}
			if d := quest.Decide(v, questrules.Student(st), f, at); !d.Visible || !d.Met {
				continue
			}
			hs := hist[r.QuestID]
			if hs != nil && hs.Latest != nil && hs.Latest.Active() {
				continue
			}
			if r, _ := quest.CanRepeat(v, questrules.History(hs, v, h.Config.Reset, at), at); r != quest.RepeatOK {
				continue
			}
			cands = append(cands, quest.Candidate{QuestID: r.QuestID.String(), ObjectiveIDs: v.ObjectiveIDs})
			byID[r.QuestID.String()] = i
		}
		picked := quest.SelectPeriodic(cands, f.Mastery, kind.n, student.String()+":"+period)
		ids := make([]uuid.UUID, 0, len(picked))
		for _, id := range picked {
			ids = append(ids, rows[byID[id]].QuestID)
		}
		ok, err := h.Store.InsertGrant(ctx, tenant, student, kind.name, period, ids, at)
		if err != nil {
			return err
		}
		if !ok {
			continue // this period was already granted
		}
		for _, id := range picked {
			row := rows[byID[id]]
			completed := 0
			if hs := hist[row.QuestID]; hs != nil {
				completed = hs.Completed
			}
			if _, err := h.start(ctx, row, st, tenant, student, kind.trigger, nil, completed+1, at); err != nil {
				return err
			}
		}
	}
	return nil
}

// forEach runs fn for each progress ref listed under the platform scope, each in the transaction of its tenant (jobs).
func (h Quests) forEach(ctx context.Context, job string, list func(ctx context.Context) ([]write.ProgressRef, error),
	fn func(ctx context.Context, p *quest.Progress, v quest.Version, now time.Time) (bool, error)) (int, error) {
	ps, err := authz.SystemPlatformScope(job)
	if err != nil {
		return 0, err
	}
	var refs []write.ProgressRef
	if err := h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
		var err error
		refs, err = list(ctx)
		return err
	}); err != nil {
		return 0, err
	}
	n := 0
	var errs []error
	for _, ref := range refs {
		sc, err := authz.EventScope(ref.TenantID)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		var push map[string]any
		var changed bool
		now := h.now()
		err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
			if err := h.Store.LockStudent(ctx, ref.StudentID); err != nil {
				return err
			}
			p, err := h.Store.ByID(ctx, ref.TenantID, ref.ID)
			if err != nil || p == nil || !p.Active() {
				return err
			}
			v, err := h.version(ctx, p)
			if err != nil {
				return err
			}
			if changed, err = fn(ctx, p, v, now); err != nil || !changed {
				return err
			}
			push = progressPush(p, "", nil)
			return nil
		})
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if changed {
			n++
			h.push(ctx, ref.StudentID, push, now)
		}
	}
	return n, errors.Join(errs...)
}

// ExpirePeriodQuests is one run of the job quest-period-expiry (F072-AC-23): an active progress whose period is over
// (expires_at, the next quest_reset_time) becomes expired with quest.abandoned (reason expired). Nothing is taken from
// the student: no XP or coin is removed, there is no streak to lose (NFR-SAFE-003, AGENTS R19).
func (h Quests) ExpirePeriodQuests(ctx context.Context, batch int) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.ExpirePeriodQuests")
	defer func() { done(err) }()
	return h.forEach(ctx, "quest-period-expiry", func(ctx context.Context) ([]write.ProgressRef, error) {
		return h.Store.Expiring(ctx, h.now(), batch)
	}, func(ctx context.Context, p *quest.Progress, v quest.Version, now time.Time) (bool, error) {
		if p.ExpiresAt == nil || p.ExpiresAt.After(now) {
			return false, nil
		}
		return true, h.end(ctx, p, v, AbandonExpired, "", "", now)
	})
}

// ApplyQuestFailures is one run of the job quest-failure-timer (F072-AC-26): a progress past its time limit applies
// on_fail of time_limit_exceeded — reset_step (default) resets only the step of the condition (or the active steps)
// and restarts the timer, fail_quest ends it as failed (it may be accepted again at once while available); a progress
// whose availability ended applies availability_ended — fail_quest → failed, otherwise expired. No XP or coin is
// taken.
func (h Quests) ApplyQuestFailures(ctx context.Context, batch int) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.ApplyQuestFailures")
	defer func() { done(err) }()
	timed, err := h.forEach(ctx, "quest-failure-timer", func(ctx context.Context) ([]write.ProgressRef, error) {
		return h.Store.Overdue(ctx, h.now(), batch)
	}, func(ctx context.Context, p *quest.Progress, v quest.Version, now time.Time) (bool, error) {
		if p.DeadlineAt == nil || p.DeadlineAt.After(now) {
			return false, nil
		}
		f, ok := v.Failure(quest.FailTimeLimit)
		if !ok {
			p.DeadlineAt = nil
			return true, h.Store.Save(ctx, p)
		}
		if f.Policy() == quest.OnFailQuest {
			return true, h.end(ctx, p, v, AbandonFailed, f.StepID, quest.FailTimeLimit, now)
		}
		p.ResetSteps(v, f.StepID, now)
		return true, h.Store.Save(ctx, p)
	})
	ended, err2 := h.forEach(ctx, "quest-failure-timer", func(ctx context.Context) ([]write.ProgressRef, error) {
		return h.Store.ActiveEndingBy(ctx, h.now(), batch)
	}, func(ctx context.Context, p *quest.Progress, v quest.Version, now time.Time) (bool, error) {
		if _, to := v.Window(); to == nil || to.After(now) {
			return false, nil
		}
		if f, ok := v.Failure(quest.FailAvailability); ok && f.Policy() == quest.OnFailQuest {
			return true, h.end(ctx, p, v, AbandonFailed, f.StepID, quest.FailAvailability, now)
		}
		return true, h.end(ctx, p, v, AbandonExpired, "", "", now)
	})
	return timed + ended, errors.Join(err, err2)
}
