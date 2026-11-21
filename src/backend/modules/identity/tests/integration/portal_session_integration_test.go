//go:build integration

package integration

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	identityapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/featuretoggle"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/portalcatalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/facade"
	identityhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// facadeResolver is the in-test copy of the composition-root adapter (cmd/internal/wiring.PrincipalResolver,
// which module tests may not import): it calls the identity facade and maps its typed 404 to ErrNotMember.
type facadeResolver struct{ f facade.Principals }

func (r facadeResolver) Resolve(ctx context.Context, req authz.ResolveRequest) (authz.Principal, error) {
	iss, sub := req.Subject.Issuer, req.Subject.Subject
	resp, err := r.f.ResolvePrincipal(ctx, identityapi.ResolvePrincipalRequestObject{Body: &identityapi.PrincipalResolveRequest{
		Issuer: &iss, Subject: &sub, TenantId: req.TenantID}})
	if err != nil {
		return authz.Principal{}, err
	}
	v, ok := resp.(identityapi.ResolvePrincipal200JSONResponse)
	if !ok {
		return authz.Principal{}, authz.ErrNotMember
	}
	if v.Status != identityapi.PrincipalStatusActive {
		return authz.Principal{}, problem.New(problem.AccountDeactivated, "")
	}
	d := authz.PrincipalData{UserID: v.UserId, UserType: authz.UserType(v.UserType), IsSystemAdmin: v.IsSystemAdmin, TenantID: v.TenantId}
	if v.TenantType != nil {
		d.TenantType = authz.TenantType(*v.TenantType)
	}
	for _, g := range *v.Grants {
		grant := authz.Grant{Role: g.RoleCode, ScopeKind: authz.RoleScopeKind(g.RoleScopeKind), SchoolID: g.SchoolId, SubjectID: g.SubjectId}
		for _, p := range g.Permissions {
			if pp := authz.Permission(p); pp.Known() {
				grant.Permissions = append(grant.Permissions, pp)
			}
		}
		d.Grants = append(d.Grants, grant)
	}
	return authz.NewPrincipal(d), nil
}

