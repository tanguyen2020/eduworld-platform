package httpx_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

var (
	classesOp = httpx.Operation{ID: "listClasses", Method: http.MethodGet, Pattern: "/lms/classes", Tenant: httpx.TenantRequired}
	reportsOp = httpx.Operation{ID: "listReports", Method: http.MethodGet, Pattern: "/lms/reports", Tenant: httpx.TenantOptional}
	tenantsOp = httpx.Operation{ID: "listMyTenants", Method: http.MethodGet, Pattern: "/me/tenants", Tenant: httpx.TenantNone}
)

// F002-AC-06 (TC-TEN-011a/b).
func TestTenantHeader(t *testing.T) {
	e := newEnv(t, classesOp, reportsOp, tenantsOp)
	ta := uuid.New()
	c := e.idp.baseClaims(portalClient)
	staffFor(e, c, &ta)
	tk := e.idp.sign(tok{claims: c})

	if r := e.do(t, http.MethodGet, "/api/v1/lms/classes", tk, nil); r.status != 400 || r.code != "TENANT_HEADER_REQUIRED" {
		t.Fatalf("missing header: %d %q", r.status, r.code)
	}
	if r := e.do(t, http.MethodGet, "/api/v1/lms/classes", tk, map[string]string{"X-Tenant-Id": "not-a-uuid"}); r.status != 400 || r.code != "BAD_REQUEST" {
		t.Fatalf("bad uuid: %d %q", r.status, r.code)
	}
	if r := e.do(t, http.MethodGet, "/api/v1/lms/classes", tk, map[string]string{"X-Tenant-Id": ta.String()}); r.status != 200 || r.body.TenantID != ta.String() {
		t.Fatalf("valid header: %d %+v %v", r.status, r.body, r.raw)
	}
	// Exempt route: no header needed and a header is not used to resolve the principal.
	if r := e.do(t, http.MethodGet, "/api/v1/me/tenants", tk, nil); r.status != 200 {
		t.Fatalf("exempt route: %d %v", r.status, r.raw)
	}
	// Optional header: absent is fine.
	if r := e.do(t, http.MethodGet, "/api/v1/lms/reports", tk, nil); r.status != 200 {
		t.Fatalf("optional header: %d %v", r.status, r.raw)
	}
}

// F002-AC-07 (TC-TEN-006, TC-TEN-011): membership mismatch → 403 TENANT_MISMATCH + audit + metric.
func TestTenantMismatch(t *testing.T) {
	e := newEnv(t, classesOp)
	ta, tb := uuid.New(), uuid.New()
	c := e.idp.baseClaims(portalClient)
	staffFor(e, c, &ta)
	before := testutil.ToFloat64(httpx.TenantMismatchCounter())
	r := e.do(t, http.MethodGet, "/api/v1/lms/classes", e.idp.sign(tok{claims: c}), map[string]string{"X-Tenant-Id": tb.String()})
	if r.status != 403 || r.code != "TENANT_MISMATCH" {
		t.Fatalf("mismatch: %d %q", r.status, r.code)
	}
	if e.auditCount(audit.ActionAuthzDenied) != 1 {
		t.Fatal("no authz.denied audit")
	}
	if got := testutil.ToFloat64(httpx.TenantMismatchCounter()) - before; got != 1 {
		t.Fatalf("eduworld_auth_tenant_mismatch_total += %v", got)
	}
}

