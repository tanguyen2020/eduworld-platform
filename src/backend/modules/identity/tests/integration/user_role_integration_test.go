//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/seed"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// F012-AC-20 … AC-24 (TC-AUTH-077, TC-CMS-RBAC-02/03/06) through httpx with the real Principal resolution.
func TestUserRoleGrants(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	h, idp := realAuthHTTP(ctx, t, e)
	a := seedTenant(ctx, t, e.platform, "pgd-ur-a")
	b := seedTenant(ctx, t, e.platform, "pgd-ur-b")
	now := time.Now()
	link := func(user uuid.UUID, sub string) {
		t.Helper()
		if _, err := e.platform.ExecContext(ctx, `INSERT INTO identity.external_identities (external_identity_id, created_at, updated_at, issuer, subject, user_id)
			VALUES ($1, now(), now(), $2, $3, $4)`, uuid.New(), idp.Issuer(), sub, user); err != nil {
			t.Fatal(err)
		}
	}
	key := func(tenant string) map[string]string {
		m := map[string]string{"Idempotency-Key": ids.New().String()}
		if tenant != "" {
			m["X-Tenant-Id"] = tenant
		}
		return m
	}
	hdr := func(tenant string) map[string]string { return map[string]string{"X-Tenant-Id": tenant} }

	ta := staffUser(ctx, t, e, "ta.ur", &a.tenant) // tenant admin of A
	link(ta, "kc-ta")
	grantRole(ctx, t, e, ta, "tenant_admin", &a.tenant, "tenant", nil, "NULL")
	taTok := idp.Token("kc-ta", httpxtest.PortalClient, now)
	sch := staffUser(ctx, t, e, "sa.school.ur", &a.tenant) // School Admin of school A only
	link(sch, "kc-sch")
	grantRole(ctx, t, e, sch, "school_admin", &a.tenant, "school", &a.school, "NULL")
	schTok := idp.Token("kc-sch", httpxtest.PortalClient, now)
	gv := staffUser(ctx, t, e, "gv.ur", &a.tenant)
	link(gv, "kc-gv-ur")
	gvTok := idp.Token("kc-gv-ur", httpxtest.PortalClient, now)
	student := enrolled(ctx, t, e.platform, a, "hs.ur")
	sa := staffUser(ctx, t, e, "sa.ur", nil)
	link(sa, "kc-sa-ur")
	grantRole(ctx, t, e, sa, "system_admin", nil, "platform", nil, "NULL")
	saTok := idp.Token("kc-sa-ur", httpxtest.PortalClient, now)
	mod := staffUser(ctx, t, e, "mod.ur", nil)

	grant := func(tok string, hdrs map[string]string, path string, body map[string]any) httpResp {
		t.Helper()
		return h.do(t, "POST", path, tok, hdrs, body)
	}
	role := func(code string) string { return seed.RoleID(code).String() }

	// AC-23: tenant admin grants school_admin at a school (Idempotency-Key required).
	body := map[string]any{"user_id": gv.String(), "role_id": role("school_admin"), "scope_type": "school", "scope_id": a.school, "reason": "new principal"}
	if r := grant(taTok, hdr(a.tenant), "/tenant/user-roles", body); r.status != 428 {
		t.Fatalf("no key: %d %s", r.status, r.raw)
	}
	r := grant(taTok, key(a.tenant), "/tenant/user-roles", body)
	if r.status != 201 || r.body["role_code"] != "school_admin" || r.body["scope_type"] != "school" || r.body["source"] != "manual" ||
		r.body["granted_by"] != ta.String() {
		t.Fatalf("grant: %d %s", r.status, r.raw)
	}
	granted := r.body["user_role_id"].(string)
	if p := h.do(t, "GET", "/me/permissions", gvTok, hdr(a.tenant), nil); p.status != 200 || !strings.Contains(p.raw, `"tenant.sis.import"`) {
		t.Fatalf("permissions after grant: %d %s", p.status, p.raw)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'user_role.changed' AND envelope->'payload'->>'op' = 'granted'
		AND envelope->'payload'->>'user_id' = $1`, gv.String()); n != 1 {
		t.Fatalf("user_role.changed = %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT version FROM identity.users WHERE user_id = $1`, gv); n < 2 {
		t.Fatalf("principal version not bumped: %d", n)
	}

	// AC-20: class scope, unknown role.
	for name, c := range map[string]struct {
		body map[string]any
		code string
	}{
		"class scope":  {map[string]any{"user_id": gv.String(), "role_id": role("academic_manager"), "scope_type": "class", "scope_id": ids.New().String()}, "VALIDATION_FAILED"},
		"unknown role": {map[string]any{"user_id": gv.String(), "role_id": uuid.NewString(), "scope_type": "tenant"}, "ROLE_NOT_FOUND"},
		// AC-21: student, platform role on a tenant route.
		"student":       {map[string]any{"user_id": student.String(), "role_id": role("academic_manager"), "scope_type": "tenant"}, "VALIDATION_FAILED"},
		"platform role": {map[string]any{"user_id": gv.String(), "role_id": role("system_admin"), "scope_type": "platform"}, "VALIDATION_FAILED"},
	} {
		if r := grant(taTok, key(a.tenant), "/tenant/user-roles", c.body); r.status != 422 || !strings.Contains(r.raw, c.code) {
			t.Errorf("%s: %d %s", name, r.status, r.raw)
		}
	}
	// AC-22: a School Admin outside its school, or handing out more than it holds → 403.
	if r := grant(schTok, key(a.tenant), "/schools/"+a.otherSchool+"/user-roles", map[string]any{"user_id": gv.String(), "role_id": role("school_admin"),
		"scope_type": "school"}); r.status != 403 {
		t.Fatalf("other school: %d %s", r.status, r.raw)
	}
	if r := grant(schTok, key(a.tenant), "/schools/"+a.school+"/user-roles", map[string]any{"user_id": gv.String(), "role_id": role("tenant_admin"),
		"scope_type": "school"}); r.status != 403 {
		t.Fatalf("role above own permissions: %d %s", r.status, r.raw)
	}
	// AC-24: safety_moderator requires training_completed_at (platform route, platform staff).
	sm := map[string]any{"user_id": mod.String(), "role_id": role("safety_moderator"), "scope_type": "platform", "reason": "night shift"}
	if r := grant(saTok, key(""), "/admin/user-roles", sm); r.status != 422 || !strings.Contains(r.raw, "training_completed_at") {
		t.Fatalf("untrained moderator: %d %s", r.status, r.raw)
	}
	sm["training_completed_at"] = now.Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	if r := grant(saTok, key(""), "/admin/user-roles", sm); r.status != 201 || r.body["tenant_id"] != nil {
		t.Fatalf("trained moderator: %d %s", r.status, r.raw)
	}
	// The System Admin grants a tenant role into a tenant with X-Tenant-Id.
	gvB := staffUser(ctx, t, e, "gv.ur.b", &b.tenant)
	if r := grant(saTok, key(b.tenant), "/admin/user-roles", map[string]any{"user_id": gvB.String(), "role_id": role("academic_manager"),
		"scope_type": "tenant"}); r.status != 201 || r.body["tenant_id"] != b.tenant {
		t.Fatalf("platform grant into tenant: %d %s", r.status, r.raw)
	}

	// Lists.
	if r := h.do(t, "GET", "/tenant/user-roles?user_id="+gv.String(), taTok, hdr(a.tenant), nil); r.status != 200 || len(r.body["items"].([]any)) != 1 {
		t.Fatalf("listTenantUserRoles: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", "/schools/"+a.school+"/user-roles", schTok, hdr(a.tenant), nil); r.status != 200 || len(r.body["items"].([]any)) != 2 {
		t.Fatalf("listSchoolUserRoles: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", "/admin/user-roles?tenant_ids="+b.tenant, saTok, nil, nil); r.status != 200 || len(r.body["items"].([]any)) != 1 {
		t.Fatalf("listAllUserRoles: %d %s", r.status, r.raw)
	}

	// AC-23: revoke (reason required); another tenant's admin cannot see it; access is gone.
	if r := h.do(t, "DELETE", "/tenant/user-roles/"+granted+"?reason=ok", taTok, hdr(a.tenant), nil); r.status != 422 && r.status != 400 {
		t.Fatalf("short reason: %d %s", r.status, r.raw)
	}
	tb := staffUser(ctx, t, e, "ta.ur.b", &b.tenant)
	link(tb, "kc-tb")
	grantRole(ctx, t, e, tb, "tenant_admin", &b.tenant, "tenant", nil, "NULL")
	if r := h.do(t, "DELETE", "/tenant/user-roles/"+granted+"?reason=cleanup", idp.Token("kc-tb", httpxtest.PortalClient, now), hdr(b.tenant), nil); r.status != 404 {
		t.Fatalf("other tenant revoke: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "DELETE", "/tenant/user-roles/"+granted+"?reason=left+the+school", taTok, hdr(a.tenant), nil); r.status != 204 {
		t.Fatalf("revoke: %d %s", r.status, r.raw)
	}
	if p := h.do(t, "GET", "/me/permissions", gvTok, hdr(a.tenant), nil); p.status != 403 || !strings.Contains(p.raw, "TENANT_MISMATCH") {
		t.Fatalf("access after revoke: %d %s", p.status, p.raw)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' IN ('user_role.granted', 'user_role.revoked')`); n != 4 {
		t.Fatalf("audits = %d", n)
	}
}
