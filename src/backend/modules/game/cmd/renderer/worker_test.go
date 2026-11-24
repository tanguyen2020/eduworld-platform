package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/render"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/media"
)

// TEST-ONLY: in-memory object storage.
type fakeStorage struct {
	mu      sync.Mutex
	objects map[string][]byte
	puts    []fakePut
	putErr  error
}

// TEST-ONLY: one recorded Put.
type fakePut struct {
	bucket, key, mime, cacheControl string
	size                            int64
	body                            []byte
}

func (s *fakeStorage) Open(_ context.Context, bucket, key string) (io.ReadCloser, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.objects[bucket+"/"+key]
	if !ok {
		return nil, 0, errors.New("object not found")
	}
	return io.NopCloser(bytes.NewReader(b)), int64(len(b)), nil
}

func (s *fakeStorage) Put(_ context.Context, bucket, key string, body io.Reader, size int64, mime, cacheControl string) error {
	b, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.putErr != nil {
		return s.putErr
	}
	s.puts = append(s.puts, fakePut{bucket: bucket, key: key, mime: mime, cacheControl: cacheControl, size: size, body: b})
	return nil
}

// TEST-ONLY: stands in for Godot — checks the downloaded model and writes a PNG of the configured size.
type fakeRunner struct {
	t             *testing.T
	wantSource    []byte
	width, height int
	err           error
	block         bool // wait for the context (timeout / shutdown)
	calls         []RenderRequest
}

func (r *fakeRunner) Render(ctx context.Context, req RenderRequest) error {
	r.calls = append(r.calls, req)
	if r.block {
		<-ctx.Done()
		return ctx.Err()
	}
	if r.err != nil {
		return r.err
	}
	got, err := os.ReadFile(req.Input)
	if err != nil {
		r.t.Fatalf("runner: read input: %v", err)
	}
	if !bytes.Equal(got, r.wantSource) {
		r.t.Fatalf("runner: input is not the downloaded source")
	}
	if r.width == 0 {
		return nil // writes nothing
	}
	return os.WriteFile(filepath.Join(req.OutputDir, OutputFile(req.Kind)), pngBytes(r.t, r.width, r.height), 0o600)
}

// TEST-ONLY: records published results.
type fakePublisher struct {
	results []render.Result
	err     error
}

func (p *fakePublisher) PublishResult(_ context.Context, res render.Result) error {
	if p.err != nil {
		return p.err
	}
	p.results = append(p.results, res)
	return nil
}

// TEST-ONLY: a JetStream message.
type fakeMsg struct {
	data                         []byte
	acked, naked, termed, nakDly bool
}

