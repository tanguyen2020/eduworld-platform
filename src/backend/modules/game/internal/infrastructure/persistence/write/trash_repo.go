package write

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gameconfig"
)

// TrashRepo implements write.TrashStore on game.config_items (G table, written by game_platform under tenancy.Bypass(trash)).
type TrashRepo struct{}

var _ write.TrashStore = TrashRepo{}

// RestoreConfig implements write.TrashStore: the deleted row is locked, a live row with the same key is a conflict (the
// partial unique index config_items_key_uq is the backstop of a concurrent create), then deleted_* are cleared.
func (TrashRepo) RestoreConfig(ctx context.Context, itemType string, id, by uuid.UUID, at time.Time) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	var key string
	// tenancyguard:ok platform content (G table, no tenant_id) under tenancy.Bypass(trash); the soft-deleted row itself (restore)
	err = t.QueryRowContext(ctx, `SELECT item_key FROM game.config_items WHERE config_id = $1 AND item_type = $2 AND deleted_at IS NOT NULL
		FOR UPDATE`, id, itemType).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		return gameconfig.ErrNotInTrash
	}
	if err != nil {
		return fmt.Errorf("game trash: %w", err)
	}
	var taken bool
	// tenancyguard:ok platform content (G table, no tenant_id); live rows holding the unique key
	if err := t.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM game.config_items WHERE item_type = $1 AND item_key = $2
		AND deleted_at IS NULL)`, itemType, key).Scan(&taken); err != nil {
		return fmt.Errorf("game trash: %w", err)
	}
	if taken {
		return gameconfig.ErrRestoreConflict
	}
	// tenancyguard:ok platform content (G table, no tenant_id); restore of the soft-deleted row (R21: no hard delete)
	_, err = t.ExecContext(ctx, `UPDATE game.config_items SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL, updated_at = $2,
		updated_by = $3, version = version + 1 WHERE config_id = $1 AND deleted_at IS NOT NULL`, id, at, by)
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "23505" {
		return gameconfig.ErrRestoreConflict
	}
	if err != nil {
		return fmt.Errorf("game trash: %w", err)
	}
	return nil
}