// F002-AC-08: Principal.TenantID comes from the resolver; tenant_ids is ignored for tenant users and
// honoured for System Admin with platform.cross_tenant.read + MFA (CrossTenantScope).
func TestTenantIDsOnlyForSystemAdmin(t *testing.T) {
	e := newEnv(t, reportsOp)
	ta, tb := uuid.New(), uuid.New()
	c := e.idp.baseClaims(portalClient)
	staffFor(e, c, &ta)
	r := e.do(t, http.MethodGet, "/api/v1/lms/reports?tenant_ids="+tb.String(), e.idp.sign(tok{claims: c}), map[string]string{"X-Tenant-Id": ta.String()})
	if r.status != 200 || len(r.body.Scope) != 1 || r.body.Scope[0] != ta.String() {
		t.Fatalf("tenant user scope: %d %+v", r.status, r.body)
	}

	sa := e.idp.baseClaims(portalClient)
	sa["acr"] = "mfa"
	e.res.set(sa["sub"].(string), authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStaff, IsSystemAdmin: true,
		Grants: []authz.Grant{{Role: "system_admin", ScopeKind: authz.RoleScopePlatform, Permissions: []authz.Permission{authz.PermPlatformCrossTenantRead}}}})
	r = e.do(t, http.MethodGet, "/api/v1/lms/reports?tenant_ids="+ta.String()+","+tb.String(), e.idp.sign(tok{claims: sa}), nil)
	if r.status != 200 || len(r.body.Scope) != 2 || r.body.TenantID != "" {
		t.Fatalf("system admin cross-tenant: %d %+v %v", r.status, r.body, r.raw)
	}
	if e.auditCount(audit.ActionCrossTenantAccessed) != 0 {
		// Recorded by data access when the scope is used (tenancy.RecordCrossTenantScope), not here.
		t.Fatal("cross_tenant.accessed recorded by middleware")
	}
	// Without MFA the parameter is refused rather than silently widened.
	sa["acr"] = "1"
	r = e.do(t, http.MethodGet, "/api/v1/lms/reports?tenant_ids="+ta.String(), e.idp.sign(tok{claims: sa}), nil)
	if r.status != 403 || r.code != "MFA_REQUIRED" {
		t.Fatalf("cross-tenant without MFA: %d %q", r.status, r.code)
	}
	// Platform staff without tenant_ids gets the platform scope.
	sa["acr"] = "mfa"
	r = e.do(t, http.MethodGet, "/api/v1/lms/reports", e.idp.sign(tok{claims: sa}), nil)
	if r.status != 200 || !r.body.Platform {
		t.Fatalf("platform scope: %d %+v", r.status, r.body)
	}
}

// F002-AC-09 through the middleware: resolver down → 503 UNAVAILABLE retryable.
func TestResolverDownIs503(t *testing.T) {
	e := newEnv(t, classesOp)
	cached, err := authz.NewCachedResolver(e.res, authz.CacheOptions{Module: "learning", TTL: 60e9, L1TTL: 15e9, L1Size: 10})
	if err != nil {
		t.Fatal(err)
	}
	srv, err := httpx.NewServer(httpx.Deps{Auth: authCfg(e.idp), Keys: e.jwks, Resolver: cached, Clock: e.clk})
	if err != nil {
		t.Fatal(err)
	}
	_ = srv.Handle(classesOp, http.HandlerFunc(echo))
	e.srv = srv
	e.res.err = errors.New("identity down")
	ta := uuid.New()
	r := e.do(t, http.MethodGet, "/api/v1/lms/classes", e.idp.sign(tok{claims: e.idp.baseClaims(portalClient)}), map[string]string{"X-Tenant-Id": ta.String()})
	if r.status != 503 || r.code != "UNAVAILABLE" || r.raw["retryable"] != true {
		t.Fatalf("resolver down: %d %q %v", r.status, r.code, r.raw)
	}
}

// F013-AC-25 (TC-TEN-007): a token whose organization claim names several tenants (list or object form)
// is refused with 403 TENANT_NOT_SELECTED whatever X-Tenant-Id says; one organization passes.
func TestOrganizationClaimOfSeveralTenants(t *testing.T) {
	e := newEnv(t, classesOp)
	ta := uuid.New()
	for _, org := range []any{[]string{"th-a", "th-b"}, map[string]any{"th-a": map[string]string{"id": "1"}, "th-b": map[string]string{"id": "2"}}} {
		c := e.idp.baseClaims(portalClient)
		staffFor(e, c, &ta)
		c["organization"] = org
		r := e.do(t, http.MethodGet, "/api/v1/lms/classes", e.idp.sign(tok{claims: c}), map[string]string{"X-Tenant-Id": ta.String()})
		if r.status != 403 || r.code != "TENANT_NOT_SELECTED" {
			t.Fatalf("organization %v: %d %q", org, r.status, r.code)
		}
	}
	c := e.idp.baseClaims(portalClient)
	staffFor(e, c, &ta)
	c["organization"] = []string{"th-a"}
	if r := e.do(t, http.MethodGet, "/api/v1/lms/classes", e.idp.sign(tok{claims: c}), map[string]string{"X-Tenant-Id": ta.String()}); r.status != 200 {
		t.Fatalf("one organization: %d %q", r.status, r.code)
	}
}
