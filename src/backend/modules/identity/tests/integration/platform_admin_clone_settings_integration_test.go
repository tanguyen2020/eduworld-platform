//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// OQ F045-4 (T060): tenant_settings and school_settings are cloned by module identity from the settings of F015
// (row version bumped, tenant.updated / school.updated published, settings audit in the target); access hours wider
// than those of the target tenant are dropped; class_template is refused at once (422, no job); nothing about
// students is copied (R22).
func TestPlatformAdminCloneSettings(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	p := platformHTTP(ctx, t, e, nil)
	a := seedTenant(ctx, t, e.platform, "pgd-cs-a")
	b := seedTenant(ctx, t, e.platform, "pgd-cs-b")
	c := seedTenant(ctx, t, e.platform, "pgd-cs-c")
	platformUsers(ctx, t, p, a, "cs")
	sa := p.token("sa.cs")
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := e.platform.ExecContext(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	wide := `{"allowed_hours":{"windows":[{"days":["mon","tue","wed","thu","fri"],"from":"06:00","to":"21:00"}]},"default_locale":"vi-VN"}`
	narrow := `{"allowed_hours":{"windows":[{"days":["mon","tue","wed","thu","fri"],"from":"07:00","to":"17:00"}]}}`
	exec(`UPDATE platform.tenants SET settings = $2::jsonb WHERE tenant_id = $1`, a.tenant, wide)
	exec(`UPDATE platform.tenants SET settings = $2::jsonb WHERE tenant_id = $1`, b.tenant, narrow)
	exec(`UPDATE platform.tenants SET settings = $2::jsonb WHERE tenant_id = $1`, c.tenant, narrow)
	exec(`UPDATE org.schools SET settings = $2::jsonb WHERE school_id = $1`, a.school,
		`{"daily_play_limit_minutes":90,"student_club_creation":true,"allowed_hours":{"windows":[{"days":["mon"],"from":"06:30","to":"20:00"}]}}`)
	clone := func(body map[string]any) httpResp {
		t.Helper()
		return p.do(t, "POST", "/admin/tenant-clones", sa, map[string]string{"Idempotency-Key": newKey()}, body)
	}
	jobs := count(ctx, t, e.platform, `SELECT count(*) FROM platform.tenant_clone_jobs`)
	for name, bad := range map[string]map[string]any{
		"class_template": {"source_tenant_id": a.tenant, "target_tenant_id": b.tenant, "resource_type": "class_template", "resource_ids": []string{ids.New().String()}},
		"tenant ids":     {"source_tenant_id": a.tenant, "target_tenant_id": b.tenant, "resource_type": "tenant_settings", "resource_ids": []string{b.tenant}},
		"two schools": {"source_tenant_id": a.tenant, "target_tenant_id": b.tenant, "resource_type": "school_settings",
			"resource_ids": []string{a.school, a.otherSchool}},
	} {
		if r := clone(bad); r.status != 422 || r.body["code"] != "VALIDATION_FAILED" {
			t.Fatalf("%s: %d %s", name, r.status, r.raw)
		}
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM platform.tenant_clone_jobs`) != jobs {
		t.Fatal("an invalid request created a job")
	}
	succeeded := func(job string) {
		t.Helper()
		if r := p.do(t, "GET", "/admin/tenant-clones/"+job, sa, nil, nil); r.status != 200 || r.body["status"] != "succeeded" {
			t.Fatalf("job %s: %d %s", job, r.status, r.raw)
		}
	}
	// Tenant settings first: the target tenant gets the wide hours and the locale.
	r := clone(map[string]any{"source_tenant_id": a.tenant, "target_tenant_id": b.tenant, "resource_type": "tenant_settings", "resource_ids": []string{a.tenant}})
	if r.status != 202 {
		t.Fatalf("tenant_settings: %d %s", r.status, r.raw)
	}
	p.runSagas(ctx, t)
	succeeded(r.body["job_id"].(string))
	if count(ctx, t, e.platform, `SELECT count(*) FROM platform.tenants WHERE tenant_id = $1 AND settings->>'default_locale' = 'vi-VN'
		AND settings->'allowed_hours'->'windows'->0->>'from' = '06:00'`, b.tenant) != 1 {
		t.Fatal("tenant settings not cloned")
	}
	if outboxCount(ctx, t, p, "tenant.updated", b.tenant) != 1 || outboxCount(ctx, t, p, "audit.recorded", "tenant.settings_updated") != 1 {
		t.Fatal("tenant.updated / audit of the tenant settings clone")
	}
	// School settings onto every school of B (hours within B's new hours: kept) and of C (narrower: dropped).
	for _, target := range []string{b.tenant, c.tenant} {
		r = clone(map[string]any{"source_tenant_id": a.tenant, "target_tenant_id": target, "resource_type": "school_settings", "resource_ids": []string{a.school}})
		if r.status != 202 {
			t.Fatalf("school_settings: %d %s", r.status, r.raw)
		}
		p.runSagas(ctx, t)
		succeeded(r.body["job_id"].(string))
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM org.schools WHERE tenant_id = $1 AND deleted_at IS NULL
		AND (settings->>'daily_play_limit_minutes')::int = 90 AND settings->>'student_club_creation' = 'true' AND settings ? 'allowed_hours'`, b.tenant) != 2 {
		t.Fatal("school settings not cloned onto every school of B")
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM org.schools WHERE tenant_id = $1 AND deleted_at IS NULL
		AND (settings->>'daily_play_limit_minutes')::int = 90 AND NOT settings ? 'allowed_hours'`, c.tenant) != 2 {
		t.Fatal("wider school hours must be dropped for C")
	}
	if outboxCount(ctx, t, p, "school.updated", `"daily_play_limit_minutes": 90`) != 4 ||
		outboxCount(ctx, t, p, "audit.recorded", "school.settings_updated") != 4 {
		t.Fatal("school.updated / audit of the school settings clone")
	}
	for _, target := range []string{b.tenant, c.tenant} {
		if count(ctx, t, e.platform, `SELECT count(*) FROM org.students WHERE tenant_id = $1`, target) != 0 {
			t.Fatal("students in the target")
		}
	}
}
