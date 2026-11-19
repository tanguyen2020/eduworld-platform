//go:build integration

package integration

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

func taskIDs(r resp) []string {
	var out []string
	for _, x := range r.items("data") {
		out = append(out, x["content_id"].(string))
	}
	slices.Sort(out)
	return out
}

func sortedStr(s ...string) []string { slices.Sort(s); return s }

// F025-AC-12, AC-13, AC-14 (US2, quickstart 9, 11; SC-003): the queue shows the tenant lane of the session tenant (the
// schools of a school-scoped reviewer only, tenant_ids ignored), the platform lane to platform reviewers, the tenant lane
// of the requested tenants to System Admin (audited); a caller without learning_content.review is denied.
func TestReviewQueue(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	c.assign(ctx, t, qc.cur, c.tenantB)
	authztest.Cover(t, "listContentReviewTasks", authztest.CrossTenant, authztest.RoleDenied)
	school := uuid.New()
	c.user("duyet.truong.a", authztest.Staff(c.tenantA, "tenant_content_reviewer", authztest.AtSchool(school)).PrincipalData, true)
	c.user("duyet.a", authztest.Staff(c.tenantA, "tenant_content_reviewer").PrincipalData, true)
	submit := func(sub string) string {
		extra := map[string]any{"grade_levels": []int{4}}
		if sub == cmNoidung {
			extra["owner_scope"] = "platform"
		}
		id, _ := c.question(t, sub, qc, extra)
		must(t, c.act(ctx, t, sub, "question", id, 1, "submit_review", ""), http.StatusOK, "submit "+sub)
		return id
	}
	a1, a2, b1, p1 := submit(qlcmA), submit(edA), submit(qlcmB), submit(cmNoidung)
	list := func(sub, q string) resp {
		return must(t, c.do(t, http.MethodGet, "/cms/learning/review-tasks"+q, sub, nil, nil), http.StatusOK, "queue "+sub)
	}
	// AC-12: tenant reviewers, tenant_ids ignored.
	if got := taskIDs(list("duyet.a", "?tenant_ids="+c.tenantB.String())); !slices.Equal(got, sortedStr(a1, a2)) {
		t.Fatalf("tenant A reviewer: %v", got)
	}
	// A school-scoped reviewer sees the tasks of its schools only: tenant-wide tasks have no school [Proposed].
	if got := taskIDs(list("duyet.truong.a", "")); len(got) != 0 {
		t.Fatalf("school reviewer: %v", got)
	}
	if got := taskIDs(list("duyet.a", "?lane=platform_learning")); len(got) != 0 {
		t.Fatalf("tenant reviewer sees the platform lane: %v", got)
	}
	r := list("duyet.a", "")
	for _, x := range r.items("data") {
		if x["lane"] != "tenant" || x["status"] != "open" || x["tenant_id"] != c.tenantA.String() || x["submitted_at"] == nil {
			t.Fatalf("task view %v", x)
		}
	}
	// The school reviewer cannot decide on a tenant-wide task; the tenant reviewer can.
	if r := c.act(ctx, t, "duyet.truong.a", "question", a1, 1, "approve", ""); r.status != http.StatusForbidden {
		t.Fatalf("school reviewer, tenant task: %d %s", r.status, r.raw)
	}
	must(t, c.act(ctx, t, "duyet.a", "question", a2, 1, "approve", ""), http.StatusOK, "tenant reviewer approves")
	// AC-13: platform lane for the platform reviewer; tenant lanes for System Admin with tenant_ids (audited).
	if got := taskIDs(list(cmDuyet, "")); !slices.Equal(got, []string{p1}) {
		t.Fatalf("platform reviewer: %v", got)
	}
	cross := c.audits(ctx, t, "cross_tenant.accessed", nil)
	r = list(saAdmin, "?tenant_ids="+c.tenantA.String()+","+c.tenantB.String())
	if got := taskIDs(r); !slices.Equal(got, sortedStr(a1, b1)) {
		t.Fatalf("system admin: %v", got)
	}
	if c.audits(ctx, t, "cross_tenant.accessed", nil) <= cross {
		t.Fatal("cross-tenant queue read not audited")
	}
	if got := taskIDs(list(cmDuyet, "?status=approved")); len(got) != 0 {
		t.Fatalf("approved platform tasks: %v", got)
	}
	// AC-14.
	if r := c.do(t, http.MethodGet, "/cms/learning/review-tasks", noRoleA, nil, nil); r.status != http.StatusForbidden {
		t.Fatalf("no role: %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodGet, "/cms/learning/review-tasks", edA, nil, nil); r.status != http.StatusForbidden {
		t.Fatalf("editor: %d %s", r.status, r.raw)
	}
	// Paging.
	p := list("duyet.a", "?limit=1&status=open")
	if len(p.items("data")) != 1 || p.str("page", "has_more") != "false" {
		t.Fatalf("page: %s", p.raw)
	}
}
