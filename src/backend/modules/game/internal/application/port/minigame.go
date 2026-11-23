package port

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/minigame"
)

// MiniGameRow is one published version of a mini-game config (game.minigame_config_versions).
type MiniGameRow struct {
	ConfigID  uuid.UUID
	VersionID uuid.UUID
	VersionNo int
	Config    minigame.Config
}

// MiniGameContent reads the published mini-game configs (G) and their tenant assignment (F073).
type MiniGameContent interface {
	// CurrentMiniGame returns the current published version of a config, nil when unknown or unpublished.
	CurrentMiniGame(ctx context.Context, configID uuid.UUID) (*MiniGameRow, error)
	// MiniGameVersion returns a version by id (a pinned version stays readable when archived), nil when unknown.
	MiniGameVersion(ctx context.Context, versionID uuid.UUID) (*MiniGameRow, error)
	// MiniGameVersionNo returns the version number no of a config, nil when unknown.
	MiniGameVersionNo(ctx context.Context, configID uuid.UUID, no int) (*MiniGameRow, error)
	// MiniGameAssigned reports a config assigned to tenant (infra.tenant_resource_assignments, ADR-0029).
	MiniGameAssigned(ctx context.Context, tenant, configID uuid.UUID) (bool, error)
}

// MiniGameSources is validateMinigameQuestionSource of the learning facade (x-allowed-callers: game), called only on
// the publish path (F073-AC-02): module learning holds the compatibility table and the Question Bank.
type MiniGameSources interface {
	ValidateSource(ctx context.Context, c minigame.Config) (minigame.SourceCheck, error)
}
