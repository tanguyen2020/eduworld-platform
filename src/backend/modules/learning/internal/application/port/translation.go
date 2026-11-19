package port

import (
	"context"
	"time"
)

// TranslationUnit is one text of a translation file: Key is <content_type>/<content_id>/<version>/<json-pointer>
// (ADMIN_PORTAL §1.5), Source the text in the source language, Target the translation (may be empty on export).
type TranslationUnit struct {
	Key    string
	Source string
	Target string
}

// TranslationCodec encodes and decodes translation files (XLIFF 2.1, CSV).
type TranslationCodec interface {
	Encode(format, source, target string, units []TranslationUnit) (data []byte, mime string, err error)
	Decode(format string, data []byte) ([]TranslationUnit, error)
}

// ExportFiles stores generated export files (bucket ew-exports, ADR-0044) and signs their download URL.
type ExportFiles interface {
	Put(ctx context.Context, key string, data []byte, mime string) error
	DownloadURL(ctx context.Context, key string, at time.Time) (string, time.Time, error)
}
