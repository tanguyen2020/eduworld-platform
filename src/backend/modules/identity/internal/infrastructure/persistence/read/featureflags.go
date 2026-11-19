package read

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/settings"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// FeatureFlagModel implements read.FeatureFlagReader with explicit SQL (global tables, deleted_at filtered).
type FeatureFlagModel struct {
	Session *entx.ReadSession
}

var _ read.FeatureFlagReader = FeatureFlagModel{}

// Catalog implements read.FeatureFlagReader (parent-first order: workspaces, menus, actions).
func (m FeatureFlagModel) Catalog(ctx context.Context) (version string, items []settings.CatalogItem, err error) {
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		tx, ok := entx.Tx(ctx)
		if !ok {
			return entx.ErrNoTransaction
		}
		// tenancyguard:ok platform.portal_menu_catalog is global (G); deleted_at filtered
		rows, err := tx.QueryContext(ctx, `SELECT item_key, item_kind, workspace_key, coalesce(parent_key, ''), coalesce(path, ''), label_key,
				coalesce(title, ''), array_to_json(required_permissions)::text, array_to_json(operation_ids)::text, sort_order, can_disable, catalog_version
			FROM platform.portal_menu_catalog WHERE deleted_at IS NULL
			ORDER BY CASE item_kind WHEN 'workspace' THEN 0 WHEN 'menu' THEN 1 ELSE 2 END, sort_order, item_key`)
		if err != nil {
			return fmt.Errorf("catalog: %w", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var it settings.CatalogItem
			var perms, ops string
			if err := rows.Scan(&it.Key, &it.Kind, &it.Workspace, &it.Parent, &it.Path, &it.LabelKey, &it.Title, &perms, &ops, &it.Order,
				&it.CanDisable, &version); err != nil {
				return fmt.Errorf("catalog: %w", err)
			}
			_ = json.Unmarshal([]byte(perms), &it.RequiredPermissions)
			_ = json.Unmarshal([]byte(ops), &it.OperationIDs)
			items = append(items, it)
		}
		return rows.Err()
	})
	return version, items, err
}

// Flags implements read.FeatureFlagReader.
func (m FeatureFlagModel) Flags(ctx context.Context) (out []settings.Flag, err error) {
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		tx, ok := entx.Tx(ctx)
		if !ok {
			return entx.ErrNoTransaction
		}
		// tenancyguard:ok platform.feature_flags is global (G); deleted_at filtered
		rows, err := tx.QueryContext(ctx, `SELECT `+flagColumns+` FROM platform.feature_flags WHERE deleted_at IS NULL ORDER BY flag_key`)
		if err != nil {
			return fmt.Errorf("flags: %w", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			f, err := ScanFlag(rows)
			if err != nil {
				return err
			}
			out = append(out, f)
		}
		return rows.Err()
	})
	return out, err
}

// flagColumns are the columns read by ScanFlag.
const flagColumns = `flag_key, flag_kind, coalesce(parent_key, ''), coalesce(description, ''), default_enabled, tenant_overrides, version,
	updated_at, updated_by`

// ScanFlag scans one row of flagColumns.
func ScanFlag(rows interface{ Scan(...any) error }) (settings.Flag, error) {
	var f settings.Flag
	var overrides []byte
	var at time.Time
	var by uuid.NullUUID
	if err := rows.Scan(&f.Key, &f.Kind, &f.Parent, &f.Description, &f.Default, &overrides, &f.Version, &at, &by); err != nil {
		return f, fmt.Errorf("flag: %w", err)
	}
	if err := json.Unmarshal(overrides, &f.Overrides); err != nil {
		return f, fmt.Errorf("flag overrides: %w", err)
	}
	f.UpdatedAt = &at
	if by.Valid {
		v := by.UUID
		f.UpdatedBy = &v
	}
	return f, nil
}
