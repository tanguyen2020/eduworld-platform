//go:build integration

package integration

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F014-AC-09, AC-11 (TC-AUTH-019, SC-003): one append-only login event per entry (server time, platform, shortened
// device, client version, HMAC ip_hash) and student.game_logged_in in the same transaction; the raw IP is stored
// nowhere (login events, sessions, outbox); the end is written once and nothing else can change.
func TestLoginEventRecorded(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	r := e.start(t, an, e.device(an), newKey(), nil)
	if r.status != http.StatusCreated {
		t.Fatalf("start: %d %s", r.status, r.raw)
	}
	id := r.body["game_session_id"].(string)
	var hash []byte
	var login string
	if err := e.platform.QueryRowContext(ctx, `SELECT ip_hash, login_event_id::text FROM game.login_events WHERE game_session_id = $1
		AND platform = 'windows' AND client_version = '1.2.3' AND device_model = 'Máy tính Windows' AND login_at IS NOT NULL
		AND logout_at IS NULL`, id).Scan(&hash, &login); err != nil {
		t.Fatalf("login event: %v", err)
	}
	if len(hash) != 32 || bytes.Contains(hash, []byte("127.0.0.1")) {
		t.Fatalf("ip_hash %x", hash)
	}
	ps := e.payloads(ctx, t, "student.game_logged_in")
	if len(ps) != 1 || ps[0]["login_event_id"] != login || ps[0]["game_session_id"] != id || ps[0]["tenant_id"] != an.tenant.String() {
		t.Fatalf("student.game_logged_in: %v", ps)
	}
	// No raw IP anywhere (the test client connects from loopback).
	for q, pool := range map[string]querier{
		`SELECT count(*) FROM game.login_events WHERE login_events::text LIKE '%127.0.0.1%' OR login_events::text LIKE '%::1%'`: e.platform,
		`SELECT count(*) FROM game.game_sessions WHERE game_sessions::text LIKE '%127.0.0.1%'`:                                  e.platform,
		`SELECT count(*) FROM infra.outbox_events WHERE envelope::text LIKE '%127.0.0.1%'`:                                      e.relay,
	} {
		if n := count(ctx, t, pool, q); n != 0 {
			t.Fatalf("raw IP found: %s", q)
		}
	}
	// Append-only: no DELETE for the application; the end is written once, other columns never change.
	if _, err := e.app.ExecContext(ctx, deleteAll("game.login_events")); err == nil {
		t.Fatal("game_app may delete login events")
	}
	if _, err := e.platform.ExecContext(ctx, `UPDATE game.login_events SET platform = 'ios' WHERE game_session_id = $1`, id); err == nil {
		t.Fatal("platform of a login event changed")
	}
	if r := e.end(t, an, e.device(an), id); r.status != http.StatusNoContent {
		t.Fatalf("end: %d %s", r.status, r.raw)
	}
	if _, err := e.platform.ExecContext(ctx, `UPDATE game.login_events SET logout_at = now(), end_reason = 'timeout'
		WHERE game_session_id = $1`, id); err == nil || !strings.Contains(err.Error(), "LOGIN_EVENT_CLOSED") {
		t.Fatalf("second close: %v", err)
	}
}

// F014-AC-12 (TC-TEN-001): staff with lms.student_activity.read list the login history of a student of their scope;
// a student of another school of the same tenant (school-limited grant) or of another tenant → 404; safety_moderator
// (no permission) → 403. F014-AC-11: the response has no IP, ip_hash or device_id.
func TestTenantStudentLoginEvents(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	other := e.newStudent(ctx, t, e.tenantA, e.schoolA2, "active")
	dung := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	for _, s := range []student{an, an, other, dung} {
		d := e.device(s)
		r := e.start(t, s, d, newKey(), nil)
		if r.status != http.StatusCreated {
			t.Fatalf("start: %d %s", r.status, r.raw)
		}
		e.end(t, s, d, r.body["game_session_id"].(string))
	}
	adA := e.staff(e.tenantA, "school_admin", authztest.AtSchool(e.schoolA))
	manager := e.staff(e.tenantA, "academic_manager")
	adB := e.staff(e.tenantB, "school_admin", authztest.AtSchool(e.schoolB))
	list := func(sub string, tenant string, s student, extra string) resp {
		tok := e.idp.Token(sub, "eduworld-portal", nowWall())
		return e.do(t, http.MethodGet, "/tenant/students/"+s.id.String()+"/login-events"+extra, tok, map[string]string{"X-Tenant-Id": tenant}, nil)
	}
	r := list(adA, e.tenantA.String(), an, "?limit=1")
	data, _ := r.body["data"].([]any)
	page, _ := r.body["page"].(map[string]any)
	if r.status != http.StatusOK || len(data) != 1 || page["has_more"] != true || page["next_cursor"] == nil {
		t.Fatalf("school admin A: %d %s", r.status, r.raw)
	}
	row := data[0].(map[string]any)
	if row["end_reason"] != "logout" || row["logout_at"] == nil || row["platform"] != "windows" || row["device_label"] != "Máy tính Windows" {
		t.Fatalf("row: %v", row)
	}
	for _, leak := range []string{"device_id", "ip_hash", "127.0.0.1", "\"ip\""} {
		if strings.Contains(r.raw, leak) {
			t.Fatalf("response leaks %s: %s", leak, r.raw)
		}
	}
	r2 := list(adA, e.tenantA.String(), an, "?limit=1&cursor="+page["next_cursor"].(string))
	if d2, _ := r2.body["data"].([]any); r2.status != http.StatusOK || len(d2) != 1 || d2[0].(map[string]any)["login_event_id"] == row["login_event_id"] {
		t.Fatalf("second page: %s", r2.raw)
	}
	if r := list(adA, e.tenantA.String(), other, ""); r.status != http.StatusNotFound {
		t.Fatalf("other school: %d %s", r.status, r.raw)
	}
	if r := list(manager, e.tenantA.String(), other, ""); r.status != http.StatusOK || len(r.body["data"].([]any)) != 1 {
		t.Fatalf("academic manager (tenant-wide), other school: %d %s", r.status, r.raw)
	}
	if r := list(adB, e.tenantB.String(), an, ""); r.status != http.StatusNotFound {
		t.Fatalf("tenant B reading tenant A: %d %s", r.status, r.raw)
	}
	if r := list(adA, e.tenantA.String(), dung, ""); r.status != http.StatusNotFound {
		t.Fatalf("tenant A reading tenant B: %d %s", r.status, r.raw)
	}
	mod := e.platformStaff("safety_moderator")
	tok := e.idp.Token(mod, "eduworld-portal", nowWall())
	if r := e.do(t, http.MethodGet, "/tenant/students/"+an.id.String()+"/login-events", tok, map[string]string{"X-Tenant-Id": e.tenantA.String()}, nil); r.status != http.StatusForbidden {
		t.Fatalf("safety_moderator: %d %s", r.status, r.raw)
	}
	for _, role := range []string{"tenant_content_editor", "tenant_admin"} {
		if r := list(e.staff(e.tenantA, role), e.tenantA.String(), an, ""); r.status != http.StatusForbidden || r.code() != string(problem.PermissionDenied) {
			t.Fatalf("%s: %d %s", role, r.status, r.raw)
		}
	}
	authztest.Cover(t, "listTenantStudentLoginEvents", authztest.CrossTenant, authztest.RoleDenied)
}

// F014-AC-13 (TC-TEN-018): System Admin lists with tenant_ids through the CrossTenantScope (tenant_id on each row,
// audit cross_tenant.accessed); other roles → 403 PERMISSION_DENIED.
func TestLoginEventsCrossTenant(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	dung := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	for _, s := range []student{an, dung} {
		if r := e.start(t, s, e.device(s), newKey(), nil); r.status != http.StatusCreated {
			t.Fatalf("start: %d %s", r.status, r.raw)
		}
	}
	sa := e.idp.Token(e.platformStaff("system_admin"), "eduworld-portal", nowWall())
	r := e.do(t, http.MethodGet, "/admin/login-events?tenant_ids="+e.tenantB.String(), sa, nil, nil)
	data, _ := r.body["data"].([]any)
	if r.status != http.StatusOK || len(data) != 1 || data[0].(map[string]any)["tenant_id"] != e.tenantB.String() ||
		data[0].(map[string]any)["student_id"] != dung.id.String() {
		t.Fatalf("system admin: %d %s", r.status, r.raw)
	}
	if e.audits(ctx, t, "cross_tenant.accessed") == 0 {
		t.Fatal("cross-tenant read not audited")
	}
	// E2E Portal v2 finding 4: without tenant_ids ("Tất cả tenant") System Admin reads every tenant (contract TenantIds:
	// absent = all tenants) through an audited bypass; other roles and tenant users stay 403.
	audited := e.audits(ctx, t, "cross_tenant.accessed")
	r = e.do(t, http.MethodGet, "/admin/login-events", sa, nil, nil)
	data, _ = r.body["data"].([]any)
	seen := map[any]bool{}
	for _, d := range data {
		seen[d.(map[string]any)["tenant_id"]] = true
	}
	if r.status != http.StatusOK || !seen[e.tenantA.String()] || !seen[e.tenantB.String()] {
		t.Fatalf("system admin, every tenant: %d %s", r.status, r.raw)
	}
	if e.audits(ctx, t, "cross_tenant.accessed") <= audited {
		t.Fatal("every-tenant read not audited")
	}
	for _, role := range []string{"safety_moderator", "content_admin"} {
		tok := e.idp.Token(e.platformStaff(role), "eduworld-portal", nowWall())
		if r := e.do(t, http.MethodGet, "/admin/login-events", tok, nil, nil); r.status != http.StatusForbidden {
			t.Fatalf("%s every tenant: %d %s", role, r.status, r.raw)
		}
	}
	if r := e.do(t, http.MethodGet, "/admin/login-events", e.idp.Token(e.staff(e.tenantA, "tenant_admin"), "eduworld-portal", nowWall()), nil, nil); r.status != http.StatusForbidden {
		t.Fatalf("tenant admin every tenant: %d %s", r.status, r.raw)
	}
	for _, role := range []string{"safety_moderator", "content_admin"} {
		tok := e.idp.Token(e.platformStaff(role), "eduworld-portal", nowWall())
		if r := e.do(t, http.MethodGet, "/admin/login-events?tenant_ids="+e.tenantA.String(), tok, nil, nil); r.status != http.StatusForbidden {
			t.Fatalf("%s: %d %s", role, r.status, r.raw)
		}
	}
	tenantAdmin := e.idp.Token(e.staff(e.tenantA, "tenant_admin"), "eduworld-portal", nowWall())
	if r := e.do(t, http.MethodGet, "/admin/login-events?tenant_ids="+e.tenantA.String(), tenantAdmin, nil, nil); r.status != http.StatusForbidden {
		t.Fatalf("tenant admin: %d %s", r.status, r.raw)
	}
	authztest.Cover(t, "listLoginEventsCrossTenant", authztest.RoleDenied)
}
