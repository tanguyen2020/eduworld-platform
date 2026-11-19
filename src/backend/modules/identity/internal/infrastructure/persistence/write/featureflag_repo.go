package write

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/settings"
)

// FeatureFlagRepo implements write.FeatureFlagRepository with explicit SQL on the global tables
// platform.feature_flags and platform.portal_menu_catalog (platform scope, identity_platform).
type FeatureFlagRepo struct{}

var _ write.FeatureFlagRepository = FeatureFlagRepo{}

// Flag implements write.FeatureFlagRepository.
func (FeatureFlagRepo) Flag(ctx context.Context, key string) (*settings.Flag, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	var (
		f         settings.Flag
		overrides []byte
		at        time.Time
		by        uuid.NullUUID
	)
	// tenancyguard:ok platform.feature_flags is global (G); deleted_at filtered
	err = tx.QueryRowContext(ctx, `SELECT flag_key, flag_kind, coalesce(parent_key, ''), coalesce(description, ''), default_enabled,
			tenant_overrides, version, updated_at, updated_by
		FROM platform.feature_flags WHERE flag_key = $1 AND deleted_at IS NULL FOR UPDATE`, key).
		Scan(&f.Key, &f.Kind, &f.Parent, &f.Description, &f.Default, &overrides, &f.Version, &at, &by)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("feature flag: %w", err)
	}
	if err := json.Unmarshal(overrides, &f.Overrides); err != nil {
		return nil, err
	}
	f.UpdatedAt = &at
	if by.Valid {
		v := by.UUID
		f.UpdatedBy = &v
	}
	return &f, nil
}

// CatalogItem implements write.FeatureFlagRepository.
func (FeatureFlagRepo) CatalogItem(ctx context.Context, key string) (*settings.CatalogItem, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	var it settings.CatalogItem
	// tenancyguard:ok platform.portal_menu_catalog is global (G); deleted_at filtered
	err = tx.QueryRowContext(ctx, `SELECT item_key, item_kind, workspace_key, coalesce(parent_key, ''), label_key, coalesce(title, ''), can_disable
		FROM platform.portal_menu_catalog WHERE item_key = $1 AND deleted_at IS NULL`, key).
		Scan(&it.Key, &it.Kind, &it.Workspace, &it.Parent, &it.LabelKey, &it.Title, &it.CanDisable)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("catalog item: %w", err)
	}
	return &it, nil
}

// Save implements write.FeatureFlagRepository.
func (FeatureFlagRepo) Save(ctx context.Context, f settings.Flag, expected int64, by uuid.UUID) (int64, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return 0, err
	}
	overrides := f.Overrides
	if overrides == nil {
		overrides = map[string]bool{}
	}
	raw, err := json.Marshal(overrides)
	if err != nil {
		return 0, err
	}
	var parent, desc any
	if f.Parent != "" {
		parent = f.Parent
	}
	if f.Description != "" {
		desc = f.Description
	}
	var v int64
	if expected == 0 {
		// tenancyguard:ok platform.feature_flags is global (G); a soft-deleted row of the key is revived
		err = tx.QueryRowContext(ctx, `INSERT INTO platform.feature_flags (flag_key, flag_kind, parent_key, description, default_enabled,
				tenant_overrides, updated_by)
			VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7)
			ON CONFLICT (flag_key) DO UPDATE SET flag_kind = EXCLUDED.flag_kind, parent_key = EXCLUDED.parent_key,
				description = EXCLUDED.description, default_enabled = EXCLUDED.default_enabled, tenant_overrides = EXCLUDED.tenant_overrides,
				version = platform.feature_flags.version + 1, updated_at = now(), updated_by = EXCLUDED.updated_by,
				deleted_at = NULL, deleted_by = NULL, delete_reason = NULL
			WHERE platform.feature_flags.deleted_at IS NOT NULL
			RETURNING version`, f.Key, f.Kind, parent, desc, f.Default, string(raw), by).Scan(&v)
	} else {
		// tenancyguard:ok platform.feature_flags is global (G); one live row by key and version
		err = tx.QueryRowContext(ctx, `UPDATE platform.feature_flags SET description = $2, default_enabled = $3, tenant_overrides = $4::jsonb,
				version = version + 1, updated_at = now(), updated_by = $6
			WHERE flag_key = $1 AND version = $5 AND deleted_at IS NULL RETURNING version`,
			f.Key, desc, f.Default, string(raw), expected, by).Scan(&v)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return 0, stale()
	}
	if err != nil {
		return 0, fmt.Errorf("save feature flag: %w", err)
	}
	return v, nil
}
