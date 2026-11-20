// Package media is the domain of the media assets of learning content (F027, CONTENT_MODEL §7, ADR-0044): the kinds
// and their allowed types and size limits, the bucket of an asset, and the pipeline state machine
// quarantined → processing → ready | rejected. Pure Go, no I/O.
package media

import (
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// Kind is a MediaKind.
type Kind string

// Media kinds (DATABASE_DESIGN §3.13).
const (
	KindImage    Kind = "image"
	KindAudio    Kind = "audio"
	KindVideo    Kind = "video"
	KindModel3D  Kind = "model3d"
	KindDocument Kind = "document"
	// KindSubtitle is a WebVTT subtitle of a video lesson (F058 Clarifications [Confirmed]; F027 follow-up of F065).
	KindSubtitle Kind = "subtitle"
)

// Status is a MediaStatus.
type Status string

// Pipeline states.
const (
	StatusQuarantined Status = "quarantined"
	StatusProcessing  Status = "processing"
	StatusReady       Status = "ready"
	StatusRejected    Status = "rejected"
)

// RejectReason is media_assets.reject_reason (EVENT_DESIGN §4.2 media.rejected).
type RejectReason string

// Reject reasons.
const (
	RejectVirus           RejectReason = "virus"
	RejectTypeMismatch    RejectReason = "type_mismatch"
	RejectTooLarge        RejectReason = "too_large"
	RejectTranscodeFailed RejectReason = "transcode_failed"
)

// Buckets (ADR-0044 §2).
const (
	BucketPublic  = "ew-media-public"
	BucketPrivate = "ew-media-private"
)

const mb = int64(1) << 20

// Limits are media_max_bytes_by_kind (CONFIG_DEFAULTS, CONTENT_MODEL §7) and the duration limits of a kind.
type Limits struct {
	MaxBytes    int64
	MaxDuration time.Duration
}

// limits per kind; document is csv_import_max (files of createContentImport).
var limits = map[Kind]Limits{
	KindImage:    {MaxBytes: 10 * mb},
	KindAudio:    {MaxBytes: 20 * mb, MaxDuration: 10 * time.Minute},
	KindVideo:    {MaxBytes: 300 * mb, MaxDuration: 20 * time.Minute},
	KindModel3D:  {MaxBytes: 50 * mb},
	KindDocument: {MaxBytes: 10 * mb},
	// No subtitle limit in CONFIG_DEFAULTS yet: the document limit (csv_import_max) [Proposed, F027 T040].
	KindSubtitle: {MaxBytes: 10 * mb},
}

// allowed lists the MIME types accepted per kind (CONTENT_MODEL §7 inputs; documents are the import formats).
var allowed = map[Kind][]string{
	KindImage:   {"image/png", "image/jpeg", "image/webp", "image/svg+xml"},
	KindAudio:   {"audio/mpeg", "audio/wav", "audio/x-wav", "audio/ogg"},
	KindVideo:   {"video/mp4", "video/webm"},
	KindModel3D: {"model/gltf-binary", "application/x-godot-pck"},
	KindDocument: {"text/csv", "application/json", "application/x-yaml", "application/yaml", "text/yaml", "application/xml", "text/xml",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "application/zip"},
	KindSubtitle: {"text/vtt"},
}

// LimitsOf returns the limits of k.
func LimitsOf(k Kind) Limits { return limits[k] }

// Known reports a media kind.
func (k Kind) Known() bool { _, ok := limits[k]; return ok }

// Errors of an upload request (F027-AC-02).
var (
	ErrUnknownKind     = errors.New("media: unknown kind")
	ErrTypeNotAllowed  = errors.New("media: type not allowed for the kind")
	ErrTooLarge        = errors.New("media: file larger than the limit of the kind")
	ErrAltTextRequired = errors.New("media: alt_text is required for images")
	ErrPlatformOnly    = errors.New("media: 3D models are platform content only")
	ErrNotQuarantined  = errors.New("media: the asset is not waiting for its upload")
	ErrNotProcessing   = errors.New("media: the asset is not processing")
)

// NormalizeMIME lower-cases a MIME type and drops its parameters.
func NormalizeMIME(m string) string {
	return strings.ToLower(strings.TrimSpace(strings.SplitN(m, ";", 2)[0]))
}

// Upload is a presign request.
type Upload struct {
	Kind       Kind
	MIME       string
	Size       int64
	AltText    string
	Owner      contentmodel.Owner
	ExamMedia  bool
	SHA256     string
	Filename   string
	License    string
	UploadedBy uuid.UUID
}

// Validate applies the type, size and alt-text rules of an upload (F027-AC-02) and the platform-only rule of 3D
// models (CONTENT_MODEL §7).
func (u Upload) Validate() error {
	if !u.Kind.Known() {
		return ErrUnknownKind
	}
	if !slices.Contains(allowed[u.Kind], NormalizeMIME(u.MIME)) {
		return ErrTypeNotAllowed
	}
	if u.Size <= 0 || u.Size > limits[u.Kind].MaxBytes {
		return ErrTooLarge
	}
	if u.Kind == KindImage && strings.TrimSpace(u.AltText) == "" {
		return ErrAltTextRequired
	}
	if u.Kind == KindModel3D && !u.Owner.Platform() {
		return ErrPlatformOnly
	}
	return nil
}

// BucketFor is the bucket of the variants (ADR-0044 §2): platform content in ew-media-public; tenant content, exam
// material and import documents in ew-media-private (signed URLs).
func BucketFor(o contentmodel.Owner, exam bool, k Kind) string {
	if o.Platform() && !exam && k != KindDocument {
		return BucketPublic
	}
	return BucketPrivate
}

// Variant is one entry of media_assets.variants: the main object of a variant.
type Variant struct {
	Key   string `json:"key"`
	Bytes int64  `json:"bytes"`
	MIME  string `json:"mime"`
}

// Asset is a media asset (content.media_assets).
type Asset struct {
	ID                uuid.UUID
	Owner             contentmodel.Owner
	Kind              Kind
	OriginalFilename  string
	MIME              string
	Size              int64
	Width, Height     *int
	DurationMS        *int
	SHA256            string
	Bucket            string
	Variants          map[string]Variant
	AltText           *string
	License           string
	Status            Status
	RejectReason      *string
	QuarantineKey     string
	ExamMaterial      bool
	UploadCompletedAt *time.Time
	ProcessedAt       *time.Time
	UploadedBy        uuid.UUID
	RowVersion        int64
	CreatedAt         time.Time
}

// New builds the quarantined asset of a validated upload.
func New(id uuid.UUID, u Upload, quarantineKey string) Asset {
	a := Asset{ID: id, Owner: u.Owner, Kind: u.Kind, OriginalFilename: u.Filename, MIME: NormalizeMIME(u.MIME), Size: u.Size,
		SHA256: strings.ToLower(u.SHA256), Bucket: BucketFor(u.Owner, u.ExamMedia, u.Kind), Variants: map[string]Variant{}, License: u.License,
		Status: StatusQuarantined, QuarantineKey: quarantineKey, ExamMaterial: u.ExamMedia, UploadedBy: u.UploadedBy}
	if alt := strings.TrimSpace(u.AltText); alt != "" {
		a.AltText = &alt
	}
	return a
}

// Uploaded is what completeMediaUpload found in the quarantine bucket.
type Uploaded struct {
	Exists bool
	Size   int64
	SHA256 string
}

// Complete moves a quarantined asset after its upload (F027-AC-03): processing when the object has the declared
// size and sha256, rejected (too_large / type_mismatch) otherwise. It reports whether the asset now processes.
func (a *Asset) Complete(u Uploaded, at time.Time) (bool, error) {
	if a.Status != StatusQuarantined {
		return false, ErrNotQuarantined
	}
	t := at
	a.UploadCompletedAt = &t
	switch {
	case u.Size > a.Size || u.Size > limits[a.Kind].MaxBytes:
		a.reject(RejectTooLarge)
		return false, nil
	case u.Size != a.Size || !strings.EqualFold(u.SHA256, a.SHA256):
		a.reject(RejectTypeMismatch)
		return false, nil
	}
	a.Status = StatusProcessing
	return true, nil
}

// MarkReady records the variants of a processed asset (F027-AC-05).
func (a *Asset) MarkReady(variants map[string]Variant, w, h, durationMS int, at time.Time) error {
	if a.Status != StatusProcessing {
		return ErrNotProcessing
	}
	a.Status, a.Variants = StatusReady, variants
	a.Width, a.Height, a.DurationMS = positive(w), positive(h), positive(durationMS)
	t := at
	a.ProcessedAt = &t
	return nil
}

// Reject ends the pipeline of a processing asset (F027-AC-04, AC-05).
func (a *Asset) Reject(r RejectReason, at time.Time) error {
	if a.Status != StatusProcessing {
		return ErrNotProcessing
	}
	a.reject(r)
	t := at
	a.ProcessedAt = &t
	return nil
}

func (a *Asset) reject(r RejectReason) {
	s := string(r)
	a.Status, a.RejectReason = StatusRejected, &s
}

func positive(n int) *int {
	if n <= 0 {
		return nil
	}
	return &n
}

// RefKeys are the keys of rich content that reference a media asset (QUESTION_BANK §2, CONTENT_MODEL §9).
var RefKeys = []string{"media_ref", "audio_ref", "background_media_ref", "image_media_ref"}
