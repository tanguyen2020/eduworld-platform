package read

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/media"
)

// MediaFilter selects assets of the media library (listMediaAssets, F027-AC-09).
type MediaFilter struct {
	Kind   string
	Status string
	Q      string // original file name or alt text, case-insensitive
	After  *uuid.UUID
	Limit  int // rows to return (the caller asks limit+1 to know has_more)
}

// MediaUsage is one content version using an asset (F027-AC-10).
type MediaUsage struct {
	ItemType  string
	ItemID    uuid.UUID
	VersionID uuid.UUID
	VersionNo int
}

// MediaReadModel reads the media library with the scope of ctx (own tenant + assigned platform assets, or the
// platform, or the tenants of System Admin).
type MediaReadModel interface {
	List(ctx context.Context, f MediaFilter) ([]media.Asset, error)
	Asset(ctx context.Context, id uuid.UUID) (media.Asset, error)
	Usages(ctx context.Context, assetID uuid.UUID, limit int) ([]MediaUsage, error)
}
