package write

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// TrashStore restores a soft-deleted sticker pack of chat.sticker_packs (ctx carries tenancy.Bypass(trash) and the
// UnitOfWork transaction on chat_platform): chatasset.ErrNotInTrash when the pack is not deleted,
// chatasset.ErrRestoreConflict when a live pack holds its code. Restoring clears deleted_at/deleted_by only (R21).
type TrashStore interface {
	RestorePack(ctx context.Context, id uuid.UUID, at time.Time) error
}
