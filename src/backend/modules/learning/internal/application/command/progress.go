package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/analytics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// ProgressDayZone is the calendar of progress_daily.bucket_date: Vietnam time (UTC+7, no daylight saving) [Proposed]
// until a platform setting owns it.
var ProgressDayZone = time.FixedZone("ICT", 7*3600)

// Progress keeps the counters of learning.progress_daily (F031 US3, LEARNING_ANALYTICS §4): consumer
// learning.progress_daily. Exactly-once per event is the processed_events row of the consumer (F031-AC-17).
type Progress struct {
	Store    write.ProgressStore
	Evidence write.EvidenceReader
	Lessons  write.LessonStore // F031 US6: lessons_completed
	// F043 US1: the class/school/tenant daily rollups of the dashboards, in the same transaction and processed_events row
	// (consumer learning.rollup_dashboard = this consumer, plan F043 [Proposed]); nil = progress_daily only.
	Rollups    write.RollupStore
	LateWindow time.Duration // dashboard_late_window_hours
	Late       LateMetrics
	Clock      func() time.Time
}

// LateMetrics counts the late events of the rollups (eduworld_rollup_late_events_total{bucket}).
type LateMetrics interface {
	LateEvent(bucket string)
}

// add writes one delta of an event that occurred at occurred: progress_daily and the rollups of its day, unless the day is
// closed and the event is later than dashboard_late_window_hours (F043-AC-04: rollups unchanged, metric only); a late
// event within the window marks the day for reclosing.
func (h Progress) add(ctx context.Context, d write.ProgressDelta, occurred time.Time) error {
	if h.Rollups == nil {
		return h.Store.AddDaily(ctx, d)
	}
	final, err := h.Rollups.DayFinal(ctx, d.Day)
	if err != nil {
		return err
	}
	window := h.LateWindow
	if window <= 0 {
		window = analytics.DefaultConfig().LateWindow
	}
	switch analytics.Late(occurred, now(h.Clock), window, final) {
	case analytics.Drop:
		if h.Late != nil {
			h.Late.LateEvent(">48h")
		}
		return nil
	case analytics.ApplyReclose:
		if h.Late != nil {
			h.Late.LateEvent("<=48h")
		}
		if err := h.Rollups.MarkReclose(ctx, d.Day); err != nil {
			return err
		}
	}
	if err := h.Store.AddDaily(ctx, d); err != nil {
		return err
	}
	return h.Rollups.AddRollups(ctx, d)
}

// RecordLessonCompleted adds one completed lesson to the day of its completion (F031-AC-30); the student and school come
// from the lesson session of the event.
func (h Progress) RecordLessonCompleted(ctx context.Context, tenantID, sessionID uuid.UUID, subjectID *uuid.UUID, at time.Time) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.UpsertProgressDailyLesson")
	defer func() { done(err) }()
	s, err := h.Lessons.Session(ctx, sessionID)
	if err != nil || s == nil {
		return err
	}
	subject := uuid.Nil
	if subjectID != nil {
		subject = *subjectID
	}
	return h.add(ctx, write.ProgressDelta{TenantID: tenantID, StudentID: s.StudentID, SchoolID: s.SchoolID, SubjectID: subject,
		Day: at.In(ProgressDayZone), LessonsCompleted: 1}, at)
}

// RecordGraded adds a graded attempt: one activity completed (mode ≠ exam), its first scored answers, its active time.
func (h Progress) RecordGraded(ctx context.Context, tenantID, attemptID uuid.UUID, at time.Time) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.UpsertProgressDaily")
	defer func() { done(err) }()
	f, err := h.Evidence.Attempt(ctx, attemptID)
	if err != nil {
		return err
	}
	counts, err := h.Evidence.FirstAnswers(ctx, attemptID)
	if err != nil {
		return err
	}
	if !f.GradedAt.IsZero() {
		at = f.GradedAt
	}
	d := write.ProgressDelta{TenantID: tenantID, StudentID: f.StudentID, SchoolID: f.SchoolID, SubjectID: f.SubjectID,
		Day: at.In(ProgressDayZone), Items: counts, ActiveSeconds: f.DurationMS / 1000}
	if f.Mode != "exam" {
		d.ActivitiesCompleted = 1
	}
	return h.add(ctx, d, at)
}
