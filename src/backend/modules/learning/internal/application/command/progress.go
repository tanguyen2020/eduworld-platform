package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
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
	return h.Store.AddDaily(ctx, d)
}
