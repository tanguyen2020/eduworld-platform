package command

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // image dimensions of uploaded game media
	_ "image/png"
	"io"
	"time"

	"github.com/google/uuid"
	_ "golang.org/x/image/webp"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/gameadmin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gameconfig"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	pkgmedia "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/media"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Events of the game media pipeline (owner F078).
const (
	EventMediaUploaded  = "media.uploaded"
	EventMediaProcessed = "media.processed"
	EventMediaRejected  = "media.rejected"
)

// RenderMetrics observes eduworld_game_render_job_total{result} and eduworld_game_render_duration_seconds; nil = none.
type RenderMetrics interface {
	RenderJob(result string)
	RenderDuration(d time.Duration)
}

// GameMedia are the use cases of game media and renders (F078 US4, US5): the presigned upload, the pipeline job that
// checks an upload and publishes it content-addressed (quarantined → processing → ready | rejected), the render jobs of
// a published version with a 3D asset and the record of their results.
type GameMedia struct {
	UoW     write.UnitOfWork
	Store   write.GameMediaStore
	Storage port.ObjectStorage
	Scanner port.MediaScanner // nil = virus scan disabled (local only)
	Queue   port.RenderQueue
	Events  write.EventPublisher
	Audit   write.Auditor
	Metrics RenderMetrics
	// RenderBackoff is the base of the retry backoff; RenderMaxAttempts the attempts before the dead letter.
	RenderBackoff     time.Duration
	RenderMaxAttempts int
	Batch             int
	Now               func() time.Time
}

func (h GameMedia) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func (h GameMedia) batch() int {
	if h.Batch <= 0 {
		return 20
	}
	return h.Batch
}

