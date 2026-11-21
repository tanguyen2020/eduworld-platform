package httpx_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

var (
	cmsOp       = httpx.Operation{ID: "listQuestions", Method: http.MethodGet, Pattern: "/cms/learning/questions", Tenant: httpx.TenantRequired}
	gameOp      = httpx.Operation{ID: "createGameSession", Method: http.MethodPost, Pattern: "/game/sessions", Tenant: httpx.TenantRequired, Guard: httpx.GuardGamePlayer}
	gameAdminOp = httpx.Operation{ID: "listQuests", Method: http.MethodGet, Pattern: "/game-admin/quests", Tenant: httpx.TenantNone}
)

// F002-AC-16, F002-AC-17 (TC-AUTH-070, TC-AUTH-014): azp × user_type × route group.
func TestGuardMatrix(t *testing.T) {
	e := newEnv(t, cmsOp, gameOp)
	ta := uuid.New()
	mk := func(azp string, ut authz.UserType) string {
		c := e.idp.baseClaims(azp)
		e.res.set(c["sub"].(string), authz.PrincipalData{UserID: uuid.New(), UserType: ut, TenantID: &ta})
		return e.idp.sign(tok{claims: c})
	}
	h := map[string]string{"X-Tenant-Id": ta.String()}
	cases := []struct {
		name   string
		method string
		path   string
		azp    string
		ut     authz.UserType
		status int
		code   string
	}{
		{"portal staff → cms", http.MethodGet, "/api/v1/cms/learning/questions", portalClient, authz.UserTypeStaff, 200, ""},
		{"portal student → cms", http.MethodGet, "/api/v1/cms/learning/questions", portalClient, authz.UserTypeStudent, 403, "PORTAL_ACCESS_STAFF_ONLY"},
		{"game token → cms", http.MethodGet, "/api/v1/cms/learning/questions", gameClient, authz.UserTypeStudent, 403, "PORTAL_ACCESS_STAFF_ONLY"},
		{"game staff token → cms", http.MethodGet, "/api/v1/cms/learning/questions", gameClient, authz.UserTypeStaff, 403, "PORTAL_ACCESS_STAFF_ONLY"},
		{"game student → game", http.MethodPost, "/api/v1/game/sessions", gameClient, authz.UserTypeStudent, 200, ""},
		{"game staff → game", http.MethodPost, "/api/v1/game/sessions", gameClient, authz.UserTypeStaff, 403, "GAME_ACCESS_STUDENT_ONLY"},
		{"portal staff → game", http.MethodPost, "/api/v1/game/sessions", portalClient, authz.UserTypeStaff, 403, "GAME_ACCESS_STUDENT_ONLY"},
		{"portal student → game", http.MethodPost, "/api/v1/game/sessions", portalClient, authz.UserTypeStudent, 403, "GAME_ACCESS_STUDENT_ONLY"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := e.auditCount(audit.ActionAuthzDenied)
			r := e.do(t, c.method, c.path, mk(c.azp, c.ut), h)
			if r.status != c.status || r.code != c.code {
				t.Fatalf("status %d code %q, want %d %q (%v)", r.status, r.code, c.status, c.code, r.raw)
			}
			if c.status == 403 && e.auditCount(audit.ActionAuthzDenied) != before+1 {
				t.Fatal("denial not audited")
			}
		})
	}
}

// Portal route prefixes get portal_staff even when the operation forgot to declare it.
func TestPortalGuardIsForcedByPrefix(t *testing.T) {
	e := newEnv(t, httpx.Operation{ID: "x", Method: http.MethodGet, Pattern: "/tenant/settings", Tenant: httpx.TenantRequired})
	ta := uuid.New()
	c := e.idp.baseClaims(gameClient)
	e.res.set(c["sub"].(string), authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStudent, TenantID: &ta})
	if r := e.do(t, http.MethodGet, "/api/v1/tenant/settings", e.idp.sign(tok{claims: c}), map[string]string{"X-Tenant-Id": ta.String()}); r.code != "PORTAL_ACCESS_STAFF_ONLY" {
		t.Fatalf("got %d %q", r.status, r.code)
	}
}

// F002-AC-18 (TC-CMS-WS-02, TC-CMS-AUTHZ-02) as changed by ADR-0094: platform routes need the portal client, a
// staff user and a platform role — no MFA. A password-only token (amr ["pwd"], no acr, no auth_time, issued
// long ago in the session) reaches the route; a tenant role or a game token is still refused and audited.
func TestPlatformRoleGuard(t *testing.T) {
	e := newEnv(t, gameAdminOp)
	platform := []authz.Grant{{Role: "game_designer", ScopeKind: authz.RoleScopePlatform, Permissions: []authz.Permission{authz.PermGameQuestWrite}}}
	mk := func(azp string, grants []authz.Grant, claims map[string]any) string {
		c := e.idp.baseClaims(azp)
		for k, v := range claims {
			c[k] = v
		}
		e.res.set(c["sub"].(string), authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStaff, Grants: grants})
		return e.idp.sign(tok{claims: c})
	}
	cases := []struct {
		name   string
		token  string
		status int
		code   string
	}{
		{"password only + platform role", mk(portalClient, platform, nil), 200, ""},
		{"no amr, no acr, no auth_time", mk(portalClient, platform, map[string]any{"amr": nil}), 200, ""},
		{"old Keycloak claims are ignored", mk(portalClient, platform, map[string]any{"acr": "0", "auth_time": now.Add(-24 * time.Hour).Unix()}), 200, ""},
		{"tenant role only", mk(portalClient, []authz.Grant{{Role: "tenant_admin", ScopeKind: authz.RoleScopeTenant,
			Permissions: []authz.Permission{authz.PermTenantUserRead}}}, nil), 403, "PERMISSION_DENIED"},
		{"game client", mk(gameClient, platform, nil), 403, "PORTAL_ACCESS_STAFF_ONLY"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := e.auditCount(audit.ActionAuthzDenied)
			r := e.do(t, http.MethodGet, "/api/v1/game-admin/quests", c.token, nil)
			if r.status != c.status || r.code != c.code {
				t.Fatalf("status %d code %q (%v)", r.status, r.code, r.raw)
			}
			if c.code != "" && e.auditCount(audit.ActionAuthzDenied) != before+1 {
				t.Fatalf("%s not audited", c.code)
			}
		})
	}
	if r := e.do(t, http.MethodGet, "/api/v1/game-admin/quests", mk(portalClient, platform, nil), nil); r.body.Channel != string(authz.ChannelPortal) {
		t.Fatalf("principal channel %q", r.body.Channel)
	}
}
