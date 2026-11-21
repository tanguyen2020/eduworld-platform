//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/gameauth"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/settingscfg"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F015 US4: F015-AC-16, AC-17, AC-19, AC-22 (school settings over HTTP).
func TestSchoolSettings(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	h, idp := realAuthHTTP(ctx, t, e)
	a := seedTenant(ctx, t, e.platform, "pgd-ss-a")
	b := seedTenant(ctx, t, e.platform, "pgd-ss-b")
	portalUser(ctx, t, e, idp.Issuer(), "ad.truong.ss", "school_admin", &a.tenant, "school", &a.school)
	tok := idp.Token("sub-ad.truong.ss", httpxtest.PortalClient, time.Now())
	// Tenant hours Mon–Fri 06:00–21:00.
	if _, err := e.platform.ExecContext(ctx, `UPDATE platform.tenants SET settings = '{"allowed_hours":{"windows":[{"days":["mon","tue","wed","thu","fri"],"from":"06:00","to":"21:00"}]}}'
		WHERE tenant_id = $1`, a.tenant); err != nil {
		t.Fatal(err)
	}
	path := "/schools/" + a.school + "/settings"
	hdr := func(etag string) map[string]string {
		return map[string]string{"X-Tenant-Id": a.tenant, "Content-Type": "application/merge-patch+json", "If-Match": etag}
	}
	r := h.do(t, "GET", path, tok, map[string]string{"X-Tenant-Id": a.tenant}, nil)
	pc, _ := r.body["platform_curfew"].(map[string]any)
	if r.status != 200 || pc["start"] != "21:30" || r.body["curfew_window"] != nil {
		t.Fatalf("get: %d %v", r.status, r.body)
	}
	etag := r.header.Get("ETag")
	// AC-22: a window narrower than platform_curfew → 422; without reason → 422; containing it → stored, union.
	if r = h.do(t, "PATCH", path, tok, hdr(etag), map[string]any{"curfew_window": map[string]string{"start": "22:00", "end": "06:30"}, "reason": "Thử hẹp"}); r.status != 422 {
		t.Fatalf("narrower curfew: %d", r.status)
	}
	if r = h.do(t, "PATCH", path, tok, hdr(etag), map[string]any{"curfew_window": map[string]string{"start": "21:00", "end": "06:30"}}); r.status != 422 {
		t.Fatalf("curfew without reason: %d", r.status)
	}
	r = h.do(t, "PATCH", path, tok, hdr(etag), map[string]any{"curfew_window": map[string]string{"start": "21:00", "end": "06:30"}, "reason": "Siết giờ ngủ"})
	eff, _ := r.body["effective_curfew"].(map[string]any)
	if r.status != 200 || eff["start"] != "21:00" {
		t.Fatalf("curfew: %d %v", r.status, r.body)
	}
	etag = r.header.Get("ETag")
	// AC-19: hours wider than the tenant hours → 422; within → stored with school.updated.
	wide := map[string]any{"windows": []any{map[string]any{"days": []string{"sat"}, "from": "07:00", "to": "17:00"}}}
	if r = h.do(t, "PATCH", path, tok, hdr(etag), map[string]any{"allowed_hours": wide}); r.status != 422 {
		t.Fatalf("wider hours: %d", r.status)
	}
	within := map[string]any{"windows": []any{map[string]any{"days": []string{"mon", "tue", "wed", "thu", "fri"}, "from": "07:00", "to": "17:00"}}}
	if r = h.do(t, "PATCH", path, tok, hdr(etag), map[string]any{"allowed_hours": within, "student_club_creation": true}); r.status != 200 || r.body["student_club_creation"] != true {
		t.Fatalf("hours: %d %v", r.status, r.body)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'school.updated' AND tenant_id = $1`, a.tenant); n != 2 {
		t.Fatalf("school.updated = %d", n)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'platform_settings.changed' AND envelope::text LIKE '%curfew_window%'`); n != 1 {
		t.Fatal("platform_settings.changed scope school")
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded' AND envelope::text LIKE '%school.settings_updated%'
		AND envelope::text LIKE '%`+a.school+`%'`); n != 2 {
		t.Fatal("audit with school_id")
	}
	// AC-30: daily_play_limit_minutes off by default (null); out of 15..300 → 422; without reason → 422; set with a
	// reason → stored, audited and carried in school.updated; null turns it off again.
	etag = r.header.Get("ETag")
	if r = h.do(t, "GET", path, tok, map[string]string{"X-Tenant-Id": a.tenant}, nil); r.status != 200 || r.body["daily_play_limit_minutes"] != nil {
		t.Fatalf("limit default: %d %v", r.status, r.body)
	}
	if _, present := r.body["daily_play_limit_minutes"]; !present {
		t.Fatal("daily_play_limit_minutes must be present (null)")
	}
	for _, bad := range []any{0, 14, 301, "120", 12.5} {
		if r = h.do(t, "PATCH", path, tok, hdr(etag), map[string]any{"daily_play_limit_minutes": bad, "reason": "Giới hạn chơi"}); r.status != 422 {
			t.Fatalf("limit %v: %d", bad, r.status)
		}
	}
	if r = h.do(t, "PATCH", path, tok, hdr(etag), map[string]any{"daily_play_limit_minutes": 120}); r.status != 422 {
		t.Fatalf("limit without reason: %d", r.status)
	}
	r = h.do(t, "PATCH", path, tok, hdr(etag), map[string]any{"daily_play_limit_minutes": 120, "reason": "Giới hạn chơi mỗi ngày"})
	if r.status != 200 || r.body["daily_play_limit_minutes"] != float64(120) {
		t.Fatalf("limit: %d %v", r.status, r.body)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'school.updated' AND tenant_id = $1
		AND envelope::text LIKE '%"daily_play_limit_minutes": 120%'`, a.tenant); n != 1 {
		t.Fatalf("school.updated with the limit = %d", n)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope::text LIKE '%school.settings_updated%' AND envelope::text LIKE '%Giới hạn chơi mỗi ngày%'`); n != 1 {
		t.Fatal("audit with the reason")
	}
	if r = h.do(t, "PATCH", path, tok, hdr(r.header.Get("ETag")), map[string]any{"daily_play_limit_minutes": nil, "reason": "Tắt giới hạn"}); r.status != 200 ||
		r.body["daily_play_limit_minutes"] != nil {
		t.Fatalf("limit off: %d %v", r.status, r.body)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'school.updated' AND tenant_id = $1
		AND envelope::text LIKE '%"daily_play_limit_minutes": null%' AND envelope::text LIKE '%settings.daily_play_limit_minutes%'`, a.tenant); n != 1 {
		t.Fatalf("school.updated with the limit off = %d", n)
	}
	// AC-17: another school of the tenant → 403; a school of another tenant → 404.
	if r = h.do(t, "GET", "/schools/"+a.otherSchool+"/settings", tok, map[string]string{"X-Tenant-Id": a.tenant}, nil); r.status != 403 {
		t.Fatalf("other school: %d", r.status)
	}
	if r = h.do(t, "PATCH", "/schools/"+b.school+"/settings", tok, map[string]string{"X-Tenant-Id": a.tenant, "Content-Type": "application/merge-patch+json",
		"If-Match": `W/"1"`}, map[string]any{"student_club_creation": true}); r.status != 403 && r.status != 404 {
		t.Fatalf("school of another tenant: %d", r.status)
	}
}

