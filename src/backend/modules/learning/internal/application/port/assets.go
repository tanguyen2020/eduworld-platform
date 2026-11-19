package port

import (
	"context"
	"io"

	"github.com/google/uuid"
)

// AssetReader opens a file uploaded through the media store (createContentImport.asset_id). The adapter over
// object storage comes with F027 (presignMediaUpload); until then no binary wires it and tests use a labelled
// test double (F021 clarification 2026-11-19, OQ F021-4). size is the stored size in bytes.
type AssetReader interface {
	Open(ctx context.Context, assetID uuid.UUID) (rc io.ReadCloser, size int64, err error)
}

// ErrAssetNotFound: the asset does not exist or is not readable by the owner of the import.
var ErrAssetNotFound = assetError("port: asset not found")

type assetError string

func (e assetError) Error() string { return string(e) }
