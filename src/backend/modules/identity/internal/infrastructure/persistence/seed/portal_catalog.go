package seed

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/settings"
)

// CatalogStats summarizes one portal catalog sync.
type CatalogStats struct {
	Items, Retired, FlagsRetired int
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// SyncPortalCatalog writes the menu catalog (specs/portal/menu-catalog.yaml) into platform.portal_menu_catalog
// (F015 T040, F015-AC-23, ADR-0090) as identity_migrator in one transaction: upsert by item_key with the
// catalog_version, soft-delete of the keys removed from the file and of the feature flags naming them
// (audited by the delete_reason; the rows stay for history). Idempotent.
func SyncPortalCatalog(ctx context.Context, db *sql.DB, version string, items []settings.CatalogItem) (st CatalogStats, err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return st, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	keys := make([]string, 0, len(items))
	for _, it := range items {
		keys = append(keys, it.Key)
		perms, ops := it.RequiredPermissions, it.OperationIDs
		if perms == nil {
			perms = []string{}
		}
		if ops == nil {
			ops = []string{}
		}
		// tenancyguard:ok platform.portal_menu_catalog is global (G), written by identity_migrator only
		if _, err = tx.ExecContext(ctx, `INSERT INTO platform.portal_menu_catalog (item_key, item_kind, workspace_key, parent_key, path, label_key,
				title, required_permissions, operation_ids, sort_order, can_disable, catalog_version, synced_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, ARRAY(SELECT jsonb_array_elements_text($8::jsonb))::varchar(64)[],
				ARRAY(SELECT jsonb_array_elements_text($9::jsonb))::varchar(128)[], $10, $11, $12, now())
			ON CONFLICT (item_key) DO UPDATE SET item_kind = EXCLUDED.item_kind, workspace_key = EXCLUDED.workspace_key,
				parent_key = EXCLUDED.parent_key, path = EXCLUDED.path, label_key = EXCLUDED.label_key, title = EXCLUDED.title,
				required_permissions = EXCLUDED.required_permissions, operation_ids = EXCLUDED.operation_ids,
				sort_order = EXCLUDED.sort_order, can_disable = EXCLUDED.can_disable, catalog_version = EXCLUDED.catalog_version,
				synced_at = now(), deleted_at = NULL, deleted_by = NULL, delete_reason = NULL`,
			it.Key, it.Kind, it.Workspace, nullable(it.Parent), nullable(it.Path), it.LabelKey, nullable(it.Title), jsonText(perms), jsonText(ops), it.Order,
			it.CanDisable, version); err != nil {
			return st, fmt.Errorf("portal catalog %s: %w", it.Key, err)
		}
		st.Items++
	}
	// tenancyguard:ok platform.feature_flags is global (G): flags of keys removed from the catalog
	res, err := tx.ExecContext(ctx, `UPDATE platform.feature_flags SET deleted_at = now(), delete_reason = 'catalog_item_retired'
		WHERE deleted_at IS NULL AND flag_kind <> 'feature' AND NOT (flag_key = ANY(ARRAY(SELECT jsonb_array_elements_text($1::jsonb))))`, jsonText(keys))
	if err != nil {
		return st, fmt.Errorf("retire flags: %w", err)
	}
	n, _ := res.RowsAffected()
	st.FlagsRetired = int(n)
	// tenancyguard:ok platform.portal_menu_catalog is global (G): items removed from the file are soft-deleted
	res, err = tx.ExecContext(ctx, `UPDATE platform.portal_menu_catalog SET deleted_at = now(), delete_reason = 'removed_from_catalog'
		WHERE deleted_at IS NULL AND NOT (item_key = ANY(ARRAY(SELECT jsonb_array_elements_text($1::jsonb))))`, jsonText(keys))
	if err != nil {
		return st, fmt.Errorf("retire catalog items: %w", err)
	}
	n, _ = res.RowsAffected()
	st.Retired = int(n)
	return st, tx.Commit()
}

func jsonText(xs []string) string {
	b, _ := json.Marshal(xs)
	return string(b)
}