func (m *fakeMsg) Data() []byte                     { return m.data }
func (m *fakeMsg) Ack() error                       { m.acked = true; return nil }
func (m *fakeMsg) Nak() error                       { m.naked = true; return nil }
func (m *fakeMsg) NakWithDelay(time.Duration) error { m.nakDly = true; return nil }
func (m *fakeMsg) Term() error                      { m.termed = true; return nil }

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sha(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

var source = []byte("glTF\x02\x00\x00\x00 fake binary model of a published item")

func newJob(kind string) render.Job {
	frames := 1
	if kind == render.KindTurntable {
		frames = render.TurntableFrames
	}
	return render.Job{RenderJobID: uuid.New(), Kind: kind, Size: render.Size, Frames: frames, Attempt: 2,
		Source: render.Source{AssetID: uuid.New(), Bucket: render.OutputBucket, Key: "media/" + sha(source) + "/original.glb",
			MIME: "model/gltf-binary", SHA256: sha(source)}}
}

type harness struct {
	storage *fakeStorage
	runner  *fakeRunner
	pub     *fakePublisher
	worker  *Worker
	workdir string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	st := &fakeStorage{objects: map[string][]byte{}}
	st.objects[render.OutputBucket+"/media/"+sha(source)+"/original.glb"] = source
	rn := &fakeRunner{t: t, wantSource: source}
	pub := &fakePublisher{}
	dir := t.TempDir()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &harness{storage: st, runner: rn, pub: pub, workdir: dir, worker: &Worker{
		Processor: &Processor{Storage: st, Runner: rn, WorkDir: dir, Timeout: 5 * time.Second, Log: log},
		Publisher: pub, Log: log, Metrics: NewMetrics(nil), RepublishDelay: time.Second,
	}}
}

func (h *harness) handle(t *testing.T, job render.Job) (*fakeMsg, render.Result) {
	t.Helper()
	b, err := json.Marshal(job)
	if err != nil {
		t.Fatal(err)
	}
	m := &fakeMsg{data: b}
	h.worker.Handle(context.Background(), m)
	if len(h.pub.results) != 1 {
		t.Fatalf("published %d results, want 1", len(h.pub.results))
	}
	if !m.acked || m.naked || m.nakDly || m.termed {
		t.Fatalf("message state %+v, want acked only", m)
	}
	res := h.pub.results[0]
	if res.RenderJobID != job.RenderJobID || res.Attempt != job.Attempt {
		t.Fatalf("result is for %s/%d, want %s/%d", res.RenderJobID, res.Attempt, job.RenderJobID, job.Attempt)
	}
	entries, err := os.ReadDir(h.workdir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("per-job directory not removed: %v %v", entries, err)
	}
	return m, res
}

func (h *harness) wantFailed(t *testing.T, res render.Result, code string) {
	t.Helper()
	if res.Status != render.StatusFailed || res.Output != nil {
		t.Fatalf("status %q output %+v, want failed without output", res.Status, res.Output)
	}
	if !strings.HasPrefix(res.Error, code) {
		t.Fatalf("error %q, want prefix %q", res.Error, code)
	}
	if len(res.Error) > render.MaxErrorLen {
		t.Fatalf("error longer than %d", render.MaxErrorLen)
	}
	if len(h.storage.puts) != 0 {
		t.Fatalf("uploaded %d objects on failure", len(h.storage.puts))
	}
}

func TestThumbnailSuccess(t *testing.T) {
	h := newHarness(t)
	h.runner.width, h.runner.height = 512, 512
	job := newJob(render.KindThumbnail)
	_, res := h.handle(t, job)

	if res.Status != render.StatusDone || res.Error != "" || res.Output == nil {
		t.Fatalf("result %+v, want done", res)
	}
	if len(h.storage.puts) != 1 {
		t.Fatalf("puts = %d, want 1", len(h.storage.puts))
	}
	put := h.storage.puts[0]
	wantKey := "media/" + sha(put.body) + "/render_thumbnail.png"
	if put.bucket != "ew-media-public" || put.key != wantKey || put.mime != "image/png" ||
		put.cacheControl != media.ImmutableCacheControl || put.size != int64(len(put.body)) {
		t.Fatalf("put %s/%s %s %q size %d", put.bucket, put.key, put.mime, put.cacheControl, put.size)
	}
	want := render.Output{Bucket: "ew-media-public", Key: wantKey, MIME: "image/png", SHA256: sha(put.body),
		SizeBytes: int64(len(put.body)), Width: 512, Height: 512, Frames: 1}
	if *res.Output != want {
		t.Fatalf("output %+v, want %+v", *res.Output, want)
	}
	if len(h.runner.calls) != 1 {
		t.Fatalf("runner calls = %d", len(h.runner.calls))
	}
	if c := h.runner.calls[0]; c.Kind != render.KindThumbnail || c.Size != 512 || c.Frames != 1 ||
		filepath.Base(c.Input) != "source.glb" {
		t.Fatalf("render request %+v", c)
	}
}

func TestTurntableSuccess(t *testing.T) {
	h := newHarness(t)
	h.runner.width, h.runner.height = 3072, 2048
	job := newJob(render.KindTurntable)
	_, res := h.handle(t, job)

	if res.Status != render.StatusDone || res.Output == nil {
		t.Fatalf("result %+v, want done", res)
	}
	o := res.Output
	if o.Width != 3072 || o.Height != 2048 || o.Frames != 24 || o.Key != render.OutputKey(o.SHA256, render.KindTurntable) {
		t.Fatalf("output %+v", o)
	}
	if c := h.runner.calls[0]; c.Kind != render.KindTurntable || c.Frames != 24 || c.Size != 512 {
		t.Fatalf("render request %+v", c)
	}
}

func TestSourceSHA256MismatchFails(t *testing.T) {
	h := newHarness(t)
	h.runner.width, h.runner.height = 512, 512
	job := newJob(render.KindThumbnail)
	job.Source.SHA256 = sha([]byte("another version of the asset"))
	_, res := h.handle(t, job)
	h.wantFailed(t, res, "source_sha256_mismatch")
	if len(h.runner.calls) != 0 {
		t.Fatal("godot ran on an unverified source")
	}
}

func TestSourceMissingFails(t *testing.T) {
	h := newHarness(t)
	job := newJob(render.KindThumbnail)
	job.Source.Key = "media/missing/original.glb"
	_, res := h.handle(t, job)
	h.wantFailed(t, res, "source_unavailable")
}

func TestGodotErrorFails(t *testing.T) {
	h := newHarness(t)
	h.runner.err = errors.New("exit status 4: render_thumbnails: cannot load source.glb (File corrupt)")
	_, res := h.handle(t, newJob(render.KindTurntable))
	h.wantFailed(t, res, "render_failed")
	if !strings.Contains(res.Error, "cannot load source.glb") {
		t.Fatalf("error %q does not carry the godot reason", res.Error)
	}
}

func TestGodotTimeoutFails(t *testing.T) {
	h := newHarness(t)
	h.runner.block = true
	h.worker.Processor.Timeout = 20 * time.Millisecond
	_, res := h.handle(t, newJob(render.KindThumbnail))
	h.wantFailed(t, res, "render_timeout")
}

func TestWrongOutputSizeFails(t *testing.T) {
	for name, tc := range map[string]struct {
		kind string
		w, h int
	}{
		"thumbnail 256":           {render.KindThumbnail, 256, 256},
		"thumbnail as a sheet":    {render.KindThumbnail, 3072, 2048},
		"turntable single frame":  {render.KindTurntable, 512, 512},
		"turntable 4 by 6 sheet ": {render.KindTurntable, 2048, 3072},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			h.runner.width, h.runner.height = tc.w, tc.h
			_, res := h.handle(t, newJob(tc.kind))
			h.wantFailed(t, res, "render_output")
		})
	}
}

