package port

import (
	"context"
	"errors"
	"io"
	"time"
)

// ErrObjectNotFound: the object does not exist (yet).
var ErrObjectNotFound = errors.New("object storage: not found")

// ObjectStorage is the S3-compatible storage of SIS import files (bucket ew-sis-imports, private; ADR-0075:
// floci locally, managed S3-compatible storage in staging/production). Objects are never public.
type ObjectStorage interface {
	// PresignPut returns a URL to upload exactly size bytes to key before ttl expires.
	PresignPut(ctx context.Context, key string, size int64, ttl time.Duration) (string, error)
	// Head returns the size of the object (ErrObjectNotFound when absent).
	Head(ctx context.Context, key string) (int64, error)
	// Open streams the object.
	Open(ctx context.Context, key string) (io.ReadCloser, error)
}

// MalwareScanner scans an uploaded file before it is read (clamd INSTREAM).
type MalwareScanner interface {
	// Scan returns clean=false with the signature name when malware is found.
	Scan(ctx context.Context, r io.Reader) (clean bool, signature string, err error)
}

// ParsedRecord is one valid row of an import file.
type ParsedRecord struct {
	Row  int
	Data map[string]any
}

// ParsedViolation is one contract violation of a row (row 1 = header).
type ParsedViolation struct {
	Row                   int
	Column, Code, Message string
}

// ParsedFile is an import file read against the SIS contract (unknown columns dropped and counted).
type ParsedFile struct {
	Kind       string // entity kind of the pipeline: school, student (class/enrollment ignored, ADR-0095)
	Records    []ParsedRecord
	Violations []ParsedViolation
}

// SisFileParser reads a CSV file of one entity kind ("students" …) against the SIS contract DTO.
type SisFileParser interface {
	Parse(kind string, r io.Reader, now time.Time) (ParsedFile, error)
}
