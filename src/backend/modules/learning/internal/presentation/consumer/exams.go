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

// ExamGrader grades a submitted exam attempt (command.Exams).
type ExamGrader interface {
	GradeExamAttempt(ctx context.Context, attemptID uuid.UUID) error
}

// ExamGrade is the consumer learning.exam_grade (worker exam.grade, F042-AC-19): exam.submitted → the attempt is graded
// in the transaction and tenant scope of the event; a redelivery of a graded attempt does nothing.
type ExamGrade struct {
	Grader ExamGrader
}

// ExamGradeOptions are the outbox consumer options of learning.exam_grade.
func ExamGradeOptions(h ExamGrade, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "learning", Name: messaging.ExamGradeName, Stream: messaging.AttemptStream,
		Subjects: messaging.ExamGradeSubjects(), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c ExamGrade) Handle(ctx context.Context, e outbox.Envelope) error {
	var p struct {
		AttemptID uuid.UUID `json:"attempt_id"`
	}
	if err := json.Unmarshal(e.Payload, &p); err != nil || p.AttemptID == uuid.Nil || e.TenantID == nil {
		return invalid(e.EventType, "payload has no attempt")
	}
	return c.Grader.GradeExamAttempt(ctx, p.AttemptID)
}
