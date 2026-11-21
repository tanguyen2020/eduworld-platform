package consumer

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// WorldProgress applies progress facts to the world of a student (command.World).
type WorldProgress interface {
	ApplyProgress(ctx context.Context, ev command.ProgressEvent) error
	ApplyMastery(ctx context.Context, w command.MasteryWriter, m command.MasteryUpdate) error
}

// UnlockOnEvents is the consumer game.unlock_on_events (F071 T031, T040): quest.completed advances the story chapters
// and every progress event re-evaluates the unlock rules of the student (F071-AC-14, AC-15, AC-21). The facts come from
// the events only (ADR-0016). Payload fields read: student_id plus quest_id (quest.completed, F072), to_level
// (level.reached, F074), achievement_id (achievement.unlocked, F075), event_id or event_run_id
// (event.participation_started, F077). Idempotency per event_id is pkg/outbox.Consumer; replays change nothing.
type UnlockOnEvents struct {
	World WorldProgress
}

// UnlockOnEventsOptions returns the outbox consumer options (tenant events of the game stream).
func UnlockOnEventsOptions(h UnlockOnEvents, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "game", Name: messaging.UnlockOnEventsName, Stream: messaging.GameStream,
		Subjects: messaging.GameSubjects(messaging.UnlockOnEventsGameEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

type progressPayload struct {
	StudentID     uuid.UUID `json:"student_id"`
	QuestID       string    `json:"quest_id"`
	ToLevel       int       `json:"to_level"`
	AchievementID string    `json:"achievement_id"`
	EventID       string    `json:"event_id"`
	EventRunID    string    `json:"event_run_id"`
}

func reject(name string, err error) error {
	observability.ConsumerRejected().WithLabelValues(name, "invalid_payload").Inc()
	return outbox.Permanent(err)
}

// Handle implements outbox.Handler.
func (c UnlockOnEvents) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.TenantID == nil {
		return reject(messaging.UnlockOnEventsName, invalid(e.EventType, "tenant event expected"))
	}
	var p progressPayload
	if err := decode(e, &p); err != nil {
		return reject(messaging.UnlockOnEventsName, err)
	}
	if p.StudentID == uuid.Nil {
		return reject(messaging.UnlockOnEventsName, invalid(e.EventType, "student_id is required"))
	}
	ev := command.ProgressEvent{TenantID: *e.TenantID, StudentID: p.StudentID, At: e.OccurredAt.UTC()}
	switch e.EventType {
	case "quest.completed":
		if _, err := uuid.Parse(p.QuestID); err != nil {
			return reject(messaging.UnlockOnEventsName, invalid(e.EventType, "quest_id is required"))
		}
		ev.Kind, ev.Ref = command.ProgressQuestCompleted, p.QuestID
	case "story.chapter_completed":
		ev.Kind = command.ProgressChapterCompleted
	case "level.reached":
		if p.ToLevel < 1 {
			return reject(messaging.UnlockOnEventsName, invalid(e.EventType, "to_level is required"))
		}
		ev.Kind, ev.Value = command.ProgressLevelReached, p.ToLevel
	case "achievement.unlocked":
		if p.AchievementID == "" || len(p.AchievementID) > 64 {
			return reject(messaging.UnlockOnEventsName, invalid(e.EventType, "achievement_id is required"))
		}
		ev.Kind, ev.Ref = command.ProgressAchievement, p.AchievementID
	case "event.participation_started":
		ref := p.EventID
		if ref == "" {
			ref = p.EventRunID
		}
		if ref == "" || len(ref) > 64 {
			return reject(messaging.UnlockOnEventsName, invalid(e.EventType, "event_id or event_run_id is required"))
		}
		ev.Kind, ev.Ref = command.ProgressEventJoined, ref
	default:
		return reject(messaging.UnlockOnEventsName, invalid(e.EventType, "unexpected event"))
	}
	return c.World.ApplyProgress(ctx, ev)
}

// MasteryProjection is the consumer game.mastery_projection (F071 T011): learning_objective.progressed (owner F031) is
// projected into ref.mastery_summary by source_version (envelope occurred_at) and the objective_progress unlock rules
// of the student are evaluated again. Never a synchronous call to module learning (R29).
type MasteryProjection struct {
	World   WorldProgress
	Mastery command.MasteryWriter
	Now     func() time.Time
}

// MasteryProjectionOptions returns the outbox consumer options (tenant events of the learning stream).
func MasteryProjectionOptions(h MasteryProjection, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "game", Name: messaging.MasteryProjectionName, Stream: messaging.LearningStream,
		Subjects: messaging.LearningSubjects(messaging.MasteryProjectionEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

var masteryLevels = map[string]bool{"not_started": true, "beginning": true, "developing": true, "proficient": true, "mastered": true}

// Handle implements outbox.Handler.
func (c MasteryProjection) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.TenantID == nil {
		return reject(messaging.MasteryProjectionName, invalid(e.EventType, "tenant event expected"))
	}
	var p struct {
		StudentID   uuid.UUID `json:"student_id"`
		ObjectiveID uuid.UUID `json:"objective_id"`
		To          float64   `json:"to"`
		Level       string    `json:"level"`
	}
	if err := decode(e, &p); err != nil {
		return reject(messaging.MasteryProjectionName, err)
	}
	if p.StudentID == uuid.Nil || p.ObjectiveID == uuid.Nil || !masteryLevels[p.Level] || p.To < 0 || p.To > 1 {
		return reject(messaging.MasteryProjectionName, invalid(e.EventType, "student_id, objective_id, level and to are required"))
	}
	err := c.World.ApplyMastery(ctx, c.Mastery, command.MasteryUpdate{TenantID: *e.TenantID, StudentID: p.StudentID, ObjectiveID: p.ObjectiveID,
		Level: p.Level, Value: p.To, SourceVersion: e.OccurredAt.UTC().UnixMicro()})
	if err == nil {
		now := time.Now
		if c.Now != nil {
			now = c.Now
		}
		observability.ProjectionLag().WithLabelValues("mastery_summary").Observe(now().Sub(e.OccurredAt).Seconds())
	}
	if errors.Is(err, errPayload) {
		return reject(messaging.MasteryProjectionName, err)
	}
	return err
}
