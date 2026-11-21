package featuregate_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/clock"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
)

// gateSpec declares operations with x-portal-action (single key and {param, map}), a shell operation and a
// protected (can_disable = false) action; keys come from specs/portal/menu-catalog.yaml.
const gateSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /lms/assignments:
    get:
      operationId: listAssignments
      x-portal-action: action.school_lms.assignments.view
      responses: { "200": { description: ok } }
  /catalog/schools:
    get:
      operationId: listCatalogSchools
      x-portal-action: action.school_lms.schools.view
      responses: { "200": { description: ok } }
  /me/permissions:
    get:
      operationId: getMyPermissions
      x-portal-shell: true
      responses: { "200": { description: ok } }
  /tenant/users:
    get:
      operationId: listTenantUsers
      x-portal-action: action.school_lms.access_control.view_users
      responses: { "200": { description: ok } }
  /cms/learning/content/{content_type}:
    get:
      operationId: listContent
      parameters:
        - { name: content_type, in: path, required: true, schema: { type: string, enum: [lesson, question] } }
      x-portal-action:
        param: content_type
        map:
          lesson: action.content_studio.lessons.view
          question: action.content_studio.question_bank.view
      responses: { "200": { description: ok } }
  /game/profile:
    get:
      operationId: getGameProfile
      responses: { "200": { description: ok } }
  /lms/unknown:
    get:
      operationId: unknownAction
      x-portal-action: action.school_lms.nope.view
      responses: { "200": { description: ok } }
  /auth/odd:
    post:
      operationId: publicWithAction
      x-portal-action: action.school_lms.assignments.view
      responses: { "200": { description: ok } }
