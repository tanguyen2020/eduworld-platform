package write

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
)

// MinigameConfigProjector inserts the immutable versions of ref.minigame_config_versions. Only the command
// UpsertMiniGameConfigSnapshot, called by the consumer learning.minigame_config_projection, uses it (F073-AC-04): an
// existing version is never changed (applied = false).
type MinigameConfigProjector interface {
	InsertMinigameConfigVersion(ctx context.Context, v dto.MinigameConfigVersion) (bool, error)
	// SetMinigamePointer moves the release pointer of a config (nil = withdrawn) unless a newer event already did (F078).
	SetMinigamePointer(ctx context.Context, configID uuid.UUID, versionID *uuid.UUID, sourceVersion int64, at time.Time) (bool, error)
}

// MinigameConfigs reads the projection ref.minigame_config_versions in the transaction of ctx (attempt engine, activity
// checks); nil when the version is not projected (yet).
type MinigameConfigs interface {
	MinigameConfigVersion(ctx context.Context, versionID uuid.UUID) (*dto.MinigameConfigVersion, error)
	// MinigameConfigVersionNo is the version number no of a configuration.
	MinigameConfigVersionNo(ctx context.Context, configID uuid.UUID, no int) (*dto.MinigameConfigVersion, error)
	// CurrentMinigameConfigVersion is the version of a configuration published last.
	CurrentMinigameConfigVersion(ctx context.Context, configID uuid.UUID) (*dto.MinigameConfigVersion, error)
}
