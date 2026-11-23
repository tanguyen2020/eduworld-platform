// Package minigamesql reads the published mini-game configs (G tables) and their tenant assignment
// (infra.tenant_resource_assignments) on the transaction of ctx (F073). Platform content carries its deleted_at
// condition; the assignment its explicit tenant_id (AGENTS R27).
package minigamesql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/minigame"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// Content implements port.MiniGameContent (read session or write transaction).
type Content struct{}

var _ port.MiniGameContent = Content{}

const cols = `v.minigame_config_id, v.minigame_config_version_id, v.version_no, v.payload`

func one(ctx context.Context, query string, args ...any) (*port.MiniGameRow, error) {
	t, ok := entx.Tx(ctx)
	if !ok {
		return nil, entx.ErrNoTransaction
	}
	var r port.MiniGameRow
	var raw []byte
	err := t.QueryRowContext(ctx, query, args...).Scan(&r.ConfigID, &r.VersionID, &r.VersionNo, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("minigame config: %w", err)
	}
	if r.Config, err = minigame.Parse(raw); err != nil {
		return nil, fmt.Errorf("minigame config %s: %w", r.VersionID, err)
	}
	return &r, nil
}

// CurrentMiniGame implements port.MiniGameContent.
func (Content) CurrentMiniGame(ctx context.Context, configID uuid.UUID) (*port.MiniGameRow, error) {
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on both tables
	return one(ctx, `SELECT `+cols+` FROM game.minigame_configs d
		JOIN game.minigame_config_versions v ON v.minigame_config_version_id = d.current_version_id
		WHERE d.minigame_config_id = $1 AND d.deleted_at IS NULL AND v.deleted_at IS NULL AND v.status = 'published'`, configID)
}

// MiniGameVersion implements port.MiniGameContent: a pinned version stays readable when archived by a newer publish.
func (Content) MiniGameVersion(ctx context.Context, versionID uuid.UUID) (*port.MiniGameRow, error) {
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered
	return one(ctx, `SELECT `+cols+` FROM game.minigame_config_versions v
		WHERE v.minigame_config_version_id = $1 AND v.deleted_at IS NULL AND v.status IN ('published', 'archived')`, versionID)
}

// MiniGameVersionNo implements port.MiniGameContent.
func (Content) MiniGameVersionNo(ctx context.Context, configID uuid.UUID, no int) (*port.MiniGameRow, error) {
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered
	return one(ctx, `SELECT `+cols+` FROM game.minigame_config_versions v
		WHERE v.minigame_config_id = $1 AND v.version_no = $2 AND v.deleted_at IS NULL AND v.status IN ('published', 'archived')`, configID, no)
}

// MiniGameAssigned implements port.MiniGameContent.
func (Content) MiniGameAssigned(ctx context.Context, tenant, configID uuid.UUID) (bool, error) {
	t, ok := entx.Tx(ctx)
	if !ok {
		return false, entx.ErrNoTransaction
	}
	var assigned bool
	// tenancyguard:ok infra.tenant_resource_assignments (no RLS, ADR-0089 §4): explicit tenant_id + deleted_at IS NULL
	err := t.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM infra.tenant_resource_assignments
		WHERE resource_type = 'minigame_config' AND resource_id = $2 AND tenant_id = $1 AND deleted_at IS NULL)`, tenant, configID).Scan(&assigned)
	if err != nil {
		return false, fmt.Errorf("minigame assignment: %w", err)
	}
	return assigned, nil
}
