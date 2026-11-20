package consumer

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// MasteryApplier applies a graded attempt to mastery (command.Mastery).
type MasteryApplier interface {
	ApplyGraded(ctx context.Context, in command.GradedAttempt) error
}

// MasteryOnGraded is the consumer learning.mastery_on_graded (F031-AC-08…AC-14, AC-21): activity.graded of the tenant of
// the event updates the mastery of the objectives answered in the attempt; attempt.re_evaluated recomputes them with the
// re-evaluated item results (the original learning events stay). Exactly-once per event_id is pkg/outbox.Consumer
// (processed_events in the same transaction, F031-AC-02).
type MasteryOnGraded struct {
	Apply MasteryApplier
}

// MasteryOptions returns the outbox consumer options (tenant events only).
func MasteryOptions(h MasteryOnGraded, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "learning", Name: messaging.MasteryName, Stream: messaging.AttemptStream,
		Subjects: messaging.MasterySubjects(), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// attemptOf reads attempt_id of an attempt event.
func attemptOf(e outbox.Envelope) (uuid.UUID, error) {
	var p struct {
		AttemptID uuid.UUID `json:"attempt_id"`
	}
	if err := json.Unmarshal(e.Payload, &p); err != nil || p.AttemptID == uuid.Nil || e.TenantID == nil {
		return uuid.Nil, invalid(e.EventType, "payload has no attempt_id or no tenant")
	}
	return p.AttemptID, nil
}

// Handle implements outbox.Handler.
func (c MasteryOnGraded) Handle(ctx context.Context, e outbox.Envelope) error {
	id, err := attemptOf(e)
	if err != nil {
		return err
	}
	kind := command.MasterySourceEvidence
	if e.EventType == command.EventAttemptReEvaluated {
		kind = command.MasterySourceReEvaluation
	}
	return c.Apply.ApplyGraded(ctx, command.GradedAttempt{EventID: e.EventID, TenantID: *e.TenantID, AttemptID: id, OccurredAt: e.OccurredAt,
		Kind: kind})
}
