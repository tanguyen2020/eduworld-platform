package gameconfig

import (
	"bytes"
	"errors"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// MediaKind is game.media_assets.kind.
type MediaKind string

// Media kinds of game content (CONTENT_MODEL §7) and the renders of a published version (DESIGN_SYSTEM §7).
const (
	MediaImage           MediaKind = "image"
	MediaAudio           MediaKind = "audio"
	MediaModel3D         MediaKind = "model3d"
	MediaRenderThumbnail MediaKind = "render_thumbnail"
	MediaRenderTurntable MediaKind = "render_turntable"
	// MediaDocument is a translation file to import (XLIFF/CSV, F082): checked, kept private, never published.
	MediaDocument MediaKind = "document"
)

// MediaStatus is game.media_assets.status.
type MediaStatus string

// Media statuses (quarantined → processing → ready | rejected).
const (
	MediaQuarantined MediaStatus = "quarantined"
	MediaProcessing  MediaStatus = "processing"
	MediaReady       MediaStatus = "ready"
	MediaRejected    MediaStatus = "rejected"
)

// Reject reasons (media.rejected.reason).
const (
	RejectTypeMismatch     = "type_mismatch"
	RejectTooLarge         = "too_large"
	RejectChecksumMismatch = "checksum_mismatch"
	RejectUploadMissing    = "upload_missing"
	RejectVirus            = "virus"
)

// PresignTTL is media_presign_ttl (CONFIG_DEFAULTS, owner CONTENT_MODEL §7): 15 minutes.
const PresignTTL = 15 * time.Minute

// Errors of a presign request (415 MEDIA_TYPE_NOT_ALLOWED, 413 MEDIA_TOO_LARGE, 422 VALIDATION_FAILED).
var (
	ErrMediaKind     = errors.New("gameconfig: media kind not allowed for game content")
	ErrMediaType     = errors.New("gameconfig: file type not allowed for this kind")
	ErrMediaTooLarge = errors.New("gameconfig: file larger than the limit of its kind")
	ErrMediaInput    = errors.New("gameconfig: invalid media metadata")
)

// mediaRule is the allowed MIME types and the size limit of a kind (media_max_bytes_by_kind, CONTENT_MODEL §7).
type mediaRule struct {
	MIME     []string
	MaxBytes int64
	Ext      map[string]string // MIME → extension of the stored original
}

const mb = 1 << 20

var mediaRules = map[MediaKind]mediaRule{
	MediaImage: {MIME: []string{"image/png", "image/jpeg", "image/webp"}, MaxBytes: 10 * mb,
		Ext: map[string]string{"image/png": "png", "image/jpeg": "jpg", "image/webp": "webp"}},
	MediaAudio: {MIME: []string{"audio/ogg", "audio/mpeg", "audio/wav", "audio/x-wav"}, MaxBytes: 20 * mb,
		Ext: map[string]string{"audio/ogg": "ogg", "audio/mpeg": "mp3", "audio/wav": "wav", "audio/x-wav": "wav"}},
	MediaModel3D: {MIME: []string{"model/gltf-binary", "application/x-godot-pck"}, MaxBytes: 50 * mb,
		Ext: map[string]string{"model/gltf-binary": "glb", "application/x-godot-pck": "pck"}},
	// document: translation files to import (Clarifications F082 2026-11-26, media_max_bytes_by_kind).
	MediaDocument: {MIME: []string{"application/xliff+xml", "text/csv"}, MaxBytes: 5 * mb,
		Ext: map[string]string{"application/xliff+xml": "xlf", "text/csv": "csv"}},
}

var shaRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Upload is a presign request of game media.
type Upload struct {
	Kind     MediaKind
	MIME     string
	Size     int64
	SHA256   string
	Filename string
	License  string
}

// Validate checks the kind (image, audio, model3d — the render kinds are produced by the renderer only), the MIME type
// and the size limit of the kind (F078-AC-18).
func (u Upload) Validate() error {
	r, ok := mediaRules[u.Kind]
	if !ok {
		return ErrMediaKind
	}
	if !contains(r.MIME, strings.ToLower(u.MIME)) {
		return ErrMediaType
	}
	if u.Size > r.MaxBytes {
		return ErrMediaTooLarge
	}
	if u.Size < 1 || !shaRe.MatchString(u.SHA256) || strings.TrimSpace(u.Filename) == "" || strings.TrimSpace(u.License) == "" {
		return ErrMediaInput
	}
	return nil
}

// TextDocument reports a translation file body: valid UTF-8 text without NUL bytes (XLIFF/CSV are text; anything binary
// is refused by the media pipeline as type_mismatch).
func TextDocument(b []byte) bool {
	return utf8.Valid(b) && !bytes.ContainsRune(b, 0)
}

// Extension returns the extension of the stored original of a kind/MIME ("bin" when unknown).
func Extension(kind MediaKind, mime string) string {
	if e, ok := mediaRules[kind].Ext[strings.ToLower(mime)]; ok {
		return e
	}
	return "bin"
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// Render rules (FR-CMS-017, DESIGN_SYSTEM §7): a 512×512 thumbnail and a 24-frame turntable from the 3D asset of a
// published version; a failed render is retried with exponential backoff, then dead-lettered.
const (
	RenderSize        = 512
	RenderFrames      = 24
	RenderMaxAttempts = 5
)

// RenderStatus is game.render_jobs.status.
type RenderStatus string

// Render job statuses.
const (
	RenderQueued RenderStatus = "queued"
	RenderDone   RenderStatus = "done"
	RenderFailed RenderStatus = "failed" // retry pending
	RenderDead   RenderStatus = "dead"   // retries exhausted (DLQ)
)

// RenderFailure returns the status after a failed attempt and when to retry: backoff base·2^(attempts-1), capped at
// one hour; dead after max attempts.
func RenderFailure(attempts, maxAttempts int, base time.Duration, now time.Time) (RenderStatus, *time.Time) {
	if attempts >= maxAttempts {
		return RenderDead, nil
	}
	d := time.Duration(float64(base) * math.Pow(2, float64(attempts-1)))
	if d > time.Hour || d <= 0 {
		d = time.Hour
	}
	at := now.Add(d)
	return RenderFailed, &at
}

// RenderKey is the content-addressed object key of a render (media/<sha256>/<kind>.png, ADR-0044 immutable keys).
func RenderKey(sha256 string, kind MediaKind) string {
	return "media/" + sha256 + "/" + string(kind) + ".png"
}

// OriginalKey is the content-addressed object key of an uploaded original.
func OriginalKey(sha256 string, kind MediaKind, mime string) string {
	return "media/" + sha256 + "/original." + Extension(kind, mime)
}