// F015-AC-18, AC-22 at the game login: outside the school hours → ACCESS_OUTSIDE_ALLOWED_HOURS; the school
// curfew (union with platform_curfew) applies through the settings policy.
func TestGameLoginSchoolPolicy(t *testing.T) {
	ctx := context.Background()
	g := newGameIntegration(ctx, t, gameOpts{})
	defaults, err := settingscfg.DefaultsFrom(config.Load())
	if err != nil {
		t.Fatal(err)
	}
	policy := gameauth.SettingsPolicy{Reader: read.SettingsModel{Session: g.read}, Defaults: defaults}
	g.auth.Hours = policy
	g.auth.Curfew = policy
	a := seedTenant(ctx, t, g.platform, "pgd-gp-a")
	g.student(ctx, t, a, "hs.gio")
	ict := time.FixedZone("ICT", 7*3600)
	now := time.Now().In(ict)
	m := now.Hour()*60 + now.Minute()
	inPlatformCurfew := m >= 21*60+30 || m < 6*60+30
	if !inPlatformCurfew {
		// Hours that exclude now (two hours later, one hour long) → refused before the curfew.
		from := (m + 120) % 1440
		to := from + 60
		if to >= 1440 {
			from, to = 0, 60
		}
		hours := fmt.Sprintf(`{"allowed_hours":{"windows":[{"days":["mon","tue","wed","thu","fri","sat","sun"],"from":"%02d:%02d","to":"%02d:%02d"}]}}`,
			from/60, from%60, to/60, to%60)
		if _, err := g.platform.ExecContext(ctx, `UPDATE org.schools SET settings = $2::jsonb WHERE school_id = $1`, a.school, hours); err != nil {
			t.Fatal(err)
		}
		r := g.login(t, "hs.gio", uuid.New())
		if r.code != 403 || codeOf(r) != string(problem.AccessOutsideAllowedHours) {
			t.Fatalf("outside hours: %d %v", r.code, r.body)
		}
		if n := count(ctx, t, g.platform, `SELECT count(*) FROM identity.login_events WHERE result = 'access_outside_allowed_hours'`); n != 1 {
			t.Fatal("journal result")
		}
		// A school curfew from 30 minutes ago to 06:30 contains platform_curfew and now → CURFEW_ACTIVE.
		start := (m + 1440 - 30) % 1440
		curfew := fmt.Sprintf(`{"curfew_window":{"start":"%02d:%02d","end":"06:30"}}`, start/60, start%60)
		if _, err := g.platform.ExecContext(ctx, `UPDATE org.schools SET settings = $2::jsonb WHERE school_id = $1`, a.school, curfew); err != nil {
			t.Fatal(err)
		}
	}
	r := g.login(t, "hs.gio", uuid.New())
	if r.code != 403 || codeOf(r) != string(problem.CurfewActive) || r.body["reopen_at"] == nil {
		t.Fatalf("school curfew: %d %v", r.code, r.body)
	}
	// Without school settings and outside the curfew the login works and shows the effective curfew.
	if !inPlatformCurfew {
		if _, err := g.platform.ExecContext(ctx, `UPDATE org.schools SET settings = '{}' WHERE school_id = $1`, a.school); err != nil {
			t.Fatal(err)
		}
		r = g.login(t, "hs.gio", uuid.New())
		if r.code != 200 {
			t.Fatalf("login: %d %v", r.code, r.body)
		}
	}
}
