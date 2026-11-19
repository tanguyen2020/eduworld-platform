//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
)

// F021-AC-01…05, AC-11, AC-12 (US1, US2) through the HTTP chain: create curriculum + version 1 draft (idempotent),
// LO code and yccd_ref/competency rules, topic tree, prerequisite DAG with the cycle path.
func TestCurriculumAuthoring(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	math := c.subject(t, "math")

	// AC-01: curriculum + version 1 in draft; replay with the same Idempotency-Key returns the stored answer.
	key := idem()
	body := map[string]any{"code": "TEST_MATH", "name": map[string]string{"vi": "Toán thử"}, "framework": "GDPT_2018"}
	r := must(t, c.do(t, http.MethodPost, "/cms/learning/curricula", cmNoidung, key, body), http.StatusCreated, "create")
	if r.str("owner_scope") != "platform" || r.str("latest_version", "status") != "draft" || r.str("latest_version", "version_label") != "1.0" ||
		r.str("latest_version_no") != "1" || r.header.Get("ETag") == "" {
		t.Fatalf("created curriculum %s", r.raw)
	}
	again := must(t, c.do(t, http.MethodPost, "/cms/learning/curricula", cmNoidung, key, body), http.StatusCreated, "replay")
	if again.header.Get("Idempotent-Replayed") != "true" || again.str("curriculum_id") != r.str("curriculum_id") {
		t.Fatalf("replay %v %s", again.header, again.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM curriculum.curricula WHERE code = 'TEST_MATH'`); n != 1 {
		t.Fatalf("%d curricula after replay", n)
	}
	// Code unique per owner scope (409), UPPER_SNAKE (422 from the contract).
	must(t, c.do(t, http.MethodPost, "/cms/learning/curricula", cmNoidung, idem(), body), http.StatusConflict, "duplicate code")
	bad := must(t, c.do(t, http.MethodPost, "/cms/learning/curricula", cmNoidung, idem(), map[string]any{"code": "test-math",
		"name": map[string]string{"vi": "x"}}), http.StatusUnprocessableEntity, "lower case code")
	if !slices.Contains(bad.errorCodes(), "code:INVALID") {
		t.Fatalf("code error %v", bad.errorCodes())
	}
	// AC-02: several curricula coexist; a tenant may reuse a platform code (owner scope).
	cur, version := r.str("curriculum_id"), r.str("latest_version", "curriculum_version_id")
	c.curriculum(t, cmNoidung, "TEST_VIET")
	c.curriculum(t, qlcmA, "TEST_MATH")
	list := must(t, c.do(t, http.MethodGet, "/cms/learning/curricula", cmNoidung, nil, nil), http.StatusOK, "list")
	if len(list.items("data")) != 2 {
		t.Fatalf("platform staff sees %d curricula: %s", len(list.items("data")), list.raw)
	}

	// Topics (AC-05): the subject × grade pair is created on the fly; a parent of another grade or a loop is refused.
	t1 := c.topic(t, cmNoidung, version, math, "MATH.G3.T01", 3, "")
	t2 := c.topic(t, cmNoidung, version, math, "MATH.G3.T02", 3, t1)
	t4 := c.topic(t, cmNoidung, version, math, "MATH.G4.T01", 4, "")
	r = c.do(t, http.MethodPost, "/cms/learning/topics", cmNoidung, idem(), map[string]any{"curriculum_version_id": version, "code": "MATH.G3.T03",
		"name": "x", "subject_id": math, "grade_level": 3, "parent_topic_id": t4})
	if must(t, r, http.StatusUnprocessableEntity, "parent of another grade"); !slices.Contains(r.errorCodes(), "parent_topic_id:INVALID") {
		t.Fatalf("parent error %v", r.errorCodes())
	}
	tag := c.do(t, http.MethodGet, "/cms/learning/topics?curriculum_version_id="+version, cmNoidung, nil, nil)
	if len(tag.items("data")) != 3 {
		t.Fatalf("topics %s", tag.raw)
	}
	t1etag := topicETag(t, c, version, t1)
	must(t, c.do(t, http.MethodPatch, "/cms/learning/topics/"+t1, cmNoidung, ifm(t1etag), map[string]any{"parent_topic_id": t2}),
		http.StatusUnprocessableEntity, "topic loop")
	v := must(t, c.do(t, http.MethodGet, "/cms/learning/curricula/"+cur+"/versions/1", cmNoidung, nil, nil), http.StatusOK, "version")
	if len(v.items("subject_grades")) != 2 {
		t.Fatalf("subject grades %s", v.raw)
	}

	// AC-03: LO code format and G<grade> = topic grade, 422 naming the field.
	for _, code := range []string{"math-g3-x", "MATH.G4.NUM.MUL", "MATH.G3.NUMBERS.X"} {
		r = c.objective(t, cmNoidung, t1, code, nil)
		if must(t, r, http.StatusUnprocessableEntity, "LO code "+code); !slices.Contains(r.errorCodes(), "code:INVALID") {
			t.Fatalf("%s: %v", code, r.errorCodes())
		}
	}
	a := must(t, c.objective(t, cmNoidung, t1, "MATH.G3.NUM.MUL_TABLE.2", map[string]any{"bloom_level": "remember"}), http.StatusCreated, "LO A")
	if a.str("stable_key") != "MATH.G3.NUM.MUL_TABLE.2" {
		t.Fatalf("stable_key %s", a.raw)
	}
	must(t, c.objective(t, cmNoidung, t1, "MATH.G3.NUM.MUL_TABLE.2", nil), http.StatusConflict, "duplicate LO code")
	// AC-04: yccd_ref of 65 characters; a competency of another version.
	r = c.objective(t, cmNoidung, t1, "MATH.G3.NUM.Y", map[string]any{"yccd_ref": strings.Repeat("y", 65)})
	if must(t, r, http.StatusUnprocessableEntity, "yccd_ref 65"); !slices.Contains(r.errorCodes(), "yccd_ref:MAX_LENGTH") {
		t.Fatalf("yccd_ref %v", r.errorCodes())
	}
	_, otherVersion := c.curriculum(t, cmNoidung, "TEST_OTHER")
	comp := must(t, c.do(t, http.MethodPost, "/cms/learning/competencies", cmNoidung, idem(), map[string]any{"curriculum_version_id": otherVersion,
		"code": "NL_TU_CHU", "name": map[string]string{"vi": "Tự chủ"}, "kind": "general_competency"}), http.StatusCreated, "competency")
	r = c.objective(t, cmNoidung, t1, "MATH.G3.NUM.C", map[string]any{"competency_ids": []string{comp.str("competency_id")}})
	if must(t, r, http.StatusUnprocessableEntity, "competency of another version"); !slices.Contains(r.errorCodes(), "competency_ids:INVALID") {
		t.Fatalf("competency %v", r.errorCodes())
	}
	own := must(t, c.do(t, http.MethodPost, "/cms/learning/competencies", cmNoidung, idem(), map[string]any{"curriculum_version_id": version,
		"code": "NL_TOAN", "name": map[string]string{"vi": "Tư duy toán"}, "kind": "subject_competency", "subject_id": math}), http.StatusCreated, "own competency")
	must(t, c.objective(t, cmNoidung, t1, "MATH.G3.NUM.C", map[string]any{"competency_ids": []string{own.str("competency_id")},
		"yccd_ref": strings.Repeat("y", 64)}), http.StatusCreated, "LO with competency + yccd 64")

	// AC-11, AC-12: A→B→C then C→A is a cycle listing A, B, C; prerequisites of another version, self, > 20.
	b := must(t, c.objective(t, cmNoidung, t1, "MATH.G3.NUM.B", map[string]any{"prerequisite_ids": []string{a.str("objective_id")}}), http.StatusCreated, "B")
	cc := must(t, c.objective(t, cmNoidung, t2, "MATH.G3.NUM.CC", map[string]any{"prerequisite_ids": []string{b.str("objective_id")}}), http.StatusCreated, "C")
	aTag := objectiveETag(t, c, version, a.str("objective_id"))
	r = c.do(t, http.MethodPatch, "/cms/learning/objectives/"+a.str("objective_id"), cmNoidung, ifm(aTag),
		map[string]any{"prerequisite_ids": []string{cc.str("objective_id")}})
	if must(t, r, http.StatusUnprocessableEntity, "cycle"); !slices.Contains(r.errorCodes(), "prerequisite_ids:PREREQUISITE_CYCLE") {
		t.Fatalf("cycle error %v", r.errorCodes())
	}
	cycle, _ := r.body["cycle"].([]any)
	got := make([]string, len(cycle))
	for i, x := range cycle {
		got[i] = fmt.Sprint(x)
	}
	for _, code := range []string{"MATH.G3.NUM.MUL_TABLE.2", "MATH.G3.NUM.B", "MATH.G3.NUM.CC"} {
		if !slices.Contains(got, code) {
			t.Fatalf("cycle %v misses %s", got, code)
		}
	}
	if aTag2 := objectiveETag(t, c, version, a.str("objective_id")); aTag2 != aTag {
		t.Fatal("the cycle was saved")
	}
	foreign := c.topic(t, cmNoidung, otherVersion, math, "MATH.G3.T01", 3, "")
	fo := must(t, c.objective(t, cmNoidung, foreign, "MATH.G3.NUM.F", nil), http.StatusCreated, "foreign LO")
	r = c.objective(t, cmNoidung, t1, "MATH.G3.NUM.D", map[string]any{"prerequisite_ids": []string{fo.str("objective_id")}})
	if must(t, r, http.StatusUnprocessableEntity, "prerequisite of another version"); !slices.Contains(r.errorCodes(), "prerequisite_ids:INVALID") {
		t.Fatalf("foreign prerequisite %v", r.errorCodes())
	}
	r = c.do(t, http.MethodPatch, "/cms/learning/objectives/"+a.str("objective_id"), cmNoidung, ifm(aTag),
		map[string]any{"prerequisite_ids": []string{a.str("objective_id")}})
	must(t, r, http.StatusUnprocessableEntity, "self prerequisite")
	many := make([]string, 21)
	for i := range many {
		many[i] = uuid.NewString()
	}
	must(t, c.objective(t, cmNoidung, t1, "MATH.G3.NUM.E", map[string]any{"prerequisite_ids": many}), http.StatusUnprocessableEntity, "21 prerequisites")
	// Edge case: an LO that is a prerequisite cannot be deleted (422 lists the dependents).
	r = c.do(t, http.MethodDelete, "/cms/learning/objectives/"+b.str("objective_id"), cmNoidung, ifm(objectiveETag(t, c, version, b.str("objective_id"))), nil)
	if must(t, r, http.StatusUnprocessableEntity, "delete a prerequisite"); !strings.Contains(r.raw, "MATH.G3.NUM.CC") {
		t.Fatalf("dependents %s", r.raw)
	}
	// Filters of listCmsObjectives.
	l := must(t, c.do(t, http.MethodGet, "/cms/learning/objectives?curriculum_version_id="+version+"&topic_id="+t1, cmNoidung, nil, nil), http.StatusOK, "list LO")
	if len(l.items("data")) != 3 {
		t.Fatalf("LOs of t1: %s", l.raw)
	}
}

func topicETag(t *testing.T, c *curEnv, _ string, id string) string {
	t.Helper()
	return rowETag(t, c, "curriculum.topics", "topic_id", id)
}

func objectiveETag(t *testing.T, c *curEnv, _ string, id string) string {
	t.Helper()
	return rowETag(t, c, "curriculum.learning_objectives", "objective_id", id)
}

// rowETag reads the row version of a child row (lists carry no ETag) — TEST helper reading as learning_platform (BYPASSRLS).
func rowETag(t *testing.T, c *curEnv, table, idColumn, id string) string {
	t.Helper()
	return fmt.Sprintf(`W/"%d"`, count(context.Background(), t, c.platform, `SELECT version FROM `+table+` WHERE `+idColumn+` = $1`, id))
}

// listCurriculumTrash lists the ids in the trash of resource type curriculum through the F020 use case (System Admin).
func listCurriculumTrash(ctx context.Context, t *testing.T, c *curEnv) []string {
	t.Helper()
	codec, _ := page.NewCodec([]byte(strings.Repeat("k", 32)))
	q := query.ListTrash{Session: c.read, Resources: c.cmd.Resources, Cursors: codec}
	p, err := q.Handle(authztest.Ctx(ctx, authztest.Platform("system_admin")), query.ListTrashInput{ResourceType: "curriculum", Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, len(p.Data))
	for i, e := range p.Data {
		out[i] = e.ResourceID.String()
	}
	return out
}

// F021-AC-06, AC-08 (TC-TEN-001): permissions and tenant isolation of the CMS endpoints.
func TestCurriculumAuthzIsolation(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	math := c.subject(t, "math")
	curA, verA := c.curriculum(t, qlcmA, "TRUONG_A")
	topicA := c.topic(t, qlcmA, verA, math, "MATH.G3.T01", 3, "")

	// AC-06: school_admin has no curriculum.write → 403 + authz.denied; tenant user writing the subject catalog or a
	// platform curriculum → 403 + authz.denied.
	before := c.audits(ctx, t, "authz.denied", nil)
	must(t, c.do(t, http.MethodPost, "/cms/learning/topics", adA, idem(), map[string]any{"curriculum_version_id": verA, "code": "X",
		"name": "x", "subject_id": math, "grade_level": 3}), http.StatusForbidden, "school_admin creates a topic")
	must(t, c.do(t, http.MethodPost, "/cms/learning/subjects", qlcmA, idem(), map[string]any{"code": "science",
		"name": map[string]string{"vi": "Khoa học"}, "grade_levels": []int{4}}), http.StatusForbidden, "tenant creates a subject")
	platformCur, platformVer := c.curriculum(t, cmNoidung, "PLATFORM_CUR")
	c.assign(ctx, t, platformCur, c.tenantA)
	must(t, c.do(t, http.MethodPost, "/cms/learning/topics", qlcmA, idem(), map[string]any{"curriculum_version_id": platformVer, "code": "X",
		"name": "x", "subject_id": math, "grade_level": 3}), http.StatusForbidden, "tenant writes an assigned platform curriculum")
	if after := c.audits(ctx, t, "authz.denied", nil); after < before+3 {
		t.Fatalf("authz.denied %d → %d", before, after)
	}
	// The assigned platform curriculum is readable by tenant A, not by tenant B.
	la := must(t, c.do(t, http.MethodGet, "/cms/learning/curricula", qlcmA, nil, nil), http.StatusOK, "list A")
	lb := must(t, c.do(t, http.MethodGet, "/cms/learning/curricula", qlcmB, nil, nil), http.StatusOK, "list B")
	if len(la.items("data")) != 2 || len(lb.items("data")) != 0 {
		t.Fatalf("A sees %d, B sees %d", len(la.items("data")), len(lb.items("data")))
	}
	// AC-08: tenant B gets 404 on everything of tenant A.
	for _, p := range []string{"/cms/learning/curricula/" + curA, "/cms/learning/curricula/" + curA + "/versions/1",
		"/cms/learning/curricula/" + curA + "/versions/1/lo-mappings", "/cms/learning/curricula/" + curA + "/versions/1:export"} {
		must(t, c.do(t, http.MethodGet, p, qlcmB, nil, nil), http.StatusNotFound, "B reads "+p)
	}
	tb := must(t, c.do(t, http.MethodGet, "/cms/learning/topics?curriculum_version_id="+verA, qlcmB, nil, nil), http.StatusOK, "B topics")
	if len(tb.items("data")) != 0 {
		t.Fatalf("B sees topics of A: %s", tb.raw)
	}
	must(t, c.do(t, http.MethodPost, "/cms/learning/objectives", qlcmB, idem(), map[string]any{"topic_id": topicA, "code": "MATH.G3.NUM.X",
		"description": "x"}), http.StatusNotFound, "B writes into A")
	must(t, c.do(t, http.MethodPatch, "/cms/learning/topics/"+topicA, qlcmB, ifm(`W/"1"`), map[string]any{"name": "y"}), http.StatusNotFound, "B patches A")
	must(t, c.do(t, http.MethodDelete, "/cms/learning/curricula/"+curA, qlcmB, ifm(`W/"1"`), nil), http.StatusNotFound, "B deletes A")
	// Platform staff without a session tenant sees only platform curricula.
	lp := must(t, c.do(t, http.MethodGet, "/cms/learning/curricula", cmNoidung, nil, nil), http.StatusOK, "platform list")
	if len(lp.items("data")) != 1 {
		t.Fatalf("platform sees %s", lp.raw)
	}
	// Student and staff without role: 403.
	must(t, c.do(t, http.MethodGet, "/cms/learning/curricula", noRoleA, nil, nil), http.StatusForbidden, "no role lists")
	must(t, c.do(t, http.MethodGet, "/cms/learning/curricula", hsAn, nil, nil), http.StatusForbidden, "student on the portal API")
	_ = authztest.TenantA
}

// F021-AC-07, AC-09, AC-10 (TC-CMS-002, TC-TEN-015): only drafts change, If-Match, soft-delete into the trash.
func TestCurriculumEditability(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	math := c.subject(t, "math")
	cur, ver := c.curriculum(t, cmNoidung, "EDIT_MATH")
	topic := c.topic(t, cmNoidung, ver, math, "MATH.G3.T01", 3, "")
	lo := must(t, c.objective(t, cmNoidung, topic, "MATH.G3.NUM.A", nil), http.StatusCreated, "LO").str("objective_id")

	// AC-09: PATCH without If-Match → 428; stale → 412.
	must(t, c.do(t, http.MethodPatch, "/cms/learning/objectives/"+lo, cmNoidung, nil, map[string]any{"description": "x"}),
		http.StatusPreconditionRequired, "no If-Match")
	must(t, c.do(t, http.MethodPatch, "/cms/learning/objectives/"+lo, cmNoidung, ifm(`W/"99"`), map[string]any{"description": "x"}),
		http.StatusPreconditionFailed, "stale If-Match")
	r := must(t, c.do(t, http.MethodPatch, "/cms/learning/objectives/"+lo, cmNoidung, ifm(objectiveETag(t, c, ver, lo)),
		map[string]any{"description": "Học sinh có thể nhân"}), http.StatusOK, "patch LO")
	if r.str("description") != "Học sinh có thể nhân" || r.header.Get("ETag") != `W/"2"` {
		t.Fatalf("patched %v %s", r.header.Get("ETag"), r.raw)
	}
	tag := c.versionETag(t, cmNoidung, cur, 1)
	must(t, c.do(t, http.MethodPatch, "/cms/learning/curricula/"+cur+"/versions/1", cmNoidung, ifm(tag),
		map[string]any{"effective_from": "2025-09-01", "change_note": "rà soát"}), http.StatusOK, "patch draft version")

	// AC-10: delete a topic of a draft (with its LO first) → soft-delete, audit, trash entry.
	t2 := c.topic(t, cmNoidung, ver, math, "MATH.G3.T02", 3, "")
	must(t, c.do(t, http.MethodDelete, "/cms/learning/topics/"+t2, cmNoidung, ifm(topicETag(t, c, ver, t2)), map[string]any{"reason": "trùng"}),
		http.StatusNoContent, "delete topic")
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM curriculum.topics WHERE topic_id = $1 AND deleted_at IS NOT NULL`, t2); n != 1 {
		t.Fatal("topic not soft-deleted")
	}
	must(t, c.do(t, http.MethodDelete, "/cms/learning/topics/"+topic, cmNoidung, ifm(topicETag(t, c, ver, topic)), nil),
		http.StatusUnprocessableEntity, "topic with an LO")
	trashEntries := listCurriculumTrash(ctx, t, c)
	if !slices.Contains(trashEntries, t2) {
		t.Fatalf("trash %v misses the topic", trashEntries)
	}

	// AC-07: once in review, nothing of the version changes (409 CONTENT_NOT_EDITABLE), in the use case and the trigger.
	must(t, c.transition(t, cmNoidung, cur, 1, "submit_review", ""), http.StatusOK, "submit")
	r = c.do(t, http.MethodPatch, "/cms/learning/objectives/"+lo, cmNoidung, ifm(objectiveETag(t, c, ver, lo)), map[string]any{"description": "y"})
	if must(t, r, http.StatusConflict, "edit in review"); r.str("code") != "CONTENT_NOT_EDITABLE" {
		t.Fatalf("code %s", r.raw)
	}
	must(t, c.do(t, http.MethodPost, "/cms/learning/topics", cmNoidung, idem(), map[string]any{"curriculum_version_id": ver, "code": "MATH.G3.T09",
		"name": "x", "subject_id": math, "grade_level": 3}), http.StatusConflict, "add topic in review")
	if _, err := c.platform.ExecContext(ctx, `UPDATE curriculum.learning_objectives SET description = 'z' WHERE objective_id = $1`, lo); err == nil ||
		!strings.Contains(err.Error(), "CONTENT_NOT_EDITABLE") {
		t.Fatalf("trigger: %v", err)
	}
	// AC-10: a curriculum with a published version can only be archived (409); a never-published one goes to the trash.
	must(t, c.transition(t, saAdmin, cur, 1, "approve", ""), http.StatusOK, "approve")
	must(t, c.do(t, http.MethodPost, "/cms/learning/curricula/"+cur+"/versions/1:publish", saAdmin,
		merge(idem(), ifm(c.versionETag(t, saAdmin, cur, 1))), nil), http.StatusOK, "publish")
	curTag := must(t, c.do(t, http.MethodGet, "/cms/learning/curricula/"+cur, cmNoidung, nil, nil), http.StatusOK, "get").header.Get("ETag")
	must(t, c.do(t, http.MethodDelete, "/cms/learning/curricula/"+cur, cmNoidung, ifm(curTag), nil), http.StatusConflict, "delete published")
	draftCur, _ := c.curriculum(t, cmNoidung, "DRAFT_ONLY")
	dTag := must(t, c.do(t, http.MethodGet, "/cms/learning/curricula/"+draftCur, cmNoidung, nil, nil), http.StatusOK, "get draft").header.Get("ETag")
	must(t, c.do(t, http.MethodDelete, "/cms/learning/curricula/"+draftCur, cmNoidung, ifm(dTag), map[string]any{"reason": "thử"}),
		http.StatusNoContent, "delete draft curriculum")
	must(t, c.do(t, http.MethodGet, "/cms/learning/curricula/"+draftCur, cmNoidung, nil, nil), http.StatusNotFound, "deleted curriculum")
	if c.audits(ctx, t, "content.deleted", nil) < 2 || !slices.Contains(listCurriculumTrash(ctx, t, c), draftCur) {
		t.Fatal("curriculum delete audit/trash")
	}
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'curriculum.deleted'`); n != 1 {
		t.Fatalf("%d curriculum.deleted events", n)
	}
}
