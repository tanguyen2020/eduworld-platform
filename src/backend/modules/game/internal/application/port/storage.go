package port

import (
	"context"
	"errors"
	"io"
	"time"
)

// ObjectStorage is the S3-compatible object storage of game media and renders (F078, ADR-0044, ADR-0071: floci in every
// environment through EW_S3_ENDPOINT). Every call names its bucket (ew-media-quarantine, ew-media-public).
type ObjectStorage interface {
	// PresignPut signs a PUT of exactly size bytes of type mime, valid for ttl; headers are the headers the client must
	// send with the PUT.
	PresignPut(ctx context.Context, bucket, key string, size int64, mime string, ttl time.Duration) (url string, headers map[string]string, err error)
	// PresignGet signs a GET of a private object valid for ttl (download link of a translation file, F082).
	PresignGet(ctx context.Context, bucket, key string, ttl time.Duration) (string, error)
	// Head returns the size of an object or ErrObjectNotFound.
	Head(ctx context.Context, bucket, key string) (int64, error)
	// Open streams an object (the caller closes it) or returns ErrObjectNotFound.
	Open(ctx context.Context, bucket, key string) (io.ReadCloser, int64, error)
	// Put writes an object with its content type and Cache-Control.
	Put(ctx context.Context, bucket, key string, body io.Reader, size int64, mime, cacheControl string) error
}

// ErrObjectNotFound: no object at the key.
var ErrObjectNotFound = errors.New("port: object not found")

// MediaScanner is the virus scanner of the media pipeline (pkg/media Clamd); nil = scan disabled (local only).
type MediaScanner interface {
	Scan(ctx context.Context, r io.Reader) (clean bool, err error)
}
