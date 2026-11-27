package command

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/scope"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/mathx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/media"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	pkgmedia "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/media"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Media events and audit (EVENT_DESIGN §4.2, ADMIN_PORTAL audit actions).
const (
	EventMediaUploaded  = "media.uploaded"
	EventMediaProcessed = "media.processed"
	EventMediaRejected  = "media.rejected"
	ActionMediaUploaded = "media.uploaded"
)

// Code defaults of CONFIG_DEFAULTS (owner CONTENT_MODEL §7, ADMIN_PORTAL §5) until module learning reads platform
// settings (same rule as AssetsPerVersionMax, OQ F022-9).
const (
	MediaPresignTTL   = 15 * time.Minute // media_presign_ttl
	MediaSignedURLTTL = 10 * time.Minute // media_signed_url_ttl
	PreviewTokenTTL   = 30 * time.Minute // preview_token_ttl
)

// MediaMetrics observes the pipeline (eduworld_media_pipeline_duration_seconds, eduworld_media_rejected_total);
// nil = not observed.
type MediaMetrics interface {
	Pipeline(kind, result string, d time.Duration)
	Rejected(reason string)
}

// Media holds the collaborators of the media commands (F027 US1).
type Media struct {
	UoW       write.UnitOfWork
	Store     write.MediaStore
	Storage   port.ObjectStorage
	Signer    port.URLSigner
	Processor port.MediaProcessor
	Events    write.EventPublisher
	Audit     write.Auditor
	Clock     func() time.Time
	// ScanDisabled skips the virus scan (EW_MEDIA_SCAN=disabled, local development only; never in staging/CI).
	ScanDisabled bool
	WorkDir      string // temporary files of audio/video (default os.TempDir)
	Metrics      MediaMetrics
}

// PresignInput is MediaPresignRequest.
type PresignInput struct {
	Kind         string
	Filename     string
	MIME         string
	Size         int64
	SHA256       string
	License      string
	AltText      string
	ExamMaterial bool
	OwnerScope   string
	TenantID     *uuid.UUID
}

// PresignResult is MediaPresignResponse.
type PresignResult struct {
	AssetID     uuid.UUID         `json:"asset_id"`
	Status      string            `json:"status"`
	UploadURL   string            `json:"upload_url,omitempty"`
	Headers     map[string]string `json:"upload_headers,omitempty"`
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
	DuplicateOf *uuid.UUID        `json:"duplicate_of,omitempty"`
}

// uploadProblem maps a refused upload to its problem (F027-AC-02).
func uploadProblem(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, media.ErrTypeNotAllowed):
		return problem.New(problem.MediaTypeNotAllowed, "the file type is not allowed for this kind of media")
	case errors.Is(err, media.ErrTooLarge):
		return problem.New(problem.MediaTooLarge, "the file is larger than the limit of this kind of media")
	case errors.Is(err, media.ErrAltTextRequired):
		return problem.Invalid("alt_text", problem.SubRequired, "an image needs an alternative text")
	case errors.Is(err, media.ErrUnknownKind):
		return problem.Invalid("kind", problem.SubInvalid, "is not a media kind")
	case errors.Is(err, media.ErrPlatformOnly):
		return denied(ctx, "model3d_platform_only")
	}
	return err
}

// mediaOwner chooses the owner and scope of a new asset like the content items: a tenant user (or System Admin
// with X-Tenant-Id) for its session tenant, platform staff for the platform, System Admin for a tenant_id target.
func mediaOwner(ctx context.Context, p authz.Principal, ownerScope string, target *uuid.UUID) (contentmodel.Owner, tenancy.Scope, error) {
	if ownerScope == string(contentmodel.OwnerPlatform) && p.TenantID != nil {
		return contentmodel.Owner{}, nil, denied(ctx, "platform_media_by_tenant")
	}
	// The OpenAPI default fills owner_scope "tenant": for platform staff without a target tenant it means platform.
	kind := contentmodel.OwnerPlatform
	if p.TenantID != nil || target != nil {
		kind = contentmodel.OwnerTenant
	}
	sc, err := scope.ForWrite(p, kind, target)
	if err != nil {
		return contentmodel.Owner{}, nil, err
	}
	o := contentmodel.Owner{Scope: kind}
	if kind == contentmodel.OwnerTenant {
		o.TenantID = ptrClone(p.TenantID)
		if o.TenantID == nil {
			o.TenantID = ptrClone(target)
		}
	}
	return o, sc, nil
}

