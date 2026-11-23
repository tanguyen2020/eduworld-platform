//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

func (e *env) portalToken(sub string) string { return e.idp.Token(sub, "eduworld-portal", nowWall()) }

func (e *env) adjust(t *testing.T, tok string, s student, body map[string]any) resp {
	t.Helper()
	b := map[string]any{"tenant_id": s.tenant, "student_id": s.id, "reason": "Sửa lỗi cộng Xu trùng do sự cố #42"}
	for k, v := range body {
		b[k] = v
	}
	return e.do(t, http.MethodPost, "/admin/economy/adjustments", tok, map[string]string{"X-Tenant-Id": s.tenant.String(),
		"Idempotency-Key": newKey()}, b)
}

func (e *env) decideAdjustment(t *testing.T, tok string, s student, id, action string, body any) resp {
	t.Helper()
	return e.do(t, http.MethodPost, "/admin/economy/adjustments/"+id+":"+action, tok, map[string]string{"X-Tenant-Id": s.tenant.String(),
		"Idempotency-Key": newKey()}, body)
}

// F075-AC-25, AC-26: a System Admin creates a pending compensating posting (nothing posted, audited); the creator approving
// is 403 PERMISSION_DENIED; a second System Admin approves → the adjustment entry (Xu, item or XP) in one transaction,
// economy.adjusted, audit; a balance going negative is 409 INSUFFICIENT_BALANCE; a decided adjustment 422
// INVALID_STATE_TRANSITION; a rejection posts nothing; listEconomyAdjustments shows the pending ones to the approver.
func TestEconomyAdjustments(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	dung := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	e.credit(ctx, t, an, 100)
	sa1, sa2 := e.portalToken(e.platformStaff("system_admin")), e.portalToken(e.platformStaff("system_admin"))
	r := e.adjust(t, sa1, an, map[string]any{"kind": "currency", "currency_code": "xu", "delta": -50})
	if r.status != http.StatusAccepted || r.body["status"] != "pending_approval" || r.body["delta"] != float64(-50) {
		t.Fatalf("create: %d %s", r.status, r.raw)
	}
	id := r.body["adjustment_id"].(string)
	if e.balanceOf(ctx, t, an) != 100 || e.audits(ctx, t, "economy_adjustment.created") != 1 {
		t.Fatal("a pending adjustment posted or was not audited")
	}
	denied := e.audits(ctx, t, "authz.denied")
	if r := e.decideAdjustment(t, sa1, an, id, "approve", nil); r.status != http.StatusForbidden || r.code() != string(problem.PermissionDenied) {
		t.Fatalf("self approval: %d %s", r.status, r.raw)
	}
	if e.audits(ctx, t, "authz.denied") != denied+1 {
		t.Fatal("self approval not audited")
	}
	r = e.do(t, http.MethodGet, "/admin/economy/adjustments?status=pending_approval&tenant_ids="+e.tenantA.String(), sa2, nil, nil)
	if r.status != http.StatusOK || len(r.body["data"].([]any)) != 1 || r.body["data"].([]any)[0].(map[string]any)["adjustment_id"] != id {
		t.Fatalf("pending list: %d %s", r.status, r.raw)
	}
	r = e.decideAdjustment(t, sa2, an, id, "approve", nil)
	if r.status != http.StatusOK || r.body["status"] != "applied" || r.body["approved_by"] == nil || r.body["applied_at"] == nil {
		t.Fatalf("approve: %d %s", r.status, r.raw)
	}
	if e.balanceOf(ctx, t, an) != 50 || e.ledgerSum(ctx, t, an) != 50 ||
		count(ctx, t, e.platform, `SELECT count(*) FROM gamification.currency_ledger WHERE student_id = $1 AND reason_code = 'adjustment'
			AND source_type = 'adjustment' AND delta = -50`, an.id) != 1 {
		t.Fatalf("adjustment posting: balance %d", e.balanceOf(ctx, t, an))
	}
	if e.events(ctx, t, "economy.adjusted") != 1 || e.audits(ctx, t, "economy_adjustment.approved") != 1 {
		t.Fatal("economy.adjusted or audit")
	}
	if r := e.decideAdjustment(t, sa2, an, id, "approve", nil); r.status != http.StatusUnprocessableEntity || r.code() != string(problem.InvalidStateTransition) {
		t.Fatalf("approve twice: %d %s", r.status, r.raw)
	}
	big := e.adjust(t, sa1, an, map[string]any{"kind": "currency", "currency_code": "xu", "delta": -500}).body["adjustment_id"].(string)
	if r := e.decideAdjustment(t, sa2, an, big, "approve", nil); r.status != http.StatusConflict || r.code() != string(problem.InsufficientBalance) {
		t.Fatalf("negative: %d %s", r.status, r.raw)
	}
	if e.balanceOf(ctx, t, an) != 50 || count(ctx, t, e.platform, `SELECT count(*) FROM gamification.economy_adjustments WHERE adjustment_id = $1
		AND status = 'pending_approval'`, big) != 1 {
		t.Fatal("a refused approval changed something")
	}
	if r := e.decideAdjustment(t, sa2, an, big, "reject", map[string]any{"reason": "Số Xu thu hồi vượt số dư"}); r.status != http.StatusOK ||
		r.body["status"] != "rejected" || r.body["reject_reason"] == nil {
		t.Fatalf("reject: %d %s", r.status, r.raw)
	}
	if r := e.decideAdjustment(t, sa2, an, big, "approve", nil); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("approve rejected: %d %s", r.status, r.raw)
	}
	item := e.adjust(t, sa1, an, map[string]any{"kind": "item", "item_id": "pet_cat_01", "delta": 1}).body["adjustment_id"].(string)
	if r := e.decideAdjustment(t, sa2, an, item, "approve", nil); r.status != http.StatusOK || e.ownedOf(ctx, t, an, "pet_cat_01") != 1 {
		t.Fatalf("item adjustment: %d %s", r.status, r.raw)
	}
	_, before := e.xpOf(ctx, t, an)
	xp := e.adjust(t, sa1, an, map[string]any{"kind": "xp", "delta": 200}).body["adjustment_id"].(string)
	if r := e.decideAdjustment(t, sa2, an, xp, "approve", nil); r.status != http.StatusOK {
		t.Fatalf("xp adjustment: %d %s", r.status, r.raw)
	}
	if _, after := e.xpOf(ctx, t, an); after != before+200 || count(ctx, t, e.platform, `SELECT count(*) FROM gamification.xp_ledger
		WHERE student_id = $1 AND reason_code = 'adjustment' AND source_type = 'adjustment'`, an.id) != 1 {
		t.Fatalf("xp: %d → %d", before, after)
	}
	// Another tenant: the adjustment of A is not found from the tenant of B.
	if r := e.decideAdjustment(t, sa2, dung, xp, "approve", nil); r.status != http.StatusNotFound {
		t.Fatalf("adjustment of A from B: %d %s", r.status, r.raw)
	}
	r = e.do(t, http.MethodGet, "/admin/economy/adjustments?limit=2", sa2, nil, nil)
	if r.status != http.StatusOK || len(r.body["data"].([]any)) != 2 || r.body["page"].(map[string]any)["has_more"] != true {
		t.Fatalf("all tenants page: %d %s", r.status, r.raw)
	}
	if seen := e.checkEventSchemas(ctx, t); seen["economy.adjusted"] != 3 {
		t.Fatalf("schemas %v", seen)
	}
}

