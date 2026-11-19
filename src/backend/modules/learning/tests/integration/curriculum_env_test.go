//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/importer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write"
	learninghttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// memAssets is a TEST DOUBLE of port.AssetReader (media store of F027, not implemented yet): files by asset id in
// memory. Never used by a binary (R17).
type memAssets struct {
	mu    sync.Mutex
	files map[uuid.UUID][]byte
	sizes map[uuid.UUID]int64 // overrides the reported size (csv_import_max test)
}

func (m *memAssets) put(b []byte) uuid.UUID {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.New()
	m.files[id] = b
	return id
}

func (m *memAssets) Open(_ context.Context, id uuid.UUID) (io.ReadCloser, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.files[id]
	if !ok {
		return nil, 0, port.ErrAssetNotFound
	}
	size := int64(len(b))
	if s, ok := m.sizes[id]; ok {
		size = s
	}
	return io.NopCloser(bytes.NewReader(b)), size, nil
}

// curEnv is the HTTP environment of the curriculum tests: the real pkg/httpx chain (auth, tenant, guards, OpenAPI
// validation, idempotency) over the use cases of F021, with demo principals of tenants A and B.
type curEnv struct {
	*env
	srv    *httptest.Server
	idp    *httpxtest.IDP
	res    *httpxtest.Resolver
	cmd    command.Curriculum
	qry    query.Curriculum
	assets *memAssets
	users  map[string]authz.PrincipalData
	tokens map[string]string
}

// Demo principals (quickstart §1): content admin, system admin, content reviewer (platform); academic manager and
// school admin of tenants A and B; a staff member without role; a student of tenant A.
const (
	cmNoidung = "cm.noidung"
	saAdmin   = "sa.admin"
	cmDuyet   = "cm.duyet"
	qlcmA     = "qlcm.truong.a"
	qlcmB     = "qlcm.truong.b"
	adA       = "ad.truong.a"
	adB       = "ad.truong.b"
	noRoleA   = "staff.norole.a"
	hsAn      = "hs.an.3a"
)

func newCurEnv(ctx context.Context, t *testing.T) *curEnv {
	t.Helper()
	e := setup(ctx, t, true)
	authz.Configure(authz.Hooks{Recorder: e.rec})
	t.Cleanup(func() { authz.Configure(authz.Hooks{}) })
	idp := httpxtest.NewIDP(t)
	res := &httpxtest.Resolver{}
	spec, err := httpx.LoadSpec(ctx, filepath.Join(testkit.RepoRoot(t), "specs", "openapi", "eduworld-api.v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	srv, err := httpx.NewServer(httpx.Deps{Auth: idp.AuthConfig(), Keys: idp.JWKS(t), Resolver: res, Spec: spec,
		Idempotency: idempotency.NewStore(e.app, 0, nil)})
	if err != nil {
		t.Fatal(err)
	}
	codec, _ := page.NewCodec([]byte(strings.Repeat("k", 32)))
	reg, err := port.NewRegistry(command.CurriculumResource{Store: write.CurriculumRepo{}, Trash: write.CurriculumTrash{}})
	if err != nil {
		t.Fatal(err)
	}
	assets := &memAssets{files: map[uuid.UUID][]byte{}, sizes: map[uuid.UUID]int64{}}
	cmd := command.Curriculum{UoW: e.uow, Store: write.CurriculumRepo{}, Catalog: write.CatalogRepo{}, Imports: write.ImportJobRepo{},
		Resources: reg, Events: write.Publisher{W: e.writer}, Audit: write.Auditor{R: e.rec}, Assets: assets, Codec: importer.Codec{},
		Metrics: observability.CurriculumCounter{}}
	qry := query.Curriculum{Session: e.read, Model: read.CurriculumModel{}, Settings: read.SchoolSettings{}, Codec: importer.Codec{}, Cursors: codec}
	if err := learninghttp.Register(srv, learninghttp.Handlers{Commands: cmd, Queries: qry}); err != nil {
		t.Fatal(err)
	}
	hs := httptest.NewServer(srv)
	t.Cleanup(hs.Close)
	ce := &curEnv{env: e, srv: hs, idp: idp, res: res, cmd: cmd, qry: qry, assets: assets, users: map[string]authz.PrincipalData{},
		tokens: map[string]string{}}
	school := uuid.New()
	ce.user(cmNoidung, authztest.Platform("content_admin").PrincipalData, true)
	ce.user(saAdmin, authztest.Platform("system_admin").PrincipalData, true)
	ce.user(cmDuyet, authztest.Platform("content_reviewer").PrincipalData, true)
	ce.user(qlcmA, authztest.Staff(e.tenantA, "academic_manager").PrincipalData, true)
	ce.user(qlcmB, authztest.Staff(e.tenantB, "academic_manager").PrincipalData, true)
	ce.user(adA, authztest.Staff(e.tenantA, "school_admin", authztest.AtSchool(school)).PrincipalData, true)
	ce.user(adB, authztest.Staff(e.tenantB, "school_admin").PrincipalData, true)
	ce.user(noRoleA, authztest.NoRole(e.tenantA).PrincipalData, true)
	ce.user(hsAn, authztest.Student(e.tenantA).PrincipalData, false)
	return ce
}

