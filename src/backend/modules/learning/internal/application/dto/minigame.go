package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// MinigameConfigVersion is a row of the projection ref.minigame_config_versions (F073-AC-04): one published
// MiniGameConfigVersion of module game, immutable, with its snapshot (minigame-config.schema.json; no question, no answer).
type MinigameConfigVersion struct {
	ID            uuid.UUID
	ConfigID      uuid.UUID
	Version       int
	Type          string
	GradeLevels   []int
	Snapshot      json.RawMessage
	PublishedAt   time.Time
	SourceVersion int64
}

// MinigameSourceReason is one reason of MinigameQuestionSourceResult.
type MinigameSourceReason struct {
	Code    string
	Message string
}

// MinigameSourceResult is MinigameQuestionSourceResult of validateMinigameQuestionSource (F073-AC-02): never a question,
// an answer or a scoring rule.
type MinigameSourceResult struct {
	Compatible     bool
	QuestionTypes  []string
	AvailableCount int
	Reasons        []MinigameSourceReason
}