// PresignInput is MediaPresignRequest (owner_scope/tenant_id are ignored: game media is platform content).
type PresignInput struct {
	Kind     string
	Filename string
	MIME     string
	Size     int64
	SHA256   string
	License  string
	AltText  string
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

// Presign is presignGameMediaUpload (F078-AC-18): a quarantined asset and a presigned PUT to ew-media-quarantine valid
// media_presign_ttl, limited to the type and size of the kind (415 MEDIA_TYPE_NOT_ALLOWED, 413 MEDIA_TOO_LARGE); the
// same file again returns the existing asset as duplicate_of.
func (h GameMedia) Presign(ctx context.Context, in PresignInput) (out PresignResult, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.PresignGameMediaUpload")
	defer func() { done(err) }()
	p, sc, err := gameadmin.Principal(ctx)
	if err != nil {
		return out, err
	}
	if err := gameadmin.RequireAnyWrite(ctx, p); err != nil {
		return out, err
	}
	u := gameconfig.Upload{Kind: gameconfig.MediaKind(in.Kind), MIME: in.MIME, Size: in.Size, SHA256: in.SHA256, Filename: in.Filename, License: in.License}
	switch err := u.Validate(); {
	case errors.Is(err, gameconfig.ErrMediaKind), errors.Is(err, gameconfig.ErrMediaType):
		return out, problem.New(problem.MediaTypeNotAllowed, "this file type is not allowed for game media of this kind")
	case errors.Is(err, gameconfig.ErrMediaTooLarge):
		return out, problem.New(problem.MediaTooLarge, "the file is larger than the limit of its kind")
	case err != nil:
		return out, problem.Invalid("sha256", problem.SubInvalid, "filename, license and a sha256 hex digest are required")
	}
	if h.Storage == nil {
		return out, problem.New(problem.Unavailable, "object storage is not configured").Retry()
	}
	now := h.now()
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		dup, err := h.Store.AssetBySHA(ctx, in.SHA256)
		if err != nil {
			return err
		}
		if dup != nil {
			id := dup.ID
			out = PresignResult{AssetID: id, Status: dup.Status, DuplicateOf: &id}
			return idempotency.SetResult(ctx, out)
		}
		id := ids.New()
		qk := pkgmedia.QuarantineKey(id.String())
		exp := now.Add(gameconfig.PresignTTL)
		by := p.UserID
		a := write.MediaAsset{ID: id, Kind: in.Kind, MIME: in.MIME, Filename: in.Filename, Size: in.Size, SHA256: in.SHA256,
			Bucket: pkgmedia.BucketQuarantine, QuarantineKey: &qk, Status: string(gameconfig.MediaQuarantined), License: &in.License,
			UploadedBy: &by, UploadExpiresAt: &exp}
		if in.AltText != "" {
			a.AltText = &in.AltText
		}
		if err := h.Store.InsertAsset(ctx, a, now); err != nil {
			if errors.Is(err, write.ErrConflict) {
				return problem.New(problem.Conflict, "the same file is being registered").Retry()
			}
			return err
		}
		url, headers, err := h.Storage.PresignPut(ctx, pkgmedia.BucketQuarantine, qk, in.Size, in.MIME, gameconfig.PresignTTL)
		if err != nil {
			return problem.New(problem.Unavailable, "object storage unavailable").Retry().Wrap(err)
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: "media.upload_requested", TargetType: "game_media", TargetID: id.String(),
			Result: audit.ResultSuccess, Metadata: map[string]any{"kind": in.Kind, "size_bytes": in.Size}}); err != nil {
			return err
		}
		out = PresignResult{AssetID: id, Status: a.Status, UploadURL: url, Headers: headers, ExpiresAt: &exp}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// ProcessUploads is the job game-media-pipeline (F078-AC-18, ADR-0044): each quarantined asset whose file arrived is
// checked (size, sha256, magic bytes against the declared type, virus scan when configured) and copied to
// ew-media-public under media/<sha256>/original.<ext> (ready, media.processed) or rejected (media.rejected); an upload
// missing after media_presign_ttl is rejected upload_missing. Platform job scope.
func (h GameMedia) ProcessUploads(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.ProcessGameMedia")
	defer func() { done(err) }()
	if h.Storage == nil {
		return 0, nil
	}
	sc, err := authz.SystemPlatformScope("game-media-pipeline")
	if err != nil {
		return 0, err
	}
	ctx = tenancy.WithScope(ctx, sc)
	now := h.now()
	var pending []write.MediaAsset
	if err := h.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		pending, err = h.Store.Pending(ctx, now, h.batch())
		return err
	}); err != nil {
		return 0, err
	}
	for _, a := range pending {
		changed, err := h.processOne(ctx, a, now)
		if err != nil {
			return n, err
		}
		if changed {
			n++
		}
	}
	return n, nil
}

