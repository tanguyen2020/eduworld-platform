//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/scope"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/tests/integration/fixtures"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// listFixtureOp is a fixture portal list route (TEST FIXTURE — R17): F020 has no public content endpoint, so
// the real pkg/httpx chain (X-Tenant-Id, principal, guards) is exercised on the fixture read model with the
// scope helper every content list of F021+ uses.
var listFixtureOp = httpx.Operation{ID: "listFixtureContents", Module: "learning", Method: http.MethodGet,
	Pattern: "/cms/fixture-contents", Tenant: httpx.TenantOptional}

func fixtureListHandler(e *env) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		p, _ := authz.PrincipalFrom(ctx)
		var requested []uuid.UUID
		if raw := r.URL.Query().Get("tenant_ids"); raw != "" {
			for _, s := range strings.Split(raw, ",") {
				requested = append(requested, uuid.MustParse(s))
			}
		}
		var sc tenancy.Scope
		if err := e.read.Read(ctx, func(ctx context.Context) error {
			var err error
			sc, err = scope.ForRead(ctx, p, requested, read.TenantDirectory{})
			return err
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		var rows []fixtures.Row
		if err := e.read.InTenantRead(ctx, sc, func(ctx context.Context) error {
			var err error
			rows, err = fixtures.List(ctx)
			return err
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": rows})
	})
}

type listResult struct {
	status int
	code   string
	rows   []fixtures.Row
}

func get(t *testing.T, srv *httptest.Server, path, token string, tenant *uuid.UUID) listResult {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, srv.URL+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if tenant != nil {
		req.Header.Set(httpx.TenantHeader, tenant.String())
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body struct {
		Code string         `json:"code"`
		Data []fixtures.Row `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return listResult{status: resp.StatusCode, code: body.Code, rows: body.Data}
}

// F020-AC-11 (TC-TEN-011, TC-CMS-TEN-01): tenant_ids of a tenant user is ignored; X-Tenant-Id of another
// tenant → 403 TENANT_MISMATCH + audit. F020-AC-13 (TC-TEN-018, TC-CMS-TEN-03): System Admin lists with
// tenant_ids (empty = every active tenant) through CrossTenantScope, tenant_id on each row, audit
// cross_tenant.accessed {reason, tenant_ids, mode=read}.
func TestScopeOfContentLists(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, true)
	authz.Configure(authz.Hooks{Recorder: e.rec})
	t.Cleanup(func() { authz.Configure(authz.Hooks{}) })
	w := newHybridWorld(ctx, t, e)

	idp := httpxtest.NewIDP(t)
	res := &httpxtest.Resolver{}
	staffA := authztest.Staff(e.tenantA, "school_admin")
	res.Set("staff-a", staffA.PrincipalData)
	res.Set("sa", authztest.Platform("system_admin").PrincipalData)
	s, err := httpx.NewServer(httpx.Deps{Auth: idp.AuthConfig(), Keys: idp.JWKS(t), Resolver: res})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Handle(listFixtureOp, fixtureListHandler(e)); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	now := time.Now()
	staffTok := idp.Token("staff-a", httpxtest.PortalClient, false, now)
	saTok := idp.Token("sa", httpxtest.PortalClient, true, now)

	// Tenant user: tenant_ids of tenant B is ignored.
	r := get(t, srv, "/api/v1/cms/fixture-contents?tenant_ids="+e.tenantB.String(), staffTok, &e.tenantA)
	if r.status != 200 || !slices.Equal(idsOf(r.rows), sorted(w.a, w.p1)) {
		t.Fatalf("tenant user with tenant_ids: %d %+v", r.status, r.rows)
	}
	// X-Tenant-Id of another tenant.
	denied := e.audits(ctx, t, "authz.denied", nil)
	if r := get(t, srv, "/api/v1/cms/fixture-contents", staffTok, &e.tenantB); r.status != 403 || r.code != "TENANT_MISMATCH" {
		t.Fatalf("header of another tenant: %d %q", r.status, r.code)
	}
	if e.audits(ctx, t, "authz.denied", nil) != denied+1 {
		t.Fatal("TENANT_MISMATCH not audited")
	}

	// System Admin with tenant_ids A,B: rows of both, each with its tenant_id.
	accessed := e.audits(ctx, t, "cross_tenant.accessed", nil)
	r = get(t, srv, "/api/v1/cms/fixture-contents?tenant_ids="+e.tenantA.String()+","+e.tenantB.String(), saTok, nil)
	if r.status != 200 || !slices.Equal(idsOf(r.rows), sorted(w.a, w.b, w.p1)) {
		t.Fatalf("system admin A+B: %d %+v", r.status, r.rows)
	}
	for _, row := range r.rows {
		if row.ID != w.p1 && (row.TenantID == nil || (*row.TenantID != e.tenantA && *row.TenantID != e.tenantB)) {
			t.Fatalf("row without its tenant: %+v", row)
		}
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' = 'cross_tenant.accessed'
		AND envelope->'payload'->'metadata'->>'mode' = 'read'
		AND envelope->'payload'->'metadata'->>'reason' = 'system_admin_cross_tenant'
		AND envelope->'payload'->'metadata'->>'tenant_ids' LIKE '%' || $1 || '%'`, e.tenantB.String()); n < 1 ||
		e.audits(ctx, t, "cross_tenant.accessed", nil) <= accessed {
		t.Fatalf("cross_tenant.accessed audit: %d", n)
	}
	// Empty tenant_ids = every active tenant (the disabled tenant is excluded).
	r = get(t, srv, "/api/v1/cms/fixture-contents", saTok, nil)
	if r.status != 200 || !slices.Equal(idsOf(r.rows), sorted(w.a, w.b, w.p1)) {
		t.Fatalf("system admin all tenants: %d %+v", r.status, r.rows)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' = 'cross_tenant.accessed'
		AND envelope->'payload'->'metadata'->>'tenant_ids' LIKE '%' || $1 || '%'`, e.tenantOff.String()); n != 0 {
		t.Fatal("disabled tenant in the all-tenants scope")
	}
}
