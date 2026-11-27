package read

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
)

// Trash implements read.TrashReadModel on game.config_items (G table, platform content; no tenant_id): soft-deleted rows
// only (module Thùng rác, R21), keyset (deleted_at, config_id) DESC and LIMIT in SQL; the title is the title of the latest
// version, else the item key.
type Trash struct{}

var _ read.TrashReadModel = Trash{}

// Deleted implements read.TrashReadModel.
func (Trash) Deleted(ctx context.Context, f read.TrashFilter) ([]read.TrashRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var afterAt sql.NullTime
	if f.AfterAt != nil {
		afterAt = sql.NullTime{Time: *f.AfterAt, Valid: true}
	}
	// tenancyguard:ok platform content (G table, no tenant_id) under tenancy.Bypass(trash); soft-deleted rows only (trash, R21)
	rows, err := t.QueryContext(ctx, `SELECT c.item_type, c.config_id, COALESCE(lv.title, c.item_key), c.deleted_at, c.deleted_by,
			COALESCE(c.delete_reason, ''),
			EXISTS (SELECT 1 FROM game.config_items l WHERE l.item_type = c.item_type AND l.item_key = c.item_key
				AND l.config_id <> c.config_id AND l.deleted_at IS NULL)
		FROM game.config_items c
		LEFT JOIN LATERAL (SELECT v.title FROM game.config_versions v WHERE v.config_id = c.config_id
			ORDER BY v.version_no DESC LIMIT 1) lv ON true
		WHERE c.deleted_at IS NOT NULL AND c.item_type = ANY($1::text[])
			AND ($2::timestamptz IS NULL OR (c.deleted_at, c.config_id) < ($2::timestamptz, $3::uuid))
		ORDER BY c.deleted_at DESC, c.config_id DESC
		LIMIT $4`, f.ItemTypes, afterAt, f.AfterID, f.Limit)
	if err != nil {
		return nil, fmt.Errorf("game trash: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []read.TrashRow{}
	for rows.Next() {
		var r read.TrashRow
		var by uuid.NullUUID
		if err := rows.Scan(&r.ItemType, &r.ConfigID, &r.Title, &r.DeletedAt, &by, &r.DeleteReason, &r.Conflict); err != nil {
			return nil, fmt.Errorf("game trash: %w", err)
		}
		r.DeletedAt = r.DeletedAt.UTC()
		if by.Valid {
			id := by.UUID
			r.DeletedBy = &id
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
