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

const setsPath = "/cms/learning/question-sets"

// F024-AC-01, AC-02, AC-03, AC-05 (TC-CMS-ASG-01, TC-CMS-002): static, rule_based and mixed sets; references in
// scope only; rule consistency; drafts only with If-Match; one open draft.
func TestQuestionSetAuthoring(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	c.assign(ctx, t, qc.cur, c.tenantB)
	authztest.Cover(t, "createCmsQuestionSet", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateCmsQuestionSetDraft", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createCmsQuestionSetVersion", authztest.CrossTenant, authztest.RoleDenied)
	_, q1 := c.question(t, qlcmA, qc, nil)
	_, q2 := c.question(t, qlcmA, qc, nil)
	key := idem()
	r := must(t, c.do(t, http.MethodPost, setsPath, qlcmA, key, setBody(qc, staticDef(q1, q2))), http.StatusCreated, "static")
	if r.str("status") != "draft" || r.str("version") != "1" || r.str("content_type") != "question_set" || r.header.Get("ETag") == "" {
		t.Fatalf("static: %s", r.raw)
	}
	again := must(t, c.do(t, http.MethodPost, setsPath, qlcmA, key, setBody(qc, staticDef(q1, q2))), http.StatusCreated, "replay")
	if again.str("content_id") != r.str("content_id") {
		t.Fatal("idempotency replay created another set")
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.question_set_items WHERE question_set_id = $1 AND deleted_at IS NULL`,
		r.str("content_id")); n != 2 {
		t.Fatalf("items %d", n)
	}
	rule := map[string]any{"filters": map[string]any{"subject_id": qc.subject}, "count": 2}
	must(t, c.do(t, http.MethodPost, setsPath, qlcmA, idem(), setBody(qc, map[string]any{"kind": "rule_based", "purpose": "minigame",
		"selection_rules": []any{rule}})), http.StatusCreated, "rule_based")
	must(t, c.do(t, http.MethodPost, setsPath, qlcmA, idem(), setBody(qc, map[string]any{"kind": "mixed", "purpose": "quest",
		"items": []any{map[string]any{"question_version_id": q1}}, "selection_rules": []any{rule}})), http.StatusCreated, "mixed")
	must(t, c.do(t, http.MethodPost, setsPath, qlcmA, idem(), setBody(qc, map[string]any{"kind": "static", "purpose": "exercise"})),
		http.StatusUnprocessableEntity, "static without items")
	// AC-03: distribution sum, difficulty order.
	bad := map[string]any{"filters": map[string]any{"difficulty": map[string]any{"min": 4, "max": 2}}, "count": 10,
		"distribution": []any{map[string]any{"difficulty": 2, "count": 9}}}
	r3 := must(t, c.do(t, http.MethodPost, setsPath, qlcmA, idem(), setBody(qc, map[string]any{"kind": "rule_based", "purpose": "exercise",
		"selection_rules": []any{bad}})), http.StatusUnprocessableEntity, "rule errors")
	if !slices.Contains(r3.errorCodes(), "/definition/selection_rules/0/distribution:INVALID") ||
		!slices.Contains(r3.errorCodes(), "/definition/selection_rules/0/filters/difficulty:INVALID") {
		t.Fatalf("rule errors %v", r3.errorCodes())
	}
	// AC-02: tenant B question → 422; platform question not assigned → 403 RESOURCE_NOT_ASSIGNED, assigned → ok;
	// a platform set referencing a tenant question → 422.
	_, qb := c.question(t, qlcmB, qc, nil)
	must(t, c.do(t, http.MethodPost, setsPath, qlcmA, idem(), setBody(qc, staticDef(qb))), http.StatusUnprocessableEntity, "tenant B question")
	pq, pqv := c.question(t, cmNoidung, qc, map[string]any{"owner_scope": "platform"})
	r4 := must(t, c.do(t, http.MethodPost, setsPath, qlcmA, idem(), setBody(qc, staticDef(pqv))), http.StatusForbidden, "platform not assigned")
	if r4.str("code") != "RESOURCE_NOT_ASSIGNED" {
		t.Fatalf("code %s", r4.raw)
	}
	c.assignResource(ctx, t, "question", pq, c.tenantA)
	must(t, c.do(t, http.MethodPost, setsPath, qlcmA, idem(), setBody(qc, staticDef(pqv))), http.StatusCreated, "platform assigned")
	must(t, c.do(t, http.MethodPost, setsPath, cmNoidung, idem(), setBody(qc, staticDef(q1))), http.StatusUnprocessableEntity, "platform set, tenant question")
	must(t, c.do(t, http.MethodPost, setsPath, cmNoidung, idem(), setBody(qc, staticDef(pqv))), http.StatusCreated, "platform set")
	// AC-05: If-Match, published versions, one open draft.
	id := r.str("content_id")
	vpath := setsPath + "/" + id + "/versions/1"
	must(t, c.do(t, http.MethodPatch, vpath, qlcmA, nil, map[string]any{"title": map[string]any{"vi": "Mới"}}), http.StatusPreconditionRequired, "no If-Match")
	must(t, c.do(t, http.MethodPatch, vpath, qlcmA, ifm(`W/"99"`), map[string]any{"title": map[string]any{"vi": "Mới"}}), http.StatusPreconditionFailed, "stale")
	p := must(t, c.do(t, http.MethodPatch, vpath, qlcmA, ifm(r.header.Get("ETag")), map[string]any{"definition": staticDef(q2)}), http.StatusOK, "patch")
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.question_set_items WHERE question_set_id = $1 AND deleted_at IS NULL`, id); n != 1 {
		t.Fatalf("items after patch %d (%s)", n, p.raw)
	}
	must(t, c.do(t, http.MethodPost, setsPath+"/"+id+"/versions", qlcmA, idem(), map[string]any{"from_version": 1}), http.StatusConflict, "open draft")
	c.publishDoc(ctx, t, "question_sets", "question_set_versions", "question_set_id", id, 1)
	r5 := must(t, c.do(t, http.MethodPatch, vpath, qlcmA, ifm(p.header.Get("ETag")), map[string]any{"title": map[string]any{"vi": "x"}}),
		http.StatusConflict, "published")
	if r5.str("code") != "CONTENT_NOT_EDITABLE" {
		t.Fatalf("code %s", r5.raw)
	}
	v2 := must(t, c.do(t, http.MethodPost, setsPath+"/"+id+"/versions", qlcmA, idem(), map[string]any{"from_version": 1}), http.StatusCreated, "v2")
	if v2.str("version") != "2" || len(v2.items("objective_ids")) != 0 && v2.str("status") != "draft" {
		t.Fatalf("v2 %s", v2.raw)
	}
	// Reads: other tenant 404, no role 403.
	must(t, c.do(t, http.MethodGet, setsPath+"/"+id, qlcmA, nil, nil), http.StatusOK, "get")
	must(t, c.do(t, http.MethodGet, setsPath+"/"+id, qlcmB, nil, nil), http.StatusNotFound, "tenant B get")
	must(t, c.do(t, http.MethodGet, setsPath+"/"+id, noRoleA, nil, nil), http.StatusForbidden, "no role")
	_ = uuid.Nil
}

