package read

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// TrashFilter selects soft-deleted console items (platform content, G table): the item types, a keyset after
// (deleted_at, config_id) in descending order and the page size (limit+1 to know whether more rows follow).
type TrashFilter struct {
	ItemTypes []string
	AfterAt   *time.Time
	AfterID   uuid.UUID
	Limit     int
}

// TrashRow is one soft-deleted console item; Conflict: a live item holds its key (item_type, item_key).
type TrashRow struct {
	ItemType     string
	ConfigID     uuid.UUID
	Title        string
	DeletedAt    time.Time
	DeletedBy    *uuid.UUID
	DeleteReason string
	Conflict     bool
}

// TrashReadModel lists the soft-deleted rows of game.config_items (ctx carries tenancy.Bypass(trash)); paged in SQL.
type TrashReadModel interface {
	Deleted(ctx context.Context, f TrashFilter) ([]TrashRow, error)
}