// F012-AC-04, AC-05, AC-07, AC-08, AC-10 through the httpx chain with the real Principal resolution.
func TestPortalSession(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	h, idp := realAuthHTTP(ctx, t, e)
	model := read.PrincipalModel{Session: e.read}
	_ = model

	a := seedTenant(ctx, t, e.platform, "pgd-sess-a")
	b := seedTenant(ctx, t, e.platform, "pgd-sess-b")
	c := seedTenant(ctx, t, e.platform, "pgd-sess-c")
	link := func(user uuid.UUID, sub string) {
		t.Helper()
		if _, err := e.platform.ExecContext(ctx, `INSERT INTO identity.external_identities (external_identity_id, created_at, updated_at, issuer, subject, user_id)
			VALUES ($1, now(), now(), $2, $3, $4)`, uuid.New(), idp.Issuer(), sub, user); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	hdr := func(tenant string) map[string]string { return map[string]string{"X-Tenant-Id": tenant} }

	// Staff of two tenants (one account, ADR-0067).
	gv := staffUser(ctx, t, e, "gv.sess", &a.tenant)
	link(gv, "kc-gv")
	grantRole(ctx, t, e, gv, "school_admin", &a.tenant, "tenant", nil, "NULL")
	grantRole(ctx, t, e, gv, "academic_manager", &b.tenant, "tenant", nil, "NULL")
	tok := idp.Token("kc-gv", httpxtest.PortalClient, now)

	// AC-04: listMyTenants without X-Tenant-Id.
	r := h.do(t, "GET", "/me/tenants", tok, nil, nil)
	if r.status != 200 || r.body["has_platform_role"] != false || len(r.body["items"].([]any)) != 2 || !strings.Contains(r.raw, `"role_codes":["school_admin"]`) {
		t.Fatalf("listMyTenants: %d %s", r.status, r.raw)
	}
	// AC-08/AC-10: getMyPermissions in tenant A: tenant roles, workspaces, no platform permission.
	r = h.do(t, "GET", "/me/permissions", tok, hdr(a.tenant), nil)
	if r.status != 200 || r.body["tenant_id"] != a.tenant || !strings.Contains(r.raw, `"tenant.sis.import"`) || strings.Contains(r.raw, `"platform.`) ||
		strings.Contains(r.raw, `"game.`) || !slices.Contains(anyStrings(r.body["workspaces"]), "school_lms") {
		t.Fatalf("getMyPermissions A: %d %s", r.status, r.raw)
	}
	// AC-07: same token, other tenant of the user → its own roles; a tenant without membership → TENANT_MISMATCH.
	if r = h.do(t, "GET", "/me/permissions", tok, hdr(b.tenant), nil); r.status != 200 || !strings.Contains(r.raw, `"academic_manager"`) ||
		strings.Contains(r.raw, `"school_admin"`) {
		t.Fatalf("getMyPermissions B: %d %s", r.status, r.raw)
	}
	if r = h.do(t, "GET", "/me/permissions", tok, hdr(c.tenant), nil); r.status != 403 || !strings.Contains(r.raw, "TENANT_MISMATCH") {
		t.Fatalf("tenant without membership: %d %s", r.status, r.raw)
	}
	if r = h.do(t, "GET", "/auth/me", tok, hdr(a.tenant), nil); r.status != 200 || r.body["display_name"] != "gv.sess" || r.body["user_type"] != "staff" {
		t.Fatalf("getMe: %d %s", r.status, r.raw)
	}
	// OQ F051-3: getMe of a tenant user without X-Tenant-Id → 400 TENANT_HEADER_REQUIRED.
	if r = h.do(t, "GET", "/auth/me", tok, nil, nil); r.status != 400 || !strings.Contains(r.raw, "TENANT_HEADER_REQUIRED") {
		t.Fatalf("getMe of a tenant user without header: %d %s", r.status, r.raw)
	}

	// AC-05: a student on the portal, and staff without any portal role → PORTAL_ACCESS_STAFF_ONLY.
	st := enrolled(ctx, t, e.platform, a, "hs.sess")
	if _, err := e.platform.ExecContext(ctx, `UPDATE identity.users SET status = 'active' WHERE user_id = $1`, st); err != nil {
		t.Fatal(err)
	}
	link(st, "kc-hs")
	stTok := idp.Token("kc-hs", httpxtest.PortalClient, now)
	if r = h.do(t, "GET", "/me/tenants", stTok, nil, nil); r.status != 403 || !strings.Contains(r.raw, "PORTAL_ACCESS_STAFF_ONLY") {
		t.Fatalf("student listMyTenants on portal: %d %s", r.status, r.raw)
	}
	if r = h.do(t, "GET", "/me/permissions", stTok, hdr(a.tenant), nil); r.status != 403 || !strings.Contains(r.raw, "PORTAL_ACCESS_STAFF_ONLY") {
		t.Fatalf("student getMyPermissions on portal: %d %s", r.status, r.raw)
	}
	if r = h.do(t, "GET", "/me/tenants", idp.Token("kc-hs", httpxtest.GameClient, now), nil, nil); r.status != 200 || len(r.body["items"].([]any)) != 1 {
		t.Fatalf("student listMyTenants on game: %d %s", r.status, r.raw)
	}
	none := staffUser(ctx, t, e, "gv.none", &a.tenant)
	link(none, "kc-none")
	if r = h.do(t, "GET", "/me/tenants", idp.Token("kc-none", httpxtest.PortalClient, now), nil, nil); r.status != 403 ||
		!strings.Contains(r.raw, "PORTAL_ACCESS_STAFF_ONLY") {
		t.Fatalf("staff without role: %d %s", r.status, r.raw)
	}

	// System Admin: platform scope without header.
	sa := staffUser(ctx, t, e, "sa.sess", nil)
	link(sa, "kc-sa-sess")
	grantRole(ctx, t, e, sa, "system_admin", nil, "platform", nil, "NULL")
	saTok := idp.Token("kc-sa-sess", httpxtest.PortalClient, now)
	if r = h.do(t, "GET", "/me/tenants", saTok, nil, nil); r.status != 200 || r.body["has_platform_role"] != true {
		t.Fatalf("system admin listMyTenants: %d %s", r.status, r.raw)
	}
	if r = h.do(t, "GET", "/me/permissions", saTok, nil, nil); r.status != 200 || r.body["tenant_id"] != nil ||
		!slices.Contains(anyStrings(r.body["workspaces"]), "platform_admin") || !strings.Contains(r.raw, `"scope_type":"platform"`) {
		t.Fatalf("system admin getMyPermissions: %d %s", r.status, r.raw)
	}
	// OQ F051-3: platform staff without X-Tenant-Id get their profile in the platform scope.
	if r = h.do(t, "GET", "/auth/me", saTok, nil, nil); r.status != 200 || r.body["display_name"] != "sa.sess" || !strings.Contains(r.raw, `"scope_type":"platform"`) {
		t.Fatalf("getMe of platform staff without header: %d %s", r.status, r.raw)
	}
}

func anyStrings(v any) []string {
	var out []string
	for _, x := range v.([]any) {
		out = append(out, x.(string))
	}
	return out
}

// realAuthHTTP serves the identity routes behind httpx with JWTs of a fake IDP and the real Principal
// resolution (facade + PrincipalModel), so permissions come from the user roles in the database.
func realAuthHTTP(ctx context.Context, t *testing.T, e *env) (*httpEnv, *httpxtest.IDP) {
	t.Helper()
	idp := httpxtest.NewIDP(t)
	model := read.PrincipalModel{Session: e.read}
	res := facadeResolver{f: facade.Principals{Resolve: query.ResolvePrincipal{Model: model}}}
	spec, err := httpx.LoadSpec(ctx, filepath.Join(testkit.RepoRoot(t), "specs", "openapi", "eduworld-api.v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	srv, err := httpx.NewServer(httpx.Deps{Auth: idp.AuthConfig(), Keys: idp.JWKS(t), Resolver: res, Spec: spec,
		Idempotency: idempotency.NewStore(e.app, 0, nil)})
	if err != nil {
		t.Fatal(err)
	}
	ws, err := portalcatalog.Load(filepath.Join(testkit.RepoRoot(t), "specs", "portal", "menu-catalog.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	codec, _ := page.NewCodec([]byte(strings.Repeat("k", 32)))
	h := identityhttp.Handlers{Me: query.Me{Model: model, Workspaces: ws, Toggles: featuretoggle.Source{Reader: read.FeatureFlagModel{Session: e.read}}},
		UserRoles:    command.UserRoles{UoW: e.uow, Repo: write.UserRoleRepo{}, Events: e.deps.Events, Audit: e.deps.Audit},
		UserRoleList: query.UserRoles{Model: model, Cursors: codec},
		Roles:        command.Roles{UoW: e.uow, Repo: write.RoleRepo{}, Events: e.deps.Events, Audit: e.deps.Audit},
		RoleList:     query.Roles{Model: model, Cursors: codec},
		UserList:     query.TenantUsers{Model: model, Cursors: codec}}
	kc := newKCUsers()
	h.Staff = command.Staff{UoW: e.uow, Repo: write.StaffRepo{}, Roles: h.UserRoles, KC: kc, Events: e.deps.Events, Audit: e.deps.Audit,
		LocalIssuer: "http://kc/realms/eduworld"}
	if err := identityhttp.Register(srv, h); err != nil {
		t.Fatal(err)
	}
	if err := identityhttp.RegisterSettings(srv, settingsHandlers(e, codec)); err != nil { // F015
		t.Fatal(err)
	}
	rec := &revokeRecorder{}
	if err := identityhttp.RegisterLifecycle(srv, lifecycleHandlers(e, kc, rec, codec)); err != nil { // F016
		t.Fatal(err)
	}
	if err := identityhttp.RegisterPlatformUsers(srv, platformUserHandlers(e, h, kc, idp, codec)); err != nil { // F059-1, F059-2
		t.Fatal(err)
	}
	hs := httptest.NewServer(srv)
	t.Cleanup(hs.Close)
	return &httpEnv{env: e, srv: hs, idp: idp, kc: kc, revoker: rec}, idp
}
