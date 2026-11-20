// Package catalogsql reads the published character catalog (platform content, G tables) on a transaction; shared by
// the write repositories and the read models of module game.
package catalogsql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/catalog"
)

// Querier is the part of *sql.Tx the reader needs.
type Querier interface {
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
}

// Current returns the published version of the catalog code, nil when none is published.
func Current(ctx context.Context, t Querier, code string) (*catalog.Catalog, error) {
	var c catalog.Catalog
	var raw []byte
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on both tables
	err := t.QueryRowContext(ctx, `SELECT v.catalog_version_id, v.version_no, v.payload FROM game.character_catalogs c
		JOIN game.character_catalog_versions v ON v.catalog_version_id = c.current_version_id
		WHERE c.code = $1 AND c.deleted_at IS NULL AND v.deleted_at IS NULL AND v.status = 'published'`, code).Scan(&c.VersionID, &c.Version, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("character catalog: %w", err)
	}
	if err := json.Unmarshal(raw, &c.Payload); err != nil {
		return nil, fmt.Errorf("character catalog: payload: %w", err)
	}
	return &c, nil
}
