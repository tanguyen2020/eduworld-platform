//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
)

// F015 US3: F015-AC-11..AC-15, AC-21 through the httpx chain with the real Principal resolution.
func TestPlatformAndTenantSettings(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	h, idp := realAuthHTTP(ctx, t, e)
	a := seedTenant(ctx, t, e.platform, "pgd-set-a")
	b := seedTenant(ctx, t, e.platform, "pgd-set-b")
	iss := idp.Issuer()
	portalUser(ctx, t, e, iss, "sa.set", "system_admin", nil, "platform", nil)
	portalUser(ctx, t, e, iss, "ad.donvi.set", "tenant_admin", &a.tenant, "tenant", nil)
	portalUser(ctx, t, e, iss, "ad.truong.set", "school_admin", &a.tenant, "school", &a.school)
	now := time.Now()
	tok := func(user string) string { return idp.Token("sub-"+user, httpxtest.PortalClient, now) }
	sa := tok("sa.set")
	ifm := func(etag string) map[string]string { return map[string]string{"If-Match": etag} }
	events := func(key string) int {
		return count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'platform_settings.changed' AND envelope::text LIKE '%"key": "`+key+`"%'`)
	}

	// AC-21: curfew not configured → CONFIG_DEFAULTS value, is_default; write with reason + If-Match.
	r := h.do(t, "GET", "/admin/settings/curfew", sa, nil, nil)
	if r.status != 200 || r.body["start"] != "21:30" || r.body["is_default"] != true || r.header.Get("ETag") != `W/"0"` {
		t.Fatalf("curfew default: %d %v %s", r.status, r.body, r.header.Get("ETag"))
	}
	if r = h.do(t, "PUT", "/admin/settings/curfew", sa, nil, map[string]any{"start": "21:00", "end": "06:30", "reason": "Siết giờ"}); r.status != 428 {
		t.Fatalf("no If-Match: %d", r.status)
	}
	r = h.do(t, "PUT", "/admin/settings/curfew", sa, ifm(`W/"0"`), map[string]any{"start": "21:00", "end": "06:30", "reason": "Siết giờ"})
	if r.status != 200 || r.header.Get("ETag") != `W/"1"` {
		t.Fatalf("put curfew: %d %v", r.status, r.body)
	}
	if r = h.do(t, "PUT", "/admin/settings/curfew", sa, ifm(`W/"0"`), map[string]any{"start": "20:00", "end": "06:30", "reason": "Lần hai"}); r.status != 412 {
		t.Fatalf("stale version: %d", r.status)
	}
	if r = h.do(t, "PUT", "/admin/settings/curfew", sa, ifm(`W/"1"`), map[string]any{"start": "21:00", "end": "21:00", "reason": "Sai"}); r.status != 422 {
		t.Fatalf("empty window: %d", r.status)
	}
	if r = h.do(t, "GET", "/admin/settings/curfew", sa, nil, nil); r.body["start"] != "21:00" || r.body["is_default"] != false {
		t.Fatalf("curfew after put: %v", r.body)
	}
	if events("platform_curfew") != 1 || count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope::text LIKE '%platform_settings.updated%'`) != 1 {
		t.Fatal("platform_settings.changed / audit of the curfew")
	}
	// Tenant roles cannot write platform settings.
	if r = h.do(t, "PUT", "/admin/settings/curfew", tok("ad.donvi.set"), ifm(`W/"1"`), map[string]any{"start": "20:00", "end": "06:30", "reason": "x x x"}); r.status != 403 {
		t.Fatalf("tenant admin on platform curfew: %d", r.status)
	}

	// AC-11, AC-12: chat retention below the minimum → RETENTION_BELOW_MINIMUM; a valid value is stored.
	r = h.do(t, "PUT", "/admin/settings/chat-retention", sa, ifm(`W/"0"`), map[string]any{"gameplay_chat_retention_days": 2,
		"ephemeral_retention_hours": 24, "allow_tenant_override": true, "reason": "Thử"})
	if r.status != 422 || (r.body["code"] != "RETENTION_BELOW_MINIMUM" && r.body["code"] != "VALIDATION_FAILED") {
		t.Fatalf("below minimum: %d %v", r.status, r.body)
	}
	// Student cleanup: the platform forbids tenant overrides.
	r = h.do(t, "PUT", "/admin/settings/student-cleanup", sa, ifm(`W/"0"`), map[string]any{"default_days": 180, "notice_days": 14,
		"allow_tenant_override": false, "approval_threshold_percent": 10, "reason": "Khóa ghi đè"})
	if r.status != 200 || events("student_cleanup") != 1 {
		t.Fatalf("student cleanup: %d %v", r.status, r.body)
	}

	// AC-13: overriding the cleanup days while the platform forbids it → 403.
	inA := map[string]string{"X-Tenant-Id": a.tenant, "Content-Type": "application/merge-patch+json"}
	r = h.do(t, "GET", "/tenant/settings", tok("ad.donvi.set"), map[string]string{"X-Tenant-Id": a.tenant}, nil)
	etag := r.header.Get("ETag")
	if r.status != 200 || r.body["effective_cleanup_days"] != float64(180) || etag == "" {
		t.Fatalf("tenant settings: %d %v", r.status, r.body)
	}
	withIf := func(m map[string]string, v string) map[string]string {
		out := map[string]string{"If-Match": v}
		for k, x := range m {
			out[k] = x
		}
		return out
	}
	if r = h.do(t, "PATCH", "/tenant/settings", tok("ad.donvi.set"), withIf(inA, etag), map[string]any{"inactive_student_cleanup_days": 90}); r.status != 403 {
		t.Fatalf("override forbidden: %d %v", r.status, r.body)
	}
	// Allowed again → stored, effective value follows (AC-14), tenant.updated published.
	r = h.do(t, "PUT", "/admin/settings/student-cleanup", sa, ifm(`W/"1"`), map[string]any{"default_days": 180, "notice_days": 14,
		"allow_tenant_override": true, "approval_threshold_percent": 10, "reason": "Mở ghi đè"})
	if r.status != 200 {
		t.Fatalf("reopen overrides: %d %v", r.status, r.body)
	}
	r = h.do(t, "PATCH", "/tenant/settings", tok("ad.donvi.set"), withIf(inA, etag), map[string]any{"inactive_student_cleanup_days": 90,
		"gameplay_chat_retention_days": 5, "allowed_hours": map[string]any{"windows": []any{map[string]any{"days": []string{"mon", "tue"}, "from": "06:00", "to": "21:00"}}}})
	if r.status != 200 || r.body["effective_cleanup_days"] != float64(90) || r.body["gameplay_chat_retention_days"] != float64(5) {
		t.Fatalf("patch: %d %v", r.status, r.body)
	}
	if count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'tenant.updated' AND tenant_id = $1`, a.tenant) != 1 {
		t.Fatal("tenant.updated")
	}
	// AC-15: stale If-Match → 412; chat below minimum → 422; School Admin lacks tenant.settings.write → 403.
	if r = h.do(t, "PATCH", "/tenant/settings", tok("ad.donvi.set"), withIf(inA, etag), map[string]any{"default_locale": "en-US"}); r.status != 412 {
		t.Fatalf("stale: %d", r.status)
	}
	r = h.do(t, "GET", "/tenant/settings", tok("ad.donvi.set"), map[string]string{"X-Tenant-Id": a.tenant}, nil)
	etag = r.header.Get("ETag")
	if r = h.do(t, "PATCH", "/tenant/settings", tok("ad.donvi.set"), withIf(inA, etag), map[string]any{"gameplay_chat_retention_days": 2}); r.status != 422 {
		t.Fatalf("chat below minimum: %d", r.status)
	}
	if r = h.do(t, "PATCH", "/tenant/settings", tok("ad.truong.set"), withIf(inA, etag), map[string]any{"default_locale": "en-US"}); r.status != 403 {
		t.Fatalf("school admin on tenant settings: %d", r.status)
	}
	// Another tenant: the tenant admin of A cannot use X-Tenant-Id of B.
	if r = h.do(t, "GET", "/tenant/settings", tok("ad.donvi.set"), map[string]string{"X-Tenant-Id": b.tenant}, nil); r.status != 403 {
		t.Fatalf("other tenant: %d", r.status)
	}
}
