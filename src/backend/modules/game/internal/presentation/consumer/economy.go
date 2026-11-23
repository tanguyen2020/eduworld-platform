package consumer

import (
	"context"
	"database/sql"
	"encoding/json"
	"slices"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/achievement"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// AchievementEvaluator is the achievement use case (command.Economy).
type AchievementEvaluator interface {
	EvaluateAchievements(ctx context.Context, in command.AchievementEvent) error
}

// AchievementConsumer is the achievement-evaluator of F075 (T035, F075-AC-21): the consumers game.achievement_evaluator
// (game stream), game.achievement_evaluator_learning and game.achievement_evaluator_competition map a verified server event
// to the students it concerns (payload student_id, member_ids or results[].student_id) and evaluate the published
// achievements in the consumer transaction. Never a client claim (R5); a student the projection of the event tenant does
// not hold is retried, then dead-lettered.
type AchievementConsumer struct {
	Achievements AchievementEvaluator
	Name         string
}

// AchievementEvaluatorOptions returns the options of game.achievement_evaluator (tenant events of the game stream).
func AchievementEvaluatorOptions(h AchievementConsumer, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	h.Name = messaging.AchievementEvaluatorName
	return outbox.ConsumerOptions{Module: "game", Name: h.Name, Stream: messaging.GameStream,
		Subjects: messaging.GameSubjects(messaging.AchievementEvaluatorGameEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// AchievementEvaluatorLearningOptions returns the options of game.achievement_evaluator_learning.
func AchievementEvaluatorLearningOptions(h AchievementConsumer, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	h.Name = messaging.AchievementEvaluatorLearningName
	return outbox.ConsumerOptions{Module: "game", Name: h.Name, Stream: messaging.LearningStream,
		Subjects: messaging.LearningSubjects(messaging.AchievementEvaluatorLearningEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// AchievementEvaluatorCompetitionOptions returns the options of game.achievement_evaluator_competition.
func AchievementEvaluatorCompetitionOptions(h AchievementConsumer, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	h.Name = messaging.AchievementEvaluatorCompetitionName
	return outbox.ConsumerOptions{Module: "game", Name: h.Name, Stream: messaging.CompetitionStream,
		Subjects: messaging.CompetitionSubjects(messaging.AchievementEvaluatorCompetitionEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

func (c AchievementConsumer) name() string {
	if c.Name == "" {
		return messaging.AchievementEvaluatorName
	}
	return c.Name
}

// Handle implements outbox.Handler.
func (c AchievementConsumer) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.TenantID == nil {
		return reject(c.name(), invalid(e.EventType, "tenant event expected"))
	}
	if !slices.Contains(achievement.TriggerEvents, e.EventType) {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal(e.Payload, &payload); err != nil || payload == nil {
		return reject(c.name(), invalid(e.EventType, "payload must be a JSON object"))
	}
	students := StudentsOf(payload)
	if len(students) == 0 {
		return reject(c.name(), invalid(e.EventType, "no student in the payload"))
	}
	return c.Achievements.EvaluateAchievements(ctx, command.AchievementEvent{TenantID: *e.TenantID, Students: students,
		Event: achievement.Event{Type: e.EventType, ID: e.EventID, At: e.OccurredAt.UTC(), Payload: payload}})
}

// StudentsOf returns the students an event concerns: student_id, the party member_ids or the student_id of every match
// result (exported for tests).
func StudentsOf(p map[string]any) []uuid.UUID {
	var out []uuid.UUID
	add := func(v any) {
		s, _ := v.(string)
		if id, err := uuid.Parse(s); err == nil && id != uuid.Nil && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	add(p["student_id"])
	if ms, ok := p["member_ids"].([]any); ok {
		for _, m := range ms {
			add(m)
		}
	}
	if rs, ok := p["results"].([]any); ok {
		for _, r := range rs {
			if m, ok := r.(map[string]any); ok {
				add(m["student_id"])
			}
		}
	}
	return out
}

// KnowledgeCards is the knowledge card use case (command.Economy).
type KnowledgeCards interface {
	GrantKnowledgeCard(ctx context.Context, tenant, student, objective uuid.UUID, level string) error
}

// CollectionOnMastery is the consumer game.collection_on_mastery (F075 T030, F075-AC-19): learning_objective.progressed
// (owner F031) at proficient or above grants the knowledge card of the objective once; a lower level is consumed and
// takes nothing back. Never a synchronous call to module learning (R29).
type CollectionOnMastery struct {
	Cards KnowledgeCards
}

// CollectionOnMasteryOptions returns the outbox consumer options (tenant events of the learning stream).
func CollectionOnMasteryOptions(h CollectionOnMastery, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "game", Name: messaging.CollectionOnMasteryName, Stream: messaging.LearningStream,
		Subjects: messaging.LearningSubjects(messaging.CollectionOnMasteryEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c CollectionOnMastery) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.TenantID == nil {
		return reject(messaging.CollectionOnMasteryName, invalid(e.EventType, "tenant event expected"))
	}
	var p struct {
		StudentID   uuid.UUID `json:"student_id"`
		ObjectiveID uuid.UUID `json:"objective_id"`
		Level       string    `json:"level"`
	}
	if err := decode(e, &p); err != nil {
		return reject(messaging.CollectionOnMasteryName, err)
	}
	if p.StudentID == uuid.Nil || p.ObjectiveID == uuid.Nil || !masteryLevels[p.Level] {
		return reject(messaging.CollectionOnMasteryName, invalid(e.EventType, "student_id, objective_id and a mastery level are required"))
	}
	return c.Cards.GrantKnowledgeCard(ctx, *e.TenantID, p.StudentID, p.ObjectiveID, p.Level)
}
