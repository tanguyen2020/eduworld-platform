package read

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/catalog"
)

// Profile is the read model of the game profile of a student (game.game_profiles): never a real name, username,
// school or class (R16).
type Profile struct {
	StudentID         uuid.UUID
	Nickname          *string
	Level             int
	XPTotal           int64
	AvatarConfig      json.RawMessage
	AvatarConfigHash  *string
	AvatarCreatedAt   *time.Time
	LastAreaID        *string
	Settings          json.RawMessage
	PreferredLanguage *string
}

// Preset is a saved outfit.
type Preset struct {
	Slot         int
	Name         *string
	AvatarConfig json.RawMessage
	UpdatedAt    time.Time
}

// ProfileReadModel reads profiles and saved outfits of the session tenant.
type ProfileReadModel interface {
	// Profile returns the live profile of student in tenant, nil when none.
	Profile(ctx context.Context, tenant, student uuid.UUID) (*Profile, error)
	// Presets returns the live saved outfits of student ordered by slot.
	Presets(ctx context.Context, tenant, student uuid.UUID) ([]Preset, error)
	// TakenNicknames returns, lower-cased, the names among candidates used by a live profile of tenant.
	TakenNicknames(ctx context.Context, tenant uuid.UUID, candidates []string) (map[string]bool, error)
}

// CatalogReadModel reads the published character catalog (platform content).
type CatalogReadModel interface {
	// Current returns the published catalog, nil when none is published.
	Current(ctx context.Context) (*catalog.Catalog, error)
}
