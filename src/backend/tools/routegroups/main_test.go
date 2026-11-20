package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// A route of gateway both gives a public and an internal group; ${VAR} values do not matter; a wildcard is trimmed.
func TestRender(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "routes.yaml")
	if err := os.WriteFile(f, []byte(`upstreams:
  core-api: ${EW_UPSTREAM_CORE_API}
login_profiles:
  game: { fields: [username], device_count: ${EW_RATE} }
routes:
  - id: a
    gateway: both
    uris: [/api/v1/cms/*, /api/v1/me/permissions]
    upstream: core-api
  - id: b
    gateway: internal
    uris: [/api/v1/admin/*]
    upstream: core-api
`), 0o600); err != nil {
		t.Fatal(err)
	}
	raw, err := Render(f)
	if err != nil {
		t.Fatal(err)
	}
	var got []Group
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 || got[0] != (Group{"/api/v1/cms/", "public", "core-api"}) || got[2] != (Group{"/api/v1/admin/", "internal", "core-api"}) {
		t.Fatalf("groups: %+v", got)
	}
	if err := os.WriteFile(f, []byte("routes:\n  - id: x\n    gateway: lan\n    uris: [/a]\n    upstream: core-api\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Render(f); err == nil {
		t.Fatal("unknown gateway accepted")
	}
}
