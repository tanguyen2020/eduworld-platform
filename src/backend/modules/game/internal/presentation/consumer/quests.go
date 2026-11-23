package consumer

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// QuestEvents applies server events to the quest progress of a student (command.Quests, F072).
type QuestEvents interface {
	ApplyActivityResult(ctx context.Context, g command.GradedAttempt) error
	ApplyDiscovery(ctx context.Context, d command.Discovery) error
	GrantPeriodicQuests(ctx context.Context, tenant, student uuid.UUID, at time.Time) error
}

// QuestOnActivityGraded is the consumer game.quest_on_activity_graded (F072-AC-15, T031): activity.graded (owner F030,
// payload attempt_id, student_id, correct_count, item_count, passed) of an attempt linked to a quest step evaluates its
// pass rule once. Attempts of other contexts are ignored. Idempotency per event_id is pkg/outbox.Consumer; a replay
// with a new event id finds the step attempt already graded.
type QuestOnActivityGraded struct{ Quests QuestEvents }

// QuestOnActivityGradedOptions returns the outbox consumer options (tenant events of the learning stream).
func QuestOnActivityGradedOptions(h QuestOnActivityGraded, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "game", Name: messaging.QuestActivityGradedName, Stream: messaging.LearningStream,
		Subjects: messaging.LearningSubjects(messaging.QuestActivityGradedEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c QuestOnActivityGraded) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.TenantID == nil {
		return reject(messaging.QuestActivityGradedName, invalid(e.EventType, "tenant event expected"))
	}
	var p struct {
		AttemptID    uuid.UUID `json:"attempt_id"`
		StudentID    uuid.UUID `json:"student_id"`
		Mode         string    `json:"mode"`
		CorrectCount *int      `json:"correct_count"`
		ItemCount    *int      `json:"item_count"`
		Passed       *bool     `json:"passed"`
		MiniGame     *struct {
			CompletionMet bool `json:"completion_met"`
		} `json:"minigame"`
	}
	if err := decode(e, &p); err != nil {
		return reject(messaging.QuestActivityGradedName, err)
	}
	if p.AttemptID == uuid.Nil || p.StudentID == uuid.Nil {
		return reject(messaging.QuestActivityGradedName, invalid(e.EventType, "attempt_id and student_id are required"))
	}
	if p.Mode != "" && p.Mode != "quest" {
		return nil // only quest attempts can be linked to a quest step
	}
	g := command.GradedAttempt{TenantID: *e.TenantID, AttemptID: p.AttemptID, StudentID: p.StudentID, CorrectCount: p.CorrectCount,
		ItemCount: p.ItemCount, Passed: p.Passed, OccurredAt: e.OccurredAt.UTC()}
	if p.MiniGame != nil { // F073: a minigame step completes on completion_met
		met := p.MiniGame.CompletionMet
		g.CompletionMet = &met
	}
	return c.Quests.ApplyActivityResult(ctx, g)
}

// QuestOnDiscoveryFound is the consumer game.quest_on_discovery_found (F072-AC-12, T024): world.discovery_found (owner
// F071, payload student_id, area_id, interactable_id) advances the active discover steps that list the interactable.
type QuestOnDiscoveryFound struct{ Quests QuestEvents }

// QuestOnDiscoveryFoundOptions returns the outbox consumer options (tenant events of the game stream).
func QuestOnDiscoveryFoundOptions(h QuestOnDiscoveryFound, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "game", Name: messaging.QuestDiscoveryName, Stream: messaging.GameStream,
		Subjects: messaging.GameSubjects(messaging.QuestDiscoveryEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c QuestOnDiscoveryFound) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.TenantID == nil {
		return reject(messaging.QuestDiscoveryName, invalid(e.EventType, "tenant event expected"))
	}
	var p struct {
		StudentID      uuid.UUID `json:"student_id"`
		AreaID         string    `json:"area_id"`
		InteractableID string    `json:"interactable_id"`
	}
	if err := decode(e, &p); err != nil {
		return reject(messaging.QuestDiscoveryName, err)
	}
	if p.StudentID == uuid.Nil || p.AreaID == "" || p.InteractableID == "" || len(p.AreaID) > 64 || len(p.InteractableID) > 64 {
		return reject(messaging.QuestDiscoveryName, invalid(e.EventType, "student_id, area_id and interactable_id are required"))
	}
	return c.Quests.ApplyDiscovery(ctx, command.Discovery{TenantID: *e.TenantID, StudentID: p.StudentID, AreaID: p.AreaID,
		InteractableID: p.InteractableID})
}

// PeriodicQuestsOnSessionStarted is the consumer game.periodic_quests_on_session_started (F072-AC-22, T038):
// game_session.started (owner F014, payload student_id, started_at) grants the daily and weekly quests of the period
// once.
type PeriodicQuestsOnSessionStarted struct{ Quests QuestEvents }

// PeriodicQuestsOptions returns the outbox consumer options (tenant events of the game stream).
func PeriodicQuestsOptions(h PeriodicQuestsOnSessionStarted, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "game", Name: messaging.PeriodicQuestsName, Stream: messaging.GameStream,
		Subjects: messaging.GameSubjects(messaging.PeriodicQuestsEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c PeriodicQuestsOnSessionStarted) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.TenantID == nil {
		return reject(messaging.PeriodicQuestsName, invalid(e.EventType, "tenant event expected"))
	}
	var p struct {
		StudentID uuid.UUID  `json:"student_id"`
		StartedAt *time.Time `json:"started_at"`
	}
	if err := decode(e, &p); err != nil {
		return reject(messaging.PeriodicQuestsName, err)
	}
	if p.StudentID == uuid.Nil {
		return reject(messaging.PeriodicQuestsName, invalid(e.EventType, "student_id is required"))
	}
	at := e.OccurredAt.UTC()
	if p.StartedAt != nil {
		at = p.StartedAt.UTC()
	}
	return c.Quests.GrantPeriodicQuests(ctx, *e.TenantID, p.StudentID, at)
}
