package consumer

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// ProgressRecorder adds a graded attempt to progress_daily (command.Progress).
type ProgressRecorder interface {
	RecordGraded(ctx context.Context, tenantID, attemptID uuid.UUID, at time.Time) error
}

// ProgressDaily is the consumer learning.progress_daily (F031-AC-17): counters per student × subject × day, exactly once
// per event (processed_events of pkg/outbox.Consumer).
type ProgressDaily struct {
	Record ProgressRecorder
}

// ProgressDailyOptions returns the outbox consumer options (tenant events only).
func ProgressDailyOptions(h ProgressDaily, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "learning", Name: messaging.ProgressDailyName, Stream: messaging.AttemptStream,
		Subjects: messaging.ProgressDailySubjects(), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c ProgressDaily) Handle(ctx context.Context, e outbox.Envelope) error {
	id, err := attemptOf(e)
	if err != nil {
		return err
	}
	return c.Record.RecordGraded(ctx, *e.TenantID, id, e.OccurredAt)
}
