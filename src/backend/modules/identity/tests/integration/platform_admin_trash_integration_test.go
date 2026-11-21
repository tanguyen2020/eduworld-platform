//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// platformUsers creates the F045 actors: System Admin, content_admin, safety_moderator (platform), the Tenant
// Admin of a and a School Admin of a (no tenant.trash.*).
func platformUsers(ctx context.Context, t *testing.T, p *platformEnv, a fixture, suffix string) {
	t.Helper()
	iss := p.idp.Issuer()
	portalUser(ctx, t, p.env, iss, "sa."+suffix, "system_admin", nil, "platform", nil)
	portalUser(ctx, t, p.env, iss, "ca."+suffix, "content_admin", nil, "platform", nil)
	portalUser(ctx, t, p.env, iss, "sm."+suffix, "safety_moderator", nil, "platform", nil)
	portalUser(ctx, t, p.env, iss, "ad.donvi."+suffix, "tenant_admin", &a.tenant, "tenant", nil)
	portalUser(ctx, t, p.env, iss, "ad.truong."+suffix, "school_admin", &a.tenant, "school", &a.school)
}

func newKey() string { return ids.New().String() }

func outboxCount(ctx context.Context, t *testing.T, p *platformEnv, eventType, like string) int {
	t.Helper()
	return count(ctx, t, p.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = $1 AND envelope::text LIKE $2`, eventType, "%"+like+"%")
}

// F045 US1, US4, US5: F045-AC-01..06, AC-19..23, AC-25, AC-26 through the httpx chain with the real Principal
// resolution; the learning facade is the TEST DOUBLE fakeLearningFacade behind the real gateway.
func TestPlatformAdminCrossTenantTrash(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	p := platformHTTP(ctx, t, e, nil)
	a := seedTenant(ctx, t, e.platform, "pgd-pa-a")
	b := seedTenant(ctx, t, e.platform, "pgd-pa-b")
	ta, tb := uuid.MustParse(a.tenant), uuid.MustParse(b.tenant)
	platformUsers(ctx, t, p, a, "tr")
	sa, ca, sm := p.token("sa.tr"), p.token("ca.tr"), p.token("sm.tr")
	ad, adNoTrash := p.token("ad.donvi.tr"), p.token("ad.truong.tr")
	inA := map[string]string{"X-Tenant-Id": a.tenant}

	qa := p.learn.add(&ta, "q-a", true)
	qb := p.learn.add(&tb, "q-b", true)
	qp := p.learn.add(nil, "q-p", true)

	// AC-01, AC-19: explicit tenant_ids → only A and B, tenant_id on each row, audit cross_tenant.accessed.
	r := p.do(t, "GET", "/admin/trash?resource_type=question&tenant_ids="+a.tenant+"&tenant_ids="+b.tenant, sa, nil, nil)
	if r.status != 200 || len(r.body["data"].([]any)) != 2 {
		t.Fatalf("listTrash A,B: %d %s", r.status, r.raw)
	}
	for _, it := range r.body["data"].([]any) {
		row := it.(map[string]any)
		if row["tenant_id"] != a.tenant && row["tenant_id"] != b.tenant || row["deleted_at"] == nil || row["deleted_by"] == nil {
			t.Fatalf("row: %v", row)
		}
	}
	if seen := p.learn.trashSeen[len(p.learn.trashSeen)-1]; seen.TenantIds == nil || len(*seen.TenantIds) != 2 {
		t.Fatalf("facade tenant_ids: %+v", seen)
	}
	if outboxCount(ctx, t, p, "audit.recorded", "cross_tenant.accessed") == 0 || outboxCount(ctx, t, p, "audit.recorded", b.tenant) == 0 {
		t.Fatal("audit cross_tenant.accessed with tenant_ids")
	}
	// tenant_ids empty = every active tenant (explicit list in the audit) + platform data from the owner.
	before := outboxCount(ctx, t, p, "audit.recorded", "cross_tenant.accessed")
	if r = p.do(t, "GET", "/admin/trash?resource_type=question", sa, nil, nil); r.status != 200 || len(r.body["data"].([]any)) != 3 {
		t.Fatalf("listTrash all: %d %s", r.status, r.raw)
	}
	if outboxCount(ctx, t, p, "audit.recorded", "cross_tenant.accessed") <= before {
		t.Fatal("audit of the all-tenant read")
	}
	// AC-03, AC-04: other platform roles 403, tenant role 403 (+ authz.denied); System Admin with a password-only
	// token is allowed (ADR-0094).
	for _, tok := range []string{ca, sm, ad} {
		if r = p.do(t, "GET", "/admin/trash?resource_type=question", tok, nil, nil); r.status != 403 || r.body["code"] != "PERMISSION_DENIED" {
			t.Fatalf("role denied: %d %s", r.status, r.raw)
		}
	}
	if outboxCount(ctx, t, p, "audit.recorded", "authz.denied") == 0 {
		t.Fatal("audit authz.denied")
	}
	if r = p.do(t, "GET", "/admin/trash?resource_type=question", p.token("sa.tr"), nil, nil); r.status != 200 {
		t.Fatalf("password-only System Admin: %d %s", r.status, r.raw)
	}
	authztest.Cover(t, "listTrash", authztest.RoleDenied)

	// AC-02, AC-22: a tenant user's tenant_ids is ignored; missing permission 403.
	if r = p.do(t, "GET", "/tenant/trash?resource_type=question&tenant_ids="+b.tenant, ad, inA, nil); r.status != 200 ||
		len(r.body["data"].([]any)) != 1 || r.body["data"].([]any)[0].(map[string]any)["tenant_id"] != a.tenant {
		t.Fatalf("tenant trash: %d %s", r.status, r.raw)
	}
	if last := p.learn.trashTenant[len(p.learn.trashTenant)-1]; last == nil || *last != ta {
		t.Fatal("tenant trash must call the owner with the session tenant")
	}
	if r = p.do(t, "GET", "/tenant/trash?resource_type=question", adNoTrash, inA, nil); r.status != 403 {
		t.Fatalf("school admin without tenant.trash.read: %d", r.status)
	}
	authztest.Cover(t, "listTenantTrash", authztest.CrossTenant, authztest.RoleDenied)

	// AC-05, AC-26: listTenants q case-insensitive; q / tenant_ids over the limits → 422.
	if r = p.do(t, "GET", "/admin/tenants?q=PGD-PA", sa, nil, nil); r.status != 200 || len(r.body["data"].([]any)) != 2 ||
		r.body["data"].([]any)[0].(map[string]any)["tenant_type"] != "unit" {
		t.Fatalf("listTenants: %d %s", r.status, r.raw)
	}
	if r = p.do(t, "GET", "/admin/tenants?q="+strings.Repeat("x", 101), sa, nil, nil); r.status != 422 || r.body["code"] != "VALIDATION_FAILED" {
		t.Fatalf("q too long: %d %s", r.status, r.raw)
	}
	many := "/admin/trash?resource_type=question"
	for i := 0; i < 101; i++ {
		many += "&tenant_ids=" + uuid.NewString()
	}
	if r = p.do(t, "GET", many, sa, nil, nil); r.status != 422 {
		t.Fatalf("tenant_ids > 100: %d %s", r.status, r.raw)
	}

	// AC-20: System Admin restores a tenant record (X-Tenant-Id = its tenant) and a platform record, with a reason.
	restore := func(path, tok string, hdr map[string]string, reason string) httpResp {
		h := map[string]string{"Idempotency-Key": newKey()}
		for k, v := range hdr {
			h[k] = v
		}
		return p.do(t, "POST", path, tok, h, map[string]any{"reason": reason})
	}
	if r = restore("/admin/trash/question/"+qa.String()+":restore", sa, inA, "khôi phục nhầm xóa"); r.status != 204 {
		t.Fatalf("restore A: %d %s", r.status, r.raw)
	}
	if !p.learn.visible(ta, qa) || outboxCount(ctx, t, p, "audit.recorded", "question.restored") != 1 {
		t.Fatal("restored item and audit")
	}
	if r = restore("/admin/trash/question/"+qp.String()+":restore", sa, nil, "khôi phục dữ liệu platform"); r.status != 204 {
		t.Fatalf("restore platform: %d %s", r.status, r.raw)
	}
	if r = restore("/admin/trash/question/"+qb.String()+":restore", sa, nil, "ab"); r.status != 422 {
		t.Fatalf("short reason: %d", r.status)
	}
	// Idempotency: same key + same body → replay; same key + another body → 409 IDEMPOTENCY_KEY_REUSED.
	q2 := p.learn.add(&ta, "q-a2", true)
	k := map[string]string{"Idempotency-Key": newKey(), "X-Tenant-Id": a.tenant}
	if r = p.do(t, "POST", "/admin/trash/question/"+q2.String()+":restore", sa, k, map[string]any{"reason": "lý do một"}); r.status != 204 {
		t.Fatalf("restore q2: %d %s", r.status, r.raw)
	}
	if r = p.do(t, "POST", "/admin/trash/question/"+q2.String()+":restore", sa, k, map[string]any{"reason": "lý do một"}); r.status != 204 ||
		r.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %d %v", r.status, r.header)
	}
	if r = p.do(t, "POST", "/admin/trash/question/"+q2.String()+":restore", sa, k, map[string]any{"reason": "lý do khác"}); r.status != 409 ||
		r.body["code"] != "IDEMPOTENCY_KEY_REUSED" {
		t.Fatalf("key reuse: %d %s", r.status, r.raw)
	}
	// AC-21: a live record holds the code → 409 CONFLICT, nothing changes.
	p.learn.add(&ta, "dup", false)
	dup := p.learn.add(&ta, "dup", true)
	if r = restore("/admin/trash/question/"+dup.String()+":restore", sa, inA, "khôi phục bản cũ"); r.status != 409 || r.body["code"] != "CONFLICT" {
		t.Fatalf("unique conflict: %d %s", r.status, r.raw)
	}
	if p.learn.visible(ta, dup) {
		t.Fatal("conflicting restore changed data")
	}
	// AC-22: Tenant Admin restores in its tenant only; another tenant's record 404; no permission 403.
	q3 := p.learn.add(&ta, "q-a3", true)
	if r = restore("/tenant/trash/question/"+q3.String()+":restore", ad, inA, "khôi phục trong trường"); r.status != 204 {
		t.Fatalf("tenant restore: %d %s", r.status, r.raw)
	}
	if r = restore("/tenant/trash/question/"+qb.String()+":restore", ad, inA, "khôi phục tenant khác"); r.status != 404 {
		t.Fatalf("other tenant: %d %s", r.status, r.raw)
	}
	if r = restore("/tenant/trash/question/"+q3.String()+":restore", adNoTrash, inA, "không có quyền"); r.status != 403 {
		t.Fatalf("no permission: %d", r.status)
	}
	authztest.Cover(t, "restoreTenantTrashItem", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "restoreTrashItem", authztest.RoleDenied)

	// Identity-owned trash: a cleaned-up student is listed (restorable through F016), merged accounts are not;
	// the read goes through tenancy.Bypass(trash) (AC-06: audit + eduworld_tenant_filter_bypass_total).
	st := enrolled(ctx, t, e.platform, a, "hs.trash.a")
	if _, err := e.platform.ExecContext(ctx, `UPDATE identity.users SET deleted_at = now(), delete_reason = 'inactive_cleanup' WHERE user_id = $1`, st); err != nil {
		t.Fatal(err)
	}
	bypass := testutil.ToFloat64(tenancy.BypassCounter().WithLabelValues("identity", string(tenancy.ReasonTrash)))
	if r = p.do(t, "GET", "/admin/trash?resource_type=student&tenant_ids="+a.tenant, sa, nil, nil); r.status != 200 || len(r.body["data"].([]any)) != 1 ||
		r.body["data"].([]any)[0].(map[string]any)["restorable"] != true || r.body["data"].([]any)[0].(map[string]any)["title"] != "student" {
		t.Fatalf("student trash: %d %s", r.status, r.raw)
	}
	if testutil.ToFloat64(tenancy.BypassCounter().WithLabelValues("identity", string(tenancy.ReasonTrash))) <= bypass {
		t.Fatal("bypass metric")
	}
	if r = p.do(t, "GET", "/admin/trash?resource_type=user&tenant_ids="+a.tenant, sa, nil, nil); r.status != 200 || len(r.body["data"].([]any)) != 0 {
		t.Fatalf("user trash: %d %s", r.status, r.raw)
	}
	if r = p.do(t, "GET", "/tenant/trash?resource_type=student", ad, inA, nil); r.status != 200 || len(r.body["data"].([]any)) != 1 {
		t.Fatalf("tenant student trash: %d %s", r.status, r.raw)
	}
	// F016 restore saga not wired in this harness → 503, never a silent success.
	if r = restore("/tenant/trash/student/"+st.String()+":restore", ad, inA, "học sinh quay lại"); r.status != 503 {
		t.Fatalf("student restore unwired: %d %s", r.status, r.raw)
	}

	// AC-23: no hard-delete route; a deleted record stays out of normal reads.
	if r = p.do(t, "DELETE", "/admin/trash/question/"+qb.String(), sa, nil, nil); r.status != 404 {
		t.Fatalf("hard delete route: %d", r.status)
	}
	if p.learn.visible(tb, qb) {
		t.Fatal("deleted record visible")
	}

	// AC-25: owner down or not wired → 503 UNAVAILABLE.
	if r = p.do(t, "GET", "/admin/trash?resource_type=quest", sa, nil, nil); r.status != 503 || r.body["code"] != "UNAVAILABLE" {
		t.Fatalf("game not wired: %d %s", r.status, r.raw)
	}
	p.learn.down = true
	if r = p.do(t, "GET", "/admin/trash?resource_type=question", sa, nil, nil); r.status != 503 || r.body["code"] != "UNAVAILABLE" {
		t.Fatalf("learning down: %d %s", r.status, r.raw)
	}
	_ = http.MethodGet
}
