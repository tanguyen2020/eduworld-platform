package consumer

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// LessonGraded completes lesson sessions on graded attempts (command.Lessons).
type LessonGraded interface {
	OnGraded(ctx context.Context, attemptID uuid.UUID) error
}

// LessonCompletion is the consumer learning.lesson_completion (F031-AC-26, AC-27): activity.graded of an attempt started in
// the context of a lesson; exactly once per event (processed_events), lesson.completed once per student × version.
type LessonCompletion struct {
	Lessons LessonGraded
}

// LessonCompletionOptions returns the outbox consumer options (tenant events only).
func LessonCompletionOptions(h LessonCompletion, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "learning", Name: messaging.LessonCompletionName, Stream: messaging.AttemptStream,
		Subjects: messaging.LessonCompletionSubjects(), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c LessonCompletion) Handle(ctx context.Context, e outbox.Envelope) error {
	id, err := attemptOf(e)
	if err != nil {
		return err
	}
	return c.Lessons.OnGraded(ctx, id)
}
