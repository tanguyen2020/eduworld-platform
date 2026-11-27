package write

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// TrashStore restores a soft-deleted template of competition.game_configs (ctx carries tenancy.Bypass(trash) and the
// UnitOfWork transaction): challenge.ErrNotInTrash when the template of configType is not deleted,
// challenge.ErrRestoreConflict when a live configuration holds its key (config_type, code). Restoring clears
// deleted_at/deleted_by/delete_reason only (R21).
type TrashStore interface {
	RestoreTemplate(ctx context.Context, configType string, id, by uuid.UUID, at time.Time) error
}
