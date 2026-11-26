package port

import (
	"time"

	"github.com/google/uuid"
)

// Rows of the competition configurations shared by the write and read contexts (F108).

// ConfigItem is one competition.game_configs row (G, platform data).
type ConfigItem struct {
	ConfigID uuid.UUID
	Type     string
	Code     string
	Current  *uuid.UUID
	LatestNo int
}

// ConfigVersion is one competition.game_config_versions row; Revision is its optimistic version (ETag).
type ConfigVersion struct {
	ID            uuid.UUID
	ConfigID      uuid.UUID
	No            int
	Status        string
	Title         string
	Payload       []byte
	EffectiveFrom *time.Time
	AuthorID      *uuid.UUID
	ReviewerID    *uuid.UUID
	ReviewNote    *string
	ChangeNote    *string
	BasedOn       *uuid.UUID
	PublishedAt   *time.Time
	PublishedBy   *uuid.UUID
	UpdatedAt     time.Time
	Revision      int64
}

// ConfigRelease is one competition.content_releases row (append-only, ADR-0043).
type ConfigRelease struct {
	ID            uuid.UUID
	ConfigID      uuid.UUID
	Action        string
	From, To      *uuid.UUID
	Reason        *string
	By            uuid.UUID
	At            time.Time
	EffectiveFrom time.Time
}
