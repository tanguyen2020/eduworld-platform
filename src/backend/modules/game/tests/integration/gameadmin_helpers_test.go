//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/gameadmin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/modules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/questsql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/read"
	writerepo "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
)

// memStorage is a TEST-ONLY in-memory object storage (R17): presigned URLs are fake; objects live in a map.
type memStorage struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func newMemStorage() *memStorage { return &memStorage{objects: map[string][]byte{}} }

func (m *memStorage) PresignPut(_ context.Context, bucket, key string, size int64, mime string, ttl time.Duration) (string, map[string]string, error) {
	return "https://s3.test/" + bucket + "/" + key + "?ttl=" + ttl.String(), map[string]string{"Content-Type": mime}, nil
}

func (m *memStorage) PresignGet(_ context.Context, bucket, key string, ttl time.Duration) (string, error) {
	return "https://s3.test/" + bucket + "/" + key + "?get&ttl=" + ttl.String(), nil
}

func (m *memStorage) Head(_ context.Context, bucket, key string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.objects[bucket+"/"+key]
	if !ok {
		return 0, port.ErrObjectNotFound
	}
	return int64(len(b)), nil
}

func (m *memStorage) Open(_ context.Context, bucket, key string) (io.ReadCloser, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.objects[bucket+"/"+key]
	if !ok {
		return nil, 0, port.ErrObjectNotFound
	}
	return io.NopCloser(bytes.NewReader(b)), int64(len(b)), nil
}

func (m *memStorage) Put(_ context.Context, bucket, key string, body io.Reader, _ int64, _, _ string) error {
	b, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[bucket+"/"+key] = b
	return nil
}

func (m *memStorage) put(bucket, key string, b []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[bucket+"/"+key] = b
}

func (m *memStorage) has(bucket, key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.objects[bucket+"/"+key]
	return ok
}

// memRenders is a TEST-ONLY render queue (R17): it keeps the enqueued jobs; the test plays the renderer.
type memRenders struct {
	mu   sync.Mutex
	jobs []port.RenderRequest
}

func (q *memRenders) Enqueue(_ context.Context, r port.RenderRequest) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.jobs = append(q.jobs, r)
	return nil
}

func (q *memRenders) all() []port.RenderRequest {
	q.mu.Lock()
	defer q.mu.Unlock()
	return append([]port.RenderRequest(nil), q.jobs...)
}

func (e *env) adminPolicy() command.GameAdminPolicy {
	return command.GameAdminPolicy{MinLead: 5 * time.Minute, RequiredLanguages: []string{"vi", "en"}, AssetsPerVersionMax: 50, PreviewTTL: 30 * time.Minute,
		ScheduleBatch: 50}
}

// learningFacade answers checkPublishedLearningContent from e.released (TEST-ONLY double of module learning).
func (e *env) learningFacade() modules.LearningAttempts {
	e.learning.f.CheckPublishedLearningContentFunc = func(_ context.Context, req learningapi.CheckPublishedLearningContentRequestObject) (
		learningapi.CheckPublishedLearningContentResponseObject, error) {
		out := learningapi.CheckPublishedLearningContent200JSONResponse{}
		for _, r := range req.Body.Refs {
			v := struct {
				ContentId   uuid.UUID                                                `json:"content_id"`
				ContentType learningapi.LearningContentCheckResultResultsContentType `json:"content_type"`
				Published   bool                                                     `json:"published"`
			}{ContentId: r.ContentId, ContentType: learningapi.LearningContentCheckResultResultsContentType(r.ContentType)}
			v.Published = e.released == nil || e.released[r.ContentId.String()]
			out.Results = append(out.Results, v)
		}
		return out, nil
	}
	return modules.LearningAttempts{F: e.learning.f}
}

func (e *env) adminUC() command.GameAdmin {
	k := e.publisher()
	return command.GameAdmin{UoW: e.uow, Store: writerepo.GameConfigRepo{}, Kernel: &k, Learning: e.learningFacade(),
		Events: writerepo.Publisher{W: e.writer}, Audit: writerepo.Auditor{R: e.rec}, Policy: e.adminPolicy(), Now: e.clock.Now,
		Translations: writerepo.TranslationRepo{}}
}

func (e *env) adminQueries() query.GameAdmin {
	codec, _ := page.NewCodec([]byte(strings.Repeat("c", 32)))
	return query.GameAdmin{Read: e.read, Data: read.GameConfigs{}, Cursors: codec, URLs: cdn{}, Now: e.clock.Now,
		Simulation: query.SimulationPolicy{MinuteRate: 14, DailyCap: 1000}}
}

// cdn is the CDN URL builder of the tests.
type cdn struct{}

func (cdn) URL(bucket, key string) string { return "https://cdn.test/" + bucket + "/" + key }

func (e *env) mediaUC() command.GameMedia {
	if e.storage == nil {
		e.storage = newMemStorage()
	}
	if e.renders == nil {
		e.renders = &memRenders{}
	}
	return command.GameMedia{UoW: e.uow, Store: writerepo.GameMediaRepo{}, Storage: e.storage, Queue: e.renders,
		Events: writerepo.Publisher{W: e.writer}, Audit: writerepo.Auditor{R: e.rec}, RenderBackoff: time.Minute, RenderMaxAttempts: 3, Now: e.clock.Now}
}

func (e *env) assignUC() command.GameAssignments {
	codec, _ := page.NewCodec([]byte(strings.Repeat("c", 32)))
	m := e.gameManifest()
	m.Quests = questsql.Content{} // the manifest lists the quests assigned to the tenant (as module game wires it)
	return command.GameAssignments{UoW: e.uow, Store: writerepo.TenantAssignmentRepo{}, Manifests: m,
		Audit: writerepo.Auditor{R: e.rec}, Cursors: codec, Now: e.clock.Now}
}