func TestMissingOrInvalidOutputFails(t *testing.T) {
	h := newHarness(t)
	_, res := h.handle(t, newJob(render.KindThumbnail)) // runner writes nothing
	h.wantFailed(t, res, "render_output")

	h = newHarness(t)
	h.worker.Processor.Runner = garbageRunner{}
	_, res = h.handle(t, newJob(render.KindThumbnail))
	h.wantFailed(t, res, "render_output")
}

// TEST-ONLY: writes a file that is not a PNG.
type garbageRunner struct{}

func (garbageRunner) Render(_ context.Context, req RenderRequest) error {
	return os.WriteFile(filepath.Join(req.OutputDir, OutputFile(req.Kind)), []byte("not a png"), 0o600)
}

func TestUploadErrorFails(t *testing.T) {
	h := newHarness(t)
	h.runner.width, h.runner.height = 512, 512
	h.storage.putErr = errors.New("s3: status 503")
	_, res := h.handle(t, newJob(render.KindThumbnail))
	h.wantFailed(t, res, "upload_failed")
}

func TestInvalidJobFails(t *testing.T) {
	for name, mutate := range map[string]func(j *render.Job){
		"unknown kind":          func(j *render.Job) { j.Kind = "render_preview" },
		"size 256":              func(j *render.Job) { j.Size = 256 },
		"thumbnail 24 frames":   func(j *render.Job) { j.Frames = 24 },
		"turntable 1 frame":     func(j *render.Job) { j.Kind = render.KindTurntable },
		"no bucket":             func(j *render.Job) { j.Source.Bucket = "" },
		"no key":                func(j *render.Job) { j.Source.Key = " " },
		"sha256 not hex":        func(j *render.Job) { j.Source.SHA256 = strings.ToUpper(j.Source.SHA256) },
		"sha256 missing":        func(j *render.Job) { j.Source.SHA256 = "" },
		"godot pck, not a glTF": func(j *render.Job) { j.Source.MIME = "application/x-godot-pck" },
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			h.runner.width, h.runner.height = 512, 512
			job := newJob(render.KindThumbnail)
			mutate(&job)
			_, res := h.handle(t, job)
			code := "invalid_job"
			if job.Source.MIME == "application/x-godot-pck" {
				code = "unsupported_source"
			}
			h.wantFailed(t, res, code)
			if len(h.runner.calls) != 0 {
				t.Fatal("godot ran for an invalid job")
			}
		})
	}
}

