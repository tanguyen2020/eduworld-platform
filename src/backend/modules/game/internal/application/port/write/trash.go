package write

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// TrashStore restores a soft-deleted console item of game.config_items (ctx carries tenancy.Bypass(trash) and the
// UnitOfWork transaction): gameconfig.ErrNotInTrash when the item of itemType is not deleted, gameconfig.ErrRestoreConflict
// when a live item holds its key (item_type, item_key). Restoring clears deleted_at/deleted_by/delete_reason only (R21).
type TrashStore interface {
	RestoreConfig(ctx context.Context, itemType string, id, by uuid.UUID, at time.Time) error
}