// F075-AC-27 (TC-TEN-004): without game.economy.adjust (tenant roles, a platform role without it), or with a token of
// another client, the adjustment routes are 403 and audited as authz.denied; a reason over 1000 characters is 422
// VALIDATION_FAILED; nothing is written.
func TestEconomyAdjustmentAuthz(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	authztest.Cover(t, "listEconomyAdjustments", authztest.RoleDenied)
	authztest.Cover(t, "createEconomyAdjustment", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "approveEconomyAdjustment", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "rejectEconomyAdjustment", authztest.RoleDenied, authztest.CrossTenant)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	sa := e.platformStaff("system_admin")
	denied := e.audits(ctx, t, "authz.denied")
	cases := map[string]struct {
		tok  string
		code problem.Code
	}{
		"school_admin":  {e.portalToken(e.staff(e.tenantA, "school_admin")), problem.PermissionDenied},
		"tenant_admin":  {e.portalToken(e.staff(e.tenantA, "tenant_admin")), problem.PermissionDenied},
		"game_designer": {e.portalToken(e.platformStaff("game_designer")), problem.PermissionDenied},
		"game client":   {e.idp.Token(sa, httpxtest.GameClient, nowWall()), problem.PortalAccessStaffOnly},
	}
	n := 0
	for name, c := range cases {
		r := e.adjust(t, c.tok, an, map[string]any{"kind": "currency", "currency_code": "xu", "delta": 10})
		if r.status != http.StatusForbidden || (r.code() != string(c.code) && r.code() != string(problem.TenantMismatch)) {
			t.Fatalf("create by %s: %d %s", name, r.status, r.raw)
		}
		if r := e.do(t, http.MethodGet, "/admin/economy/adjustments", c.tok, nil, nil); r.status != http.StatusForbidden {
			t.Fatalf("list by %s: %d %s", name, r.status, r.raw)
		}
		n += 2
	}
	if got := e.audits(ctx, t, "authz.denied"); got < denied+n-2 {
		t.Fatalf("authz.denied %d → %d", denied, got)
	}
	r := e.adjust(t, e.portalToken(sa), an, map[string]any{"kind": "currency", "currency_code": "xu", "delta": 10, "reason": strings.Repeat("a", 1001)})
	if r.status != http.StatusUnprocessableEntity || r.code() != string(problem.ValidationFailed) {
		t.Fatalf("long reason: %d %s", r.status, r.raw)
	}
	r = e.adjust(t, e.portalToken(sa), an, map[string]any{"kind": "currency", "currency_code": "xu", "delta": 10, "tenant_id": e.tenantB})
	if r.status != http.StatusUnprocessableEntity {
		t.Fatalf("body tenant differs from X-Tenant-Id: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM gamification.economy_adjustments`); n != 0 {
		t.Fatalf("adjustments written: %d", n)
	}
}