func TestUndecodableMessageIsTerminated(t *testing.T) {
	for name, data := range map[string][]byte{
		"not json":    []byte("{"),
		"no job id":   []byte(`{"kind":"render_thumbnail","size":512,"frames":1}`),
		"wrong types": []byte(`{"render_job_id":42}`),
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			m := &fakeMsg{data: data}
			h.worker.Handle(context.Background(), m)
			if !m.termed || m.acked || len(h.pub.results) != 0 {
				t.Fatalf("message %+v, results %d: want terminated without result", m, len(h.pub.results))
			}
		})
	}
}

func TestResultNotPublishedIsRedelivered(t *testing.T) {
	h := newHarness(t)
	h.runner.width, h.runner.height = 512, 512
	h.pub.err = errors.New("nats: timeout")
	b, _ := json.Marshal(newJob(render.KindThumbnail))
	m := &fakeMsg{data: b}
	h.worker.Handle(context.Background(), m)
	if !m.nakDly || m.acked {
		t.Fatalf("message %+v, want nak with delay", m)
	}
}

func TestShutdownReleasesTheJob(t *testing.T) {
	h := newHarness(t)
	h.runner.block = true
	ctx, cancel := context.WithCancel(context.Background())
	b, _ := json.Marshal(newJob(render.KindTurntable))
	m := &fakeMsg{data: b}
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()
	h.worker.Handle(ctx, m)
	if !m.naked || m.acked || len(h.pub.results) != 0 {
		t.Fatalf("message %+v, results %d: want nak without result", m, len(h.pub.results))
	}
}

func TestSanitizeHidesWorkdirAndBoundsLength(t *testing.T) {
	p := &Processor{WorkDir: filepath.Join(os.TempDir(), "renderer")}
	err := fail("render_failed", "exit status 1: %s\x07 %s", filepath.Join(p.WorkDir, "render-1", "source.glb"),
		strings.Repeat("é", render.MaxErrorLen))
	msg := p.sanitize(err)
	if strings.Contains(msg, p.WorkDir) || !strings.Contains(msg, "<workdir>") {
		t.Fatalf("workdir not hidden: %q", msg[:80])
	}
	if strings.ContainsRune(msg, '\x07') || len(msg) > render.MaxErrorLen {
		t.Fatalf("control character or length %d", len(msg))
	}
	if got := p.sanitize(errors.New("boom")); got != "internal: boom" {
		t.Fatalf("plain error %q", got)
	}
}

func TestExpectedSize(t *testing.T) {
	if w, h := ExpectedSize(newJob(render.KindThumbnail)); w != 512 || h != 512 {
		t.Fatalf("thumbnail %dx%d", w, h)
	}
	if w, h := ExpectedSize(newJob(render.KindTurntable)); w != 3072 || h != 2048 {
		t.Fatalf("turntable %dx%d", w, h)
	}
}
