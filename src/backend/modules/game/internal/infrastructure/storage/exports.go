package storage

import (
	"context"
	"io"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
)

// Exports implements port.ExportFiles on the export bucket (ew-exports, ADR-0044): CSV files of Game Analytics and their
// signed links; the bucket lifecycle expires them after dashboard_export_file_retention_days.
type Exports struct {
	S3     *S3
	Bucket string
}

var _ port.ExportFiles = Exports{}

// Put implements port.ExportFiles.
func (e Exports) Put(ctx context.Context, key string, body io.Reader, size int64) error {
	return e.S3.Put(ctx, e.Bucket, key, body, size, "text/csv; charset=utf-8", "private, no-store")
}

// SignGet implements port.ExportFiles.
func (e Exports) SignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	return e.S3.PresignGet(ctx, e.Bucket, key, ttl)
}