// F024-AC-04: pool per rule from the current releases of the tenant scope, insufficient and warning flags.
func TestQuestionSetPoolStats(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	c.assign(ctx, t, qc.cur, c.tenantB)
	authztest.Cover(t, "getQuestionSetPoolStats", authztest.CrossTenant, authztest.RoleDenied)
	for range 4 {
		c.published(ctx, t, qlcmA, qc, nil)
	}
	c.published(ctx, t, qlcmB, qc, nil)  // other tenant: not in A's pool
	_, _ = c.question(t, qlcmA, qc, nil) // draft: not in the pool
	rules := []any{
		map[string]any{"filters": map[string]any{"subject_id": qc.subject}, "count": 2},
		map[string]any{"filters": map[string]any{"subject_id": qc.subject}, "count": 1},
		map[string]any{"filters": map[string]any{"subject_id": qc.subject, "difficulty": map[string]any{"min": 4}}, "count": 1},
	}
	r := must(t, c.do(t, http.MethodPost, setsPath, qlcmA, idem(), setBody(qc, map[string]any{"kind": "rule_based", "purpose": "exercise",
		"selection_rules": rules})), http.StatusCreated, "set")
	st := must(t, c.do(t, http.MethodGet, setsPath+"/"+r.str("content_id")+"/versions/1:pool-stats", qlcmA, nil, nil), http.StatusOK, "stats")
	got := st.items("rules")
	if len(got) != 3 || got[0]["available"] != float64(4) || got[0]["insufficient"] != false || got[0]["warning"] != true ||
		got[1]["warning"] != false || got[2]["available"] != float64(0) || got[2]["insufficient"] != true {
		t.Fatalf("stats %s", st.raw)
	}
	must(t, c.do(t, http.MethodGet, setsPath+"/"+r.str("content_id")+"/versions/1:pool-stats", qlcmB, nil, nil), http.StatusNotFound, "tenant B")
	must(t, c.do(t, http.MethodGet, setsPath+"/"+r.str("content_id")+"/versions/1:pool-stats", noRoleA, nil, nil), http.StatusForbidden, "no role")
}
