package identity

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// F059-2: the overview configuration holds ids, purposes and URLs only — never a secret — and refuses URLs that
// could carry credentials.
func TestIntegrationConfig(t *testing.T) {
	env := map[string]string{
		"EW_KC_PORTAL_CLIENT_ID": "eduworld-portal", "EW_KC_PORTAL_CLIENT_SECRET": "s3cr3t-portal",
		"EW_KC_GAME_CLIENT_ID": "eduworld-game-backend", "EW_KC_GAME_CLIENT_SECRET": "s3cr3t-game",
		"EW_KC_ADMIN_CLIENT_ID": "svc-identity", "EW_KC_ADMIN_CLIENT_SECRET": "s3cr3t-admin",
		"EW_OIDC_PLATFORM_MAX_AUTH_AGE": "4h", "EW_GATEWAY_INTERNAL_BASE_URL": "https://admin-api.example/api/v1",
	}
	cfg := config.FromGetenv(func(k string) string { return env[k] })
	got, err := integrationConfig(cfg, "https://sso.example/realms/eduworld")
	if err != nil {
		t.Fatal(err)
	}
	if got.Realm != "eduworld" || len(got.Clients) != 3 || got.Clients[2].Purpose != "admin_api" || got.PlatformMaxAuthAge != 4*time.Hour ||
		got.PublicBaseURL != "" || got.InternalBaseURL != "https://admin-api.example/api/v1" {
		t.Fatalf("config = %+v", got)
	}
	raw, _ := json.Marshal(got)
	if strings.Contains(string(raw), "s3cr3t") {
		t.Fatalf("a secret reached the overview configuration: %s", raw)
	}
	for name, bad := range map[string]string{
		"user info": "https://user:pass@admin-api.example", "query": "https://admin-api.example/?token=x", "scheme": "ftp://admin-api.example",
		"relative": "/api/v1",
	} {
		env["EW_GATEWAY_PUBLIC_BASE_URL"] = bad
		if _, err := integrationConfig(cfg, "https://sso.example/realms/eduworld"); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	delete(env, "EW_GATEWAY_PUBLIC_BASE_URL")
	if _, err := integrationConfig(cfg, "https://u:p@sso.example/realms/eduworld"); err == nil {
		t.Fatal("issuer with credentials accepted")
	}
}
