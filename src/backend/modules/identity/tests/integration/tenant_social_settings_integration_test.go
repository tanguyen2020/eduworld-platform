//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// F015 T044 (A-1 of F095): updateTenantSettings writes cross_school_social_enabled of a unit tenant and
// tenant.updated carries settings_subset with both social keys and source_version, valid against the payload
// schema, so ref.tenants.settings_subset of module chat follows (chat side: projection_test of module chat).
func TestTenantSocialSettingsSubset(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	h, idp := realAuthHTTP(ctx, t, e)
	a := seedTenant(ctx, t, e.platform, "pgd-soc-a")
	b := seedTenant(ctx, t, e.platform, "th-soc-b")
	if _, err := e.platform.ExecContext(ctx, `UPDATE platform.tenants SET tenant_type = 'school' WHERE tenant_id = $1`, b.tenant); err != nil {
		t.Fatal(err)
	}
	iss := idp.Issuer()
	portalUser(ctx, t, e, iss, "ad.donvi.soc", "tenant_admin", &a.tenant, "tenant", nil)
	portalUser(ctx, t, e, iss, "ad.truong.soc", "tenant_admin", &b.tenant, "tenant", nil)
	now := time.Now()
	tok := func(user string) string { return idp.Token("sub-"+user, httpxtest.PortalClient, now) }
	headers := func(tenant, etag string) map[string]string {
		return map[string]string{"X-Tenant-Id": tenant, "Content-Type": "application/merge-patch+json", "If-Match": etag}
	}
	get := func(user, tenant string) (map[string]any, string) {
		t.Helper()
		r := h.do(t, "GET", "/tenant/settings", tok(user), map[string]string{"X-Tenant-Id": tenant}, nil)
		if r.status != 200 {
			t.Fatalf("get settings: %d %v", r.status, r.body)
		}
		return r.body, r.header.Get("ETag")
	}

	schema := func() *jsonschema.Schema {
		c := jsonschema.NewCompiler()
		c.AssertFormat()
		s, err := c.Compile(filepath.Join(testkit.RepoRoot(t), "specs", "events", "payloads", "identity", "tenant.updated.v1.schema.json"))
		if err != nil {
			t.Fatal(err)
		}
		return s
	}()
	// lastPayload returns the newest tenant.updated payload of tenant after checking it against the schema.
	lastPayload := func(tenant string) map[string]any {
		t.Helper()
		var raw []byte
		if err := e.relay.QueryRowContext(ctx, `SELECT envelope->'payload' FROM infra.outbox_events WHERE event_type = 'tenant.updated'
			AND tenant_id = $1 ORDER BY (envelope->'payload'->>'source_version')::bigint DESC NULLS LAST LIMIT 1`, tenant).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(doc); err != nil {
			t.Fatalf("tenant.updated payload: %v (%s)", err, raw)
		}
		return doc.(map[string]any)
	}
	version := func(p map[string]any) int64 {
		t.Helper()
		n, _ := p["source_version"].(json.Number)
		v, err := n.Int64()
		if err != nil {
			t.Fatalf("source_version: %v", p)
		}
		return v
	}
	subset := func(p map[string]any) map[string]any {
		t.Helper()
		s, ok := p["settings_subset"].(map[string]any)
		if !ok {
			t.Fatalf("settings_subset missing: %v", p)
		}
		return s
	}

	// Default off; the unit tenant turns cross-school social on.
	body, etag := get("ad.donvi.soc", a.tenant)
	if body["cross_school_social_enabled"] != false {
		t.Fatalf("default: %v", body)
	}
	r := h.do(t, "PATCH", "/tenant/settings", tok("ad.donvi.soc"), headers(a.tenant, etag), map[string]any{"cross_school_social_enabled": true})
	if r.status != 200 || r.body["cross_school_social_enabled"] != true {
		t.Fatalf("enable: %d %v", r.status, r.body)
	}
	p := lastPayload(a.tenant)
	if s := subset(p); s["cross_school_social_enabled"] != true || s["common_social_enabled"] != false || len(s) != 2 {
		t.Fatalf("subset on: %v", s)
	}
	v1 := version(p)
	if p["tenant_type"] != "unit" || v1 <= 0 {
		t.Fatalf("payload: %v", p)
	}

	// Another key keeps the subset complete; turning it off again (null = default off) publishes false.
	_, etag = get("ad.donvi.soc", a.tenant)
	if r = h.do(t, "PATCH", "/tenant/settings", tok("ad.donvi.soc"), headers(a.tenant, etag), map[string]any{"default_locale": "en-US"}); r.status != 200 {
		t.Fatalf("locale: %d %v", r.status, r.body)
	}
	if s := subset(lastPayload(a.tenant)); s["cross_school_social_enabled"] != true {
		t.Fatalf("subset after another key: %v", s)
	}
	_, etag = get("ad.donvi.soc", a.tenant)
	if r = h.do(t, "PATCH", "/tenant/settings", tok("ad.donvi.soc"), headers(a.tenant, etag), map[string]any{"cross_school_social_enabled": nil}); r.status != 200 ||
		r.body["cross_school_social_enabled"] != false {
		t.Fatalf("disable: %d %v", r.status, r.body)
	}
	p2 := lastPayload(a.tenant)
	if s := subset(p2); s["cross_school_social_enabled"] != false {
		t.Fatalf("subset off: %v", s)
	}
	if v2 := version(p2); v2 <= v1 {
		t.Fatalf("source_version not increasing: %v -> %v", v1, p2["source_version"])
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'tenant.updated' AND tenant_id = $1`, a.tenant); n != 3 {
		t.Fatalf("tenant.updated events: %d", n)
	}

	// A school tenant cannot turn it on (422); common_social_enabled is not a tenant setting (System Admin, F120).
	_, etag = get("ad.truong.soc", b.tenant)
	if r = h.do(t, "PATCH", "/tenant/settings", tok("ad.truong.soc"), headers(b.tenant, etag), map[string]any{"cross_school_social_enabled": true}); r.status != 422 {
		t.Fatalf("school tenant: %d %v", r.status, r.body)
	}
	_, etag = get("ad.donvi.soc", a.tenant)
	if r = h.do(t, "PATCH", "/tenant/settings", tok("ad.donvi.soc"), headers(a.tenant, etag), map[string]any{"common_social_enabled": true}); r.status != 422 {
		t.Fatalf("common_social_enabled by a tenant admin: %d %v", r.status, r.body)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'tenant.updated' AND tenant_id = $1`, b.tenant); n != 0 {
		t.Fatalf("tenant.updated of a refused patch: %d", n)
	}
}
