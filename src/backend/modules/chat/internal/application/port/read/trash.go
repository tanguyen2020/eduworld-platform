package read

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// TrashFilter selects soft-deleted sticker packs (platform library, G table): a keyset after (deleted_at, pack_id) in
// descending order and the page size (limit+1 to know whether more rows follow).
type TrashFilter struct {
	AfterAt *time.Time
	AfterID uuid.UUID
	Limit   int
}

// TrashRow is one soft-deleted sticker pack; Conflict: a live pack holds its code.
type TrashRow struct {
	PackID    uuid.UUID
	Title     string
	DeletedAt time.Time
	DeletedBy *uuid.UUID
	Conflict  bool
}

// TrashReader lists the soft-deleted rows of chat.sticker_packs (ctx carries tenancy.Bypass(trash)); paged in SQL.
type TrashReader interface {
	DeletedPacks(ctx context.Context, f TrashFilter) ([]TrashRow, error)
}
