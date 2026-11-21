//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/integration"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/seed"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	identityhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
)

// platformUserHandlers are the F059-1/F059-2 use cases of the HTTP harness realAuthHTTP (Keycloak: TEST DOUBLE kcUsers).
func platformUserHandlers(e *env, h identityhttp.Handlers, kc *kcUsers, idp *httpxtest.IDP, codec *page.Codec) identityhttp.PlatformUserHandlers {
	model := read.PrincipalModel{Session: e.read}
	return identityhttp.PlatformUserHandlers{
		List: query.PlatformUsers{Model: model, Cursors: codec},
		Create: command.PlatformStaff{UoW: e.uow, Repo: write.StaffRepo{}, Roles: h.UserRoles, KC: kc, Events: e.deps.Events, Audit: e.deps.Audit,
			LocalIssuer: "http://kc/realms/eduworld"},
		Integrations: query.Integrations{Config: query.IntegrationConfig{IssuerURL: idp.Issuer(), Realm: "eduworld",
			Clients:            []query.IntegrationClient{{ClientID: httpxtest.PortalClient, Purpose: "portal"}},
			PlatformMaxAuthAge: 8 * time.Hour, RouteGroups: mustRouteGroups()}, Stats: read.SisAdminModel{Session: e.read}},
	}
}

var secretWords = []string{"password", "secret", "token", "credential", `"ip"`, "ip_address"}

func noSecret(t *testing.T, what, raw string) {
	t.Helper()
	low := strings.ToLower(raw)
	for _, w := range secretWords {
		if strings.Contains(low, w) {
			t.Fatalf("%s carries %q: %s", what, w, raw)
		}
	}
}

