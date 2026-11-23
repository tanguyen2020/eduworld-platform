package write

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// MinigameConfigRepo implements write.MinigameConfigs: explicit SQL on the projection ref.minigame_config_versions in the
// transaction of ctx (platform content: no tenant_id, no deleted_at; every role of the module may read it).
type MinigameConfigRepo struct{}

var _ write.MinigameConfigs = MinigameConfigRepo{}

const minigameColumns = `minigame_config_version_id, minigame_config_id, version, minigame_type, array_to_string(grade_levels, ','),
	snapshot, published_at, source_version`

func minigameRow(ctx context.Context, query string, args ...any) (*dto.MinigameConfigVersion, error) {
	tx, ok := entx.Tx(ctx)
	if !ok {
		return nil, entx.ErrNoTransaction
	}
	var v dto.MinigameConfigVersion
	var grades string
	var snap []byte
	err := tx.QueryRowContext(ctx, query, args...).Scan(&v.ID, &v.ConfigID, &v.Version, &v.Type, &grades, &snap, &v.PublishedAt, &v.SourceVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ref.minigame_config_versions: %w", err)
	}
	for _, g := range strings.Split(grades, ",") {
		if n, err := strconv.Atoi(g); err == nil {
			v.GradeLevels = append(v.GradeLevels, n)
		}
	}
	v.Snapshot, v.PublishedAt = snap, v.PublishedAt.UTC()
	return &v, nil
}

// MinigameConfigVersion implements write.MinigameConfigs.
func (MinigameConfigRepo) MinigameConfigVersion(ctx context.Context, versionID uuid.UUID) (*dto.MinigameConfigVersion, error) {
	// tenancyguard:ok projection of platform content (no tenant_id, no deleted_at: DATABASE_DESIGN §2.17 rule 2)
	return minigameRow(ctx, `SELECT `+minigameColumns+` FROM ref.minigame_config_versions WHERE minigame_config_version_id = $1`, versionID)
}

// MinigameConfigVersionNo implements write.MinigameConfigs.
func (MinigameConfigRepo) MinigameConfigVersionNo(ctx context.Context, configID uuid.UUID, no int) (*dto.MinigameConfigVersion, error) {
	// tenancyguard:ok projection of platform content (no tenant_id, no deleted_at: DATABASE_DESIGN §2.17 rule 2)
	return minigameRow(ctx, `SELECT `+minigameColumns+` FROM ref.minigame_config_versions WHERE minigame_config_id = $1 AND version = $2`,
		configID, no)
}

// CurrentMinigameConfigVersion implements write.MinigameConfigs: the version published last.
func (MinigameConfigRepo) CurrentMinigameConfigVersion(ctx context.Context, configID uuid.UUID) (*dto.MinigameConfigVersion, error) {
	// tenancyguard:ok projection of platform content (no tenant_id, no deleted_at: DATABASE_DESIGN §2.17 rule 2)
	return minigameRow(ctx, `SELECT `+minigameColumns+` FROM ref.minigame_config_versions WHERE minigame_config_id = $1
		ORDER BY published_at DESC, version DESC LIMIT 1`, configID)
}
