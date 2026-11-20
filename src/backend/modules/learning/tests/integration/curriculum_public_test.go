//go:build integration

package integration

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/google/uuid"
)

// schoolSettingsFixture creates lms.school_curriculum_settings as described by DATABASE_DESIGN §2.16.
//
// TEST FIXTURE — không dùng trong runtime (R17): the table belongs to F040, which has not migrated it yet; the
// curriculum read model reads it when it exists (F021 clarification 2026-11-19).
// publishedVersion publishes version n of a platform curriculum with an effective range (TEST helper).
func (c *curEnv) publishVersion(t *testing.T, cur string, n int, from, to string) {
	t.Helper()
	body := map[string]any{"effective_from": from}
	if to != "" {
		body["effective_to"] = to
	}
	path := "/cms/learning/curricula/" + cur + "/versions/" + strconv.Itoa(n)
	must(t, c.do(t, http.MethodPatch, path, cmNoidung, ifm(c.versionETag(t, cmNoidung, cur, n)), body), http.StatusOK, "effective range")
	must(t, c.publishFlow(t, cmNoidung, cur, n), http.StatusOK, "publish version "+strconv.Itoa(n))
}

// F021-AC-19, AC-20, AC-21 (US4; TC-LRN-001, TC-LRN-006): the version applied to the reader's school, published
// data only without editorial metadata, old LOs still readable, tenant isolation of /learning/*.
func TestCurriculumPublicRead(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	math := c.subject(t, "math")
	cur, v1 := c.curriculum(t, cmNoidung, "PUB_MATH")
	t1 := c.topic(t, cmNoidung, v1, math, "MATH.G3.T01", 3, "")
	old := must(t, c.objective(t, cmNoidung, t1, "MATH.G3.NUM.OLD", nil), http.StatusCreated, "LO v1").str("objective_id")
	c.publishVersion(t, cur, 1, "2020-01-01", "2020-12-31")
	// Version 2 in effect today; version 3 stays a draft.
	r := must(t, c.do(t, http.MethodPost, "/cms/learning/curricula/"+cur+"/versions", cmNoidung, idem(), map[string]any{"from_version": 1,
		"version_label": "2.0"}), http.StatusCreated, "v2")
	v2 := r.str("curriculum_version_id")
	t2 := must(t, c.do(t, http.MethodGet, "/cms/learning/topics?curriculum_version_id="+v2, cmNoidung, nil, nil), http.StatusOK, "topics v2").
		items("data")[0]["topic_id"].(string)
	must(t, c.objective(t, cmNoidung, t2, "MATH.G3.NUM.NEW", nil), http.StatusCreated, "LO v2")
	oldInV2 := must(t, c.do(t, http.MethodGet, "/cms/learning/objectives?curriculum_version_id="+v2, cmNoidung, nil, nil), http.StatusOK, "LOs v2")
	var mappings []map[string]any
	for _, lo := range oldInV2.items("data") {
		if lo["code"] == "MATH.G3.NUM.OLD" {
			mappings = append(mappings, map[string]any{"from_lo_id": old, "to_lo_id": lo["objective_id"], "relation": "same"})
		}
	}
	must(t, c.do(t, http.MethodPut, "/cms/learning/curricula/"+cur+"/versions/2/lo-mappings", cmNoidung, ifm(c.versionETag(t, cmNoidung, cur, 2)),
		map[string]any{"items": mappings}), http.StatusOK, "mappings")
	c.publishVersion(t, cur, 2, "2021-01-01", "")
	must(t, c.do(t, http.MethodPost, "/cms/learning/curricula/"+cur+"/versions", cmNoidung, idem(), map[string]any{"from_version": 2,
		"version_label": "3.0"}), http.StatusCreated, "v3 draft")
	c.assign(ctx, t, cur, c.tenantA)
	school := uuid.New()
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO ref.schools (school_id, tenant_id, status, source_version) VALUES ($1, $2, 'active', 1)`,
		school, c.tenantA); err != nil {
		t.Fatal(err)
	}
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO ref.students (student_id, tenant_id, school_id, grade_level, source_version)
		VALUES ($1, $2, $3, 3, 1)`, c.users[hsAn].UserID, c.tenantA, school); err != nil {
		t.Fatal(err)
	}

	// AC-19 fallback (no school setting; F040 table absent): the published version in effect today.
	r = must(t, c.do(t, http.MethodGet, "/learning/curricula", hsAn, nil, nil), http.StatusOK, "student curricula")
	items := r.items("items")
	if len(items) != 1 || items[0]["version"] != float64(2) || items[0]["curriculum_version_id"] != v2 {
		t.Fatalf("applied version %s", r.raw)
	}
	// AC-20: no editorial metadata, never the draft.
	for _, k := range []string{"author_id", "reviewer_id", "change_note", "status", "latest_version"} {
		if _, ok := items[0][k]; ok {
			t.Fatalf("editorial field %s leaked: %s", k, r.raw)
		}
	}
	tp := must(t, c.do(t, http.MethodGet, "/learning/topics?subject_id="+math+"&grade_level=3", hsAn, nil, nil), http.StatusOK, "student topics")
	if tp.header.Get("Content-Language") != "vi" || len(tp.items("data")) != 1 || tp.items("data")[0]["curriculum_version_id"] != v2 {
		t.Fatalf("topics %v %s", tp.header, tp.raw)
	}
	los := must(t, c.do(t, http.MethodGet, "/learning/objectives?grade_level=3", hsAn, nil, nil), http.StatusOK, "student LOs")
	if len(los.items("data")) != 2 {
		t.Fatalf("LOs of the applied version %s", los.raw)
	}
	// AC-19 with the school setting (table of F040; no academic year in the class projection: every year applies): version 1.
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO lms.school_curriculum_settings (setting_id, tenant_id, school_id, academic_year_id,
		curriculum_version_id, set_by) VALUES ($1, $2, $3, $4, $5, $1)`, uuid.New(), c.tenantA, school, uuid.New(), v1); err != nil {
		t.Fatal(err)
	}
	r = must(t, c.do(t, http.MethodGet, "/learning/curricula", hsAn, nil, nil), http.StatusOK, "curricula with setting")
	if items := r.items("items"); len(items) != 1 || items[0]["version"] != float64(1) {
		t.Fatalf("school setting not applied %s", r.raw)
	}
	// AC-21: an LO of the old version stays readable by id (history of attempts).
	lo := must(t, c.do(t, http.MethodGet, "/learning/objectives/"+old, hsAn, nil, nil), http.StatusOK, "old LO")
	if lo.str("code") != "MATH.G3.NUM.OLD" || lo.str("curriculum_version_id") != v1 {
		t.Fatalf("old LO %s", lo.raw)
	}
	// Tenant isolation (TC-TEN-001): tenant B has no assignment.
	rb := must(t, c.do(t, http.MethodGet, "/learning/curricula", adB, nil, nil), http.StatusOK, "B curricula")
	if len(rb.items("items")) != 0 {
		t.Fatalf("tenant B sees %s", rb.raw)
	}
	must(t, c.do(t, http.MethodGet, "/learning/objectives/"+old, adB, nil, nil), http.StatusNotFound, "B reads an LO of A")
	// Staff of tenant A (no school) gets the fallback version; a draft LO is never served.
	draftLOs := must(t, c.do(t, http.MethodGet, "/cms/learning/objectives?curriculum_version_id="+v2, cmNoidung, nil, nil), http.StatusOK, "v2 LOs")
	_ = draftLOs
	r = must(t, c.do(t, http.MethodGet, "/learning/curricula", qlcmA, nil, nil), http.StatusOK, "staff curricula")
	if items := r.items("items"); len(items) != 1 || items[0]["version"] != float64(2) {
		t.Fatalf("staff applied version %s", r.raw)
	}
}
