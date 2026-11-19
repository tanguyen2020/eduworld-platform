//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// Route matrix of F021 (F002-AC-28, NFR-SEC-006, TC-TEN-001): for every operation, a principal without the permission
// is refused (role_denied) and a principal of tenant B never reaches data of tenant A (cross_tenant: 404, or a list
// without it). Catalog operations (subjects, textbooks) are platform data: tenant users are refused and lists hold no
// tenant data.
func TestCurriculumRouteMatrix(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	math := c.subject(t, "math")
	curA, verA := c.curriculum(t, qlcmA, "MATRIX_A")
	topicA := c.topic(t, qlcmA, verA, math, "MATH.G3.T01", 3, "")
	loA := must(t, c.objective(t, qlcmA, topicA, "MATH.G3.NUM.A", nil), http.StatusCreated, "LO A").str("objective_id")
	compA := must(t, c.do(t, http.MethodPost, "/cms/learning/competencies", qlcmA, idem(), map[string]any{"curriculum_version_id": verA,
		"code": "NL_A", "name": map[string]string{"vi": "NL"}, "kind": "quality"}), http.StatusCreated, "competency A").str("competency_id")
	jobA := must(t, c.do(t, http.MethodPost, "/cms/learning/imports", qlcmA, idem(), map[string]any{"asset_id": c.assets.put([]byte("x")).String(),
		"kind": "curriculum_yaml", "dry_run": true}), http.StatusAccepted, "job A").str("import_id")
	series := must(t, c.do(t, http.MethodPost, "/cms/learning/textbook-series", cmNoidung, idem(), map[string]any{"code": "SERIES",
		"name": "Bộ sách"}), http.StatusCreated, "series").str("series_id")
	book := must(t, c.do(t, http.MethodPost, "/cms/learning/textbooks", cmNoidung, idem(), map[string]any{"series_id": series,
		"subject_id": math, "grade_level": 3, "volume": 1}), http.StatusCreated, "book").str("textbook_id")
	sec := must(t, c.do(t, http.MethodPost, "/cms/learning/textbook-sections", cmNoidung, idem(), map[string]any{"textbook_id": book,
		"lesson_no": 1, "title": "Bài 1"}), http.StatusCreated, "section").str("section_id")
	vpath := "/cms/learning/curricula/" + curA + "/versions/1"
	stale := ifm(`W/"1"`)
	type call struct {
		user   string
		method string
		path   string
		h      hdr
		body   any
		want   int
	}
	empty := func(r resp, key string) bool { return len(r.items(key)) == 0 }
	ops := []struct {
		id     string
		denied call
		cross  call
		check  func(resp) bool // cross-tenant answer when it is not a status
	}{
		{"listCmsCurricula", call{noRoleA, "GET", "/cms/learning/curricula", nil, nil, 403}, call{qlcmB, "GET", "/cms/learning/curricula", nil, nil, 200},
			func(r resp) bool { return empty(r, "data") }},
		{"createCurriculum", call{adA, "POST", "/cms/learning/curricula", idem(), map[string]any{"code": "X_X", "name": map[string]string{"vi": "x"}}, 403},
			call{qlcmB, "POST", "/cms/learning/curricula", idem(), map[string]any{"code": "MATRIX_A", "name": map[string]string{"vi": "x"}}, 201}, nil},
		{"getCmsCurriculum", call{noRoleA, "GET", "/cms/learning/curricula/" + curA, nil, nil, 403}, call{qlcmB, "GET", "/cms/learning/curricula/" + curA, nil, nil, 404}, nil},
		{"updateCurriculum", call{adA, "PATCH", "/cms/learning/curricula/" + curA, stale, map[string]any{"framework": "X"}, 403},
			call{qlcmB, "PATCH", "/cms/learning/curricula/" + curA, stale, map[string]any{"framework": "X"}, 404}, nil},
		{"deleteCurriculum", call{adA, "DELETE", "/cms/learning/curricula/" + curA, stale, nil, 403}, call{qlcmB, "DELETE", "/cms/learning/curricula/" + curA, stale, nil, 404}, nil},
		{"listCurriculumVersions", call{noRoleA, "GET", "/cms/learning/curricula/" + curA + "/versions", nil, nil, 403},
			call{qlcmB, "GET", "/cms/learning/curricula/" + curA + "/versions", nil, nil, 404}, nil},
		{"createCurriculumVersion", call{adA, "POST", "/cms/learning/curricula/" + curA + "/versions", idem(), map[string]any{"from_version": 1, "version_label": "2.0"}, 403},
			call{qlcmB, "POST", "/cms/learning/curricula/" + curA + "/versions", idem(), map[string]any{"from_version": 1, "version_label": "2.0"}, 404}, nil},
		{"getCurriculumVersion", call{noRoleA, "GET", vpath, nil, nil, 403}, call{qlcmB, "GET", vpath, nil, nil, 404}, nil},
		{"updateCurriculumVersionDraft", call{adA, "PATCH", vpath, stale, map[string]any{"change_note": "x"}, 403},
			call{qlcmB, "PATCH", vpath, stale, map[string]any{"change_note": "x"}, 404}, nil},
		{"transitionCurriculumVersion", call{adA, "POST", vpath + ":transition", merge(idem(), stale), map[string]any{"action": "submit_review"}, 403},
			call{qlcmB, "POST", vpath + ":transition", merge(idem(), stale), map[string]any{"action": "submit_review"}, 404}, nil},
		{"publishCurriculumVersion", call{adA, "POST", vpath + ":publish", merge(idem(), stale), nil, 403},
			call{qlcmB, "POST", vpath + ":publish", merge(idem(), stale), nil, 404}, nil},
		{"exportCurriculumVersion", call{noRoleA, "GET", vpath + ":export", nil, nil, 403}, call{qlcmB, "GET", vpath + ":export", nil, nil, 404}, nil},
		{"getCurriculumVersionLoMappings", call{noRoleA, "GET", vpath + "/lo-mappings", nil, nil, 403}, call{qlcmB, "GET", vpath + "/lo-mappings", nil, nil, 404}, nil},
		{"putCurriculumVersionLoMappings", call{adA, "PUT", vpath + "/lo-mappings", stale, map[string]any{"items": []any{}}, 403},
			call{qlcmB, "PUT", vpath + "/lo-mappings", stale, map[string]any{"items": []any{}}, 404}, nil},
		{"listCmsSubjects", call{noRoleA, "GET", "/cms/learning/subjects", nil, nil, 403}, call{qlcmB, "GET", "/cms/learning/subjects", nil, nil, 200},
			func(r resp) bool { return len(r.items("data")) == 1 }},
		{"createSubject", call{qlcmA, "POST", "/cms/learning/subjects", idem(), map[string]any{"code": "art", "name": map[string]string{"vi": "x"}, "grade_levels": []int{1}}, 403},
			call{qlcmB, "POST", "/cms/learning/subjects", idem(), map[string]any{"code": "art", "name": map[string]string{"vi": "x"}, "grade_levels": []int{1}}, 403}, nil},
		{"updateSubject", call{qlcmA, "PATCH", "/cms/learning/subjects/" + math, stale, map[string]any{"icon_key": "x"}, 403},
			call{qlcmB, "PATCH", "/cms/learning/subjects/" + math, stale, map[string]any{"icon_key": "x"}, 403}, nil},
		{"listCmsTopics", call{noRoleA, "GET", "/cms/learning/topics?curriculum_version_id=" + verA, nil, nil, 403},
			call{qlcmB, "GET", "/cms/learning/topics?curriculum_version_id=" + verA, nil, nil, 200}, func(r resp) bool { return empty(r, "data") }},
		{"createTopic", call{adA, "POST", "/cms/learning/topics", idem(), map[string]any{"curriculum_version_id": verA, "code": "T", "name": "x", "subject_id": math, "grade_level": 3}, 403},
			call{qlcmB, "POST", "/cms/learning/topics", idem(), map[string]any{"curriculum_version_id": verA, "code": "T", "name": "x", "subject_id": math, "grade_level": 3}, 404}, nil},
		{"updateTopic", call{adA, "PATCH", "/cms/learning/topics/" + topicA, stale, map[string]any{"name": "y"}, 403},
			call{qlcmB, "PATCH", "/cms/learning/topics/" + topicA, stale, map[string]any{"name": "y"}, 404}, nil},
		{"deleteTopic", call{adA, "DELETE", "/cms/learning/topics/" + topicA, stale, nil, 403}, call{qlcmB, "DELETE", "/cms/learning/topics/" + topicA, stale, nil, 404}, nil},
		{"listCmsObjectives", call{noRoleA, "GET", "/cms/learning/objectives?curriculum_version_id=" + verA, nil, nil, 403},
			call{qlcmB, "GET", "/cms/learning/objectives?curriculum_version_id=" + verA, nil, nil, 200}, func(r resp) bool { return empty(r, "data") }},
		{"createObjective", call{adA, "POST", "/cms/learning/objectives", idem(), map[string]any{"topic_id": topicA, "code": "MATH.G3.NUM.Z", "description": "x"}, 403},
			call{qlcmB, "POST", "/cms/learning/objectives", idem(), map[string]any{"topic_id": topicA, "code": "MATH.G3.NUM.Z", "description": "x"}, 404}, nil},
		{"updateObjective", call{adA, "PATCH", "/cms/learning/objectives/" + loA, stale, map[string]any{"description": "y"}, 403},
			call{qlcmB, "PATCH", "/cms/learning/objectives/" + loA, stale, map[string]any{"description": "y"}, 404}, nil},
		{"deleteObjective", call{adA, "DELETE", "/cms/learning/objectives/" + loA, stale, nil, 403}, call{qlcmB, "DELETE", "/cms/learning/objectives/" + loA, stale, nil, 404}, nil},
		{"listCmsCompetencies", call{noRoleA, "GET", "/cms/learning/competencies?curriculum_version_id=" + verA, nil, nil, 403},
			call{qlcmB, "GET", "/cms/learning/competencies?curriculum_version_id=" + verA, nil, nil, 200}, func(r resp) bool { return empty(r, "data") }},
		{"createCompetency", call{adA, "POST", "/cms/learning/competencies", idem(), map[string]any{"curriculum_version_id": verA, "code": "NL_B", "name": map[string]string{"vi": "x"}, "kind": "quality"}, 403},
			call{qlcmB, "POST", "/cms/learning/competencies", idem(), map[string]any{"curriculum_version_id": verA, "code": "NL_B", "name": map[string]string{"vi": "x"}, "kind": "quality"}, 404}, nil},
		{"updateCompetency", call{adA, "PATCH", "/cms/learning/competencies/" + compA, stale, map[string]any{"name": map[string]string{"vi": "y"}}, 403},
			call{qlcmB, "PATCH", "/cms/learning/competencies/" + compA, stale, map[string]any{"name": map[string]string{"vi": "y"}}, 404}, nil},
		{"listTextbookSeries", call{noRoleA, "GET", "/cms/learning/textbook-series", nil, nil, 403}, call{qlcmB, "GET", "/cms/learning/textbook-series", nil, nil, 200},
			func(r resp) bool { return len(r.items("data")) == 1 }},
		{"createTextbookSeries", call{qlcmA, "POST", "/cms/learning/textbook-series", idem(), map[string]any{"code": "SX", "name": "x"}, 403},
			call{qlcmB, "POST", "/cms/learning/textbook-series", idem(), map[string]any{"code": "SX", "name": "x"}, 403}, nil},
		{"listTextbooks", call{noRoleA, "GET", "/cms/learning/textbooks", nil, nil, 403}, call{qlcmB, "GET", "/cms/learning/textbooks", nil, nil, 200},
			func(r resp) bool { return len(r.items("data")) == 1 }},
		{"createTextbook", call{qlcmA, "POST", "/cms/learning/textbooks", idem(), map[string]any{"series_id": series, "subject_id": math, "grade_level": 4, "volume": 1}, 403},
			call{qlcmB, "POST", "/cms/learning/textbooks", idem(), map[string]any{"series_id": series, "subject_id": math, "grade_level": 4, "volume": 1}, 403}, nil},
		{"listTextbookSections", call{noRoleA, "GET", "/cms/learning/textbook-sections?textbook_id=" + book, nil, nil, 403},
			call{qlcmB, "GET", "/cms/learning/textbook-sections?textbook_id=" + book, nil, nil, 200}, func(r resp) bool { return len(r.items("data")) == 1 }},
		{"createTextbookSection", call{qlcmA, "POST", "/cms/learning/textbook-sections", idem(), map[string]any{"textbook_id": book, "lesson_no": 2, "title": "x"}, 403},
			call{qlcmB, "POST", "/cms/learning/textbook-sections", idem(), map[string]any{"textbook_id": book, "lesson_no": 2, "title": "x"}, 403}, nil},
		{"updateTextbookSection", call{qlcmA, "PATCH", "/cms/learning/textbook-sections/" + sec, stale, map[string]any{"title": "y"}, 403},
			call{qlcmB, "PATCH", "/cms/learning/textbook-sections/" + sec, stale, map[string]any{"title": "y"}, 403}, nil},
		// Cross-tenant of the section mapping: platform staff cannot map an LO of tenant A (invisible to PlatformScope).
		{"putTextbookSectionObjectives", call{qlcmA, "PUT", "/cms/learning/textbook-sections/" + sec + "/objectives", idem(), map[string]any{"objective_ids": []string{}}, 403},
			call{cmNoidung, "PUT", "/cms/learning/textbook-sections/" + sec + "/objectives", idem(), map[string]any{"objective_ids": []string{loA}}, 422}, nil},
		{"createContentImport", call{noRoleA, "POST", "/cms/learning/imports", idem(), map[string]any{"asset_id": loA, "kind": "curriculum_yaml"}, 403},
			call{qlcmB, "POST", "/cms/learning/imports", idem(), map[string]any{"asset_id": loA, "kind": "curriculum_yaml"}, 202}, nil},
		{"getContentImport", call{noRoleA, "GET", "/cms/learning/imports/" + jobA, nil, nil, 403}, call{qlcmB, "GET", "/cms/learning/imports/" + jobA, nil, nil, 404}, nil},
		// Public reads: a non-member of the tenant is refused (403 TENANT_MISMATCH); tenant B sees nothing of A.
		{"listCurricula", call{cmNoidung, "GET", "/learning/curricula", hdr{"X-Tenant-Id": c.tenantA.String()}, nil, 403},
			call{adB, "GET", "/learning/curricula", nil, nil, 200}, func(r resp) bool { return empty(r, "items") }},
		{"listSubjects", call{cmNoidung, "GET", "/learning/subjects", hdr{"X-Tenant-Id": c.tenantA.String()}, nil, 403},
			call{adB, "GET", "/learning/subjects", nil, nil, 200}, func(r resp) bool { return len(r.items("items")) == 1 }},
		{"listTopics", call{cmNoidung, "GET", "/learning/topics", hdr{"X-Tenant-Id": c.tenantA.String()}, nil, 403},
			call{adB, "GET", "/learning/topics", nil, nil, 200}, func(r resp) bool { return empty(r, "data") }},
		{"listLearningObjectives", call{cmNoidung, "GET", "/learning/objectives", hdr{"X-Tenant-Id": c.tenantA.String()}, nil, 403},
			call{adB, "GET", "/learning/objectives", nil, nil, 200}, func(r resp) bool { return empty(r, "data") }},
		{"getLearningObjective", call{cmNoidung, "GET", "/learning/objectives/" + loA, hdr{"X-Tenant-Id": c.tenantA.String()}, nil, 403},
			call{adB, "GET", "/learning/objectives/" + loA, nil, nil, 404}, nil},
	}
	covered := map[string]bool{}
	for _, o := range ops {
		covered[o.id] = true
		d := c.do(t, o.denied.method, o.denied.path, o.denied.user, o.denied.h, o.denied.body)
		if d.status != o.denied.want {
			t.Errorf("%s role_denied: %d, want %d: %s", o.id, d.status, o.denied.want, d.raw)
		}
		x := c.do(t, o.cross.method, o.cross.path, o.cross.user, o.cross.h, o.cross.body)
		if x.status != o.cross.want || (o.check != nil && !o.check(x)) {
			t.Errorf("%s cross_tenant: %d, want %d: %s", o.id, x.status, o.cross.want, x.raw)
		}
	}
	// The import job created by tenant B (cross case of createContentImport) is invisible to tenant A.
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.import_jobs WHERE tenant_id = $1`, c.tenantB); n != 1 {
		t.Fatalf("%d jobs of B", n)
	}
	if len(covered) != 43 {
		t.Fatalf("%d operations covered", len(covered))
	}
	authztest.Cover(t, "listCmsCurricula", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createCurriculum", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getCmsCurriculum", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateCurriculum", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "deleteCurriculum", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listCurriculumVersions", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createCurriculumVersion", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getCurriculumVersion", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateCurriculumVersionDraft", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "transitionCurriculumVersion", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "publishCurriculumVersion", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "exportCurriculumVersion", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getCurriculumVersionLoMappings", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "putCurriculumVersionLoMappings", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listCmsSubjects", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createSubject", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateSubject", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listCmsTopics", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createTopic", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateTopic", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "deleteTopic", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listCmsObjectives", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createObjective", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateObjective", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "deleteObjective", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listCmsCompetencies", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createCompetency", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateCompetency", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listTextbookSeries", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createTextbookSeries", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listTextbooks", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createTextbook", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listTextbookSections", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createTextbookSection", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateTextbookSection", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "putTextbookSectionObjectives", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createContentImport", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getContentImport", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listCurricula", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listSubjects", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listTopics", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listLearningObjectives", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getLearningObjective", authztest.CrossTenant, authztest.RoleDenied)
}
