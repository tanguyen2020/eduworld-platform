package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/challenge"
)

// TrashStore implements read.TrashReader and write.TrashStore on competition.game_configs (G table, platform
// configurations; no tenant_id), under tenancy.Bypass(trash) on the platform pool (competition_platform): soft-deleted
// rows only (module Thùng rác, R21), keyset (deleted_at, config_id) DESC and LIMIT in SQL.
type TrashStore struct{}

var (
	_ read.TrashReader = TrashStore{}
	_ write.TrashStore = TrashStore{}
)

// DeletedTemplates implements read.TrashReader: the title is the title of the latest version, else the code.
func (TrashStore) DeletedTemplates(ctx context.Context, f read.TrashFilter) ([]read.TrashRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var afterAt sql.NullTime
	afterID := uuid.Nil
	if f.After != nil {
		afterAt, afterID = sql.NullTime{Time: f.After.At, Valid: true}, f.After.ID
	}
	// tenancyguard:ok platform configurations (G table, no tenant_id) under tenancy.Bypass(trash); soft-deleted rows only (trash, R21)
	rows, err := t.QueryContext(ctx, `SELECT c.config_type, c.config_id, COALESCE(lv.title, c.code), c.deleted_at, c.deleted_by,
			COALESCE(c.delete_reason, ''),
			EXISTS (SELECT 1 FROM competition.game_configs l WHERE l.config_type = c.config_type AND l.code = c.code
				AND l.config_id <> c.config_id AND l.deleted_at IS NULL)
		FROM competition.game_configs c
		LEFT JOIN LATERAL (SELECT v.title FROM competition.game_config_versions v WHERE v.config_id = c.config_id
			ORDER BY v.version_no DESC LIMIT 1) lv ON true
		WHERE c.deleted_at IS NOT NULL AND c.config_type = ANY($1::text[])
			AND ($2::timestamptz IS NULL OR (c.deleted_at, c.config_id) < ($2::timestamptz, $3::uuid))
		ORDER BY c.deleted_at DESC, c.config_id DESC
		LIMIT $4`, f.ConfigTypes, afterAt, afterID, f.Limit)
	if err != nil {
		return nil, fmt.Errorf("competition trash: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []read.TrashRow{}
	for rows.Next() {
		var r read.TrashRow
		var by uuid.NullUUID
		if err := rows.Scan(&r.ConfigType, &r.ConfigID, &r.Title, &r.DeletedAt, &by, &r.DeleteReason, &r.Conflict); err != nil {
			return nil, fmt.Errorf("competition trash: %w", err)
		}
		r.DeletedAt, r.DeletedBy = r.DeletedAt.UTC(), uuidPtr(by)
		out = append(out, r)
	}
	return out, rows.Err()
}

// RestoreTemplate implements write.TrashStore: the deleted row is locked, a live row with the same code is a conflict (the
// partial unique index game_configs_type_code_uq is the backstop of a concurrent create), then deleted_* are cleared.
func (TrashStore) RestoreTemplate(ctx context.Context, configType string, id, by uuid.UUID, at time.Time) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	var code string
	// tenancyguard:ok platform configurations (G table, no tenant_id) under tenancy.Bypass(trash); the soft-deleted row itself (restore)
	err = t.QueryRowContext(ctx, `SELECT code FROM competition.game_configs WHERE config_id = $1 AND config_type = $2
		AND deleted_at IS NOT NULL FOR UPDATE`, id, configType).Scan(&code)
	if errors.Is(err, sql.ErrNoRows) {
		return challenge.ErrNotInTrash
	}
	if err != nil {
		return fmt.Errorf("competition trash: %w", err)
	}
	var taken bool
	// tenancyguard:ok platform configurations (G table, no tenant_id); live rows holding the unique key
	if err := t.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM competition.game_configs WHERE config_type = $1 AND code = $2
		AND deleted_at IS NULL)`, configType, code).Scan(&taken); err != nil {
		return fmt.Errorf("competition trash: %w", err)
	}
	if taken {
		return challenge.ErrRestoreConflict
	}
	// tenancyguard:ok platform configurations (G table, no tenant_id); restore of the soft-deleted row (R21: no hard delete)
	_, err = t.ExecContext(ctx, `UPDATE competition.game_configs SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL,
		updated_at = $2, updated_by = $3, version = version + 1 WHERE config_id = $1 AND deleted_at IS NOT NULL`, id, at, by)
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "23505" {
		return challenge.ErrRestoreConflict
	}
	if err != nil {
		return fmt.Errorf("competition trash: %w", err)
	}
	return nil
}
