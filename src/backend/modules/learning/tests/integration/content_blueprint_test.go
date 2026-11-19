//go:build integration

package integration

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

const blueprintsPath = "/cms/learning/assessment-blueprints"

func blueprintBody(qc qbCurriculum, total float64, rules []any, variants map[string]any) map[string]any {
	def := map[string]any{"duration_sec": 2400, "total_points": total, "attempt_policy": map[string]any{"max_attempts": 1, "score_policy": "best"},
		"sections": []any{map[string]any{"id": "sec_tn", "title": "Trắc nghiệm", "rules": rules}}}
	if variants != nil {
		def["variants"] = variants
	}
	return map[string]any{"title": map[string]any{"vi": "Kiểm tra giữa kỳ I – Toán 3"}, "exam_type": "chapter", "subject_id": qc.subject,
		"grade_levels": []int{3}, "curriculum_version_id": qc.version, "feedback_timing": "after_close", "result_release": "manual", "definition": def}
}

// F024-AC-10, AC-11, AC-12 (TC-EXM-001): the EXAM_ENGINE §3 matrix; wrong totals, immediate feedback, fixed questions;
// pool per rule (× variants with independent_draw); the configured balance default.
func TestAssessmentBlueprints(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	c.assign(ctx, t, qc.cur, c.tenantB)
	authztest.Cover(t, "listCmsAssessmentBlueprints", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createCmsAssessmentBlueprint", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getCmsAssessmentBlueprint", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createCmsAssessmentBlueprintVersion", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateCmsAssessmentBlueprintDraft", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "deleteCmsAssessmentBlueprint", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getAssessmentBlueprintPoolStats", authztest.CrossTenant, authztest.RoleDenied)
	for range 5 {
		c.published(ctx, t, qlcmA, qc, nil)
	}
	rules := []any{
		map[string]any{"objective_ids": []string{qc.lo}, "count": 8, "points_each": 0.5},
		map[string]any{"topic_ids": []string{qc.topic}, "count": 3, "points_each": 2},
	}
	r := must(t, c.do(t, http.MethodPost, blueprintsPath, qlcmA, idem(), blueprintBody(qc, 10, rules,
		map[string]any{"strategy": "independent_draw", "count": 2})), http.StatusCreated, "blueprint")
	if !strings.Contains(r.raw, `"max_mean_p_diff":0.05`) {
		t.Fatalf("AC-12 default: %s", r.raw)
	}
	r2 := must(t, c.do(t, http.MethodPost, blueprintsPath, qlcmA, idem(), blueprintBody(qc, 9.5, rules, nil)), http.StatusUnprocessableEntity, "9.5")
	if !slices.Contains(r2.errorCodes(), "/definition/total_points:INVALID") {
		t.Fatalf("total %v", r2.errorCodes())
	}
	imm := blueprintBody(qc, 10, rules, nil)
	imm["feedback_timing"] = "immediate"
	must(t, c.do(t, http.MethodPost, blueprintsPath, qlcmA, idem(), imm), http.StatusUnprocessableEntity, "immediate")
	q := c.published(ctx, t, qlcmA, qc, nil)
	fixed := []any{map[string]any{"fixed_question_version_ids": []string{q}, "count": 2, "points_each": 5}}
	must(t, c.do(t, http.MethodPost, blueprintsPath, qlcmA, idem(), blueprintBody(qc, 10, fixed, nil)), http.StatusUnprocessableEntity, "fixed length")
	_, qb := c.question(t, qlcmB, qc, nil)
	fixedB := []any{map[string]any{"fixed_question_version_ids": []string{qb}, "count": 1, "points_each": 10}}
	must(t, c.do(t, http.MethodPost, blueprintsPath, qlcmA, idem(), blueprintBody(qc, 10, fixedB, nil)), http.StatusUnprocessableEntity, "tenant B fixed")
	outside := []any{map[string]any{"objective_ids": []string{"0193a5f4-6c1e-7b2a-9d1f-000000000777"}, "count": 1, "points_each": 10}}
	must(t, c.do(t, http.MethodPost, blueprintsPath, qlcmA, idem(), blueprintBody(qc, 10, outside, nil)), http.StatusUnprocessableEntity, "LO outside")
	// AC-11: 6 published questions of the LO; independent_draw × 2 needs 16 and 6.
	st := must(t, c.do(t, http.MethodGet, blueprintsPath+"/"+r.str("content_id")+"/versions/1:pool-stats", qlcmA, nil, nil), http.StatusOK, "stats")
	got := st.items("rules")
	if len(got) != 2 || got[0]["required"] != float64(16) || got[0]["available"] != float64(6) || got[0]["insufficient"] != true ||
		got[1]["required"] != float64(6) || got[1]["insufficient"] != false || got[0]["section_id"] != "sec_tn" {
		t.Fatalf("stats %s", st.raw)
	}
	must(t, c.do(t, http.MethodGet, blueprintsPath+"/"+r.str("content_id"), qlcmB, nil, nil), http.StatusNotFound, "tenant B")
}
