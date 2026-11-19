// Package media is the platform kit of the media pipeline (ADR-0044 §3, CONTENT_MODEL §7): the clamd scanner, the
// magic-byte sniffer, the image processor (decode, orientation, metadata-free re-encode to WebP + PNG at fixed sizes)
// and the ffmpeg transcoder of audio/video, plus the content-addressed object keys. It holds no business rule:
// which kinds and types are allowed, the size limits, the buckets and the asset state machine belong to the module
// that owns the media (module learning for learning content; module game and module chat later).
package media

import (
	"errors"
	"strings"
)

// Buckets of the media pipeline (ADR-0044 §2, deploy/compose s3-init).
const (
	BucketQuarantine = "ew-media-quarantine"
	BucketPublic     = "ew-media-public"
	BucketPrivate    = "ew-media-private"
)

// ImmutableCacheControl is the Cache-Control of every content-addressed object (ADR-0044 §4).
const ImmutableCacheControl = "public, max-age=31536000, immutable"

// QuarantineKey is the key of an upload in ew-media-quarantine.
func QuarantineKey(assetID string) string { return "quarantine/" + assetID }

// VariantKey is the content-addressed key media/<sha256>/<file> of a variant (file = "<variant>.<ext>" or a path
// under the variant directory for multi-file outputs such as HLS).
func VariantKey(sha256, file string) string {
	return "media/" + sha256 + "/" + strings.TrimLeft(file, "/")
}

// MathKey is the key math/<sha256>.svg of a sanitized formula SVG (ADR-0055 §4).
func MathKey(sha256 string) string { return "math/" + sha256 + ".svg" }

// Output is one file produced by the pipeline. Data holds small outputs in memory; Path a temporary file (large
// audio/video outputs) that the caller uploads then removes with the work directory.
type Output struct {
	Variant string // e.g. webp_1024, ogg, hls_720
	File    string // relative file name under media/<sha256>/, e.g. webp_1024.webp, hls_720/index.m3u8
	MIME    string
	Data    []byte
	Path    string
	Bytes   int64 // size of Path
	Main    bool  // the file that stands for the variant (an HLS playlist, not its segments)
}

// Size returns the byte size of the output (Data length, or the file size reported by the producer).
func (o Output) Size() int64 {
	if o.Path != "" {
		return o.Bytes
	}
	return int64(len(o.Data))
}

// Result of processing one file.
type Result struct {
	Width, Height int
	DurationMS    int
	Outputs       []Output
}

// Errors of the pipeline. ErrDecode and ErrTranscode are permanent (the asset is rejected transcode_failed);
// ErrTooLong is a duration over the limit of the kind (too_large); ErrScannerUnavailable is transient (retry).
var (
	ErrDecode             = errors.New("media: cannot decode the file")
	ErrTranscode          = errors.New("media: transcoding failed")
	ErrTooLong            = errors.New("media: duration over the limit")
	ErrScannerUnavailable = errors.New("media: virus scanner unavailable")
	ErrToolMissing        = errors.New("media: transcoding tool not installed")
)
