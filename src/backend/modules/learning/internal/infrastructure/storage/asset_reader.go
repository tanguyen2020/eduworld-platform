package storage

import (
	"context"
	"errors"
	"io"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/media"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// AssetLookup reads an asset in the scope of ctx (MediaRepo or MediaModel).
type AssetLookup interface {
	Asset(ctx context.Context, id uuid.UUID) (media.Asset, error)
}

// AssetReader implements port.AssetReader over the media library (OQ F021-4): an import file is a ready document
// asset of the caller's scope; its "original" variant is streamed from object storage.
type AssetReader struct {
	Assets  AssetLookup
	Storage port.ObjectStorage
}

var _ port.AssetReader = AssetReader{}

// Open implements port.AssetReader.
func (r AssetReader) Open(ctx context.Context, id uuid.UUID) (io.ReadCloser, int64, error) {
	a, err := r.Assets.Asset(ctx, id)
	var pe *problem.Error
	if errors.As(err, &pe) && pe.Code == problem.NotFound {
		return nil, 0, port.ErrAssetNotFound
	}
	if err != nil {
		return nil, 0, err
	}
	v, ok := a.Variants["original"]
	if a.Status != media.StatusReady || !ok {
		return nil, 0, port.ErrAssetNotFound
	}
	rc, size, err := r.Storage.Open(ctx, a.Bucket, v.Key)
	if errors.Is(err, port.ErrObjectNotFound) {
		return nil, 0, port.ErrAssetNotFound
	}
	return rc, size, err
}
