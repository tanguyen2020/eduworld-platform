//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
)

// Portal E2E 2026-11-21 (F012 Clarifications): a School Admin holds learning_content.read/write/review/publish on a
// school-limited grant (ADMIN_PORTAL §2.3). Tenant content has no school, so the grant covers the tenant's content:
// the Content Studio lists answer 200 instead of 403, writes need no tenant-wide grant, and another tenant's content
// stays invisible. Permissions the role does not hold (activity.write) stay 403.
func TestSchoolAdminContentStudio(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t, withMedia)
	qc := c.qbSetup(ctx, t)
	c.assign(ctx, t, qc.cur, c.tenantB)
	stem := map[string]any{"stem": rich("Câu của trường A")}
	c.question(t, qlcmA, qc, stem)
	c.question(t, qlcmB, qc, nil)
	n := func(r resp) int { return len(r.items("data")) }

	// adA: school_admin limited to one school of tenant A.
	for _, path := range []string{"/cms/learning/questions", setsPath, lessonsPath, materialsPath, blueprintsPath, "/cms/media",
		"/cms/learning/subjects", "/cms/learning/curricula", "/cms/learning/activities", "/cms/learning/activity-types",
		"/cms/learning/simulation-templates"} {
		must(t, c.do(t, http.MethodGet, path, adA, nil, nil), http.StatusOK, "school admin lists "+path)
	}
	if r := must(t, c.do(t, http.MethodGet, "/cms/learning/questions", adA, nil, nil), http.StatusOK, "questions of A"); n(r) != 1 {
		t.Fatalf("school admin of A sees %d questions, want the 1 of tenant A: %s", n(r), r.raw)
	}
	// Authoring tenant content with the school grant (learning_content.write).
	must(t, c.do(t, http.MethodPost, lessonsPath, adA, idem(), lessonBody(qc, map[string]any{"kind": "rich_text", "content": rich("Bài của trường")})),
		http.StatusCreated, "school admin creates a lesson")
	// A permission the role does not hold stays denied.
	if r := c.do(t, http.MethodPost, "/cms/learning/activities", adA, idem(), activityBody(qc, "quiz", quizConfig(c.published(ctx, t, qlcmA, qc, nil)))); r.status != http.StatusForbidden {
		t.Fatalf("activity without learning_content.activity.write: %d %s", r.status, r.raw)
	}
	// Tenant B (tenant-wide school admin) never sees tenant A's content.
	if r := must(t, c.do(t, http.MethodGet, "/cms/learning/questions", adB, nil, nil), http.StatusOK, "questions of B"); n(r) != 1 {
		t.Fatalf("school admin of B sees %d questions, want the 1 of tenant B: %s", n(r), r.raw)
	}
}
