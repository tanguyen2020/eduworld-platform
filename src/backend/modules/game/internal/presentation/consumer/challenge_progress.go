package consumer

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// ChallengeEvents applies graded learning attempts to the challenge attempts (command.Challenges, F076).
type ChallengeEvents interface {
	ApplyGraded(ctx context.Context, g command.ChallengeGraded) error
}

// ChallengeProgress is the consumer game.challenge_progress (F076-AC-10, AC-11, T025): activity.graded (owner F030,
// payload attempt_id, student_id, mode, correct_count, item_count, passed) of a challenge attempt updates the
// challenge attempt in progress that opened it — once (processed ids; idempotency per event_id is pkg/outbox.Consumer).
// Attempts of other modes are ignored.
type ChallengeProgress struct{ Challenges ChallengeEvents }

// ChallengeProgressOptions returns the outbox consumer options (tenant events of the learning stream).
func ChallengeProgressOptions(h ChallengeProgress, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "game", Name: messaging.ChallengeProgressName, Stream: messaging.LearningStream,
		Subjects: messaging.LearningSubjects(messaging.ChallengeProgressEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c ChallengeProgress) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.TenantID == nil {
		return reject(messaging.ChallengeProgressName, invalid(e.EventType, "tenant event expected"))
	}
	var p struct {
		AttemptID    uuid.UUID `json:"attempt_id"`
		StudentID    uuid.UUID `json:"student_id"`
		Mode         string    `json:"mode"`
		CorrectCount *int      `json:"correct_count"`
		ItemCount    *int      `json:"item_count"`
		Passed       *bool     `json:"passed"`
	}
	if err := decode(e, &p); err != nil {
		return reject(messaging.ChallengeProgressName, err)
	}
	if p.AttemptID == uuid.Nil || p.StudentID == uuid.Nil {
		return reject(messaging.ChallengeProgressName, invalid(e.EventType, "attempt_id and student_id are required"))
	}
	if p.Mode != "challenge" {
		return nil // only challenge attempts belong to a challenge attempt
	}
	return c.Challenges.ApplyGraded(ctx, command.ChallengeGraded{TenantID: *e.TenantID, AttemptID: p.AttemptID, StudentID: p.StudentID,
		CorrectCount: p.CorrectCount, ItemCount: p.ItemCount, Passed: p.Passed, OccurredAt: e.OccurredAt.UTC()})
}
