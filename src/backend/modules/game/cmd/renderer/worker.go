package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/render"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/media"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// Limits of one job. maxSourceBytes mirrors the model3d limit of media_max_bytes_by_kind (CONFIG_DEFAULTS, owner
// CONTENT_MODEL §7; enforced at upload by domain/gameconfig): a larger source is not a valid game asset.
const (
	maxSourceBytes = 50 << 20
	maxOutputBytes = 64 << 20
	sourceMIME     = "model/gltf-binary"
	outputMIME     = "image/png"
)

// Storage is the part of the object storage the renderer uses (storage.S3 of module game).
type Storage interface {
	Open(ctx context.Context, bucket, key string) (io.ReadCloser, int64, error)
	Put(ctx context.Context, bucket, key string, body io.Reader, size int64, mime, cacheControl string) error
}

// RenderRequest is one run of render_thumbnails.gd: Input is the downloaded model, OutputDir receives
// thumbnail.png or turntable.png.
type RenderRequest struct {
	Input     string
	OutputDir string
	Kind      string
	Size      int
	Frames    int
}

// Runner runs the Godot script (GodotRunner in production).
type Runner interface {
	Render(ctx context.Context, req RenderRequest) error
}

// Publisher answers a job on SubjectResult.
type Publisher interface {
	PublishResult(ctx context.Context, res render.Result) error
}

// Processor turns one Job into one Result: download and verify the source, render, check and upload the PNG. It never
// returns an error: every failure becomes Result{Status: failed} (the worker of module game owns retries and the DLQ).
type Processor struct {
	Storage Storage
	Runner  Runner
	WorkDir string        // parent of the per-job temporary directories ("" = os.TempDir())
	Timeout time.Duration // Godot run of one job
	Log     *slog.Logger
}

// jobError is a failure reported to the worker: a stable code plus a short detail.
type jobError struct {
	code   string
	detail string
}

func (e *jobError) Error() string {
	if e.detail == "" {
		return e.code
	}
	return e.code + ": " + e.detail
}

func fail(code, format string, args ...any) error {
	return &jobError{code: code, detail: fmt.Sprintf(format, args...)}
}

var sha256Re = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Validate checks a job against the contract of the work queue (render.Size, 1 or render.TurntableFrames frames).
func Validate(job render.Job) error {
	if job.RenderJobID == uuid.Nil {
		return fail("invalid_job", "render_job_id is required")
	}
	switch job.Kind {
	case render.KindThumbnail:
		if job.Frames != 1 {
			return fail("invalid_job", "%s takes 1 frame, got %d", job.Kind, job.Frames)
		}
	case render.KindTurntable:
		if job.Frames != render.TurntableFrames {
			return fail("invalid_job", "%s takes %d frames, got %d", job.Kind, render.TurntableFrames, job.Frames)
		}
	default:
		return fail("invalid_job", "unknown kind %q", truncate(job.Kind, 64))
	}
	if job.Size != render.Size {
		return fail("invalid_job", "size must be %d, got %d", render.Size, job.Size)
	}
	if strings.TrimSpace(job.Source.Bucket) == "" || strings.TrimSpace(job.Source.Key) == "" {
		return fail("invalid_job", "source bucket and key are required")
	}
	if !sha256Re.MatchString(job.Source.SHA256) {
		return fail("invalid_job", "source sha256 must be 64 lowercase hex characters")
	}
	if job.Source.MIME != "" && job.Source.MIME != sourceMIME {
		return fail("unsupported_source", "source type %q is not a glTF binary", truncate(job.Source.MIME, 64))
	}
	return nil
}

// ExpectedSize is the pixel size of the PNG of a valid job: size×size, or a sheet of render.TurntableColumns columns.
func ExpectedSize(job render.Job) (int, int) {
	if job.Kind != render.KindTurntable {
		return job.Size, job.Size
	}
	cols := min(job.Frames, render.TurntableColumns)
	rows := (job.Frames + cols - 1) / cols
	return cols * job.Size, rows * job.Size
}

// OutputFile is the file written by render_thumbnails.gd for a kind.
func OutputFile(kind string) string {
	if kind == render.KindTurntable {
		return "turntable.png"
	}
	return "thumbnail.png"
}

// Process runs one job. The result carries the job id and attempt in every case.
func (p *Processor) Process(ctx context.Context, job render.Job) render.Result {
	res := render.Result{RenderJobID: job.RenderJobID, Attempt: job.Attempt}
	out, err := p.process(ctx, job)
	if err != nil {
		res.Status = render.StatusFailed
		res.Error = p.sanitize(err)
		return res
	}
	res.Status, res.Output = render.StatusDone, out
	return res
}

