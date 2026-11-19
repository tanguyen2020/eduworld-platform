//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F014-AC-19 (TC-AUTH-010): getClientConfig without token and without X-Tenant-Id returns the versions, update_required
// for the version sent, maintenance, login.login_url, remember_username_default and supported_locales, and no client id,
// secret or OIDC setting. F014-AC-20 (TC-AUTH-066): a client below min_client_version → 426 at createGameSession.
func TestClientConfig(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	old := e.do(t, http.MethodGet, "/game/client-config?platform=android&version=0.9.0", "", nil, nil)
	if old.status != http.StatusOK || old.body["update_required"] != true || old.body["min_client_version"] != MinVersion ||
		old.body["latest_client_version"] != "1.4.0" {
		t.Fatalf("old client: %d %s", old.status, old.raw)
	}
	login, _ := old.body["login"].(map[string]any)
	maint, _ := old.body["maintenance"].(map[string]any)
	if login["login_url"] == nil || login["remember_username_default"] != true || maint["active"] != false ||
		len(old.body["supported_locales"].([]any)) != 2 {
		t.Fatalf("config: %s", old.raw)
	}
	for _, secret := range []string{"client_id", "client_secret", "issuer", "openid", "authorization_endpoint", "token_endpoint"} {
		if strings.Contains(strings.ToLower(old.raw), secret) {
			t.Fatalf("client-config exposes %s: %s", secret, old.raw)
		}
	}
	if cur := e.do(t, http.MethodGet, "/game/client-config?platform=windows&version=1.2.0", "", nil, nil); cur.body["update_required"] != false {
		t.Fatalf("current client: %s", cur.raw)
	}
	if bad := e.do(t, http.MethodGet, "/game/client-config?platform=ps5&version=1.2.0", "", nil, nil); bad.status/100 != 4 {
		t.Fatalf("unknown platform: %d", bad.status)
	}

	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	r := e.start(t, an, e.device(an), newKey(), map[string]any{"client_platform": "android", "client_version": "1.1.9"})
	if r.status != http.StatusUpgradeRequired || r.code() != string(problem.ClientVersionUnsupported) || r.body["min_client_version"] != MinVersion {
		t.Fatalf("old client session: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_sessions`); n != 0 {
		t.Fatal("session created for an unsupported client")
	}
}

// F014-AC-21: while maintenance is on, client-config says maintenance.active with its message and createGameSession
// answers 503 MAINTENANCE with Retry-After.
func TestMaintenance(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{maintenance: true})
	c := e.do(t, http.MethodGet, "/game/client-config?platform=ios&version=1.2.0", "", nil, nil)
	maint, _ := c.body["maintenance"].(map[string]any)
	if c.status != http.StatusOK || maint["active"] != true || maint["message"] != "Bảo trì định kỳ" {
		t.Fatalf("client-config: %d %s", c.status, c.raw)
	}
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	r := e.start(t, an, e.device(an), newKey(), nil)
	if r.status != http.StatusServiceUnavailable || r.code() != string(problem.Maintenance) || r.header.Get("Retry-After") == "" {
		t.Fatalf("maintenance: %d %v %s", r.status, r.header, r.raw)
	}
}
