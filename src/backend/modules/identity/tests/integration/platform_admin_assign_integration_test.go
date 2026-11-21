//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// F045 US2: F045-AC-07, AC-08, AC-10, AC-11, AC-12, AC-26 (assignment saga through the learning facade TEST
// DOUBLE; read-only use and RESOURCE_NOT_ASSIGNED are enforced by module learning, F020-AC-16/AC-18).
func TestPlatformAdminAssignments(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	p := platformHTTP(ctx, t, e, nil)
	a := seedTenant(ctx, t, e.platform, "pgd-as-a")
	b := seedTenant(ctx, t, e.platform, "pgd-as-b")
	c := seedTenant(ctx, t, e.platform, "pgd-as-c")
	if _, err := e.platform.ExecContext(ctx, `UPDATE platform.tenants SET status = 'disabled' WHERE tenant_id = $1`, c.tenant); err != nil {
		t.Fatal(err)
	}
	ta, tb := uuid.MustParse(a.tenant), uuid.MustParse(b.tenant)
	platformUsers(ctx, t, p, a, "as")
	sa, ca := p.token("sa.as"), p.token("ca.as")
	qs := p.learn.add(nil, "qs-platform", false)
	tenantItem := p.learn.add(&ta, "qs-tenant", false)

	assign := func(key string, body map[string]any, tok string) httpResp {
		return p.do(t, "POST", "/admin/tenant-assignments", tok, map[string]string{"Idempotency-Key": key}, body)
	}
	body := map[string]any{"resource_type": "question_set", "resource_id": qs, "tenant_ids": []string{a.tenant}}
	k1 := newKey()
	// AC-07: one active assignment for A, event + audit per tenant; B does not see it.
	r := assign(k1, body, sa)
	if r.status != 200 || len(r.body["data"].([]any)) != 1 || r.body["data"].([]any)[0].(map[string]any)["tenant_id"] != a.tenant {
		t.Fatalf("assign: %d %s", r.status, r.raw)
	}
	if !p.learn.visible(ta, qs) || p.learn.visible(tb, qs) {
		t.Fatal("visibility after assign")
	}
	if outboxCount(ctx, t, p, "tenant_data.assigned", a.tenant) != 1 || outboxCount(ctx, t, p, "audit.recorded", "tenant_data.assigned") != 1 {
		t.Fatal("event + audit tenant_data.assigned")
	}
	// AC-12: replay → stored result; same key, other payload → 409 IDEMPOTENCY_KEY_REUSED; a new key creates no duplicate.
	if r = assign(k1, body, sa); r.status != 200 || r.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %d %v", r.status, r.header)
	}
	other := map[string]any{"resource_type": "question_set", "resource_id": qs, "tenant_ids": []string{b.tenant}}
	if r = assign(k1, other, sa); r.status != 409 || r.body["code"] != "IDEMPOTENCY_KEY_REUSED" {
		t.Fatalf("key reuse: %d %s", r.status, r.raw)
	}
	first := p.learn.assignments[[2]uuid.UUID{qs, ta}].id
	if r = assign(newKey(), body, sa); r.status != 200 || p.learn.assignments[[2]uuid.UUID{qs, ta}].id != first {
		t.Fatalf("duplicate assignment: %d", r.status)
	}
	// AC-08: tenant data → 422; unknown → 404; disabled tenant → 422 and nothing assigned (no half assignment).
	if r = assign(newKey(), map[string]any{"resource_type": "question_set", "resource_id": tenantItem, "tenant_ids": []string{b.tenant}}, sa); r.status != 422 {
		t.Fatalf("tenant resource: %d %s", r.status, r.raw)
	}
	if r = assign(newKey(), map[string]any{"resource_type": "question_set", "resource_id": uuid.New(), "tenant_ids": []string{b.tenant}}, sa); r.status != 404 {
		t.Fatalf("unknown resource: %d %s", r.status, r.raw)
	}
	if r = assign(newKey(), map[string]any{"resource_type": "question_set", "resource_id": qs, "tenant_ids": []string{b.tenant, c.tenant}}, sa); r.status != 422 ||
		p.learn.visible(tb, qs) {
		t.Fatalf("disabled tenant: %d %s", r.status, r.raw)
	}
	if r = assign(newKey(), map[string]any{"resource_type": "user", "resource_id": qs, "tenant_ids": []string{b.tenant}}, sa); r.status != 422 {
		t.Fatalf("not assignable type: %d", r.status)
	}
	// AC-26: more than 100 tenants → 422 on the field.
	many := make([]string, 101)
	for i := range many {
		many[i] = uuid.NewString()
	}
	if r = assign(newKey(), map[string]any{"resource_type": "question_set", "resource_id": qs, "tenant_ids": many}, sa); r.status != 422 {
		t.Fatalf("tenant_ids > 100: %d", r.status)
	}
	// AC-03: other platform roles 403.
	if r = assign(newKey(), body, ca); r.status != 403 {
		t.Fatalf("content_admin: %d", r.status)
	}
	authztest.Cover(t, "assignToTenants", authztest.RoleDenied)

	// listTenantAssignments from the owning module.
	if r = p.do(t, "GET", "/admin/tenant-assignments?resource_type=question_set&resource_id="+qs.String(), sa, nil, nil); r.status != 200 ||
		len(r.body["data"].([]any)) != 1 {
		t.Fatalf("list assignments: %d %s", r.status, r.raw)
	}
	authztest.Cover(t, "listTenantAssignments", authztest.RoleDenied)

	// AC-10: unassign soft-deletes the assignment at the owner; event + audit.
	if r = p.do(t, "DELETE", "/admin/tenant-assignments", sa, map[string]string{"Idempotency-Key": newKey()},
		map[string]any{"resource_type": "question_set", "resource_id": qs, "tenant_ids": []string{a.tenant}, "reason": "hết năm học"}); r.status != 204 {
		t.Fatalf("unassign: %d %s", r.status, r.raw)
	}
	if p.learn.visible(ta, qs) || p.learn.assignments[[2]uuid.UUID{qs, ta}] == nil {
		t.Fatal("unassign must soft-delete, not remove")
	}
	if outboxCount(ctx, t, p, "tenant_data.unassigned", a.tenant) != 1 || outboxCount(ctx, t, p, "audit.recorded", "tenant_data.unassigned") != 1 {
		t.Fatal("event + audit tenant_data.unassigned")
	}
	authztest.Cover(t, "unassignFromTenants", authztest.RoleDenied)

	// AC-11: apply_to_new_tenants stores a default assignment; tenant.created assigns it to the new tenant.
	qs2 := p.learn.add(nil, "qs-default", false)
	if r = assign(newKey(), map[string]any{"resource_type": "question_set", "resource_id": qs2, "apply_to_new_tenants": true}, sa); r.status != 200 {
		t.Fatalf("default assignment: %d %s", r.status, r.raw)
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM platform.tenant_default_assignments WHERE resource_id = $1 AND deleted_at IS NULL`, qs2) != 1 {
		t.Fatal("default assignment row")
	}
	d := seedTenant(ctx, t, e.platform, "pgd-as-d")
	td := uuid.MustParse(d.tenant)
	payload, _ := json.Marshal(map[string]any{"tenant_id": d.tenant})
	cons := consumer.DefaultAssignments{Apply: command.ApplyDefaultAssignments{PlatformAdminDeps: p.deps}}
	env := outbox.Envelope{EventID: uuid.Must(uuid.NewV7()), EventType: "tenant.created", OccurredAt: time.Now().UTC(), TenantID: &td, Payload: payload}
	if err := cons.Handle(ctx, env); err != nil {
		t.Fatal(err)
	}
	if !p.learn.visible(td, qs2) || outboxCount(ctx, t, p, "tenant_data.assigned", "default_for_new_tenant") != 1 {
		t.Fatal("default assignment of the new tenant")
	}
	if err := cons.Handle(ctx, env); err != nil { // redelivery: the owner replays, no duplicate assignment
		t.Fatal(err)
	}
	if len(p.learn.list("question_set", qs2, []uuid.UUID{td}).Data) != 1 {
		t.Fatal("redelivery duplicated the assignment")
	}
	// OQ F045-9: unassign with remove_default also soft-deletes the default rule (audited); a later tenant is not
	// assigned; without remove_default the rule stays.
	if r = p.do(t, "DELETE", "/admin/tenant-assignments", sa, map[string]string{"Idempotency-Key": newKey()},
		map[string]any{"resource_type": "question_set", "resource_id": qs2, "tenant_ids": []string{d.tenant}}); r.status != 204 {
		t.Fatalf("unassign without remove_default: %d %s", r.status, r.raw)
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM platform.tenant_default_assignments WHERE resource_id = $1 AND deleted_at IS NULL`, qs2) != 1 {
		t.Fatal("the default rule must stay without remove_default")
	}
	if r = p.do(t, "DELETE", "/admin/tenant-assignments", sa, map[string]string{"Idempotency-Key": newKey()},
		map[string]any{"resource_type": "question_set", "resource_id": qs2, "tenant_ids": []string{d.tenant}, "remove_default": true,
			"reason": "bỏ gán mặc định"}); r.status != 204 {
		t.Fatalf("unassign remove_default: %d %s", r.status, r.raw)
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM platform.tenant_default_assignments WHERE resource_id = $1 AND deleted_at IS NOT NULL
		AND deleted_by IS NOT NULL AND delete_reason = 'unassigned'`, qs2) != 1 {
		t.Fatal("remove_default must soft-delete the default rule")
	}
	if outboxCount(ctx, t, p, "audit.recorded", "tenant_data.default_assignment_removed") != 1 {
		t.Fatal("audit tenant_data.default_assignment_removed")
	}
	f := seedTenant(ctx, t, e.platform, "pgd-as-f")
	tf := uuid.MustParse(f.tenant)
	payload, _ = json.Marshal(map[string]any{"tenant_id": f.tenant})
	if err := cons.Handle(ctx, outbox.Envelope{EventID: uuid.Must(uuid.NewV7()), EventType: "tenant.created", OccurredAt: time.Now().UTC(),
		TenantID: &tf, Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if p.learn.visible(tf, qs2) {
		t.Fatal("a tenant created after remove_default was assigned")
	}
}
