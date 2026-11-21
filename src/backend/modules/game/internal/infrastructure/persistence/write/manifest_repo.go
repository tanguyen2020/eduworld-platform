package write

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
)

// ManifestRepo implements write.ManifestStore with explicit SQL (game.content_manifests has a composite key and no ent
// schema, F014); RLS restricts the rows to the tenant of the transaction.
type ManifestRepo struct{}

var _ write.ManifestStore = ManifestRepo{}

// Latest implements write.ManifestStore.
func (ManifestRepo) Latest(ctx context.Context, tenant uuid.UUID) (int64, json.RawMessage, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, nil, err
	}
	var (
		v       int64
		entries []byte
	)
	// tenancyguard:ok game.content_manifests has no deleted_at (immutable versions); TenantScope RLS + explicit tenant_id
	err = t.QueryRowContext(ctx, `SELECT manifest_version, entries FROM game.content_manifests WHERE tenant_id = $1
		ORDER BY manifest_version DESC LIMIT 1`, tenant).Scan(&v, &entries)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil, nil
	}
	if err != nil {
		return 0, nil, fmt.Errorf("content manifest: %w", err)
	}
	return v, entries, nil
}

// Insert implements write.ManifestStore.
func (ManifestRepo) Insert(ctx context.Context, m write.ManifestRow) error {
	// tenancyguard:ok insert of one tenant manifest row under its TenantScope (RLS tenant_insert); no soft-delete column
	_, err := exec(ctx, `INSERT INTO game.content_manifests (tenant_id, manifest_version, etag, entries, generated_at, cause_release_id)
		VALUES ($1, $2, $3, $4, $5, $6)`, m.TenantID, m.Version, m.ETag, string(m.Entries), m.GeneratedAt, m.CauseReleaseID)
	if err != nil {
		return fmt.Errorf("content manifest: insert: %w", err)
	}
	return nil
}

// ActiveTenants implements write.TenantLister.
func (ManifestRepo) ActiveTenants(ctx context.Context) ([]uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok ref.tenants is the tenant registry (not tenant-scoped, no soft-delete: status mirrors the source)
	rows, err := t.QueryContext(ctx, `SELECT tenant_id FROM ref.tenants WHERE status = 'active' ORDER BY tenant_id`)
	if err != nil {
		return nil, fmt.Errorf("ref tenants: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
