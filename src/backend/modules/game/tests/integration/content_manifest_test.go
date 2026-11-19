//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// F014-AC-22: a tenant without published content gets an empty manifest (manifest_version 0, items []) with ETag and
// min_client_version; the latest stored manifest is returned otherwise. F014-AC-23 (TC-TEN-001): If-None-Match equal to
// the current ETag → 304; another tenant's manifest is never returned.
func TestContentManifest(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	dung := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	get := func(s student, inm string) resp {
		h := map[string]string{"X-Tenant-Id": s.tenant.String()}
		if inm != "" {
			h["If-None-Match"] = inm
		}
		return e.do(t, http.MethodGet, "/game/content-manifest", e.device(s).token, h, nil)
	}
	r := get(an, "")
	if r.status != http.StatusOK || r.body["manifest_version"] != float64(0) || r.body["tenant_id"] != an.tenant.String() ||
		len(r.body["items"].([]any)) != 0 || r.body["min_client_version"] != MinVersion || r.header.Get("ETag") == "" {
		t.Fatalf("empty manifest: %d %v %s", r.status, r.header, r.raw)
	}
	if r2 := get(an, r.header.Get("ETag")); r2.status != http.StatusNotModified {
		t.Fatalf("If-None-Match: %d", r2.status)
	}
	// Tenant B publishes a manifest (as the release of F026 would); tenant A still sees its empty one.
	if _, err := e.platform.ExecContext(ctx, `INSERT INTO game.content_manifests (tenant_id, manifest_version, etag, entries)
		VALUES ($1, 1, 'b-v1', '{"items":[{"item_type":"quest","item_id":"q1"}]}'), ($1, 2, 'b-v2', '{"items":[{"item_type":"quest","item_id":"q2"}],"bundles":[]}')`,
		e.tenantB); err != nil {
		t.Fatal(err)
	}
	if r3 := get(an, r.header.Get("ETag")); r3.status != http.StatusNotModified {
		t.Fatalf("tenant A after tenant B release: %d %s", r3.status, r3.raw)
	}
	rb := get(dung, "")
	items, _ := rb.body["items"].([]any)
	if rb.status != http.StatusOK || rb.body["manifest_version"] != float64(2) || rb.body["etag"] != "b-v2" || len(items) != 1 ||
		items[0].(map[string]any)["item_id"] != "q2" || rb.header.Get("ETag") != `"b-v2"` {
		t.Fatalf("tenant B manifest: %d %s", rb.status, rb.raw)
	}
	if r4 := get(dung, `"b-v1"`); r4.status != http.StatusOK {
		t.Fatalf("stale ETag: %d", r4.status)
	}
	if r5 := get(dung, `W/"b-v2"`); r5.status != http.StatusNotModified {
		t.Fatalf("current ETag: %d", r5.status)
	}
	// X-Tenant-Id of another tenant never reaches the manifest of that tenant.
	h := map[string]string{"X-Tenant-Id": e.tenantB.String()}
	if r6 := e.do(t, http.MethodGet, "/game/content-manifest", e.device(an).token, h, nil); r6.status != http.StatusForbidden {
		t.Fatalf("cross tenant: %d %s", r6.status, r6.raw)
	}
	authztest.Cover(t, "getContentManifest", authztest.CrossTenant)
}
