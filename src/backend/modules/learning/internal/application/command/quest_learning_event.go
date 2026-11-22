package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// EventQuestCompleted is the learning event of a completed quest (EVENT_DESIGN §5.2).
const EventQuestCompleted = "quest.completed"

// QuestCompleted is the domain event quest.completed of module game (specs/events/payloads/game/quest.completed.v1).
type QuestCompleted struct {
	TenantID        uuid.UUID
	StudentID       uuid.UUID
	QuestProgressID uuid.UUID
	QuestID         uuid.UUID
	QuestVersionID  uuid.UUID
	QuestType       string
	ObjectiveIDs    []uuid.UUID
	PartyID         *uuid.UUID
	StartedAt       *time.Time
	CompletedAt     time.Time
}

// questKeys namespaces the idempotency key of the learning event quest.completed (UUIDv5 of the quest progress).
var questKeys = uuid.MustParse("0193a1f0-31f0-7031-8031-000000000072")

// QuestLearningEvents records the learning event quest.completed (F072-AC-21): once per quest_progress_id (the
// idempotency key of learning.learning_events), only for a quest that carries learning objectives, with the mode of the
// context (quest). The quest engine never sends answers or scores: the event is evidence of the quest, the mastery comes
// from the graded attempts of its learning steps. A student not projected in the tenant has no learning data (skipped).
type QuestLearningEvents struct {
	Students port.StudentDirectory
	Learning write.LearningEventWriter
}

// Record runs in the consumer transaction (TenantScope of the event).
func (h QuestLearningEvents) Record(ctx context.Context, q QuestCompleted) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.RecordQuestCompleted")
	defer func() { done(err) }()
	if len(q.ObjectiveIDs) == 0 {
		return nil
	}
	school, grade, err := h.Students.StudentSchool(ctx, q.StudentID)
	if err != nil || school == nil {
		return err
	}
	payload := map[string]any{"quest_version_id": q.QuestVersionID, "quest_type": q.QuestType, "quest_progress_id": q.QuestProgressID,
		"quest_id": q.QuestID}
	if q.PartyID != nil {
		payload["party_id"] = *q.PartyID
	}
	if q.StartedAt != nil && q.CompletedAt.After(*q.StartedAt) {
		payload["duration_ms"] = q.CompletedAt.Sub(*q.StartedAt).Milliseconds()
	}
	return h.Learning.Write(ctx, write.LearningEvent{ID: ids.New(), OccurredAt: q.CompletedAt, Type: EventQuestCompleted, StudentID: q.StudentID,
		TenantID: q.TenantID, SchoolID: *school, GradeLevel: grade, ObjectiveIDs: q.ObjectiveIDs, Context: "quest", ActorType: "student",
		IdempotencyKey: "qst:" + uuid.NewSHA1(questKeys, []byte(q.QuestProgressID.String())).String(), Payload: payload})
}
