package read

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
)

// ConfigRow is the newest version of one configuration with its pointer (listGameConfigs).
type ConfigRow struct {
	Item    port.ConfigItem
	Version port.ConfigVersion
}

// GameConfigReader reads the competition configurations under entx.ReadSession with the platform scope (F108): keyset
// pages paged in SQL (LIMIT n+1), live rows only (deleted_at IS NULL).
type GameConfigReader interface {
	// Configs lists the configurations of a type by config_id after `after` (nil = first page), at most limit+1 rows;
	// status filters on the newest version.
	Configs(ctx context.Context, configType, status string, after *uuid.UUID, limit int) ([]ConfigRow, error)
	Item(ctx context.Context, configType string, id uuid.UUID) (*port.ConfigItem, error)
	Version(ctx context.Context, configID uuid.UUID, no int) (*port.ConfigVersion, error)
	// Versions lists the versions newest first with version_no < before (0 = first page), at most limit+1 rows.
	Versions(ctx context.Context, configID uuid.UUID, before, limit int) ([]port.ConfigVersion, error)
	// VersionNos maps version ids of the configuration to their numbers (release rows).
	VersionNos(ctx context.Context, configID uuid.UUID) (map[uuid.UUID]int, error)
	// Releases lists the releases newest first after the release `after` (nil = first page), at most limit+1 rows.
	Releases(ctx context.Context, configID uuid.UUID, after *uuid.UUID, limit int) ([]port.ConfigRelease, error)
}
