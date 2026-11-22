package consumer

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// QuestRecorder records the learning event of a completed quest (command.QuestLearningEvents).
type QuestRecorder interface {
	Record(ctx context.Context, q command.QuestCompleted) error
}

// LearningEventOnQuestCompleted is the consumer learning.learning_event_on_quest_completed (F072-AC-21, T035):
// quest.completed of module game with objective_ids writes the learning event quest.completed once per
// quest_progress_id (TC-SVC-003). Idempotency per event_id is pkg/outbox.Consumer, per quest_progress_id the learning
// event key.
type LearningEventOnQuestCompleted struct {
	Recorder QuestRecorder
}

// LearningEventOnQuestCompletedOptions returns the outbox consumer options (tenant events of the game stream).
func LearningEventOnQuestCompletedOptions(h LearningEventOnQuestCompleted, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "learning", Name: messaging.QuestLearningEventName, Stream: messaging.GameStream,
		Subjects: messaging.QuestLearningEventSubjects(), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c LearningEventOnQuestCompleted) Handle(ctx context.Context, e outbox.Envelope) error {
	var p struct {
		QuestProgressID uuid.UUID   `json:"quest_progress_id"`
		QuestID         uuid.UUID   `json:"quest_id"`
		QuestVersionID  uuid.UUID   `json:"quest_version_id"`
		QuestType       string      `json:"quest_type"`
		StudentID       uuid.UUID   `json:"student_id"`
		ObjectiveIDs    []uuid.UUID `json:"objective_ids"`
		PartyID         *uuid.UUID  `json:"party_id"`
		StartedAt       *time.Time  `json:"started_at"`
		CompletedAt     time.Time   `json:"completed_at"`
	}
	if err := json.Unmarshal(e.Payload, &p); err != nil || e.TenantID == nil || p.QuestProgressID == uuid.Nil || p.QuestID == uuid.Nil ||
		p.QuestVersionID == uuid.Nil || p.StudentID == uuid.Nil || p.QuestType == "" || p.CompletedAt.IsZero() {
		return outbox.Permanent(invalid(e.EventType, "quest_progress_id, quest_id, quest_version_id, quest_type, student_id and completed_at are required"))
	}
	return c.Recorder.Record(ctx, command.QuestCompleted{TenantID: *e.TenantID, StudentID: p.StudentID, QuestProgressID: p.QuestProgressID,
		QuestID: p.QuestID, QuestVersionID: p.QuestVersionID, QuestType: p.QuestType, ObjectiveIDs: p.ObjectiveIDs, PartyID: p.PartyID,
		StartedAt: p.StartedAt, CompletedAt: p.CompletedAt.UTC()})
}
