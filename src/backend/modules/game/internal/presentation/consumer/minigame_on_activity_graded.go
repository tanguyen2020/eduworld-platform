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

// MiniGameResults records the result of a mini-game play (command.MiniGames).
type MiniGameResults interface {
	RecordResult(ctx context.Context, g command.MiniGameGraded) error
}

// MiniGameOnActivityGraded is the consumer game.minigame_on_activity_graded (F073-AC-18, T031): activity.graded (owner
// F030) of an attempt graded through the plugin minigame (payload block minigame: config, pinned version, game_score,
// completion_met, context) records the play once and publishes minigame.completed. Attempts without the block are
// ignored. Idempotency per event_id is pkg/outbox.Consumer; a replay with a new event id finds the play completed.
type MiniGameOnActivityGraded struct{ MiniGames MiniGameResults }

// MiniGameOnActivityGradedOptions returns the outbox consumer options (tenant events of the learning stream).
func MiniGameOnActivityGradedOptions(h MiniGameOnActivityGraded, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "game", Name: messaging.MiniGameActivityGradedName, Stream: messaging.LearningStream,
		Subjects: messaging.LearningSubjects(messaging.QuestActivityGradedEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c MiniGameOnActivityGraded) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.TenantID == nil {
		return reject(messaging.MiniGameActivityGradedName, invalid(e.EventType, "tenant event expected"))
	}
	var p struct {
		AttemptID    uuid.UUID `json:"attempt_id"`
		StudentID    uuid.UUID `json:"student_id"`
		Score        float64   `json:"score"`
		MaxScore     float64   `json:"max_score"`
		CorrectCount *int      `json:"correct_count"`
		ItemCount    *int      `json:"item_count"`
		MiniGame     *struct {
			ConfigID        uuid.UUID `json:"minigame_config_id"`
			ConfigVersionID uuid.UUID `json:"minigame_config_version_id"`
			GameScore       int       `json:"game_score"`
			CompletionMet   bool      `json:"completion_met"`
			ContextType     string    `json:"context_type"`
			ContextID       uuid.UUID `json:"context_id"`
		} `json:"minigame"`
	}
	if err := decode(e, &p); err != nil {
		return reject(messaging.MiniGameActivityGradedName, err)
	}
	if p.MiniGame == nil {
		return nil // not a mini-game attempt
	}
	m := p.MiniGame
	if p.AttemptID == uuid.Nil || p.StudentID == uuid.Nil || m.ConfigID == uuid.Nil || m.ConfigVersionID == uuid.Nil || len(m.ContextType) > 64 {
		return reject(messaging.MiniGameActivityGradedName, invalid(e.EventType, "attempt_id, student_id and the minigame config are required"))
	}
	return c.MiniGames.RecordResult(ctx, command.MiniGameGraded{TenantID: *e.TenantID, AttemptID: p.AttemptID, StudentID: p.StudentID,
		ConfigID: m.ConfigID, ConfigVersionID: m.ConfigVersionID, ContextType: m.ContextType, ContextID: m.ContextID, GameScore: m.GameScore,
		CompletionMet: m.CompletionMet, Score: p.Score, MaxScore: p.MaxScore, CorrectCount: p.CorrectCount, ItemCount: p.ItemCount,
		OccurredAt: e.OccurredAt.UTC()})
}
