package consumer

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// ExamQualityRunner computes the quality report of a session (command.Quality).
type ExamQualityRunner interface {
	ComputeExamQuality(ctx context.Context, sessionID uuid.UUID) error
}

// ExamQuality is the consumer learning.exam_quality (F044-AC-23): exam.closed → the reliability report of the session in
// the transaction and tenant scope of the event; a redelivery recomputes the same report (upsert).
type ExamQuality struct {
	Runner ExamQualityRunner
}

// ExamQualityOptions are the outbox consumer options of learning.exam_quality.
func ExamQualityOptions(h ExamQuality, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "learning", Name: messaging.ExamQualityName, Stream: messaging.AttemptStream,
		Subjects: messaging.ExamQualitySubjects(), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c ExamQuality) Handle(ctx context.Context, e outbox.Envelope) error {
	var p struct {
		SessionID uuid.UUID `json:"exam_session_id"`
	}
	if err := json.Unmarshal(e.Payload, &p); err != nil || p.SessionID == uuid.Nil || e.TenantID == nil {
		return invalid(e.EventType, "payload has no exam session")
	}
	return c.Runner.ComputeExamQuality(ctx, p.SessionID)
}
