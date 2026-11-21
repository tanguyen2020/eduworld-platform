//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/tenant"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	identityhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

type httpEnv struct {
	*env
	srv     *httptest.Server
	idp     *httpxtest.IDP
	res     *httpxtest.Resolver
	kc      *kcUsers        // realAuthHTTP: the Keycloak test double behind createTenantUser
	revoker *revokeRecorder // realAuthHTTP: TEST DOUBLE of the game session revoker (F016)
}

type httpResp struct {
	status int
	header http.Header
	body   map[string]any
	raw    string
}

func (h *httpEnv) do(t *testing.T, method, path, token string, hdr map[string]string, body any) httpResp {
	t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, h.srv.URL+"/api/v1"+path, rd)
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		ct := "application/json"
		if method == http.MethodPatch && strings.HasPrefix(path, "/admin/") && !strings.HasPrefix(path, "/admin/roles") {
			ct = "application/merge-patch+json" // JSON Merge Patch operations of the spec
		}
		req.Header.Set("Content-Type", ct)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	out := httpResp{status: resp.StatusCode, header: resp.Header, raw: buf.String()}
	_ = json.Unmarshal(buf.Bytes(), &out.body)
	return out
}

func newHTTPEnv(ctx context.Context, t *testing.T, kc *kcUsers) *httpEnv {
	t.Helper()
	e := setup(ctx, t)
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
	sd := command.SchoolDeps{UoW: e.uow, Tenants: write.TenantRepo{}, Schools: write.SchoolRepo{}, Catalog: write.CatalogRepo{},
		Events: e.deps.Events, Audit: e.deps.Audit}
	h := identityhttp.Handlers{
		CreateTenant: command.CreateTenant{TenantDeps: e.deps}, UpdateTenant: command.UpdateTenant{TenantDeps: e.deps},
		ListTenants:       query.ListTenants{Model: read.TenantModel{Session: e.read}, Cursors: codec},
		CreateAdminSchool: command.CreateAdminTenantSchool{SchoolDeps: sd}, UpdateAdminSchool: command.UpdateAdminTenantSchool{SchoolDeps: sd},
		UpdateTenantSchool: command.UpdateTenantSchool{SchoolDeps: sd}, ApproveTenantSchool: command.ApproveTenantSchool{SchoolDeps: sd},
		CreateCommonStudents: command.CreateCommonStudents{UoW: e.uow, Tenants: write.TenantRepo{}, Accounts: write.AccountRepo{}, KC: kc,
			Events: e.deps.Events, Audit: e.deps.Audit, LocalIssuer: "http://localhost:8180/realms/eduworld"},
		Org: query.Org{Model: read.OrgModel{Session: e.read}, Cursors: codec},
		// F011
		Sis:             query.Sis{Issues: read.SisIssueModel{Session: e.read}, Cursors: codec},
		ResolveSisIssue: command.ResolveSisIssue{UoW: e.uow, Issues: write.SisSyncRepo{}, Events: e.deps.Events, Audit: e.deps.Audit},
		CreateSisImport: command.CreateSisImport{UoW: e.uow, Imports: write.SisSyncRepo{}, Storage: &memStorage{objects: map[string][]byte{}}, Audit: e.deps.Audit},
		SisImports:      query.SisImports{Model: read.SisImportModel{Session: e.read}},
		SisAdmin:        query.SisAdmin{Model: read.SisAdminModel{Session: e.read}, Cursors: codec},
		SisConnections:  command.SisConnections{UoW: e.uow, Connections: write.SisConnectionRepo{}, Audit: e.deps.Audit},
	}
	if err := identityhttp.Register(srv, h); err != nil {
		t.Fatal(err)
	}
	hs := httptest.NewServer(srv)
	t.Cleanup(hs.Close)
	return &httpEnv{env: e, srv: hs, idp: idp, res: res}
}

