// Package wiringtest builds a complete configuration of core-api and the worker profiles for composition-root
// tests (TEST FIXTURES, R17: every value is a local test value, never a secret of an environment).
package wiringtest

import (
	"encoding/base64"
	"path/filepath"
	"strings"
)

// Modules are the module databases of the binaries.
var Modules = []string{"identity", "learning", "game", "competition", "chat"}

// Options are the endpoints of the test: DSN returns the connection of a module role ("app", "platform",
// "relay", "partition_admin"); Issuer is the OIDC issuer (httpxtest.IDP); the others are host:port / URLs.
type Options struct {
	RepoRoot   string
	DSN        func(module, role string) string
	Issuer     string
	ValkeyAddr string
	NATSURL    string
	SecretsDir string
}

// Env returns the variables of LOCAL_DEVELOPMENT §4 needed by core-api and worker (admin/HTTP on random ports).
func Env(o Options) map[string]string {
	env := map[string]string{
		"EW_ENV":                     "test", // module chat accepts its labelled ContentValidator test double only here (F115-AC-06)
		"EW_ADMIN_ADDR":              "127.0.0.1:0",
		"EW_HTTP_ADDR":               "127.0.0.1:0",
		"EW_VALKEY_ADDR":             o.ValkeyAddr,
		"EW_NATS_URLS":               o.NATSURL,
		"EW_PAGE_CURSOR_KEY":         strings.Repeat("c", 32),
		"EW_OIDC_ISSUERS":            o.Issuer,
		"EW_OIDC_AUDIENCE":           "eduworld-api",
		"EW_OIDC_ALLOWED_AZP":        "eduworld-game-backend,eduworld-portal",
		"EW_KC_PORTAL_CLIENT_ID":     "eduworld-portal",
		"EW_KC_PORTAL_CLIENT_SECRET": "test-only",
		"EW_KC_GAME_CLIENT_ID":       "eduworld-game-backend",
		"EW_KC_GAME_CLIENT_SECRET":   "test-only",
		"EW_KC_ADMIN_BASE_URL":       "http://127.0.0.1:1/admin/realms/eduworld",
		"EW_KC_ADMIN_CLIENT_ID":      "svc-identity",
		"EW_KC_ADMIN_CLIENT_SECRET":  "test-only",
		"EW_GAME_MIN_CLIENT_VERSION": "0.1.0",
		"EW_GAME_LOGIN_TICKET_KEY":   base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32))),
		"EW_SMTP_HOST":               "127.0.0.1",
		"EW_SMTP_PORT":               "1025",
		"EW_SMTP_FROM":               "no-reply@eduworld.test",
		"EW_PORTAL_BASE_URL":         "http://localhost:5173",
		"EW_SIS_SECRETS_DIR":         o.SecretsDir,
		"EW_S3_ENDPOINT":             "http://127.0.0.1:1",
		"EW_S3_ACCESS_KEY":           "test",
		"EW_S3_SECRET_KEY":           "test",
		"EW_CDN_BASE_URL":            "http://127.0.0.1:1",
		"EW_CDN_SIGNING_KEY":         "test-only",
		"EW_MEDIA_SCAN":              "disabled",
		"EW_ANALYTICS_PLAYER_KEYS":   "1:test-only-analytics-player-key-0123456789", // F080 (TEST DOUBLE of the secret store)
		"EW_OPENAPI_SPEC":            filepath.Join(o.RepoRoot, "specs", "openapi", "eduworld-api.v1.yaml"),
		"EW_PORTAL_MENU_CATALOG":     filepath.Join(o.RepoRoot, "specs", "portal", "menu-catalog.yaml"),
	}
	for _, m := range Modules {
		up := strings.ToUpper(m)
		env["EW_DB_"+up+"_DSN"] = o.DSN(m, "app")
		env["EW_DB_"+up+"_PLATFORM_DSN"] = o.DSN(m, "platform")
		env["EW_DB_"+up+"_RELAY_DSN"] = o.DSN(m, "relay")
	}
	env["EW_DB_IDENTITY_PARTITION_ADMIN_DSN"] = o.DSN("identity", "partition_admin")
	env["EW_DB_LEARNING_PARTITION_ADMIN_DSN"] = o.DSN("learning", "partition_admin")
	env["EW_DB_LEARNING_DASHBOARD_DSN"] = o.DSN("learning", "dashboard")
	env["EW_DB_LEARNING_DASHBOARD_PLATFORM_DSN"] = o.DSN("learning", "platform")
	env["EW_DB_GAME_DASHBOARD_DSN"] = o.DSN("game", "dashboard")         // F080: Game Analytics reads (game_dashboard)
	env["EW_DB_GAME_DASHBOARD_PLATFORM_DSN"] = o.DSN("game", "platform") // F080: cross-tenant platform reads
	env["EW_DB_CHAT_PURGER_DSN"] = o.DSN("chat", "purger")               // F117: gameplay-chat-purge of worker-chat (ADR-0033)
	return env
}
