package port

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/preview"
	pkgmedia "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/media"
)

// ObjectStorage is the S3-compatible object storage of the media pipeline (F027 T008, ADR-0044, ADR-0071: floci in
// every environment through EW_S3_ENDPOINT). Every call names its bucket.
type ObjectStorage interface {
	// PresignPut signs a PUT of exactly size bytes of type mime, valid for ttl; headers are the headers the client
	// must send with the PUT.
	PresignPut(ctx context.Context, bucket, key string, size int64, mime string, ttl time.Duration) (url string, headers map[string]string, err error)
	// Head returns the size of an object or ErrObjectNotFound.
	Head(ctx context.Context, bucket, key string) (int64, error)
	// Open streams an object (the caller closes it) or returns ErrObjectNotFound.
	Open(ctx context.Context, bucket, key string) (io.ReadCloser, int64, error)
	// Put writes an object with its content type and Cache-Control.
	Put(ctx context.Context, bucket, key string, body io.Reader, size int64, mime, cacheControl string) error
}

// ErrObjectNotFound: no object at the key.
var ErrObjectNotFound = errors.New("port: object not found")

// URLSigner builds the CDN URL of an object (F027-AC-06): public objects get a plain URL; objects of
// ew-media-private a signed URL valid media_signed_url_ttl (expires is then set).
type URLSigner interface {
	URL(bucket, key string, now time.Time) (url string, expires *time.Time)
}

// MediaProcessor is the media toolchain of the worker (src/backend/pkg/media): virus scan, image variants and
// audio/video transcoding.
type MediaProcessor interface {
	Scan(ctx context.Context, r io.Reader) (pkgmedia.Verdict, error)
	Image(data []byte, mime string) (pkgmedia.Result, error)
	AudioVideo(ctx context.Context, video bool, input, workDir string, maxDuration time.Duration) (pkgmedia.Result, error)
}

// PreviewStore keeps preview sessions in Valkey (ew:learning:preview:<id>, TTL preview_token_ttl). Not business
// state (R9): a lost session is created again.
type PreviewStore interface {
	Put(ctx context.Context, s preview.Session, ttl time.Duration) error
	// Get returns the session or ErrPreviewNotFound (expired or never created).
	Get(ctx context.Context, id uuid.UUID) (preview.Session, error)
}

// ErrPreviewNotFound: the preview session expired or does not exist.
var ErrPreviewNotFound = errors.New("port: preview session not found")