// F059-1 (listPlatformUsers, createPlatformUser; owner F012) through httpx with the real Principal resolution:
// platform.user.manage on /admin/* (no MFA, ADR-0094), Idempotency-Key replay, platform staff only without tenant_ids,
// members of the filtered tenants with tenant_ids (tenant A/B isolation), no secret in the responses.
func TestPlatformUsers(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	h, idp := realAuthHTTP(ctx, t, e)
	a := seedTenant(ctx, t, e.platform, "pgd-pu-a")
	b := seedTenant(ctx, t, e.platform, "pgd-pu-b")
	iss, now := idp.Issuer(), time.Now()
	portalUser(ctx, t, e, iss, "sa.pu", "system_admin", nil, "platform", nil)
	portalUser(ctx, t, e, iss, "ca.pu", "content_admin", nil, "platform", nil)
	portalUser(ctx, t, e, iss, "ta.pu", "tenant_admin", &a.tenant, "tenant", nil)
	portalUser(ctx, t, e, iss, "gv.pu.b", "academic_manager", &b.tenant, "tenant", nil)
	studentA := enrolled(ctx, t, e.platform, a, "hs.pu.a")
	studentB := enrolled(ctx, t, e.platform, b, "hs.pu.b")
	tok := func(user string) string { return idp.Token("sub-"+user, httpxtest.PortalClient, now) }
	sa, ca, ta := tok("sa.pu"), tok("ca.pu"), tok("ta.pu")
	key := func() map[string]string { return map[string]string{"Idempotency-Key": ids.New().String()} }
	body := func(username, email string, roles ...string) map[string]any {
		r := []string{}
		for _, c := range roles {
			r = append(r, seed.RoleID(c).String())
		}
		return map[string]any{"username": username, "email": email, "display_name": "Vận hành", "initial_role_ids": r,
			"reason": "new liveops engineer"}
	}

	// Authorization: tenant staff and platform staff without platform.user.manage are refused; the System Admin
	// signs in with username + password only (ADR-0094) and reaches /admin/users.
	authztest.Cover(t, "listPlatformUsers", authztest.RoleDenied)
	authztest.Cover(t, "createPlatformUser", authztest.RoleDenied)
	if r := h.do(t, "GET", "/admin/users", sa, nil, nil); r.status != 200 {
		t.Fatalf("password-only System Admin: %d %s", r.status, r.raw)
	}
	for name, tk := range map[string]string{"tenant admin": ta, "content admin": ca} {
		if r := h.do(t, "GET", "/admin/users", tk, nil, nil); r.status != 403 {
			t.Errorf("%s list: %d %s", name, r.status, r.raw)
		}
		if r := h.do(t, "POST", "/admin/users", tk, key(), body("ops.x", "ops.x@eduworld.vn", "liveops")); r.status != 403 {
			t.Errorf("%s create: %d %s", name, r.status, r.raw)
		}
	}

	// createPlatformUser: Idempotency-Key required; Keycloak user with realm role platform_staff and the
	// UPDATE_PASSWORD email; staff without home tenant; platform user role; event + audit; replay is identical.
	if r := h.do(t, "POST", "/admin/users", sa, nil, body("ops.lan", "ops.lan@eduworld.vn", "liveops")); r.status != 428 && r.status != 400 {
		t.Fatalf("no key: %d %s", r.status, r.raw)
	}
	k := key()
	r := h.do(t, "POST", "/admin/users", sa, k, body("Ops.Lan", "Ops.Lan@EduWorld.vn", "liveops", "game_designer"))
	if r.status != 201 || r.body["username"] != "ops.lan" || r.body["email"] != "ops.lan@eduworld.vn" || r.body["primary_tenant_id"] != nil ||
		r.body["user_type"] != "staff" || r.body["status"] != "active" {
		t.Fatalf("create: %d %s", r.status, r.raw)
	}
	noSecret(t, "createPlatformUser", r.raw)
	id := r.body["user_id"].(string)
	kcID := h.kc.users["ops.lan"]
	if kcID == "" || !strings.Contains(strings.Join(h.kc.roles[kcID], ","), "platform_staff") ||
		!strings.Contains(strings.Join(h.kc.roles[kcID], ","), "email:UPDATE_PASSWORD") {
		t.Fatalf("keycloak user %q roles %v", kcID, h.kc.roles[kcID])
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE user_id = $1 AND primary_tenant_id IS NULL`, id); n != 1 {
		t.Fatalf("platform staff row = %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.user_roles ur JOIN identity.roles r ON r.role_id = ur.role_id
		WHERE ur.user_id = $1 AND ur.tenant_id IS NULL AND ur.scope_type = 'platform' AND r.code IN ('liveops', 'game_designer')
		AND ur.deleted_at IS NULL`, id); n != 2 {
		t.Fatalf("initial platform roles = %d", n)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'user.provisioned'
		AND envelope->'payload'->>'user_id' = $1 AND envelope->>'tenant_id' IS NULL`, id); n != 1 {
		t.Fatalf("user.provisioned = %d", n)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' = 'user.created' AND envelope->'payload'->>'target_id' = $1`, id); n != 1 {
		t.Fatalf("audit user.created = %d", n)
	}
	replay := h.do(t, "POST", "/admin/users", sa, k, body("Ops.Lan", "Ops.Lan@EduWorld.vn", "liveops", "game_designer"))
	if replay.status != 201 || replay.body["user_id"] != id || replay.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %d %v %s", replay.status, replay.header, replay.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE username = 'ops.lan'`); n != 1 {
		t.Fatalf("replay created %d accounts", n)
	}

	// 409 on a taken username/email (any case); 422 on a tenant role, safety_moderator without training, an unknown role.
	if r := h.do(t, "POST", "/admin/users", sa, key(), body("OPS.LAN", "other@eduworld.vn", "liveops")); r.status != 409 ||
		!strings.Contains(r.raw, "USERNAME_TAKEN") {
		t.Fatalf("username taken: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "POST", "/admin/users", sa, key(), body("ops.khac", "OPS.LAN@eduworld.vn", "liveops")); r.status != 409 ||
		!strings.Contains(r.raw, "EMAIL_TAKEN") {
		t.Fatalf("email taken: %d %s", r.status, r.raw)
	}
	for name, b := range map[string]map[string]any{
		"tenant role":      body("ops.t", "ops.t@eduworld.vn", "tenant_admin"),
		"safety moderator": body("ops.sm", "ops.sm@eduworld.vn", "safety_moderator"),
	} {
		if r := h.do(t, "POST", "/admin/users", sa, key(), b); r.status != 422 || !strings.Contains(r.raw, "initial_role_ids") {
			t.Errorf("%s: %d %s", name, r.status, r.raw)
		}
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE username IN ('ops.t', 'ops.sm')`); n != 0 || h.kc.users["ops.t"] != "" {
		t.Fatalf("rejected roles created accounts: db=%d kc=%v", n, h.kc.users)
	}

	// listPlatformUsers without tenant_ids: platform staff only (never students or tenant staff, AGENTS R8).
	r = h.do(t, "GET", "/admin/users?limit=100", sa, nil, nil)
	if r.status != 200 || !strings.Contains(r.raw, `"ops.lan"`) || !strings.Contains(r.raw, `"sa.pu"`) || strings.Contains(r.raw, `"ta.pu"`) ||
		strings.Contains(r.raw, "hs.pu.") || strings.Contains(r.raw, `"tenant_ids"`) {
		t.Fatalf("platform list: %d %s", r.status, r.raw)
	}
	noSecret(t, "listPlatformUsers", r.raw)
	if r := h.do(t, "GET", "/admin/users?q=OPS.L&role_code=liveops", sa, nil, nil); r.status != 200 || len(r.body["items"].([]any)) != 1 {
		t.Fatalf("q + role_code: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", "/admin/users?q=_", sa, nil, nil); r.status != 422 && r.status != 400 {
		t.Fatalf("short q: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", "/admin/users?q=o_s", sa, nil, nil); r.status != 200 || len(r.body["items"].([]any)) != 0 {
		t.Fatalf("q wildcard must be literal: %d %s", r.status, r.raw)
	}

	// With tenant_ids (CrossTenantScope, audited): members of tenant A only — tenant B never appears.
	r = h.do(t, "GET", "/admin/users?limit=100&tenant_ids="+url.QueryEscape(a.tenant), sa, nil, nil)
	if r.status != 200 || !strings.Contains(r.raw, studentA.String()) || !strings.Contains(r.raw, `"ta.pu"`) || strings.Contains(r.raw, studentB.String()) ||
		strings.Contains(r.raw, `"gv.pu.b"`) || strings.Contains(r.raw, `"ops.lan"`) || !strings.Contains(r.raw, `"tenant_ids":["`+a.tenant+`"]`) {
		t.Fatalf("tenant A list: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", "/admin/users?user_type=student&tenant_ids="+b.tenant, sa, nil, nil); r.status != 200 ||
		len(r.body["items"].([]any)) != 1 || !strings.Contains(r.raw, studentB.String()) {
		t.Fatalf("tenant B students: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' = 'cross_tenant.accessed'`); n < 2 {
		t.Fatalf("cross_tenant.accessed = %d", n)
	}
	// A tenant session (X-Tenant-Id) never reaches the platform listing.
	if r := h.do(t, "GET", "/admin/users", ta, map[string]string{"X-Tenant-Id": a.tenant}, nil); r.status != 403 && r.status != 400 {
		t.Fatalf("tenant session: %d %s", r.status, r.raw)
	}
}

// F059-2 (getIntegrationOverview): read-only configuration and SIS counts for platform staff; never a secret.
func TestIntegrationOverview(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	h, idp := realAuthHTTP(ctx, t, e)
	a := seedTenant(ctx, t, e.platform, "pgd-io-a")
	b := seedTenant(ctx, t, e.platform, "pgd-io-b")
	for _, c := range []struct{ tenant, status string }{{a.tenant, "active"}, {b.tenant, "active"}, {b.tenant, "paused"}} {
		if _, err := e.platform.ExecContext(ctx, `INSERT INTO sis.sis_connections (sis_connection_id, tenant_id, created_at, updated_at, provider_kind, status)
			VALUES ($1, $2, now(), now(), 'csv_import', $3)`, ids.New(), c.tenant, c.status); err != nil {
			t.Fatal(err)
		}
	}
	iss, now := idp.Issuer(), time.Now()
	portalUser(ctx, t, e, iss, "sa.io", "system_admin", nil, "platform", nil)
	portalUser(ctx, t, e, iss, "ca.io", "content_admin", nil, "platform", nil)
	portalUser(ctx, t, e, iss, "ta.io", "tenant_admin", &a.tenant, "tenant", nil)
	tok := func(user string) string { return idp.Token("sub-"+user, httpxtest.PortalClient, now) }

	authztest.Cover(t, "getIntegrationOverview", authztest.RoleDenied)
	for _, u := range []string{"ca.io", "ta.io"} {
		if r := h.do(t, "GET", "/admin/integrations", tok(u), nil, nil); r.status != 403 {
			t.Errorf("%s: %d %s", u, r.status, r.raw)
		}
	}
	r := h.do(t, "GET", "/admin/integrations", tok("sa.io"), nil, nil)
	if r.status != 200 {
		t.Fatalf("overview: %d %s", r.status, r.raw)
	}
	kc := r.body["keycloak"].(map[string]any)
	sis := r.body["sis"].(map[string]any)
	byStatus := sis["connections_by_status"].(map[string]any)
	if kc["realm"] != "eduworld" || kc["platform_mfa_required"] != nil || sis["connections_total"] != float64(3) || byStatus["active"] != float64(2) ||
		byStatus["paused"] != float64(1) || r.body["apisix"].(map[string]any)["public_base_url"] != nil {
		t.Fatalf("overview: %s", r.raw)
	}
	// The route groups are public gateway paths (e.g. /api/v1/auth/game/token:refresh): checked apart from the secret scan.
	apisix := r.body["apisix"].(map[string]any)
	groups := apisix["route_groups"]
	delete(apisix, "route_groups")
	scanned, _ := json.Marshal(r.body)
	noSecret(t, "getIntegrationOverview", string(scanned))
	apisix["route_groups"] = groups
	// OQ F059-12: the route groups generated from deploy/apisix/routes/routes.yaml (schema-checked by the httpx chain).
	if groups, _ := r.body["apisix"].(map[string]any)["route_groups"].([]any); len(groups) == 0 ||
		!strings.Contains(r.raw, `{"prefix":"/api/v1/admin/","gateway":"internal","service":"core-api"}`) {
		t.Fatalf("route groups: %s", r.raw)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' = 'cross_tenant.accessed'`); n < 1 {
		t.Fatalf("bypass for the SIS counts not audited: %d", n)
	}
}

// F059-3 (action_prefix, owner F015): the access-control log reads role.* and user_role.* together on both audit
// queries; the prefix never matches a longer segment, never crosses tenants, and malformed values are 422.
func TestAuditActionPrefix(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	h, idp := realAuthHTTP(ctx, t, e)
	a := seedTenant(ctx, t, e.platform, "pgd-ap-a")
	b := seedTenant(ctx, t, e.platform, "pgd-ap-b")
	auditRow(ctx, t, e, &a.tenant, nil, "role.created")
	auditRow(ctx, t, e, &a.tenant, nil, "user_role.granted")
	auditRow(ctx, t, e, &a.tenant, nil, "role_x.created") // same first letters, other group
	auditRow(ctx, t, e, &a.tenant, nil, "tenant.updated")
	auditRow(ctx, t, e, &b.tenant, nil, "role.created")
	auditRow(ctx, t, e, nil, nil, "role.updated")
	auditRow(ctx, t, e, nil, nil, "tenant.created")
	iss, now := idp.Issuer(), time.Now()
	portalUser(ctx, t, e, iss, "sa.ap", "system_admin", nil, "platform", nil)
	portalUser(ctx, t, e, iss, "ta.ap", "tenant_admin", &a.tenant, "tenant", nil)
	sa := idp.Token("sub-sa.ap", httpxtest.PortalClient, now)
	ta := idp.Token("sub-ta.ap", httpxtest.PortalClient, now)
	window := "from=" + url.QueryEscape(now.Add(-24*time.Hour).Format(time.RFC3339)) + "&to=" + url.QueryEscape(now.Add(time.Minute).Format(time.RFC3339))
	prefixes := "&action_prefix=role&action_prefix=user_role"
	actions := func(r httpResp) string {
		var out []string
		for _, d := range r.body["data"].([]any) {
			out = append(out, d.(map[string]any)["action"].(string))
		}
		return strings.Join(out, ",")
	}

	r := h.do(t, "GET", "/tenant/audit-logs?"+window+prefixes, ta, map[string]string{"X-Tenant-Id": a.tenant}, nil)
	if got := actions(r); r.status != 200 || (got != "user_role.granted,role.created" && got != "role.created,user_role.granted") {
		t.Fatalf("tenant A access-control log: %d %s", r.status, r.raw)
	}
	r = h.do(t, "GET", "/admin/audit-logs?"+window+prefixes, sa, nil, nil)
	if r.status != 200 || actions(r) != "role.updated" {
		t.Fatalf("platform access-control log: %d %s", r.status, r.raw)
	}
	r = h.do(t, "GET", "/admin/audit-logs?"+window+prefixes+"&tenant_ids="+b.tenant, sa, nil, nil)
	if r.status != 200 || actions(r) != "role.created" || strings.Contains(r.raw, a.tenant) {
		t.Fatalf("tenant B through tenant_ids: %d %s", r.status, r.raw)
	}
	r = h.do(t, "GET", "/admin/audit-logs?"+window+"&action_prefix=role&action=role.created&tenant_ids="+a.tenant, sa, nil, nil)
	if r.status != 200 || actions(r) != "role.created" {
		t.Fatalf("prefix AND action: %d %s", r.status, r.raw)
	}
	for _, bad := range []string{"role.*", "Role", "role%25", "r"} {
		if r := h.do(t, "GET", "/admin/audit-logs?"+window+"&action_prefix="+bad, sa, nil, nil); r.status != 422 && r.status != 400 {
			t.Errorf("action_prefix=%s: %d %s", bad, r.status, r.raw)
		}
	}
	many := ""
	for i := 0; i < 11; i++ {
		many += "&action_prefix=role"
	}
	if r := h.do(t, "GET", "/admin/audit-logs?"+window+many, sa, nil, nil); r.status != 422 && r.status != 400 {
		t.Fatalf("11 prefixes: %d %s", r.status, r.raw)
	}
}

// mustRouteGroups loads the embedded route groups (generated by tools/routegroups).
func mustRouteGroups() []query.RouteGroup {
	g, err := integration.RouteGroups()
	if err != nil {
		panic(err)
	}
	return g
}