type adminProxy struct{ e *env }

func (p adminProxy) Create(ctx context.Context, t string, in command.Draft) (gameadmin.ConfigVersionView, error) {
	return p.e.adminUC().Create(ctx, t, in)
}
func (p adminProxy) CreateVersion(ctx context.Context, t string, id uuid.UUID, b *int, title, note *string) (gameadmin.ConfigVersionView, error) {
	return p.e.adminUC().CreateVersion(ctx, t, id, b, title, note)
}
func (p adminProxy) UpdateDraft(ctx context.Context, t string, id uuid.UUID, n int, m string, in command.Draft) (gameadmin.ConfigVersionView, error) {
	return p.e.adminUC().UpdateDraft(ctx, t, id, n, m, in)
}
func (p adminProxy) Transition(ctx context.Context, t string, id uuid.UUID, n int, m, a, note string) (gameadmin.ConfigVersionView, error) {
	return p.e.adminUC().Transition(ctx, t, id, n, m, a, note)
}
func (p adminProxy) Publish(ctx context.Context, t string, id uuid.UUID, n int, m string, at *time.Time, note string) (gameadmin.ReleaseView, error) {
	return p.e.adminUC().Publish(ctx, t, id, n, m, at, note)
}
func (p adminProxy) CancelSchedule(ctx context.Context, t string, id, rid uuid.UUID, reason string) (gameadmin.ReleaseView, error) {
	return p.e.adminUC().CancelSchedule(ctx, t, id, rid, reason)
}
func (p adminProxy) Rollback(ctx context.Context, t string, id uuid.UUID, to int, reason string) (gameadmin.ReleaseView, error) {
	return p.e.adminUC().Rollback(ctx, t, id, to, reason)
}
func (p adminProxy) Unpublish(ctx context.Context, t string, id uuid.UUID, reason string) (gameadmin.ReleaseView, error) {
	return p.e.adminUC().Unpublish(ctx, t, id, reason)
}
func (p adminProxy) CreatePreview(ctx context.Context, t string, id uuid.UUID, n int) (command.PreviewToken, error) {
	return p.e.adminUC().CreatePreview(ctx, t, id, n)
}

type adminQueryProxy struct{ e *env }

func (p adminQueryProxy) List(ctx context.Context, t string, in query.ConfigListInput) (query.ConfigPage, error) {
	return p.e.adminQueries().List(ctx, t, in)
}
func (p adminQueryProxy) Get(ctx context.Context, t string, id uuid.UUID, n int) (gameadmin.ConfigVersionView, error) {
	return p.e.adminQueries().Get(ctx, t, id, n)
}
func (p adminQueryProxy) Versions(ctx context.Context, t string, id uuid.UUID, cursor string, limit int) (query.VersionHistory, error) {
	return p.e.adminQueries().Versions(ctx, t, id, cursor, limit)
}
func (p adminQueryProxy) Diff(ctx context.Context, t string, id uuid.UUID, n, with int) (query.VersionDiff, error) {
	return p.e.adminQueries().Diff(ctx, t, id, n, with)
}
func (p adminQueryProxy) Releases(ctx context.Context, t string, id uuid.UUID, cursor string, limit int) (query.ReleasePage, error) {
	return p.e.adminQueries().Releases(ctx, t, id, cursor, limit)
}
func (p adminQueryProxy) Preview(ctx context.Context, tok, t string, id uuid.UUID, n int) (query.PreviewSnapshot, error) {
	return p.e.adminQueries().Preview(ctx, tok, t, id, n)
}
func (p adminQueryProxy) MediaAsset(ctx context.Context, id uuid.UUID) (query.MediaView, error) {
	return p.e.adminQueries().MediaAsset(ctx, id)
}

func (p adminQueryProxy) ClientManifest(ctx context.Context) (query.ClientManifestView, error) {
	return p.e.adminQueries().ClientManifest(ctx)
}
func (p adminQueryProxy) Simulate(ctx context.Context, t string, id uuid.UUID, n int, in query.SimulationInput) (query.SimulationView, error) {
	return p.e.adminQueries().Simulate(ctx, t, id, n, in)
}
func (p adminQueryProxy) ConfigCounts(ctx context.Context) (query.ConfigCountsView, error) {
	return p.e.adminQueries().ConfigCounts(ctx)
}

type mediaProxy struct{ e *env }

func (p mediaProxy) Presign(ctx context.Context, in command.PresignInput) (command.PresignResult, error) {
	return p.e.mediaUC().Presign(ctx, in)
}

// adminCall calls a console route with a portal token; If-Match/Idempotency-Key/headers as given.
func (e *env) adminCall(t *testing.T, method, path, token, ifMatch string, body any) resp {
	t.Helper()
	h := map[string]string{}
	if method != http.MethodGet {
		h["Idempotency-Key"] = newKey()
	}
	if ifMatch != "" {
		h["If-Match"] = ifMatch
	}
	if method == http.MethodPatch {
		h["Content-Type"] = "application/merge-patch+json"
	}
	return e.do(t, method, path, token, h, body)
}

// questDraft is the authored payload of a learning quest for the console (server fields are filled by the server).
func questDraft(t *testing.T, code string, edit func(m map[string]any)) map[string]any {
	t.Helper()
	it := learningQuest(uuid.NewString(), code, edit)
	var m map[string]any
	if err := json.Unmarshal(it.Payload, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func sha(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// glb is a minimal glTF binary header (magic "glTF", version 2) followed by padding (TEST-ONLY bytes).
func glb(n int) []byte {
	b := append([]byte("glTF\x02\x00\x00\x00"), make([]byte, n)...)
	return b
}
