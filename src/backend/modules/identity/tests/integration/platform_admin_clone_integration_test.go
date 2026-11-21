//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// F045 US3: F045-AC-13..AC-18, SC-003, SC-004 — createTenantClone answers 202, the job platform-saga runs the
// steps through the learning facade TEST DOUBLE, retries a transient failure with the same Idempotency-Key and
// closes the job with audit in both tenants and tenant_data.cloned.
func TestPlatformAdminClone(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	p := platformHTTP(ctx, t, e, nil)
	a := seedTenant(ctx, t, e.platform, "pgd-cl-a")
	b := seedTenant(ctx, t, e.platform, "pgd-cl-b")
	off := seedTenant(ctx, t, e.platform, "pgd-cl-off")
	if _, err := e.platform.ExecContext(ctx, `UPDATE platform.tenants SET status = 'disabled' WHERE tenant_id = $1`, off.tenant); err != nil {
		t.Fatal(err)
	}
	ta, tb := uuid.MustParse(a.tenant), uuid.MustParse(b.tenant)
	platformUsers(ctx, t, p, a, "cl")
	sa, ca := p.token("sa.cl"), p.token("ca.cl")
	q1, q2 := p.learn.add(&ta, "c-1", false), p.learn.add(&ta, "c-2", false)
	studentsB := count(ctx, t, e.platform, `SELECT count(*) FROM org.students WHERE tenant_id = $1`, b.tenant)

	clone := func(key string, body map[string]any, tok string) httpResp {
		return p.do(t, "POST", "/admin/tenant-clones", tok, map[string]string{"Idempotency-Key": key}, body)
	}
	body := map[string]any{"source_tenant_id": a.tenant, "target_tenant_id": b.tenant, "resource_type": "question",
		"resource_ids": []string{q1.String(), q2.String()}}
	key := newKey()
	// AC-13: 202 + job_id, queued until the worker runs it.
	r := clone(key, body, sa)
	if r.status != 202 || r.body["status"] != "queued" || r.body["job_id"] == nil {
		t.Fatalf("create clone: %d %s", r.status, r.raw)
	}
	job := r.body["job_id"].(string)
	if r = clone(key, body, sa); r.status != 202 || r.body["job_id"] != job || r.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %d %s", r.status, r.raw)
	}
	// AC-17, SC-004: the module's step took effect but the answer was lost → retried with the same key, replayed,
	// never cloned twice.
	p.learn.failAfter = 1
	p.runSagas(ctx, t)
	if p.learn.cloneCalls < 2 {
		t.Fatalf("the step was not retried: %d calls", p.learn.cloneCalls)
	}
	// AC-14, AC-17: succeeded, id_map, drafts with cloned_from in the target only.
	r = p.do(t, "GET", "/admin/tenant-clones/"+job, sa, nil, nil)
	if r.status != 200 || r.body["status"] != "succeeded" || r.body["cloned_count"] != float64(2) || len(r.body["id_map"].([]any)) != 2 {
		t.Fatalf("job: %d %s", r.status, r.raw)
	}
	copies := p.learn.clones(tb)
	if len(copies) != 2 {
		t.Fatalf("copies in the target: %d", len(copies))
	}
	for _, c := range copies {
		if c.status != "draft" || c.from == nil || c.id == *c.from {
			t.Fatalf("copy: %+v", c)
		}
	}
	// SC-003: no student data created in the target by the clone.
	if count(ctx, t, e.platform, `SELECT count(*) FROM org.students WHERE tenant_id = $1`, b.tenant) != studentsB {
		t.Fatal("students created in the target tenant")
	}
	// AC-18: audit tenant_data.cloned in the source and the target, one event tenant_data.cloned.
	if outboxCount(ctx, t, p, "audit.recorded", `"tenant_data.cloned"`) != 2 || outboxCount(ctx, t, p, "tenant_data.cloned", job) != 1 {
		t.Fatal("audit/event tenant_data.cloned")
	}
	p.runSagas(ctx, t) // nothing more: the job is closed once
	if outboxCount(ctx, t, p, "tenant_data.cloned", job) != 1 {
		t.Fatal("job closed twice")
	}

	// AC-15, AC-16: invalid requests → 422 and no job.
	jobs := count(ctx, t, e.platform, `SELECT count(*) FROM platform.tenant_clone_jobs`)
	for name, bad := range map[string]map[string]any{
		"same tenant": {"source_tenant_id": a.tenant, "target_tenant_id": a.tenant, "resource_type": "question", "resource_ids": []string{q1.String()}},
		"not allowed": {"source_tenant_id": a.tenant, "target_tenant_id": b.tenant, "resource_type": "quest", "resource_ids": []string{q1.String()}},
		"disabled":    {"source_tenant_id": a.tenant, "target_tenant_id": off.tenant, "resource_type": "question", "resource_ids": []string{q1.String()}},
		"unknown":     {"source_tenant_id": a.tenant, "target_tenant_id": uuid.NewString(), "resource_type": "question", "resource_ids": []string{q1.String()}},
		"too many":    {"source_tenant_id": a.tenant, "target_tenant_id": b.tenant, "resource_type": "question", "resource_ids": manyIDs(501)},
		"student":     {"source_tenant_id": a.tenant, "target_tenant_id": b.tenant, "resource_type": "student", "resource_ids": []string{q1.String()}},
	} {
		if r = clone(newKey(), bad, sa); r.status != 422 || r.body["code"] != "VALIDATION_FAILED" {
			t.Fatalf("%s: %d %s", name, r.status, r.raw)
		}
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM platform.tenant_clone_jobs`) != jobs {
		t.Fatal("an invalid request created a job")
	}
	if r = clone(newKey(), body, ca); r.status != 403 {
		t.Fatalf("content_admin: %d", r.status)
	}
	authztest.Cover(t, "createTenantClone", authztest.RoleDenied)

	// AC-17: a step failing past the retries of the engine → job failed with the error of the step.
	p.learn.failClone = 10
	r = clone(newKey(), map[string]any{"source_tenant_id": a.tenant, "target_tenant_id": b.tenant, "resource_type": "question",
		"resource_ids": []string{q1.String()}}, sa)
	if r.status != 202 {
		t.Fatalf("second clone: %d %s", r.status, r.raw)
	}
	failed := r.body["job_id"].(string)
	p.runSagas(ctx, t)
	r = p.do(t, "GET", "/admin/tenant-clones/"+failed, sa, nil, nil)
	if r.status != 200 || r.body["status"] != "failed" || len(r.body["errors"].([]any)) == 0 ||
		!strings.HasPrefix(r.body["errors"].([]any)[0].(string), "learning step 0") {
		t.Fatalf("failed job: %d %s", r.status, r.raw)
	}
	if r = p.do(t, "GET", "/admin/tenant-clones/"+uuid.NewString(), sa, nil, nil); r.status != 404 {
		t.Fatalf("unknown job: %d", r.status)
	}
	authztest.Cover(t, "getTenantClone", authztest.RoleDenied)
}

func manyIDs(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = uuid.NewString()
	}
	return out
}