// PresignMediaUpload creates a quarantined asset and a presigned PUT to ew-media-quarantine valid media_presign_ttl
// (F027-AC-01, AC-02, AC-07); the same file in the same scope returns the existing asset as duplicate_of, without
// a new URL.
func (h Media) PresignMediaUpload(ctx context.Context, in PresignInput) (out PresignResult, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.PresignMediaUpload")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermMediaUpload)
	if err != nil {
		return out, err
	}
	o, sc, err := mediaOwner(ctx, p, in.OwnerScope, in.TenantID)
	if err != nil {
		return out, err
	}
	ctx = tenancy.WithScope(ctx, sc)
	u := media.Upload{Kind: media.Kind(in.Kind), MIME: in.MIME, Size: in.Size, AltText: in.AltText, Owner: o, ExamMedia: in.ExamMaterial,
		SHA256: in.SHA256, Filename: in.Filename, License: in.License, UploadedBy: p.UserID}
	if err := u.Validate(); err != nil {
		return out, uploadProblem(ctx, err)
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		dup, found, err := h.Store.AssetBySHA(ctx, o, u.SHA256)
		if err != nil {
			return err
		}
		if found {
			id := dup.ID
			out = PresignResult{AssetID: id, Status: string(dup.Status), DuplicateOf: &id}
			return idempotency.SetResult(ctx, out)
		}
		id := ids.New()
		a := media.New(id, u, pkgmedia.QuarantineKey(id.String()))
		if err := h.Store.InsertAsset(ctx, a); err != nil {
			return err
		}
		url, headers, err := h.Storage.PresignPut(ctx, pkgmedia.BucketQuarantine, a.QuarantineKey, a.Size, a.MIME, MediaPresignTTL)
		if err != nil {
			return problem.New(problem.Unavailable, "object storage unavailable").Retry().Wrap(err)
		}
		exp := now(h.Clock).Add(MediaPresignTTL)
		out = PresignResult{AssetID: id, Status: string(a.Status), UploadURL: url, Headers: headers, ExpiresAt: &exp}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// UploadedPayload is media.uploaded v1.
type UploadedPayload struct {
	AssetID       uuid.UUID  `json:"asset_id"`
	TenantID      *uuid.UUID `json:"tenant_id"`
	Kind          string     `json:"kind"`
	MIME          string     `json:"mime_type"`
	QuarantineKey string     `json:"quarantine_key"`
	SizeBytes     int64      `json:"size_bytes"`
	SHA256        string     `json:"sha256"`
}

// CompleteMediaUpload checks the uploaded object (exists, size, sha256) and moves the asset to processing with the
// outbox event media.uploaded and the audit media.uploaded (F027-AC-03); a mismatch rejects it. An asset that is not
// quarantined any more is returned as it is, without a new event (edge case: called twice).
func (h Media) CompleteMediaUpload(ctx context.Context, id uuid.UUID) (out dto.MediaView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CompleteMediaUpload")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermMediaUpload)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		a, err := h.Store.Asset(ctx, id)
		if err != nil {
			return err
		}
		if err := writable(ctx, p, authz.PermMediaUpload, a.Owner); err != nil {
			return err
		}
		if a.Status != media.StatusQuarantined {
			out = h.View(a)
			return idempotency.SetResult(ctx, out)
		}
		up, err := h.inspect(ctx, a)
		if err != nil {
			return err
		}
		processing, err := a.Complete(up, now(h.Clock))
		if err != nil {
			return err
		}
		if err := h.Store.SaveAsset(ctx, a); err != nil {
			return err
		}
		if processing {
			if err := h.publish(ctx, a, EventMediaUploaded, UploadedPayload{AssetID: a.ID, TenantID: a.Owner.TenantID, Kind: string(a.Kind),
				MIME: a.MIME, QuarantineKey: a.QuarantineKey, SizeBytes: a.Size, SHA256: a.SHA256}); err != nil {
				return err
			}
		} else if err := h.rejected(ctx, a); err != nil {
			return err
		}
		if err := h.trash().recordFor(ctx, a.Owner.TenantID, audit.Event{Action: ActionMediaUploaded, TargetType: string(contentmodel.ResourceMedia),
			TargetID: a.ID.String(), Result: audit.ResultSuccess, Metadata: map[string]any{"kind": string(a.Kind), "status": string(a.Status),
				"size_bytes": a.Size}}); err != nil {
			return err
		}
		if a, err = h.Store.Asset(ctx, id); err != nil {
			return err
		}
		out = h.View(a)
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// View maps an asset with its variant URLs (signed when private).
func (h Media) View(a media.Asset) dto.MediaView {
	if h.Signer == nil {
		return dto.MediaViewOf(a, nil)
	}
	at := now(h.Clock)
	return dto.MediaViewOf(a, func(bucket, key string) (string, *time.Time) { return h.Signer.URL(bucket, key, at) })
}

func (h Media) trash() TrashDeps {
	return TrashDeps{UoW: h.UoW, Events: h.Events, Audit: h.Audit, Clock: h.Clock}
}

// inspect reads the quarantined object: missing → 422 (the PUT never happened, the asset stays quarantined).
func (h Media) inspect(ctx context.Context, a media.Asset) (media.Uploaded, error) {
	size, err := h.Storage.Head(ctx, pkgmedia.BucketQuarantine, a.QuarantineKey)
	if errors.Is(err, port.ErrObjectNotFound) {
		return media.Uploaded{}, problem.Invalid("asset_id", problem.SubInvalid, "the file has not been uploaded yet")
	}
	if err != nil {
		return media.Uploaded{}, problem.New(problem.Unavailable, "object storage unavailable").Retry().Wrap(err)
	}
	up := media.Uploaded{Exists: true, Size: size}
	if size != a.Size || size > media.LimitsOf(a.Kind).MaxBytes {
		return up, nil
	}
	rc, _, err := h.Storage.Open(ctx, pkgmedia.BucketQuarantine, a.QuarantineKey)
	if err != nil {
		return media.Uploaded{}, problem.New(problem.Unavailable, "object storage unavailable").Retry().Wrap(err)
	}
	defer func() { _ = rc.Close() }()
	hs := sha256.New()
	if _, err := io.Copy(hs, io.LimitReader(rc, a.Size+1)); err != nil {
		return media.Uploaded{}, problem.New(problem.Unavailable, "object storage unavailable").Retry().Wrap(err)
	}
	up.SHA256 = hex.EncodeToString(hs.Sum(nil))
	return up, nil
}

func (h Media) publish(ctx context.Context, a media.Asset, typ string, payload any) error {
	return h.Events.Publish(ctx, write.Event{Type: typ, SubjectType: string(contentmodel.ResourceMedia), SubjectID: a.ID.String(),
		TenantID: a.Owner.TenantID, Platform: a.Owner.Platform(), Payload: payload})
}

// RejectedPayload is media.rejected v1 (no virus signature, no file name).
type RejectedPayload struct {
	AssetID  uuid.UUID  `json:"asset_id"`
	TenantID *uuid.UUID `json:"tenant_id"`
	Reason   string     `json:"reason"`
}

func (h Media) rejected(ctx context.Context, a media.Asset) error {
	reason := ""
	if a.RejectReason != nil {
		reason = *a.RejectReason
	}
	if h.Metrics != nil {
		h.Metrics.Rejected(reason)
	}
	return h.publish(ctx, a, EventMediaRejected, RejectedPayload{AssetID: a.ID, TenantID: a.Owner.TenantID, Reason: reason})
}

// ProcessedVariant is one entry of media.processed v1.
type ProcessedVariant struct {
	Variant string `json:"variant"`
	Key     string `json:"key"`
	Bytes   int64  `json:"bytes"`
	MIME    string `json:"mime_type"`
}

// ProcessedPayload is media.processed v1.
type ProcessedPayload struct {
	AssetID  uuid.UUID          `json:"asset_id"`
	TenantID *uuid.UUID         `json:"tenant_id"`
	SHA256   string             `json:"sha256"`
	Bucket   string             `json:"bucket"`
	Variants []ProcessedVariant `json:"variants"`
}

// ErrScannerUnavailable is returned by ProcessMedia while clamd cannot be reached: the consumer retries with backoff
// and the asset stays processing (never ready unscanned, SC-002).
var ErrScannerUnavailable = errors.New("command: virus scanner unavailable")

// ProcessMedia is the media worker step of one media.uploaded event (F027-AC-04, AC-05): virus scan, magic bytes
// against the declared type, metadata-free variants written to media/<sha256>/<variant>.<ext> with Cache-Control
// immutable in the bucket of the asset, then ready + media.processed, or rejected + media.rejected. It runs in the
// transaction of the consumer (scope of the event tenant); an asset that is not processing is skipped.
func (h Media) ProcessMedia(ctx context.Context, id uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.ProcessMedia")
	defer func() { done(err) }()
	start := now(h.Clock)
	a, err := h.Store.Asset(ctx, id)
	if err != nil {
		var pe *problem.Error
		if errors.As(err, &pe) && pe.Code == problem.NotFound {
			return nil // soft-deleted or not visible to the event tenant: nothing to do
		}
		return err
	}
	if a.Status != media.StatusProcessing {
		return nil
	}
	reason, variants, res, err := h.run(ctx, a)
	if err != nil {
		return err
	}
	at := now(h.Clock)
	result := "ready"
	if reason != "" {
		result = "rejected"
		if err := a.Reject(reason, at); err != nil {
			return err
		}
	} else if err := a.MarkReady(variants, res.Width, res.Height, res.DurationMS, at); err != nil {
		return err
	}
	if err := h.Store.SaveAsset(ctx, a); err != nil {
		return err
	}
	if h.Metrics != nil {
		h.Metrics.Pipeline(string(a.Kind), result, at.Sub(start))
	}
	if reason != "" {
		return h.rejected(ctx, a)
	}
	pv := make([]ProcessedVariant, 0, len(variants))
	for _, name := range sortedKeys(variants) {
		v := variants[name]
		pv = append(pv, ProcessedVariant{Variant: name, Key: v.Key, Bytes: v.Bytes, MIME: v.MIME})
	}
	return h.publish(ctx, a, EventMediaProcessed, ProcessedPayload{AssetID: a.ID, TenantID: a.Owner.TenantID, SHA256: a.SHA256, Bucket: a.Bucket, Variants: pv})
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// run scans, checks and transcodes a; a non-empty reason rejects it, an error is retried by the consumer.
func (h Media) run(ctx context.Context, a media.Asset) (media.RejectReason, map[string]media.Variant, pkgmedia.Result, error) {
	var res pkgmedia.Result
	rc, size, err := h.Storage.Open(ctx, pkgmedia.BucketQuarantine, a.QuarantineKey)
	if err != nil {
		return "", nil, res, fmt.Errorf("media worker: open quarantine object: %w", err)
	}
	if size > media.LimitsOf(a.Kind).MaxBytes {
		_ = rc.Close()
		return media.RejectTooLarge, nil, res, nil
	}
	data, err := io.ReadAll(io.LimitReader(rc, media.LimitsOf(a.Kind).MaxBytes+1))
	_ = rc.Close()
	if err != nil {
		return "", nil, res, fmt.Errorf("media worker: read quarantine object: %w", err)
	}
	if !h.ScanDisabled {
		sctx, scanned := observability.UseCase(ctx, "media.scan")
		v, err := h.Processor.Scan(sctx, bytes.NewReader(data))
		scanned(err)
		if err != nil {
			slog.WarnContext(ctx, "media scan unavailable, retrying", "asset_id", a.ID, "error", err)
			return "", nil, res, fmt.Errorf("%w: %w", ErrScannerUnavailable, err)
		}
		if !v.Clean {
			return media.RejectVirus, nil, res, nil
		}
	}
	detected := pkgmedia.Sniff(data)
	if !pkgmedia.Compatible(a.MIME, detected) {
		return media.RejectTypeMismatch, nil, res, nil
	}
	ctx, transcoded := observability.UseCase(ctx, "media.transcode")
	defer transcoded(nil)
	switch a.Kind {
	case media.KindImage:
		if a.MIME == "image/svg+xml" {
			clean, err := mathx.SanitizeSVG(data, mathx.ImageProfile, int(media.LimitsOf(a.Kind).MaxBytes))
			if err != nil {
				return media.RejectTranscodeFailed, nil, res, nil
			}
			res.Outputs = []pkgmedia.Output{{Variant: "svg", File: "svg.svg", MIME: "image/svg+xml", Data: clean, Main: true}}
		} else if res, err = h.Processor.Image(data, a.MIME); err != nil {
			return media.RejectTranscodeFailed, nil, res, nil
		}
	case media.KindAudio, media.KindVideo:
		return h.audioVideo(ctx, a, data)
	case media.KindSubtitle:
		// A WebVTT subtitle (magic bytes WEBVTT checked above) is served as variant "vtt", the one video lessons pick
		// (F065), under the immutable media/<sha256>/ key (R31).
		res.Outputs = []pkgmedia.Output{{Variant: "vtt", File: "vtt.vtt", MIME: "text/vtt", Data: data, Main: true}}
	default: // model3d, document: stored as uploaded, with its checksum (CONTENT_MODEL §7)
		res.Outputs = []pkgmedia.Output{{Variant: "original", File: "original" + extOf(a.MIME), MIME: a.MIME, Data: data, Main: true}}
	}
	variants, err := h.upload(ctx, a, res.Outputs)
	return "", variants, res, err
}

// upload writes the outputs (content-addressed, Cache-Control immutable) and returns the variants by name.
func (h Media) upload(ctx context.Context, a media.Asset, outs []pkgmedia.Output) (map[string]media.Variant, error) {
	variants := map[string]media.Variant{}
	for _, o := range outs {
		key := pkgmedia.VariantKey(a.SHA256, o.File)
		if err := h.put(ctx, a.Bucket, key, o); err != nil {
			return nil, err
		}
		if o.Main {
			variants[o.Variant] = media.Variant{Key: key, Bytes: o.Size(), MIME: o.MIME}
		}
	}
	return variants, nil
}

// audioVideo transcodes with ffmpeg in a temporary work directory and uploads the outputs before removing it.
func (h Media) audioVideo(ctx context.Context, a media.Asset, data []byte) (media.RejectReason, map[string]media.Variant, pkgmedia.Result, error) {
	var none pkgmedia.Result
	base := h.WorkDir
	if base == "" {
		base = os.TempDir()
	}
	dir, err := os.MkdirTemp(base, "ew-media-*")
	if err != nil {
		return "", nil, none, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	in := filepath.Join(dir, "input")
	if err := os.WriteFile(in, data, 0o600); err != nil {
		return "", nil, none, err
	}
	out := filepath.Join(dir, "out")
	if err := os.MkdirAll(out, 0o750); err != nil {
		return "", nil, none, err
	}
	res, err := h.Processor.AudioVideo(ctx, a.Kind == media.KindVideo, in, out, media.LimitsOf(a.Kind).MaxDuration)
	switch {
	case errors.Is(err, pkgmedia.ErrTooLong):
		return media.RejectTooLarge, nil, none, nil
	case errors.Is(err, pkgmedia.ErrToolMissing):
		return "", nil, none, err // a worker without ffmpeg retries; it never rejects good files
	case err != nil:
		return media.RejectTranscodeFailed, nil, none, nil
	}
	variants, err := h.upload(ctx, a, res.Outputs)
	return "", variants, res, err
}

// put writes one output from memory or from its temporary file.
func (h Media) put(ctx context.Context, bucket, key string, o pkgmedia.Output) error {
	var body io.Reader = bytes.NewReader(o.Data)
	if o.Path != "" {
		f, err := os.Open(o.Path)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		body = f
	}
	if err := h.Storage.Put(ctx, bucket, key, body, o.Size(), o.MIME, pkgmedia.ImmutableCacheControl); err != nil {
		return fmt.Errorf("media worker: put %s: %w", key, err)
	}
	return nil
}

func extOf(mime string) string {
	switch mime {
	case "model/gltf-binary":
		return ".glb"
	case "application/x-godot-pck":
		return ".pck"
	case "text/csv":
		return ".csv"
	case "application/json":
		return ".json"
	case "application/x-yaml", "application/yaml", "text/yaml":
		return ".yaml"
	case "application/xml", "text/xml":
		return ".xml"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return ".xlsx"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return ".docx"
	case "application/zip":
		return ".zip"
	case "text/vtt":
		return ".vtt"
	}
	return ".bin"
}