// F010-AC-01..05, AC-07, AC-17, AC-29 through the real HTTP chain (auth, guards, OpenAPI validation,
// idempotency, ETag/If-Match).
func TestIdentityHTTP(t *testing.T) {
	ctx := context.Background()
	kc := newKCUsers()
	h := newHTTPEnv(ctx, t, kc)
	now := time.Now()
	sa := uuid.New()
	saData := authztest.Platform("system_admin").PrincipalData
	saData.UserID = sa
	h.res.Set("kc-sa", saData)
	saTok := h.idp.Token("kc-sa", httpxtest.PortalClient, now) // password only: no MFA since ADR-0094
	saGame := h.idp.Token("kc-sa", httpxtest.GameClient, now)

	key := func() map[string]string { return map[string]string{"Idempotency-Key": ids.New().String()} }
	body := map[string]any{"code": "th-http", "name": "TH HTTP", "tenant_type": "school", "keycloak_org_alias": "th-http"}

	// AC-05: Idempotency-Key required.
	if r := h.do(t, "POST", "/admin/tenants", saTok, nil, body); r.status != 428 {
		t.Fatalf("no key: %d %s", r.status, r.raw)
	}
	// AC-04 (ADR-0094): the platform route needs the portal client and a platform role, not MFA — the same System
	// Admin through the game client is refused.
	if r := h.do(t, "POST", "/admin/tenants", saGame, key(), body); r.status != 403 {
		t.Fatalf("game client: %d %s", r.status, r.raw)
	}
	// AC-05: maxLength → 422 with the field.
	long := map[string]any{"code": strings.Repeat("c", 65), "name": "x", "tenant_type": "unit", "keycloak_org_alias": "long-code"}
	if r := h.do(t, "POST", "/admin/tenants", saTok, key(), long); r.status != 422 || !strings.Contains(r.raw, "code") {
		t.Fatalf("maxLength: %d %s", r.status, r.raw)
	}
	k := key()
	created := h.do(t, "POST", "/admin/tenants", saTok, k, body)
	if created.status != 201 || created.header.Get("ETag") != `W/"1"` {
		t.Fatalf("create: %d %s %v", created.status, created.raw, created.header)
	}
	replay := h.do(t, "POST", "/admin/tenants", saTok, k, body)
	if replay.status != 201 || replay.header.Get("Idempotent-Replayed") != "true" || replay.body["tenant_id"] != created.body["tenant_id"] {
		t.Fatalf("replay: %d %v %s", replay.status, replay.header, replay.raw)
	}
	tid := created.body["tenant_id"].(string)

	// If-Match: missing → 428, stale → 412, current → 200 with the new ETag.
	patch := map[string]any{"status": "disabled", "reason": "test case"}
	if r := h.do(t, "PATCH", "/admin/tenants/"+tid, saTok, key(), patch); r.status != 428 {
		t.Fatalf("no If-Match: %d %s", r.status, r.raw)
	}
	stale := key()
	stale["If-Match"] = `W/"7"`
	if r := h.do(t, "PATCH", "/admin/tenants/"+tid, saTok, stale, patch); r.status != 412 {
		t.Fatalf("stale If-Match: %d %s", r.status, r.raw)
	}
	ok := key()
	ok["If-Match"] = `W/"1"`
	if r := h.do(t, "PATCH", "/admin/tenants/"+tid, saTok, ok, patch); r.status != 200 || r.header.Get("ETag") != `W/"2"` || r.body["status"] != "disabled" {
		t.Fatalf("update: %d %s", r.status, r.raw)
	}

	// AC-04: a tenant role never reaches /admin (403), and AC-17: another tenant's school is 404.
	ta, tb := uuid.New(), uuid.New()
	for _, tt := range []uuid.UUID{ta, tb} {
		if _, err := h.platform.ExecContext(ctx, `INSERT INTO platform.tenants (tenant_id, created_at, updated_at, code, name, tenant_type, keycloak_org_alias, settings)
			VALUES ($1, now(), now(), $2, $2, 'unit', $2, '{}')`, tt, "u-"+tt.String()[:8]); err != nil {
			t.Fatal(err)
		}
	}
	adminData := func(tenant uuid.UUID) authz.PrincipalData {
		return authztest.Staff(tenant, "tenant_admin").PrincipalData
	}
	h.res.Set("kc-ta", adminData(ta))
	h.res.Set("kc-tb", adminData(tb))
	taTok := h.idp.Token("kc-ta", httpxtest.PortalClient, now)
	tbTok := h.idp.Token("kc-tb", httpxtest.PortalClient, now)
	if r := h.do(t, "GET", "/admin/tenants", taTok, nil, nil); r.status != 403 {
		t.Fatalf("tenant admin on /admin: %d %s", r.status, r.raw)
	}
	schoolBody := map[string]any{"code": "th-ta1", "name": map[string]any{"vi": "TH TA1"}, "province_code": "vn.ha-noi",
		"school_level": "primary", "grade_min": 1, "grade_max": 5}
	ck := key()
	ck["X-Tenant-Id"] = ta.String()
	school := h.do(t, "POST", "/admin/tenants/"+ta.String()+"/schools", saTok, ck, schoolBody)
	if school.status != 201 {
		t.Fatalf("create school: %d %s", school.status, school.raw)
	}
	sid := school.body["school_id"].(string)
	if r := h.do(t, "GET", "/tenant/schools/"+sid, taTok, map[string]string{"X-Tenant-Id": ta.String()}, nil); r.status != 200 || r.header.Get("ETag") == "" {
		t.Fatalf("own school: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", "/tenant/schools/"+sid, tbTok, map[string]string{"X-Tenant-Id": tb.String()}, nil); r.status != 404 {
		t.Fatalf("other tenant's school: %d %s", r.status, r.raw)
	}
	// AC-07: a former province is refused with 422.
	former := map[string]any{"code": "th-ta2", "name": map[string]any{"vi": "TH TA2"}, "province_code": "vn.old.ha-giang",
		"school_level": "primary", "grade_min": 1, "grade_max": 5}
	ck2 := key()
	ck2["X-Tenant-Id"] = ta.String()
	if r := h.do(t, "POST", "/admin/tenants/"+ta.String()+"/schools", saTok, ck2, former); r.status != 422 {
		t.Fatalf("former province: %d %s", r.status, r.raw)
	}
	// Catalog for any portal user.
	if r := h.do(t, "GET", "/catalog/provinces?country=VN&status=active", taTok, nil, nil); r.status != 200 || len(r.body["items"].([]any)) != 34 {
		t.Fatalf("provinces: %d", r.status)
	}

	// OQ F057-3: list items carry the version If-Match expects (tenant updated once above → 2).
	for _, path := range []string{"/catalog/countries", "/catalog/provinces?country=VN&status=active"} {
		r := h.do(t, "GET", path, saTok, nil, nil)
		if r.status != 200 {
			t.Fatalf("%s: %d %s", path, r.status, r.raw)
		}
		for _, it := range r.body["items"].([]any) {
			if v, _ := it.(map[string]any)["version"].(float64); v < 1 {
				t.Fatalf("%s: item without version: %v", path, it)
			}
		}
	}
	tenants := h.do(t, "GET", "/admin/tenants?q=th-http", saTok, nil, nil)
	if tenants.status != 200 || len(tenants.body["data"].([]any)) != 1 || tenants.body["data"].([]any)[0].(map[string]any)["version"] != float64(2) {
		t.Fatalf("tenant version: %d %s", tenants.status, tenants.raw)
	}
	if r := h.do(t, "PATCH", "/admin/tenants/"+tid, saTok, map[string]string{"Idempotency-Key": ids.New().String(), "If-Match": "2"},
		map[string]any{"status": "active", "reason": "version from the list"}); r.status != 200 || r.header.Get("ETag") != `W/"3"` {
		t.Fatalf("update with the listed version: %d %s", r.status, r.raw)
	}

	// OQ F057-4: listCatalogSchools — tenant users see their tenant (status filter); a System Admin without
	// X-Tenant-Id needs tenant_ids (CrossTenantScope), never a listing of every tenant.
	inTenant := func(id uuid.UUID) map[string]string { return map[string]string{"X-Tenant-Id": id.String()} }
	schoolsOf := func(r httpResp) int { return len(r.body["items"].([]any)) }
	if r := h.do(t, "GET", "/catalog/schools", taTok, inTenant(ta), nil); r.status != 200 || schoolsOf(r) != 1 {
		t.Fatalf("tenant A schools: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", "/catalog/schools?status=pending_review", taTok, inTenant(ta), nil); r.status != 200 || schoolsOf(r) != 0 {
		t.Fatalf("tenant A pending schools: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", "/catalog/schools?status=closed", taTok, inTenant(ta), nil); r.status != 422 {
		t.Fatalf("unknown status: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", "/catalog/schools?tenant_ids="+ta.String(), tbTok, inTenant(tb), nil); r.status != 200 || schoolsOf(r) != 0 {
		t.Fatalf("tenant B (tenant_ids ignored): %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", "/catalog/schools", saTok, nil, nil); r.status != 403 {
		t.Fatalf("System Admin without tenant_ids: %d %s", r.status, r.raw)
	}
	// ADR-0094: the password-only System Admin token reads across tenants (audited cross_tenant.accessed).
	if r := h.do(t, "GET", "/catalog/schools?status=active&tenant_ids="+ta.String()+","+tb.String(), saTok, nil, nil); r.status != 200 ||
		schoolsOf(r) != 1 || r.body["items"].([]any)[0].(map[string]any)["tenant_id"] != ta.String() {
		t.Fatalf("System Admin cross-tenant: %d %s", r.status, r.raw)
	}

	// AC-29: common students — no-store, password once, replay redacted.
	if _, err := h.platform.ExecContext(ctx, `UPDATE platform.tenants SET keycloak_org_id = $1 WHERE tenant_id = $2 AND deleted_at IS NULL`, uuid.New(), tenant.CommonTenantID); err != nil {
		t.Fatal(err)
	}
	ck3 := key()
	ck3["X-Tenant-Id"] = tenant.CommonTenantID.String()
	students := map[string]any{"students": []map[string]any{{"username": "hs.http", "display_name": "HTTP", "grade_level": 7}}}
	first := h.do(t, "POST", "/admin/common-tenant/students", saTok, ck3, students)
	if first.status != 200 || first.header.Get("Cache-Control") != "no-store" || !strings.Contains(first.raw, "initial_password") {
		t.Fatalf("common students: %d %v %s", first.status, first.header, first.raw)
	}
	again := h.do(t, "POST", "/admin/common-tenant/students", saTok, ck3, students)
	if again.status != 200 || strings.Contains(again.raw, "initial_password") || again.body["password_redacted"] != true {
		t.Fatalf("replay must be redacted: %d %s", again.status, again.raw)
	}
	bad := map[string]any{"students": []map[string]any{{"username": "hs.bad", "display_name": "x", "grade_level": 0}}}
	ck4 := key()
	ck4["X-Tenant-Id"] = tenant.CommonTenantID.String()
	if r := h.do(t, "POST", "/admin/common-tenant/students", saTok, ck4, bad); r.status != 422 {
		t.Fatalf("grade 0 (AC-25): %d %s", r.status, r.raw)
	}
}