func (h GameMedia) processOne(ctx context.Context, a write.MediaAsset, now time.Time) (bool, error) {
	if a.QuarantineKey == nil {
		return false, nil
	}
	size, err := h.Storage.Head(ctx, pkgmedia.BucketQuarantine, *a.QuarantineKey)
	if errors.Is(err, port.ErrObjectNotFound) {
		if a.UploadExpiresAt != nil && now.After(*a.UploadExpiresAt) {
			return true, h.finish(ctx, a.ID, nil, gameconfig.RejectUploadMissing, now)
		}
		return false, nil // not uploaded yet
	}
	if err != nil {
		return false, err
	}
	if a.Status == string(gameconfig.MediaQuarantined) {
		if err := h.UoW.Do(ctx, func(ctx context.Context) error {
			cur, err := h.Store.Asset(ctx, a.ID)
			if err != nil || cur == nil || cur.Status != string(gameconfig.MediaQuarantined) {
				return err
			}
			cur.Status = string(gameconfig.MediaProcessing)
			if err := h.Store.SaveAsset(ctx, *cur, now); err != nil {
				return err
			}
			return h.Events.Publish(ctx, write.Event{Type: EventMediaUploaded, SubjectType: "media", SubjectID: a.ID.String(), Platform: true,
				OccurredAt: now, Payload: map[string]any{"asset_id": a.ID, "tenant_id": nil, "kind": a.Kind, "mime_type": a.MIME,
					"quarantine_key": *a.QuarantineKey, "size_bytes": a.Size, "sha256": a.SHA256}})
		}); err != nil {
			return false, err
		}
	}
	if size != a.Size {
		return true, h.finish(ctx, a.ID, nil, gameconfig.RejectChecksumMismatch, now)
	}
	rc, _, err := h.Storage.Open(ctx, pkgmedia.BucketQuarantine, *a.QuarantineKey)
	if err != nil {
		return false, err
	}
	data, err := io.ReadAll(io.LimitReader(rc, a.Size+1))
	_ = rc.Close()
	if err != nil {
		return false, err
	}
	sum := sha256.Sum256(data)
	if int64(len(data)) != a.Size || hex.EncodeToString(sum[:]) != a.SHA256 {
		return true, h.finish(ctx, a.ID, nil, gameconfig.RejectChecksumMismatch, now)
	}
	if !pkgmedia.Compatible(a.MIME, pkgmedia.Sniff(data)) {
		return true, h.finish(ctx, a.ID, nil, gameconfig.RejectTypeMismatch, now)
	}
	if h.Scanner != nil {
		clean, err := h.Scanner.Scan(ctx, bytes.NewReader(data))
		if err != nil {
			return false, fmt.Errorf("virus scan: %w", err) // transient: the next run retries
		}
		if !clean {
			return true, h.finish(ctx, a.ID, nil, gameconfig.RejectVirus, now)
		}
	}
	key := gameconfig.OriginalKey(a.SHA256, gameconfig.MediaKind(a.Kind), a.MIME)
	if err := h.Storage.Put(ctx, pkgmedia.BucketPublic, key, bytes.NewReader(data), int64(len(data)), a.MIME, pkgmedia.ImmutableCacheControl); err != nil {
		return false, err
	}
	ready := a
	ready.Variants = []map[string]any{{"variant": "original", "key": key, "mime_type": a.MIME, "bytes": len(data)}}
	if a.Kind == string(gameconfig.MediaImage) {
		if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
			ready.Width, ready.Height = &cfg.Width, &cfg.Height
		}
	}
	return true, h.finish(ctx, a.ID, &ready, "", now)
}

// finish moves an asset to ready (ready != nil) or rejected (reason) with its event.
func (h GameMedia) finish(ctx context.Context, id uuid.UUID, ready *write.MediaAsset, reason string, now time.Time) error {
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		cur, err := h.Store.Asset(ctx, id)
		if err != nil || cur == nil || (cur.Status != string(gameconfig.MediaQuarantined) && cur.Status != string(gameconfig.MediaProcessing)) {
			return err
		}
		if ready == nil {
			cur.Status, cur.RejectReason = string(gameconfig.MediaRejected), &reason
			if err := h.Store.SaveAsset(ctx, *cur, now); err != nil {
				return err
			}
			return h.Events.Publish(ctx, write.Event{Type: EventMediaRejected, SubjectType: "media", SubjectID: id.String(), Platform: true,
				OccurredAt: now, Payload: map[string]any{"asset_id": id, "tenant_id": nil, "reason": reason}})
		}
		cur.Status, cur.Bucket, cur.Variants, cur.Width, cur.Height = string(gameconfig.MediaReady), pkgmedia.BucketPublic, ready.Variants,
			ready.Width, ready.Height
		if err := h.Store.SaveAsset(ctx, *cur, now); err != nil {
			return err
		}
		return h.Events.Publish(ctx, write.Event{Type: EventMediaProcessed, SubjectType: "media", SubjectID: id.String(), Platform: true,
			OccurredAt: now, Payload: map[string]any{"asset_id": id, "tenant_id": nil, "sha256": cur.SHA256, "bucket": pkgmedia.BucketPublic,
				"variants": cur.Variants}})
	})
}