func (p *Processor) process(ctx context.Context, job render.Job) (*render.Output, error) {
	if err := Validate(job); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(p.WorkDir, "render-*")
	if err != nil {
		return nil, fail("workdir", "%v", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	input := filepath.Join(dir, "source.glb")
	if err := p.download(ctx, job.Source, input); err != nil {
		return nil, err
	}
	outDir := filepath.Join(dir, "out")
	if err := os.Mkdir(outDir, 0o700); err != nil {
		return nil, fail("workdir", "%v", err)
	}
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	rctx, cancel := context.WithTimeout(ctx, timeout)
	err = p.Runner.Render(rctx, RenderRequest{Input: input, OutputDir: outDir, Kind: job.Kind, Size: job.Size, Frames: job.Frames})
	timedOut := ctx.Err() == nil && errors.Is(rctx.Err(), context.DeadlineExceeded)
	cancel()
	switch {
	case timedOut:
		return nil, fail("render_timeout", "godot did not finish within %s", timeout)
	case err != nil:
		return nil, fail("render_failed", "%v", err)
	}

	png, err := readBounded(filepath.Join(outDir, OutputFile(job.Kind)), maxOutputBytes)
	if err != nil {
		return nil, fail("render_output", "%v", err)
	}
	w, h, err := pngSize(png)
	if err != nil {
		return nil, fail("render_output", "%v", err)
	}
	if ew, eh := ExpectedSize(job); w != ew || h != eh {
		return nil, fail("render_output", "png is %dx%d, expected %dx%d", w, h, ew, eh)
	}
	sum := sha256.Sum256(png)
	sha := hex.EncodeToString(sum[:])
	key := render.OutputKey(sha, job.Kind)
	if err := p.Storage.Put(ctx, render.OutputBucket, key, bytes.NewReader(png), int64(len(png)), outputMIME,
		media.ImmutableCacheControl); err != nil {
		return nil, fail("upload_failed", "%v", err)
	}
	return &render.Output{Bucket: render.OutputBucket, Key: key, MIME: outputMIME, SHA256: sha, SizeBytes: int64(len(png)),
		Width: w, Height: h, Frames: job.Frames}, nil
}

// download copies the source to path and checks its SHA-256 against the job (the asset of the published version).
func (p *Processor) download(ctx context.Context, src render.Source, path string) error {
	body, _, err := p.Storage.Open(ctx, src.Bucket, src.Key)
	if err != nil {
		return fail("source_unavailable", "%v", err)
	}
	defer func() { _ = body.Close() }()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fail("workdir", "%v", err)
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(body, maxSourceBytes+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	switch {
	case err != nil:
		return fail("source_unavailable", "download: %v", err)
	case n > maxSourceBytes:
		return fail("source_too_large", "source is larger than %d bytes", maxSourceBytes)
	case n == 0:
		return fail("source_empty", "source object is empty")
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != src.SHA256 {
		return fail("source_sha256_mismatch", "downloaded %s, expected %s", got, src.SHA256)
	}
	return nil
}

func readBounded(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("godot wrote no %s", filepath.Base(path))
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("png is larger than %d bytes", limit)
	}
	return b, nil
}

func pngSize(b []byte) (int, int, error) {
	cfg, err := png.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return 0, 0, fmt.Errorf("not a png: %w", err)
	}
	return cfg.Width, cfg.Height, nil
}

// sanitize turns an error into the short message of a failed Result: the per-job directory is hidden, credentials,
// tokens and addresses are redacted, control characters dropped, the length bounded by render.MaxErrorLen.
func (p *Processor) sanitize(err error) string {
	msg := err.Error()
	var je *jobError
	if !errors.As(err, &je) {
		msg = "internal: " + msg
	}
	for _, dir := range []string{p.WorkDir, os.TempDir()} {
		if dir != "" {
			msg = strings.ReplaceAll(msg, filepath.Clean(dir), "<workdir>")
		}
	}
	msg = observability.RedactString(msg)
	msg = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, msg)
	return truncate(strings.Join(strings.Fields(msg), " "), render.MaxErrorLen)
}

// truncate bounds s to n bytes on a rune boundary.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

// Message is the part of a JetStream message the loop uses (jetstream.Msg satisfies it).
type Message interface {
	Data() []byte
	Ack() error
	Nak() error
	NakWithDelay(delay time.Duration) error
	Term() error
}

// Worker handles the messages of the durable consumer game_renderer one at a time (a Godot run is CPU/GPU heavy; scale
// with more containers).
type Worker struct {
	Processor *Processor
	Publisher Publisher
	Log       *slog.Logger
	Metrics   *Metrics
	// RepublishDelay is the redelivery delay of a job whose result could not be published.
	RepublishDelay time.Duration
}

// Handle processes one message: undecodable → Term (no job id to answer); otherwise one Result is published, then the
// job is acked. A job interrupted by shutdown is Nak'ed for another instance; a result that cannot be published is
// Nak'ed with a delay (the render is redone; the upload is content-addressed, so idempotent).
func (w *Worker) Handle(ctx context.Context, m Message) {
	var job render.Job
	if err := json.Unmarshal(m.Data(), &job); err != nil || job.RenderJobID == uuid.Nil {
		w.Log.WarnContext(ctx, "render job undecodable, terminated", "bytes", len(m.Data()))
		_ = m.Term()
		return
	}
	start := time.Now()
	res := w.Processor.Process(ctx, job)
	if ctx.Err() != nil {
		w.Log.InfoContext(ctx, "render job interrupted by shutdown, released", "render_job_id", job.RenderJobID)
		_ = m.Nak()
		return
	}
	w.Metrics.observe(job.Kind, res.Status, time.Since(start))
	attrs := []any{"render_job_id", job.RenderJobID, "attempt", job.Attempt, "kind", job.Kind, "status", res.Status,
		"duration_ms", time.Since(start).Milliseconds()}
	if res.Status == render.StatusFailed {
		w.Log.WarnContext(ctx, "render job failed", append(attrs, "error", res.Error)...)
	} else {
		w.Log.InfoContext(ctx, "render job done", append(attrs, "key", res.Output.Key, "size_bytes", res.Output.SizeBytes)...)
	}
	if err := w.Publisher.PublishResult(ctx, res); err != nil {
		w.Log.ErrorContext(ctx, "render result publish failed, job redelivered", "render_job_id", job.RenderJobID,
			"error", err)
		_ = m.NakWithDelay(w.RepublishDelay)
		return
	}
	if err := m.Ack(); err != nil {
		w.Log.WarnContext(ctx, "render job ack failed", "render_job_id", job.RenderJobID, "error", err)
	}
}
