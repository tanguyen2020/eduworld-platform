package read

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// TrashFilter selects soft-deleted templates (platform configurations, G table): the config types, a keyset after
// (deleted_at, config_id) in descending order and the page size (limit+1 to know whether more rows follow).
type TrashFilter struct {
	ConfigTypes []string
	After       *Keyset
	Limit       int
}

// TrashRow is one soft-deleted template; Conflict: a live configuration holds its key (config_type, code).
type TrashRow struct {
	ConfigType   string
	ConfigID     uuid.UUID
	Title        string
	DeletedAt    time.Time
	DeletedBy    *uuid.UUID
	DeleteReason string
	Conflict     bool
}

// TrashReader lists the soft-deleted rows of competition.game_configs (ctx carries tenancy.Bypass(trash)); paged in SQL.
type TrashReader interface {
	DeletedTemplates(ctx context.Context, f TrashFilter) ([]TrashRow, error)
}