// ScheduleRenders is the consumer game.render_thumbnails (F078-AC-19): a published version of a type with 3D content
// referencing ready model3d assets gets one render job per (asset, kind) — thumbnail 512×512 and turntable 24 frames —
// created once (unique key) and enqueued on the work queue. A publish never waits for, nor fails because of, a render.
func (h GameMedia) ScheduleRenders(ctx context.Context, itemType string, versionID uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "game.command.ScheduleRenders")
	defer func() { done(err) }()
	t, ok := gameconfig.ByItemType(itemType)
	if !ok || !t.Has3D || h.Queue == nil {
		return nil
	}
	now := h.now()
	var jobs []port.RenderRequest
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		payload, _, _, err := h.Store.VersionPayload(ctx, versionID)
		if err != nil || payload == nil {
			return err
		}
		var doc map[string]any
		if err := json.Unmarshal(payload, &doc); err != nil {
			return fmt.Errorf("version payload: %w", err)
		}
		var refs []uuid.UUID
		for _, a := range gameconfig.CollectRefs(doc).Assets {
			if id, err := uuid.Parse(a); err == nil {
				refs = append(refs, id)
			}
		}
		models, err := h.Store.Content3D(ctx, refs)
		if err != nil {
			return err
		}
		for _, m := range models {
			key := gameconfig.OriginalKey(m.SHA256, gameconfig.MediaModel3D, m.MIME)
			for _, kind := range []gameconfig.MediaKind{gameconfig.MediaRenderThumbnail, gameconfig.MediaRenderTurntable} {
				j := write.RenderJob{ID: ids.New(), ConfigType: t.Name, ItemType: t.ItemType, VersionID: versionID, SourceAssetID: m.ID,
					Kind: string(kind), Status: string(gameconfig.RenderQueued)}
				created, err := h.Store.InsertRenderJob(ctx, j, now)
				if err != nil {
					return err
				}
				if !created {
					continue // replayed event: the job exists (F078-AC-19, no duplicate)
				}
				frames := 1
				if kind == gameconfig.MediaRenderTurntable {
					frames = gameconfig.RenderFrames
				}
				jobs = append(jobs, port.RenderRequest{RenderJobID: j.ID, Kind: string(kind), Frames: frames, Attempt: 1, SourceID: m.ID,
					Bucket: pkgmedia.BucketPublic, Key: key, MIME: m.MIME, SHA256: m.SHA256})
			}
		}
		for _, j := range jobs {
			if err := h.Queue.Enqueue(ctx, j); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

// RecordRenderResult is the consumer game.render_results (F078-AC-19, AC-20): a done render registers its PNG as a ready
// game media asset (kind render_thumbnail/render_turntable, reused when the same file exists) linked to the version; a
// failed render is retried with backoff up to RenderMaxAttempts, then dead-lettered (infra.dead_letters, metric). The
// published version is never changed nor rolled back. A result of a finished job is ignored.
func (h GameMedia) RecordRenderResult(ctx context.Context, r port.RenderOutcome) (err error) {
	ctx, done := observability.UseCase(ctx, "game.command.RecordRenderResult")
	defer func() { done(err) }()
	sc, err := authz.SystemPlatformScope("game-render-results")
	if err != nil {
		return err
	}
	now := h.now()
	result := ""
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		j, err := h.Store.RenderJob(ctx, r.RenderJobID)
		if err != nil || j == nil || (j.Status != string(gameconfig.RenderQueued) && j.Status != string(gameconfig.RenderFailed)) {
			return err
		}
		j.Attempts++
		if !r.Done || r.SHA256 == "" || r.Key != gameconfig.RenderKey(r.SHA256, gameconfig.MediaKind(j.Kind)) {
			msg := r.Error
			if msg == "" {
				msg = "invalid render output"
			}
			if len(msg) > 1000 {
				msg = msg[:1000]
			}
			st, next := gameconfig.RenderFailure(j.Attempts, h.maxAttempts(), h.backoff(), now)
			j.Status, j.LastError, j.NextAttemptAt = string(st), &msg, next
			if err := h.Store.SaveRenderJob(ctx, *j, now); err != nil {
				return err
			}
			result = "failed"
			if st == gameconfig.RenderDead {
				result = "dead"
				return h.Store.DeadLetter(ctx, *j, now)
			}
			return nil
		}
		asset, err := h.Store.AssetBySHA(ctx, r.SHA256)
		if err != nil {
			return err
		}
		if asset == nil {
			w, ht := r.Width, r.Height
			a := write.MediaAsset{ID: ids.New(), Kind: j.Kind, MIME: "image/png", Filename: j.Kind + ".png", Size: r.SizeBytes, SHA256: r.SHA256,
				Bucket: pkgmedia.BucketPublic, Status: string(gameconfig.MediaReady), Width: &w, Height: &ht,
				Variants: []map[string]any{{"variant": "png", "key": r.Key, "mime_type": "image/png", "bytes": r.SizeBytes}}}
			if err := h.Store.InsertAsset(ctx, a, now); err != nil {
				return err
			}
			asset = &a
			if err := h.Events.Publish(ctx, write.Event{Type: EventMediaProcessed, SubjectType: "media", SubjectID: a.ID.String(), Platform: true,
				OccurredAt: now, Payload: map[string]any{"asset_id": a.ID, "tenant_id": nil, "sha256": a.SHA256, "bucket": a.Bucket,
					"variants": a.Variants}}); err != nil {
				return err
			}
		}
		if err := h.Store.LinkRender(ctx, asset.ID, *j, now); err != nil {
			return err
		}
		id := asset.ID
		j.Status, j.ResultAssetID, j.LastError, j.NextAttemptAt = string(gameconfig.RenderDone), &id, nil, nil
		result = "done"
		return h.Store.SaveRenderJob(ctx, *j, now)
	})
	if err == nil && result != "" && h.Metrics != nil {
		h.Metrics.RenderJob(result)
	}
	return err
}

