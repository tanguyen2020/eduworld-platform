package write

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/media"
)

// MediaStore persists media assets and their usages (content.media_assets, content.media_usages) under the scope
// of ctx (F027).
type MediaStore interface {
	InsertAsset(ctx context.Context, a media.Asset) error
	// Asset returns a visible asset or 404 NOT_FOUND.
	Asset(ctx context.Context, id uuid.UUID) (media.Asset, error)
	// AssetBySHA finds the live asset of the same file in one owner scope (tenant rows of tenant, or platform rows).
	AssetBySHA(ctx context.Context, owner contentmodel.Owner, sha256 string) (media.Asset, bool, error)
	// SaveAsset writes the pipeline state of a (status, reject reason, variants, size, timestamps), checking its row
	// version (409 VERSION_MISMATCH when another worker moved it).
	SaveAsset(ctx context.Context, a media.Asset) error
	// Assets returns the visible assets among ids.
	Assets(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]media.Asset, error)
	// ReplaceUsages makes the usages of one content version exactly u.AssetIDs (rows dropped are soft-deleted).
	ReplaceUsages(ctx context.Context, u UsageSet) error
}

// UsageSet is the media referenced by one content version (F027-AC-11).
type UsageSet struct {
	Owner     contentmodel.Owner
	ItemType  contentmodel.ContentType
	ItemID    uuid.UUID
	VersionID uuid.UUID
	VersionNo int
	AssetIDs  []uuid.UUID
}

// MathSvg is a stored formula SVG (content.math_svgs).
type MathSvg struct {
	SHA256      string
	StorageKey  string
	Latex       string
	SpeechText  string
	SizeBytes   int
	SanitizedAt time.Time
}

// MathStore persists formula SVG metadata (content.math_svgs, G, insert only).
type MathStore interface {
	// MathSvg returns a stored SVG by hash.
	MathSvg(ctx context.Context, sha256 string) (MathSvg, bool, error)
	// InsertMathSvg stores a new SVG row; an existing hash is not an error (content-addressed).
	InsertMathSvg(ctx context.Context, m MathSvg) error
}
