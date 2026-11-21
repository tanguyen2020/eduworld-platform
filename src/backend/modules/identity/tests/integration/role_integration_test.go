//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/seed"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// F012-AC-16 … AC-19 (TC-CMS-RBAC-01/04/05, TC-TEN-001) through httpx with the real Principal resolution.
func TestRoles(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	h, idp := realAuthHTTP(ctx, t, e)
	a := seedTenant(ctx, t, e.platform, "pgd-role-a")
	b := seedTenant(ctx, t, e.platform, "pgd-role-b")
	now := time.Now()
	link := func(user uuid.UUID, sub string) {
		t.Helper()
		if _, err := e.platform.ExecContext(ctx, `INSERT INTO identity.external_identities (external_identity_id, created_at, updated_at, issuer, subject, user_id)
			VALUES ($1, now(), now(), $2, $3, $4)`, uuid.New(), idp.Issuer(), sub, user); err != nil {
			t.Fatal(err)
		}
	}
	admin := func(name string, tenant *string, role, scope string) string {
		u := staffUser(ctx, t, e, name, tenant)
		link(u, "kc-"+name)
		grantRole(ctx, t, e, u, role, tenant, scope, nil, "NULL")
		return idp.Token("kc-"+name, httpxtest.PortalClient, now)
	}
	taTok := admin("ta.role", &a.tenant, "tenant_admin", "tenant")
	tbTok := admin("tb.role", &b.tenant, "tenant_admin", "tenant")
	saTok := admin("sa.role", nil, "system_admin", "platform")
	key := func(tenant string) map[string]string {
		m := map[string]string{"Idempotency-Key": ids.New().String()}
		if tenant != "" {
			m["X-Tenant-Id"] = tenant
		}
		return m
	}
	hdr := func(tenant string, version any) map[string]string {
		m := map[string]string{"X-Tenant-Id": tenant}
		if version != nil {
			m["If-Match"] = `W/"` + strconv.Itoa(int(version.(float64))) + `"`
		}
		return m
	}

	// Catalogs: tenant permissions never include platform ones.
	if r := h.do(t, "GET", "/tenant/permissions", taTok, hdr(a.tenant, nil), nil); r.status != 200 || strings.Contains(r.raw, `"game.`) ||
		strings.Contains(r.raw, `"scope_kind":"platform"`) || !strings.Contains(r.raw, `"lms.student.read"`) {
		t.Fatalf("tenant permissions: %d", r.status)
	}
	if r := h.do(t, "GET", "/admin/permissions", saTok, nil, nil); r.status != 200 || !strings.Contains(r.raw, `"game.quest.write"`) {
		t.Fatalf("all permissions: %d", r.status)
	}

	// AC-16: a custom tenant role with a platform permission → 422 PERMISSION_NOT_ALLOWED_FOR_SCOPE.
	bad := map[string]any{"code": "gv_bo_mon", "name": map[string]any{"vi": "GV bộ môn"}, "permission_codes": []string{"lms.student.read", "game.quest.write"}}
	if r := h.do(t, "POST", "/tenant/roles", taTok, key(a.tenant), bad); r.status != 422 || !strings.Contains(r.raw, "PERMISSION_NOT_ALLOWED_FOR_SCOPE") {
		t.Fatalf("platform permission in a tenant role: %d %s", r.status, r.raw)
	}
	good := map[string]any{"code": "gv_bo_mon", "name": map[string]any{"vi": "GV bộ môn"}, "permission_codes": []string{"lms.student.read", "lms.assignment.read"}}
	r := h.do(t, "POST", "/tenant/roles", taTok, key(a.tenant), good)
	if r.status != 201 || r.body["tenant_id"] != a.tenant || r.body["role_scope_kind"] != "tenant" || r.body["is_system"] != false {
		t.Fatalf("create custom role: %d %s", r.status, r.raw)
	}
	custom, version := r.body["role_id"].(string), r.body["version"]
	if r := h.do(t, "POST", "/tenant/roles", taTok, key(a.tenant), good); r.status != 409 {
		t.Fatalf("duplicate code: %d %s", r.status, r.raw)
	}
	// AC-19: another tenant does not see it (404) and cannot change it.
	if r := h.do(t, "GET", "/tenant/roles/"+custom, tbTok, hdr(b.tenant, nil), nil); r.status != 404 {
		t.Fatalf("other tenant get: %d", r.status)
	}
	if r := h.do(t, "PATCH", "/tenant/roles/"+custom, tbTok, hdr(b.tenant, version), good); r.status != 404 {
		t.Fatalf("other tenant update: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", "/tenant/roles", taTok, hdr(a.tenant, nil), nil); r.status != 200 || !strings.Contains(r.raw, custom) ||
		!strings.Contains(r.raw, `"school_admin"`) || strings.Contains(r.raw, `"system_admin"`) {
		t.Fatalf("list tenant roles: %d %s", r.status, r.raw)
	}
	// F012 Clarifications 2026-11-22 (E2E Portal v2): a School Admin (grant limited to its school) reads the role list of
	// its tenant for the assignment screen — read-only, grants counted in its school only; another tenant is refused;
	// academic_manager (no tenant.role.read) still gets 403.
	scA := staffUser(ctx, t, e, "sc.role.a", &a.tenant)
	link(scA, "kc-sc.role.a")
	grantRole(ctx, t, e, scA, "school_admin", &a.tenant, "school", &a.school, "NULL")
	scOther := staffUser(ctx, t, e, "sc.role.other", &a.tenant)
	grantRole(ctx, t, e, scOther, "school_admin", &a.tenant, "school", &a.otherSchool, "NULL")
	scTok := idp.Token("kc-sc.role.a", httpxtest.PortalClient, now)
	countOf := func(raw, code string) float64 {
		t.Helper()
		var pg struct {
			Items []map[string]any `json:"items"`
		}
		if err := json.Unmarshal([]byte(raw), &pg); err != nil {
			t.Fatal(err)
		}
		for _, it := range pg.Items {
			if it["code"] == code {
				n, _ := it["active_user_role_count"].(float64)
				return n
			}
		}
		t.Fatalf("role %s not listed: %s", code, raw)
		return 0
	}
	r = h.do(t, "GET", "/tenant/roles?limit=100", scTok, hdr(a.tenant, nil), nil)
	if r.status != 200 || !strings.Contains(r.raw, custom) || strings.Contains(r.raw, `"system_admin"`) || countOf(r.raw, "school_admin") != 1 {
		t.Fatalf("school admin list tenant roles: %d %s", r.status, r.raw)
	}
	if r = h.do(t, "GET", "/tenant/roles?limit=100", taTok, hdr(a.tenant, nil), nil); r.status != 200 || countOf(r.raw, "school_admin") != 2 {
		t.Fatalf("tenant admin counts: %d %s", r.status, r.raw)
	}
	if r = h.do(t, "GET", "/tenant/roles/"+custom, scTok, hdr(a.tenant, nil), nil); r.status != 200 {
		t.Fatalf("school admin get role: %d %s", r.status, r.raw)
	}
	if r = h.do(t, "GET", "/tenant/permissions", scTok, hdr(a.tenant, nil), nil); r.status != 200 || strings.Contains(r.raw, `"game.`) {
		t.Fatalf("school admin permissions: %d", r.status)
	}
	if r = h.do(t, "GET", "/tenant/roles", scTok, hdr(b.tenant, nil), nil); r.status == 200 {
		t.Fatalf("school admin of tenant A read tenant B roles: %d %s", r.status, r.raw)
	}
	if r = h.do(t, "POST", "/tenant/roles", scTok, key(a.tenant), good); r.status != 403 {
		t.Fatalf("school admin create role: %d %s", r.status, r.raw)
	}
	amTok := admin("am.role.a", &a.tenant, "academic_manager", "tenant")
	if r = h.do(t, "GET", "/tenant/roles", amTok, hdr(a.tenant, nil), nil); r.status != 403 {
		t.Fatalf("academic manager list roles: %d %s", r.status, r.raw)
	}

	// AC-17: system roles are read-only (403), via the tenant and the platform routes.
	sys := seed.RoleID("school_admin").String()
	if r := h.do(t, "PATCH", "/tenant/roles/"+sys, taTok, hdr(a.tenant, float64(1)), map[string]any{"code": "school_admin", "name": map[string]any{"vi": "x"},
		"permission_codes": []string{"lms.student.read"}}); r.status != 403 {
		t.Fatalf("update system role: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "DELETE", "/admin/roles/"+sys+"?reason=cleanup", saTok, map[string]string{"If-Match": `W/"1"`}, nil); r.status != 403 {
		t.Fatalf("delete system role: %d %s", r.status, r.raw)
	}

	// AC-18: update with If-Match → role.updated with added/removed; the holder loses the permission.
	gv := staffUser(ctx, t, e, "gv.role", &a.tenant)
	link(gv, "kc-gv.role")
	if r := h.do(t, "POST", "/tenant/user-roles", taTok, key(a.tenant), map[string]any{"user_id": gv.String(), "role_id": custom, "scope_type": "tenant"}); r.status != 201 {
		t.Fatalf("grant custom role: %d %s", r.status, r.raw)
	}
	gvTok := idp.Token("kc-gv.role", httpxtest.PortalClient, now)
	if p := h.do(t, "GET", "/me/permissions", gvTok, hdr(a.tenant, nil), nil); !strings.Contains(p.raw, `"lms.assignment.read"`) {
		t.Fatalf("custom role permissions: %s", p.raw)
	}
	upd := map[string]any{"code": "gv_bo_mon", "name": map[string]any{"vi": "GV bộ môn"}, "permission_codes": []string{"lms.student.read", "lms.report.read"}}
	if r := h.do(t, "PATCH", "/tenant/roles/"+custom, taTok, map[string]string{"X-Tenant-Id": a.tenant}, upd); r.status != 428 {
		t.Fatalf("no If-Match: %d", r.status)
	}
	if r := h.do(t, "PATCH", "/tenant/roles/"+custom, taTok, hdr(a.tenant, float64(99)), upd); r.status != 412 {
		t.Fatalf("stale version: %d %s", r.status, r.raw)
	}
	r = h.do(t, "PATCH", "/tenant/roles/"+custom, taTok, hdr(a.tenant, version), upd)
	if r.status != 200 || r.header.Get("ETag") == "" {
		t.Fatalf("update: %d %s", r.status, r.raw)
	}
	version = r.body["version"]
	if p := h.do(t, "GET", "/me/permissions", gvTok, hdr(a.tenant, nil), nil); strings.Contains(p.raw, `"lms.assignment.read"`) ||
		!strings.Contains(p.raw, `"lms.report.read"`) {
		t.Fatalf("permissions after update: %s", p.raw)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'role.updated' AND envelope->'payload'->>'op' = 'updated'
		AND envelope->'payload'->'permissions_added' ? 'lms.report.read' AND envelope->'payload'->'permissions_removed' ? 'lms.assignment.read'`); n != 1 {
		t.Fatalf("role.updated = %d", n)
	}
	// AC-17: a role still granted cannot be deleted (409); after revocation it can.
	if r := h.do(t, "DELETE", "/tenant/roles/"+custom+"?reason=cleanup", taTok, hdr(a.tenant, version), nil); r.status != 409 {
		t.Fatalf("delete granted role: %d %s", r.status, r.raw)
	}
	if _, err := e.platform.ExecContext(ctx, `UPDATE identity.user_roles SET deleted_at = now() WHERE role_id = $1`, custom); err != nil {
		t.Fatal(err)
	}
	if r := h.do(t, "DELETE", "/tenant/roles/"+custom+"?reason=cleanup", taTok, hdr(a.tenant, version), nil); r.status != 204 {
		t.Fatalf("delete: %d %s", r.status, r.raw)
	}

	// Platform route: a shared platform role (reason required to update) and listing with tenant_ids.
	pr := map[string]any{"code": "support_l1", "name": map[string]any{"vi": "Hỗ trợ L1"}, "role_scope_kind": "platform",
		"permission_codes": []string{"platform.audit.read"}}
	r = h.do(t, "POST", "/admin/roles", saTok, key(""), pr)
	if r.status != 201 || r.body["tenant_id"] != nil || r.body["role_scope_kind"] != "platform" {
		t.Fatalf("create platform role: %d %s", r.status, r.raw)
	}
	pid := r.body["role_id"].(string)
	if r := h.do(t, "PATCH", "/admin/roles/"+pid, saTok, map[string]string{"If-Match": `W/"1"`}, pr); r.status != 422 {
		t.Fatalf("platform update without reason: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", "/admin/roles/"+pid, saTok, nil, nil); r.status != 200 {
		t.Fatalf("get platform role: %d", r.status)
	}
	if r := h.do(t, "GET", "/admin/roles?role_scope_kind=platform", saTok, nil, nil); r.status != 200 || !strings.Contains(r.raw, pid) {
		t.Fatalf("list platform roles: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "POST", "/admin/roles", taTok, key(""), pr); r.status != 403 {
		t.Fatalf("tenant admin on admin roles: %d %s", r.status, r.raw)
	}
}