`

var gateOps = []httpx.Operation{
	{ID: "listAssignments", Method: http.MethodGet, Pattern: "/lms/assignments", Tenant: httpx.TenantRequired},
	{ID: "listCatalogSchools", Method: http.MethodGet, Pattern: "/catalog/schools", Tenant: httpx.TenantRequired},
	{ID: "getMyPermissions", Method: http.MethodGet, Pattern: "/me/permissions", Tenant: httpx.TenantOptional},
	{ID: "listTenantUsers", Method: http.MethodGet, Pattern: "/tenant/users", Tenant: httpx.TenantRequired},
	{ID: "listContent", Method: http.MethodGet, Pattern: "/cms/learning/content/{content_type}", Tenant: httpx.TenantOptional},
	{ID: "getGameProfile", Method: http.MethodGet, Pattern: "/game/profile", Tenant: httpx.TenantRequired, Guard: httpx.GuardGamePlayer},
}

func loadSpec(t *testing.T, src string) *openapi3.T {
	t.Helper()
	doc, err := openapi3.NewLoader().LoadFromData([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

// requireAssignmentRead stands for a use case: it calls authz.Require like every handler (R26).
func requireAssignmentRead(w http.ResponseWriter, r *http.Request) {
	p, _ := authz.PrincipalFrom(r.Context())
	if err := authz.Require(r.Context(), p, authz.PermLmsAssignmentRead, authz.Resource{}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

type gateEnv struct {
	idp *httpxtest.IDP
	res *httpxtest.Resolver
	src *fakeSource
	clk *clock.Fake
	srv *httpx.Server
}

func newGateEnv(t *testing.T, withGate bool) *gateEnv {
	t.Helper()
	idp := httpxtest.NewIDP(t)
	clk := clock.NewFake(time.Now().UTC())
	e := &gateEnv{idp: idp, res: &httpxtest.Resolver{}, src: &fakeSource{}, clk: clk}
	d := httpx.Deps{Auth: idp.AuthConfig(), Keys: idp.JWKS(t), Resolver: e.res, Clock: clk, Spec: loadSpec(t, gateSpec),
		Registerer: prometheus.NewRegistry()}
	if withGate {
		d.FeatureGate = newGate(t, e.src, clk)
	}
	srv, err := httpx.NewServer(d)
	if err != nil {
		t.Fatal(err)
	}
	ok := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }
	for _, op := range gateOps {
		h := http.HandlerFunc(ok)
		if op.ID == "listAssignments" {
			h = requireAssignmentRead
		}
		if err := srv.Handle(op, h); err != nil {
			t.Fatal(err)
		}
	}
	e.srv = srv
	return e
}

type who struct {
	azp  string
	data authz.PrincipalData
}

func (e *gateEnv) do(t *testing.T, path string, w *who, tenantHeader *uuid.UUID) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if w != nil {
		sub := "kc-" + uuid.NewString()
		e.res.Set(sub, w.data)
		req.Header.Set("Authorization", "Bearer "+e.idp.Token(sub, w.azp, e.clk.Now()))
	}
	if tenantHeader != nil {
		req.Header.Set(httpx.TenantHeader, tenantHeader.String())
	}
	rec := httptest.NewRecorder()
	e.srv.ServeHTTP(rec, req)
	var body map[string]any
	if rec.Code >= 400 {
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
	}
	return rec.Code, body
}

// F012-AC-40 (TC-CMS-MENU-04) and F012-AC-41 (TC-CMS-MENU-02) through the real pkg/httpx chain.
func TestMiddleware(t *testing.T) {
	ta, tb := uuid.New(), uuid.New()
	readGrant := []authz.Grant{{Role: "school_admin", ScopeKind: authz.RoleScopeTenant, Permissions: []authz.Permission{authz.PermLmsAssignmentRead}}}
	staffA := &who{httpxtest.PortalClient, authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStaff, TenantID: &ta, Grants: readGrant}}
	staffANoPerm := &who{httpxtest.PortalClient, authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStaff, TenantID: &ta}}
	staffB := &who{httpxtest.PortalClient, authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStaff, TenantID: &tb, Grants: readGrant}}
	studentGame := &who{httpxtest.GameClient, authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStudent, TenantID: &ta}}
	staffGame := &who{httpxtest.GameClient, authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStaff, TenantID: &ta, Grants: readGrant}}
	studentPortal := &who{httpxtest.PortalClient, authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStudent, TenantID: &ta}}
	sysAdmin := &who{httpxtest.PortalClient, authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStaff, IsSystemAdmin: true,
		Grants: []authz.Grant{{Role: "system_admin", ScopeKind: authz.RoleScopePlatform, Permissions: []authz.Permission{authz.PermPlatformCrossTenantRead}}}}}

	type state struct {
		tenant  *uuid.UUID
		menus   []string
		actions []string
	}
	assignMenuOffA := []state{{&ta, []string{"menu.school_lms.assignments"}, nil}}
	cases := []struct {
		name    string
		states  []state
		srcErr  bool
		path    string
		who     *who
		header  *uuid.UUID
		status  int
		code    string
		feature string
	}{
		{"action disabled → 403", []state{{&ta, nil, []string{"action.school_lms.assignments.view"}}}, false,
			"/api/v1/lms/assignments", staffA, &ta, 403, "FEATURE_DISABLED", "action.school_lms.assignments.view"},
		{"parent menu disabled → 403", assignMenuOffA, false,
			"/api/v1/lms/assignments", staffA, &ta, 403, "FEATURE_DISABLED", "menu.school_lms.assignments"},
		{"other tenant unaffected", assignMenuOffA, false,
			"/api/v1/lms/assignments", staffB, &tb, 200, "", ""},
		{"enabled → handler runs", nil, false,
			"/api/v1/lms/assignments", staffA, &ta, 200, "", ""},
		{"disabled and missing permission → FEATURE_DISABLED first", assignMenuOffA, false,
			"/api/v1/lms/assignments", staffANoPerm, &ta, 403, "FEATURE_DISABLED", "menu.school_lms.assignments"},
		{"enabled and missing permission → authz.Require still denies", nil, false,
			"/api/v1/lms/assignments", staffANoPerm, &ta, 403, "PERMISSION_DENIED", ""},
		{"unauthenticated → 401 before featuregate", assignMenuOffA, false,
			"/api/v1/lms/assignments", nil, &ta, 401, "UNAUTHENTICATED", ""},
		{"portal_staff denial wins over featuregate", assignMenuOffA, false,
			"/api/v1/lms/assignments", staffGame, &ta, 403, "PORTAL_ACCESS_STAFF_ONLY", ""},
		{"portal_staff denial of a portal-client student wins over featuregate", assignMenuOffA, false,
			"/api/v1/lms/assignments", studentPortal, &ta, 403, "PORTAL_ACCESS_STAFF_ONLY", ""},
		{"tenant mismatch wins over featuregate", assignMenuOffA, false,
			"/api/v1/lms/assignments", staffB, &ta, 403, "TENANT_MISMATCH", ""},
		{"game token on x-portal-action operation untouched", []state{{&ta, []string{"menu.school_lms.schools"}, nil}}, false,
			"/api/v1/catalog/schools", studentGame, &ta, 200, "", ""},
		{"portal token on the same operation blocked", []state{{&ta, []string{"menu.school_lms.schools"}, nil}}, false,
			"/api/v1/catalog/schools", staffA, &ta, 403, "FEATURE_DISABLED", "menu.school_lms.schools"},
		{"game route untouched while source down", nil, true,
			"/api/v1/game/profile", studentGame, &ta, 200, "", ""},
		{"x-portal-shell never blocked (source down)", nil, true,
			"/api/v1/me/permissions", staffA, &ta, 200, "", ""},
		{"can_disable=false never blocked (listed by source)", []state{{&ta, []string{"menu.school_lms.access_control"}, []string{"action.school_lms.access_control.view_users"}}}, false,
			"/api/v1/tenant/users", staffA, &ta, 200, "", ""},
		{"can_disable=false never blocked (source down)", nil, true,
			"/api/v1/tenant/users", staffA, &ta, 200, "", ""},
		{"source down, empty cache → 503", nil, true,
			"/api/v1/lms/assignments", staffA, &ta, 503, "UNAVAILABLE", ""},
		{"System Admin without X-Tenant-Id → platform default", []state{{nil, []string{"menu.content_studio.lessons"}, nil}}, false,
			"/api/v1/cms/learning/content/lesson", sysAdmin, nil, 403, "FEATURE_DISABLED", "menu.content_studio.lessons"},
		{"System Admin with X-Tenant-Id → tenant state", []state{{nil, []string{"menu.content_studio.lessons"}, nil}}, false,
			"/api/v1/cms/learning/content/lesson", sysAdmin, &ta, 200, "", ""},
		{"param map → mapped action of the value", []state{{nil, nil, []string{"action.content_studio.question_bank.view"}}}, false,
			"/api/v1/cms/learning/content/question", sysAdmin, nil, 403, "FEATURE_DISABLED", "action.content_studio.question_bank.view"},
		{"param map → other value unaffected", []state{{nil, nil, []string{"action.content_studio.question_bank.view"}}}, false,
			"/api/v1/cms/learning/content/lesson", sysAdmin, nil, 200, "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newGateEnv(t, true)
			for _, s := range c.states {
				e.src.set(s.tenant, s.menus, s.actions)
			}
			if c.srcErr {
				e.src.fail(errors.New("identity database down"))
			}
			status, body := e.do(t, c.path, c.who, c.header)
			code, _ := body["code"].(string)
			if status != c.status || code != c.code {
				t.Fatalf("status %d code %q, want %d %q (%v)", status, code, c.status, c.code, body)
			}
			if fk, _ := body["feature_key"].(string); fk != c.feature {
				t.Fatalf("feature_key %q, want %q", fk, c.feature)
			}
		})
	}
}

// Without a FeatureGate (nil = off) the chain is unchanged.
func TestMiddlewareOff(t *testing.T) {
	ta := uuid.New()
	e := newGateEnv(t, false)
	w := &who{httpxtest.PortalClient, authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStaff, TenantID: &ta,
		Grants: []authz.Grant{{Role: "r", ScopeKind: authz.RoleScopeTenant, Permissions: []authz.Permission{authz.PermLmsAssignmentRead}}}}}
	if status, body := e.do(t, "/api/v1/lms/assignments", w, &ta); status != 200 {
		t.Fatalf("status %d %v", status, body)
	}
}

// Registration fails fast: a gate needs the spec, actions must exist in the catalog, and public operations
// cannot carry x-portal-action (they have no principal to gate).
func TestRegistration(t *testing.T) {
	idp := httpxtest.NewIDP(t)
	clk := clock.NewFake(time.Now().UTC())
	gate := newGate(t, &fakeSource{}, clk)
	if _, err := httpx.NewServer(httpx.Deps{Auth: idp.AuthConfig(), Keys: idp.JWKS(t), Resolver: &httpxtest.Resolver{}, Clock: clk,
		FeatureGate: gate}); err == nil {
		t.Fatal("gate without spec accepted")
	}
	srv, err := httpx.NewServer(httpx.Deps{Auth: idp.AuthConfig(), Keys: idp.JWKS(t), Resolver: &httpxtest.Resolver{}, Clock: clk,
		FeatureGate: gate, Spec: loadSpec(t, gateSpec)})
	if err != nil {
		t.Fatal(err)
	}
	h := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	if err := srv.Handle(httpx.Operation{ID: "unknownAction", Method: http.MethodGet, Pattern: "/lms/unknown", Tenant: httpx.TenantRequired}, h); err == nil {
		t.Fatal("unknown action accepted")
	}
	if err := srv.Handle(httpx.Operation{ID: "publicWithAction", Method: http.MethodPost, Pattern: "/auth/odd", Public: true, Tenant: httpx.TenantNone}, h); err == nil {
		t.Fatal("public operation with x-portal-action accepted")
	}
}

// Invalidate through the server's gate takes effect on the next request (platform_settings.changed).
func TestMiddlewareInvalidate(t *testing.T) {
	ta := uuid.New()
	idp := httpxtest.NewIDP(t)
	clk := clock.NewFake(time.Now().UTC())
	src := &fakeSource{}
	gate := newGate(t, src, clk)
	res := &httpxtest.Resolver{}
	srv, err := httpx.NewServer(httpx.Deps{Auth: idp.AuthConfig(), Keys: idp.JWKS(t), Resolver: res, Clock: clk, Spec: loadSpec(t, gateSpec), FeatureGate: gate})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Handle(gateOps[0], http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })); err != nil {
		t.Fatal(err)
	}
	res.Set("u1", authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStaff, TenantID: &ta})
	call := func() int {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/lms/assignments", nil)
		req.Header.Set("Authorization", "Bearer "+idp.Token("u1", httpxtest.PortalClient, clk.Now()))
		req.Header.Set(httpx.TenantHeader, ta.String())
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		return rec.Code
	}
	if s := call(); s != 200 {
		t.Fatalf("status %d", s)
	}
	src.set(&ta, []string{"menu.school_lms.assignments"}, nil)
	if s := call(); s != 200 {
		t.Fatalf("cached state expected, got %d", s)
	}
	gate.Invalidate(&ta)
	if s := call(); s != 403 {
		t.Fatalf("after invalidation: %d, want 403", s)
	}
}