// RetryRenders is the job render-retry: failed render jobs whose backoff elapsed are enqueued again.
func (h GameMedia) RetryRenders(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.RetryRenders")
	defer func() { done(err) }()
	if h.Queue == nil {
		return 0, nil
	}
	sc, err := authz.SystemPlatformScope("render-retry")
	if err != nil {
		return 0, err
	}
	now := h.now()
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		due, err := h.Store.DueRenderRetries(ctx, now, h.batch())
		if err != nil {
			return err
		}
		for _, j := range due {
			src, err := h.Store.Asset(ctx, j.SourceAssetID)
			if err != nil {
				return err
			}
			if src == nil {
				continue
			}
			frames := 1
			if j.Kind == string(gameconfig.MediaRenderTurntable) {
				frames = gameconfig.RenderFrames
			}
			if err := h.Queue.Enqueue(ctx, port.RenderRequest{RenderJobID: j.ID, Kind: j.Kind, Frames: frames, Attempt: j.Attempts + 1,
				SourceID: src.ID, Bucket: pkgmedia.BucketPublic, Key: gameconfig.OriginalKey(src.SHA256, gameconfig.MediaModel3D, src.MIME),
				MIME: src.MIME, SHA256: src.SHA256}); err != nil {
				return err
			}
			j.Status, j.NextAttemptAt = string(gameconfig.RenderQueued), nil
			if err := h.Store.SaveRenderJob(ctx, j, now); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return n, err
}

func (h GameMedia) maxAttempts() int {
	if h.RenderMaxAttempts <= 0 {
		return gameconfig.RenderMaxAttempts
	}
	return h.RenderMaxAttempts
}

func (h GameMedia) backoff() time.Duration {
	if h.RenderBackoff <= 0 {
		return time.Minute
	}
	return h.RenderBackoff
}
