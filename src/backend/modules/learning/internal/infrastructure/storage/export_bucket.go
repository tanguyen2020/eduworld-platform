package storage

import (
	"bytes"
	"context"
	"net/http"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
)

// BucketExports is the bucket of generated export files (ADR-0044, F025 US5).
const BucketExports = "ew-exports"

// ExportDownloadTTL is the validity of a presigned export download (media_signed_url_ttl, CONFIG_DEFAULTS: 10 min).
const ExportDownloadTTL = 10 * time.Minute

// PresignGet returns a presigned GET URL of an object valid for ttl from at.
func (s *S3) PresignGet(bucket, key string, at time.Time, ttl time.Duration) string {
	uri := s.objectURI(bucket, key)
	return s.base.Scheme + "://" + s.base.Host + uri + "?" + presign(s.cfg, http.MethodGet, s.base.Host, uri, nil, at.UTC(), ttl)
}

// ExportBucket implements port.ExportFiles on the S3-compatible object storage (floci in every environment, ADR-0071).
type ExportBucket struct {
	S3     *S3
	Bucket string // BucketExports when empty
}

var _ port.ExportFiles = ExportBucket{}

func (b ExportBucket) bucket() string {
	if b.Bucket == "" {
		return BucketExports
	}
	return b.Bucket
}

// Put implements port.ExportFiles.
func (b ExportBucket) Put(ctx context.Context, key string, data []byte, mime string) error {
	return b.S3.Put(ctx, b.bucket(), key, bytes.NewReader(data), int64(len(data)), mime, "private, no-store")
}

// DownloadURL implements port.ExportFiles.
func (b ExportBucket) DownloadURL(_ context.Context, key string, at time.Time) (string, time.Time, error) {
	return b.S3.PresignGet(b.bucket(), key, at, ExportDownloadTTL), at.Add(ExportDownloadTTL).UTC(), nil
}

var _ port.ReportFiles = ExportBucket{}

// ReportURL implements port.ReportFiles: a presigned GET of an export of F044 valid for ttl
// (dashboard_export_link_ttl_minutes); past it, the object storage answers 403.
func (b ExportBucket) ReportURL(_ context.Context, key string, at time.Time, ttl time.Duration) (string, time.Time, error) {
	return b.S3.PresignGet(b.bucket(), key, at, ttl), at.Add(ttl).UTC(), nil
}
