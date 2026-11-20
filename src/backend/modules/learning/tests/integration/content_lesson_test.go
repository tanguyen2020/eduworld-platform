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

const (
	lessonsPath   = "/cms/learning/lessons"
	materialsPath = "/cms/learning/learning-materials"
)

func rich(text string) map[string]any {
	return map[string]any{"blocks": []any{map[string]any{"type": "paragraph", "text": text}}}
}

func workedExample(id string, steps int, lo string) map[string]any {
	s := make([]any, steps)
	for i := range s {
		s[i] = map[string]any{"content": rich("Bước")}
	}
	return map[string]any{"kind": "worked_example", "id": id, "problem": rich("7 × 8 = ?"), "steps": s, "objective_ids": []string{lo}}
}

func lessonBody(qc qbCurriculum, blocks ...any) map[string]any {
	return map[string]any{"title": map[string]any{"vi": "Bảng nhân 7 – nhân vật Mèo"}, "subject_id": qc.subject, "grade_level": 3,
		"curriculum_version_id": qc.version, "topic_id": qc.topic, "objective_ids": []string{qc.lo}, "completion_rule": "view_only", "blocks": blocks}
}

// F024-AC-06, AC-07, AC-08, AC-09: blocks and worked examples; questions reference a worked example of a shared LO;
// learning material CRUD with the rules of lessons.
func TestLessonsMaterialsWorkedExamples(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	authztest.Cover(t, "listCmsLessons", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createCmsLesson", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getCmsLesson", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createCmsLessonVersion", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateCmsLessonDraft", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "deleteCmsLesson", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listCmsLearningMaterials", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createCmsLearningMaterial", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getCmsLearningMaterial", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createCmsLearningMaterialVersion", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateCmsLearningMaterialDraft", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "deleteCmsLearningMaterial", authztest.CrossTenant, authztest.RoleDenied)
	// AC-06 / AC-07.
	must(t, c.do(t, http.MethodPost, lessonsPath, qlcmA, idem(), lessonBody(qc, workedExample("we_mul7", 9, qc.lo))), http.StatusUnprocessableEntity, "9 steps")
	// Activity blocks are enabled since F030 is wired (F031 US6 lessons with activities).
	r := must(t, c.do(t, http.MethodPost, lessonsPath, qlcmA, idem(), lessonBody(qc, map[string]any{"kind": "activity",
		"activity_id": "0193a5f4-6c1e-7b2a-9d1f-000000000999"})), http.StatusCreated, "activity")
	if strings.Contains(r.raw, "BLOCK_KIND_NOT_ENABLED") {
		t.Fatalf("activity: %s", r.raw)
	}
	must(t, c.do(t, http.MethodPost, lessonsPath, qlcmA, idem(), lessonBody(qc, workedExample("we_a", 1, qc.lo), workedExample("we_a", 1, qc.lo))),
		http.StatusUnprocessableEntity, "duplicate id")
	ls := must(t, c.do(t, http.MethodPost, lessonsPath, qlcmA, idem(), lessonBody(qc, map[string]any{"kind": "rich_text", "content": rich("Học bảng nhân 7")},
		workedExample("we_mul7", 3, qc.lo))), http.StatusCreated, "lesson")
	if ls.str("grade_level") != "3" || ls.str("topic_id") != qc.topic || len(ls.items("blocks")) != 2 {
		t.Fatalf("lesson view %s", ls.raw)
	}
	lessonID := ls.str("content_id")
	// AC-08: a question may reference we_mul7 (same LO); an unknown block or no shared LO is 422.
	ref := map[string]any{"content_item_id": lessonID, "block_id": "we_mul7"}
	must(t, c.createQ(t, qlcmA, qc.body(qbFixture(t, "single_choice"), map[string]any{"worked_example_ref": ref})), http.StatusCreated, "ref ok")
	bad := map[string]any{"content_item_id": lessonID, "block_id": "we_khong_co"}
	r = must(t, c.createQ(t, qlcmA, qc.body(qbFixture(t, "single_choice"), map[string]any{"worked_example_ref": bad})), http.StatusUnprocessableEntity, "no block")
	if !slices.Contains(r.errorCodes(), "/worked_example_ref/block_id:INVALID") {
		t.Fatalf("errors %v", r.errorCodes())
	}
	r = must(t, c.createQ(t, qlcmA, qc.body(qbFixture(t, "single_choice"), map[string]any{"worked_example_ref": ref,
		"objective_ids": []string{qc.lo2}})), http.StatusUnprocessableEntity, "no shared LO")
	must(t, c.createQ(t, qlcmB, qc.body(qbFixture(t, "single_choice"), map[string]any{"worked_example_ref": ref})), http.StatusUnprocessableEntity,
		"other tenant lesson")
	// AC-09: learning material CRUD.
	mb := lessonBody(qc, workedExample("we_vd1", 2, qc.lo))
	delete(mb, "completion_rule")
	mb["material_kind"] = "worked_example"
	m := must(t, c.do(t, http.MethodPost, materialsPath, qlcmA, idem(), mb), http.StatusCreated, "material")
	mid := m.str("content_id")
	g := must(t, c.do(t, http.MethodGet, materialsPath+"/"+mid, qlcmA, nil, nil), http.StatusOK, "get material")
	if g.str("material_kind") != "worked_example" {
		t.Fatalf("material %s", g.raw)
	}
	p := must(t, c.do(t, http.MethodPatch, materialsPath+"/"+mid+"/versions/1", qlcmA, ifm(g.header.Get("ETag")),
		map[string]any{"title": map[string]any{"vi": "Ví dụ mẫu bảng nhân"}}), http.StatusOK, "patch material")
	if !strings.Contains(p.raw, "Ví dụ mẫu bảng nhân") {
		t.Fatalf("patch %s", p.raw)
	}
	must(t, c.createQ(t, qlcmA, qc.body(qbFixture(t, "single_choice"), map[string]any{"worked_example_ref": map[string]any{"content_item_id": mid,
		"block_id": "we_vd1"}})), http.StatusCreated, "ref to material")
	c.publishDoc(ctx, t, "learning_materials", "learning_material_versions", "learning_material_id", mid, 1)
	must(t, c.do(t, http.MethodPost, materialsPath+"/"+mid+"/versions", qlcmA, idem(), map[string]any{"from_version": 1}), http.StatusCreated, "material v2")
	l := must(t, c.do(t, http.MethodGet, materialsPath, qlcmA, nil, nil), http.StatusOK, "list materials")
	if len(l.items("data")) != 1 || l.items("data")[0]["version"] != float64(2) {
		t.Fatalf("list %s", l.raw)
	}
	g = must(t, c.do(t, http.MethodGet, materialsPath+"/"+mid, qlcmA, nil, nil), http.StatusOK, "get v2")
	must(t, c.do(t, http.MethodDelete, materialsPath+"/"+mid, qlcmA, ifm(g.header.Get("ETag")), nil), http.StatusConflict, "released")
	must(t, c.do(t, http.MethodGet, materialsPath+"/"+mid, qlcmB, nil, nil), http.StatusNotFound, "tenant B")
	lg := must(t, c.do(t, http.MethodGet, lessonsPath+"/"+lessonID, qlcmA, nil, nil), http.StatusOK, "get lesson")
	must(t, c.do(t, http.MethodDelete, lessonsPath+"/"+lessonID, qlcmA, nil, nil), http.StatusPreconditionRequired, "delete without If-Match")
	must(t, c.do(t, http.MethodDelete, lessonsPath+"/"+lessonID, qlcmA, ifm(lg.header.Get("ETag")), nil), http.StatusNoContent, "delete lesson")
	must(t, c.do(t, http.MethodGet, lessonsPath+"/"+lessonID, qlcmA, nil, nil), http.StatusNotFound, "deleted")
	must(t, c.do(t, http.MethodPost, materialsPath, noRoleA, idem(), mb), http.StatusForbidden, "no role")
}
