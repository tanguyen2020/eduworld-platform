package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/attempt"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// LifecycleReport counts what one run of the job attempt-lifecycle did.
type LifecycleReport struct {
	Paused, Abandoned, Expired, AutoSubmitted int
}

// RunLifecycle is the job attempt-lifecycle (F030-AC-17…AC-19, every minute in worker profile events): it lists due
// attempts of every tenant under tenancy.Bypass(platform_job), then handles each one in the scope of its tenant and
// its own transaction — idle → paused_at, past the resume window → abandoned (activity.abandoned), past deadline_at +
// deadline_grace → expired and submitted with reason timeout when it has an answer. An attempt changed meanwhile by
// the student (row version) is skipped until the next run.
func (h Attempts) RunLifecycle(ctx context.Context, limit int) (rep LifecycleReport, err error) {
	ctx, done := observability.UseCase(ctx, "learning.job.AttemptLifecycle")
	defer func() { done(err) }()
	if limit <= 0 {
		limit = 200
	}
	pol, now := h.policy(), h.now()
	bctx, err := tenancy.Bypass(tenancy.WithModule(ctx, module), tenancy.ReasonPlatformJob)
	if err != nil {
		return rep, err
	}
	var due []write.DueAttempt
	if err := h.UoW.Do(bctx, func(ctx context.Context) error {
		var err error
		due, err = h.Store.Due(ctx, now.Add(-pol.IdleTimeout), now.Add(-pol.ResumeWindow), now.Add(-pol.DeadlineGrace), limit)
		return err
	}); err != nil {
		return rep, err
	}
	for _, d := range due {
		if err := h.lifecycleOne(ctx, d.TenantID, d.ID, &rep); err != nil {
			var pe *problem.Error
			if asProblem(err, &pe) && pe.Code == problem.VersionMismatch {
				continue
			}
			return rep, err
		}
	}
	return rep, nil
}

func asProblem(err error, pe **problem.Error) bool {
	p, ok := problemOf(err)
	if ok {
		*pe = p
	}
	return ok
}

func (h Attempts) lifecycleOne(ctx context.Context, tenant, id uuid.UUID, rep *LifecycleReport) error {
	sc, err := authz.EventScope(tenant)
	if err != nil {
		return err
	}
	return h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		a, err := h.Store.Attempt(ctx, id)
		if err != nil {
			return err
		}
		pol, now := h.policy(), h.now()
		row := a.RowVersion
		switch {
		case a.Expire(now, pol):
			if h.Metrics != nil {
				h.Metrics.Expired("deadline")
			}
			if a.Answered() == 0 {
				rep.Expired++
				return h.Store.UpdateAttempt(ctx, a, row)
			}
			a.RowVersion = row
			if _, _, err := h.gradeAttempt(ctx, &a, attempt.ReasonTimeout); err != nil {
				return err
			}
			rep.AutoSubmitted++
			return nil
		case a.Abandon(now, pol):
			if err := h.Store.UpdateAttempt(ctx, a, row); err != nil {
				return err
			}
			rep.Abandoned++
			if h.Metrics != nil {
				h.Metrics.Expired("abandoned")
			}
			return h.abandoned(ctx, a, now)
		case a.IdlePause(now, pol):
			rep.Paused++
			return h.Store.UpdateAttempt(ctx, a, row)
		}
		return nil
	})
}

// abandoned records activity.abandoned (learning event + outbox, ACTIVITY_ENGINE §6.2, RB-206).
func (h Attempts) abandoned(ctx context.Context, a attempt.Attempt, at time.Time) error {
	if err := h.learningEvent(ctx, a, EventActivityAbandoned, "abn:"+a.ID.String(), at, a.ObjectiveIDs, "system",
		map[string]any{"attempt_id": a.ID, "activity_type": a.ActivityType, "answered_count": a.Answered(), "reason": "expired"}); err != nil {
		return err
	}
	return h.publish(ctx, a, EventActivityAbandoned, map[string]any{"attempt_id": a.ID, "student_id": a.StudentID,
		"activity_id": nullableID(a.ActivityID), "activity_type": a.ActivityType, "mode": a.Mode, "context_type": a.ContextType,
		"context_id": a.ContextID, "content_version_id": a.SourceVersionID, "answered_count": a.Answered(), "reason": "expired"})
}
