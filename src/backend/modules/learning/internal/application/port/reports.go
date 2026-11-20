package port

import (
	"context"
	"time"
)

// ReportFiles stores the export files in the bucket ew-exports (ADR-0044, F044 US1): immutable keys, private objects
// expiring by the bucket lifecycle (dashboard_export_file_retention_days); a presigned GET valid for ttl is issued to the
// creator only.
type ReportFiles interface {
	Put(ctx context.Context, key string, data []byte, mime string) error
	ReportURL(ctx context.Context, key string, at time.Time, ttl time.Duration) (string, time.Time, error)
}

// ReportMeta is written in every file: the watermark (top and bottom), the title and, for the reference report, the
// disclaimer (top and bottom, F044-AC-12).
type ReportMeta struct {
	Title      string
	Watermark  string
	Disclaimer string
}

// ReportWriter writes one file row by row.
type ReportWriter interface {
	Header(columns []string) error
	Row(values []string) error
	// Close writes the footer and returns the file.
	Close() ([]byte, error)
}

// ReportWriters creates the writer of a format (csv, xlsx, pdf).
type ReportWriters interface {
	New(format string, meta ReportMeta) (ReportWriter, error)
}
