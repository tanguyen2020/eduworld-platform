package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/chatasset"
)

// Trash implements read.TrashReader and write.TrashStore on chat.sticker_packs (G table, platform library; no tenant_id),
// under tenancy.Bypass(trash) on the platform pool (chat_platform writes the library): soft-deleted rows only (module
// Thùng rác, R21), keyset (deleted_at, pack_id) DESC and LIMIT in SQL. The title is the Vietnamese pack name.
type Trash struct{}

var (
	_ read.TrashReader = Trash{}
	_ write.TrashStore = Trash{}
)

// DeletedPacks implements read.TrashReader.
func (Trash) DeletedPacks(ctx context.Context, f read.TrashFilter) ([]read.TrashRow, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	var afterAt sql.NullTime
	if f.AfterAt != nil {
		afterAt = sql.NullTime{Time: *f.AfterAt, Valid: true}
	}
	// tenancyguard:ok platform library (G table, no tenant_id) under tenancy.Bypass(trash); soft-deleted rows only (trash, R21)
	rows, err := t.QueryContext(ctx, `SELECT p.pack_id, p.name_vi, p.deleted_at, p.deleted_by,
			EXISTS (SELECT 1 FROM chat.sticker_packs l WHERE l.code = p.code AND l.pack_id <> p.pack_id AND l.deleted_at IS NULL)
		FROM chat.sticker_packs p
		WHERE p.deleted_at IS NOT NULL
			AND ($1::timestamptz IS NULL OR (p.deleted_at, p.pack_id) < ($1::timestamptz, $2::uuid))
		ORDER BY p.deleted_at DESC, p.pack_id DESC
		LIMIT $3`, afterAt, f.AfterID, f.Limit)
	if err != nil {
		return nil, fmt.Errorf("chat trash: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []read.TrashRow{}
	for rows.Next() {
		var r read.TrashRow
		var by uuid.NullUUID
		if err := rows.Scan(&r.PackID, &r.Title, &r.DeletedAt, &by, &r.Conflict); err != nil {
			return nil, fmt.Errorf("chat trash: %w", err)
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

// RestorePack implements write.TrashStore: the deleted pack is locked, a live pack with the same code is a conflict (the
// partial unique index sticker_packs_code_uq is the backstop of a concurrent create), then deleted_* are cleared.
func (Trash) RestorePack(ctx context.Context, id uuid.UUID, at time.Time) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	var code string
	// tenancyguard:ok platform library (G table, no tenant_id) under tenancy.Bypass(trash); the soft-deleted row itself (restore)
	err = t.QueryRowContext(ctx, `SELECT code FROM chat.sticker_packs WHERE pack_id = $1 AND deleted_at IS NOT NULL FOR UPDATE`, id).Scan(&code)
	if errors.Is(err, sql.ErrNoRows) {
		return chatasset.ErrNotInTrash
	}
	if err != nil {
		return fmt.Errorf("chat trash: %w", err)
	}
	var taken bool
	// tenancyguard:ok platform library (G table, no tenant_id); live packs holding the unique code
	if err := t.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM chat.sticker_packs WHERE code = $1 AND deleted_at IS NULL)`,
		code).Scan(&taken); err != nil {
		return fmt.Errorf("chat trash: %w", err)
	}
	if taken {
		return chatasset.ErrRestoreConflict
	}
	// tenancyguard:ok platform library (G table, no tenant_id); restore of the soft-deleted row (R21: no hard delete)
	_, err = t.ExecContext(ctx, `UPDATE chat.sticker_packs SET deleted_at = NULL, deleted_by = NULL, updated_at = $2
		WHERE pack_id = $1 AND deleted_at IS NOT NULL`, id, at)
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "23505" {
		return chatasset.ErrRestoreConflict
	}
	if err != nil {
		return fmt.Errorf("chat trash: %w", err)
	}
	return nil
}