func (c *curEnv) user(sub string, d authz.PrincipalData, portal bool) {
	d.UserID = uuid.New()
	c.res.Set(sub, d)
	c.users[sub] = d
	azp := httpxtest.PortalClient
	if !portal {
		azp = httpxtest.GameClient
	}
	c.tokens[sub] = c.idp.Token(sub, azp, d.TenantID == nil, time.Now())
}

type resp struct {
	status int
	header http.Header
	body   map[string]any
	raw    string
}

func (r resp) str(path ...string) string {
	var cur any = r.body
	for _, p := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[p]
	}
	if s, ok := cur.(string); ok {
		return s
	}
	if cur == nil {
		return ""
	}
	return fmt.Sprint(cur)
}

func (r resp) items(key string) []map[string]any {
	arr, _ := r.body[key].([]any)
	out := make([]map[string]any, 0, len(arr))
	for _, x := range arr {
		if m, ok := x.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func (r resp) errorCodes() []string {
	var out []string
	for _, x := range r.items("errors") {
		out = append(out, fmt.Sprint(x["field"])+":"+fmt.Sprint(x["code"]))
	}
	return out
}

type hdr map[string]string

func idem() hdr { return hdr{"Idempotency-Key": ids.New().String()} }

func ifm(etag string) hdr { return hdr{"If-Match": etag} }

func merge(hs ...hdr) hdr {
	out := hdr{}
	for _, h := range hs {
		for k, v := range h {
			out[k] = v
		}
	}
	return out
}

// do sends a request as user sub (with X-Tenant-Id of its tenant unless h sets it) and decodes the JSON answer.
func (c *curEnv) do(t *testing.T, method, path, sub string, h hdr, body any) resp {
	t.Helper()
	var rd io.Reader = bytes.NewReader(nil)
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.srv.URL+"/api/v1"+path, rd)
	req.Header.Set("Authorization", "Bearer "+c.tokens[sub])
	if body != nil {
		ct := "application/json"
		if method == http.MethodPatch {
			ct = "application/merge-patch+json"
		}
		req.Header.Set("Content-Type", ct)
	}
	if d := c.users[sub]; d.TenantID != nil {
		req.Header.Set("X-Tenant-Id", d.TenantID.String())
	}
	for k, v := range h {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	b, _ := io.ReadAll(res.Body)
	out := resp{status: res.StatusCode, header: res.Header, raw: string(b)}
	_ = json.Unmarshal(b, &out.body)
	return out
}

// must fails unless the answer has status want.
func must(t *testing.T, r resp, want int, what string) resp {
	t.Helper()
	if r.status != want {
		t.Fatalf("%s: status %d, want %d: %s", what, r.status, want, r.raw)
	}
	return r
}

// subject creates (or reuses) subject code with grades 1–9 as the content admin and returns its id.
func (c *curEnv) subject(t *testing.T, code string) string {
	t.Helper()
	r := c.do(t, http.MethodPost, "/cms/learning/subjects", cmNoidung, idem(), map[string]any{"code": code,
		"name": map[string]string{"vi": "Môn " + code}, "grade_levels": []int{1, 2, 3, 4, 5, 6, 7, 8, 9}})
	return must(t, r, http.StatusCreated, "create subject "+code).str("subject_id")
}

// curriculum creates a curriculum as sub and returns (curriculum_id, version 1 id).
func (c *curEnv) curriculum(t *testing.T, sub, code string) (string, string) {
	t.Helper()
	r := must(t, c.do(t, http.MethodPost, "/cms/learning/curricula", sub, idem(), map[string]any{"code": code,
		"name": map[string]string{"vi": "Chương trình " + code}}), http.StatusCreated, "create curriculum "+code)
	return r.str("curriculum_id"), r.str("latest_version", "curriculum_version_id")
}

func (c *curEnv) topic(t *testing.T, sub, version, subject, code string, grade int, parent string) string {
	t.Helper()
	body := map[string]any{"curriculum_version_id": version, "code": code, "name": "Chủ đề " + code, "subject_id": subject, "grade_level": grade}
	if parent != "" {
		body["parent_topic_id"] = parent
	}
	return must(t, c.do(t, http.MethodPost, "/cms/learning/topics", sub, idem(), body), http.StatusCreated, "create topic "+code).str("topic_id")
}

func (c *curEnv) objective(t *testing.T, sub, topic, code string, extra map[string]any) resp {
	t.Helper()
	body := map[string]any{"topic_id": topic, "code": code, "description": "Học sinh có thể " + code}
	for k, v := range extra {
		body[k] = v
	}
	return c.do(t, http.MethodPost, "/cms/learning/objectives", sub, idem(), body)
}

// assign assigns a platform curriculum to tenant (as learning_platform, like the facade of F020).
func (c *curEnv) assign(ctx context.Context, t *testing.T, curriculumID string, tenant uuid.UUID) {
	t.Helper()
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO infra.tenant_resource_assignments
		(assignment_id, tenant_id, created_at, updated_at, resource_type, resource_id, assigned_by, assigned_at)
		VALUES ($1, $2, now(), now(), 'curriculum', $3, $1, now())`, ids.New(), tenant, curriculumID); err != nil {
		t.Fatal(err)
	}
}

// versionETag reads the ETag of a version.
func (c *curEnv) versionETag(t *testing.T, sub, curriculumID string, version int) string {
	t.Helper()
	r := must(t, c.do(t, http.MethodGet, fmt.Sprintf("/cms/learning/curricula/%s/versions/%d", curriculumID, version), sub, nil, nil),
		http.StatusOK, "get version")
	return r.header.Get("ETag")
}

// transition applies action to a version as sub.
func (c *curEnv) transition(t *testing.T, sub, curriculumID string, version int, action, note string) resp {
	t.Helper()
	tag := c.versionETag(t, saAdmin, curriculumID, version)
	if d := c.users[sub]; d.TenantID != nil {
		tag = c.versionETag(t, sub, curriculumID, version)
	}
	body := map[string]any{"action": action}
	if note != "" {
		body["note"] = note
	}
	return c.do(t, http.MethodPost, fmt.Sprintf("/cms/learning/curricula/%s/versions/%d:transition", curriculumID, version), sub,
		merge(idem(), ifm(tag)), body)
}

// publishFlow submits (author), approves (System Admin) and publishes (System Admin) a version.
func (c *curEnv) publishFlow(t *testing.T, author, curriculumID string, version int) resp {
	t.Helper()
	must(t, c.transition(t, author, curriculumID, version, "submit_review", ""), http.StatusOK, "submit_review")
	approver := saAdmin
	if c.users[author].TenantID != nil {
		approver = author + ".reviewer"
		if _, ok := c.users[approver]; !ok {
			c.user(approver, authztest.Staff(*c.users[author].TenantID, "academic_manager").PrincipalData, true)
		}
	}
	must(t, c.transition(t, approver, curriculumID, version, "approve", ""), http.StatusOK, "approve")
	tag := c.versionETag(t, approver, curriculumID, version)
	return c.do(t, http.MethodPost, fmt.Sprintf("/cms/learning/curricula/%s/versions/%d:publish", curriculumID, version), approver,
		merge(idem(), ifm(tag)), nil)
}
