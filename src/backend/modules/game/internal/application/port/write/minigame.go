package write

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ItemMiniGameConfig is the game content item type of a mini-game config (F073, release core F071).
const ItemMiniGameConfig = "minigame_config"

// MiniGamePlay is one row of game.minigame_plays (T, F073 data-model).
type MiniGamePlay struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	StudentID       uuid.UUID
	SchoolID        *uuid.UUID
	ConfigID        uuid.UUID
	ConfigVersionID uuid.UUID
	MiniGameType    string
	AttemptID       uuid.UUID
	ContextType     string
	ContextID       *uuid.UUID
	PlayDate        string // ICT day of the start (YYYY-MM-DD)
	Status          string
	LearningScore   *float64
	MaxScore        *float64
	GameScore       *int
	CorrectCount    *int
	ItemCount       *int
	CompletionMet   *bool
	RewardsEligible *bool
	StartedAt       time.Time
	CompletedAt     *time.Time
}

// MiniGamePlayStore writes the plays of the students (tenant table, RLS; tenant_id explicit).
type MiniGamePlayStore interface {
	// LockStudent serializes the plays of one student (count of the day, one minigame.completed per play).
	LockStudent(ctx context.Context, student uuid.UUID) error
	// InsertPlay inserts a play; false when the attempt already has one (replayed start or event).
	InsertPlay(ctx context.Context, p MiniGamePlay) (bool, error)
	// PlayByAttempt returns the live play of an attempt of tenant, nil when none.
	PlayByAttempt(ctx context.Context, tenant, attempt uuid.UUID) (*MiniGamePlay, error)
	// CompletePlay records the result of a play.
	CompletePlay(ctx context.Context, p MiniGamePlay) error
	// CountCompletedOn counts the completed plays of a student on a config on an ICT day.
	CountCompletedOn(ctx context.Context, tenant, student, config uuid.UUID, day string) (int, error)
}
