package read

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
)

// MinigameConfigModel implements read.MinigameConfigReadModel with explicit SQL on ref.minigame_config_versions
// (platform content: no tenant_id, no deleted_at).
type MinigameConfigModel struct{}

var _ read.MinigameConfigReadModel = MinigameConfigModel{}

// MinigameConfigVersion implements read.MinigameConfigReadModel.
func (MinigameConfigModel) MinigameConfigVersion(ctx context.Context, versionID uuid.UUID) (*dto.MinigameConfigVersion, error) {
	q, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok projection of platform content (no tenant_id, no deleted_at: DATABASE_DESIGN §2.17 rule 2)
	rows, err := q.QueryContext(ctx, `SELECT minigame_config_version_id, minigame_config_id, version, minigame_type,
		array_to_string(grade_levels, ','), snapshot, published_at, source_version
		FROM ref.minigame_config_versions WHERE minigame_config_version_id = $1`, versionID)
	if err != nil {
		return nil, fmt.Errorf("ref.minigame_config_versions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var v dto.MinigameConfigVersion
	var grades string
	var snap []byte
	if err := rows.Scan(&v.ID, &v.ConfigID, &v.Version, &v.Type, &grades, &snap, &v.PublishedAt, &v.SourceVersion); err != nil {
		return nil, err
	}
	for _, g := range strings.Split(grades, ",") {
		if n, err := strconv.Atoi(g); err == nil {
			v.GradeLevels = append(v.GradeLevels, n)
		}
	}
	v.Snapshot, v.PublishedAt = snap, v.PublishedAt.UTC()
	return &v, nil
}
