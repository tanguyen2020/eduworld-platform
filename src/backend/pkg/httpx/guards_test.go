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

// F002-AC-18 (TC-CMS-WS-02, TC-CMS-AUTHZ-02): platform routes need acr, fresh auth_time and a platform role.
func TestPlatformMFAGuard(t *testing.T) {
	e := newEnv(t, gameAdminOp)
	platform := []authz.Grant{{Role: "game_designer", ScopeKind: authz.RoleScopePlatform, Permissions: []authz.Permission{authz.PermGameQuestWrite}}}
	mk := func(acr string, authAge time.Duration, grants []authz.Grant, tenant *uuid.UUID) string {
		c := e.idp.baseClaims(portalClient)
		c["acr"] = acr
		c["auth_time"] = now.Add(-authAge).Unix()
		e.res.set(c["sub"].(string), authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStaff, Grants: grants, TenantID: tenant})
		return e.idp.sign(tok{claims: c})
	}
	cases := []struct {
		name   string
		token  string
		status int
		code   string
	}{
		{"mfa + platform role", mk("mfa", time.Hour, platform, nil), 200, ""},
		{"no mfa", mk("1", time.Hour, platform, nil), 403, "MFA_REQUIRED"},
		{"auth too old", mk("mfa", 9*time.Hour, platform, nil), 403, "MFA_REQUIRED"},
		{"tenant role only", mk("mfa", time.Hour, []authz.Grant{{Role: "tenant_admin", ScopeKind: authz.RoleScopeTenant,
			Permissions: []authz.Permission{authz.PermTenantUserRead}}}, nil), 403, "PERMISSION_DENIED"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := e.auditCount(audit.ActionAuthzDenied)
			r := e.do(t, http.MethodGet, "/api/v1/game-admin/quests", c.token, nil)
			if r.status != c.status || r.code != c.code {
				t.Fatalf("status %d code %q (%v)", r.status, r.code, r.raw)
			}
			if c.code == "PERMISSION_DENIED" && e.auditCount(audit.ActionAuthzDenied) != before+1 {
				t.Fatal("PERMISSION_DENIED not audited")
			}
		})
	}
	// MFA state is visible to handlers (used by authz.CrossTenantScope).
	if r := e.do(t, http.MethodGet, "/api/v1/game-admin/quests", mk("mfa", time.Minute, platform, nil), nil); !r.body.MFA {
		t.Fatal("principal MFA not set")
	}
}
