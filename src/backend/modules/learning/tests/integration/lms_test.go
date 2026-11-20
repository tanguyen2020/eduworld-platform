//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/lms"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// F040-AC-01…AC-05, AC-24…AC-29 (US1, US5; quickstart 1–4, 14–16; TC-TEN-001…004, TC-CMS-ASG-01): one course per subject
// of the grade on class.synced (twice → once), the curriculum version of the school setting or the latest published
// one, the settings of a school (published and assigned versions only, If-Match, audit, applied to later courses),
// courses in the scope of each role, show/hide with If-Match and audit.
func TestLMSCoursesAndSettings(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.lmsSetup(ctx, t)
	authztest.Cover(t, "listCourses", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateCourse", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getSchoolCurriculumSettings", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "putSchoolCurriculumSettings", authztest.CrossTenant, authztest.RoleDenied)
	courseID := k.courses[k.s.qc.subject]
	if len(k.courses) != 1 || courseID == "" {
		t.Fatalf("courses of the class: %v", k.courses)
	}
	// AC-01: a redelivered class.synced creates nothing more; AC-02: the latest published version without setting.
	if err := c.classSynced(ctx, t, ids.New(), k.class, k.school, 3, k.year, "active"); err != nil {
		t.Fatal(err)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM lms.courses WHERE class_id = $1 AND curriculum_version_id = $2
		AND academic_year_id = $3 AND status = 'active' AND deleted_at IS NULL`, k.class, k.s.qc.version, lms.AcademicYearID(c.tenantA, k.year)); n != 1 {
		t.Fatalf("courses after a redelivery: %d", n)
	}
	// AC-04: scope of the roles.
	list := func(sub string) []map[string]any {
		return must(t, c.do(t, http.MethodGet, "/lms/courses", sub, nil, nil), http.StatusOK, "list "+sub).items("data")
	}
	if cs := list(adA); len(cs) != 1 || cs[0]["course_id"] != courseID || cs[0]["class_name"] != "3A" || cs[0]["student_count"] != float64(2) ||
		cs[0]["curriculum_version_id"] != k.s.qc.version {
		t.Fatalf("school admin: %v", cs)
	}
	for _, sub := range []string{adA2, adB, qlcmOther} {
		if cs := list(sub); len(cs) != 0 {
			t.Fatalf("%s sees %v", sub, cs)
		}
	}
	if cs := list(taA); len(cs) != 1 {
		t.Fatalf("tenant admin: %v", cs)
	}
	if r := c.do(t, http.MethodGet, "/lms/courses?tenant_ids="+c.tenantB.String(), adA, nil, nil); r.status != 200 || len(r.items("data")) != 1 {
		t.Fatalf("tenant_ids of a tenant user are ignored: %d %s", r.status, r.raw)
	}
	// AC-05: hide with If-Match; 428 without, 412 stale; audit.
	path := "/lms/courses/" + courseID
	if r := c.do(t, http.MethodPatch, path, adA, nil, map[string]any{"status": "hidden"}); r.status != 428 {
		t.Fatalf("no If-Match: %d", r.status)
	}
	if r := c.do(t, http.MethodPatch, path, adA, ifm(`W/"9"`), map[string]any{"status": "hidden"}); r.status != 412 {
		t.Fatalf("stale: %d %s", r.status, r.raw)
	}
	h := must(t, c.do(t, http.MethodPatch, path, adA, ifm(`W/"1"`), map[string]any{"status": "hidden"}), http.StatusOK, "hide")
	if h.str("status") != "hidden" || h.header.Get("ETag") != `W/"2"` {
		t.Fatalf("hidden: %s", h.raw)
	}
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' = 'course.updated' AND envelope->'payload'->>'target_id' = $1`, courseID); n != 1 {
		t.Fatal("audit course.updated")
	}
	// AC-24…AC-27: another school 404, a subject outside the grant 403, tenant admin writes 403, students 403, tenant B 404,
	// System Admin without X-Tenant-Id 400.
	for _, cs := range []struct {
		sub    string
		status int
		code   string
	}{{adA2, 404, "NOT_FOUND"}, {qlcmOther, 403, "PERMISSION_DENIED"}, {taA, 403, "PERMISSION_DENIED"}, {adB, 404, "NOT_FOUND"},
		{hsAn, 403, "PORTAL_ACCESS_STAFF_ONLY"}} {
		if r := c.do(t, http.MethodPatch, path, cs.sub, ifm(`W/"2"`), map[string]any{"status": "active"}); r.status != cs.status || r.str("code") != cs.code {
			t.Fatalf("%s: %d %s", cs.sub, r.status, r.raw)
		}
	}
	if r := c.do(t, http.MethodGet, "/lms/courses", saAdmin, nil, nil); r.status != 400 || r.str("code") != "TENANT_HEADER_REQUIRED" {
		t.Fatalf("system admin without tenant: %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodGet, "/lms/courses", saAdmin, hdr{"X-Tenant-Id": c.tenantA.String()}, nil); r.status != 200 || len(r.items("data")) != 1 {
		t.Fatalf("system admin of tenant A: %d %s", r.status, r.raw)
	}
	// AC-03: settings of the school.
	settings := "/lms/schools/" + k.school.String() + "/curriculum-settings"
	g := must(t, c.do(t, http.MethodGet, settings, adA, nil, nil), http.StatusOK, "settings")
	if g.header.Get("ETag") != `W/"1"` || len(g.items("items")) != 0 {
		t.Fatalf("settings: %s", g.raw)
	}
	year := lms.AcademicYearID(c.tenantA, k.year)
	cur2, ver2 := c.curriculum(t, cmNoidung, "QB_MATH_2")
	put := func(version string, h hdr) resp {
		return c.do(t, http.MethodPut, settings, adA, h, map[string]any{"items": []any{map[string]any{"academic_year_id": year, "grade_level": 3,
			"curriculum_version_id": version}}})
	}
	if r := put(ver2, ifm(`W/"1"`)); r.status != 403 || r.str("code") != "RESOURCE_NOT_ASSIGNED" {
		t.Fatalf("not assigned: %d %s", r.status, r.raw)
	}
	c.assign(ctx, t, cur2, c.tenantA)
	if r := put(ver2, ifm(`W/"1"`)); r.status != 409 || r.str("code") != "CONTENT_NOT_PUBLISHED" {
		t.Fatalf("not published: %d %s", r.status, r.raw)
	}
	c.subjectGrade(ctx, t, cur2, ver2, k.s.qc.subject, 3)
	c.publishCurriculum(ctx, t, ver2)
	if r := put(ver2, nil); r.status != 428 {
		t.Fatalf("no If-Match: %d", r.status)
	}
	if r := c.do(t, http.MethodPut, settings, qlcmA, ifm(`W/"1"`), map[string]any{"items": []any{}}); r.status != 403 {
		t.Fatalf("academic manager writes settings: %d", r.status)
	}
	p := must(t, put(ver2, ifm(`W/"1"`)), http.StatusOK, "put settings")
	if items := p.items("items"); len(items) != 1 || items[0]["curriculum_version_id"] != ver2 || p.header.Get("ETag") != `W/"2"` {
		t.Fatalf("put: %s", p.raw)
	}
	if r := put(ver2, ifm(`W/"1"`)); r.status != 412 {
		t.Fatalf("stale settings: %d", r.status)
	}
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' = 'school_curriculum.set'`); n != 1 {
		t.Fatal("audit school_curriculum.set")
	}
	// Applied only to courses created after: the existing course keeps its version, a new class takes the setting.
	class2 := uuid.New()
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO ref.classes (class_id, tenant_id, school_id, grade_level, name, source_version)
		VALUES ($1, $2, $3, 3, '3B', 1)`, class2, c.tenantA, k.school); err != nil {
		t.Fatal(err)
	}
	if err := c.classSynced(ctx, t, ids.New(), class2, k.school, 3, k.year, "active"); err != nil {
		t.Fatal(err)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM lms.courses WHERE class_id = $1 AND curriculum_version_id = $2`, class2, ver2); n != 1 {
		t.Fatal("new course with the setting")
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM lms.courses WHERE course_id = $1 AND curriculum_version_id = $2`, courseID,
		k.s.qc.version); n != 1 {
		t.Fatal("the old course changed")
	}
	// T044 (OQ F021-5): the default version of the school follows the academic year of its classes when the projection
	// knows it; a class of another year makes the setting of this year not apply, no academic year keeps the old rule.
	defaultVersion := func() *uuid.UUID {
		var out *uuid.UUID
		sctx := authztest.Ctx(ctx, authzPrincipal(c, adA))
		if err := c.read.Read(tenancy.WithScope(sctx, scopeOf(t, c.tenantA)), func(ctx context.Context) error {
			g := 3
			var err error
			out, err = read.SchoolSettings{}.DefaultVersion(ctx, k.school, &g)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return out
	}
	if v := defaultVersion(); v == nil || v.String() != ver2 {
		t.Fatalf("default without academic year: %v", v)
	}
	if _, err := c.platform.ExecContext(ctx, `UPDATE ref.classes SET academic_year_id = $2 WHERE class_id = $1`, k.class, uuid.New()); err != nil {
		t.Fatal(err)
	}
	if v := defaultVersion(); v != nil {
		t.Fatalf("setting of another year applied: %v", v)
	}
	if _, err := c.platform.ExecContext(ctx, `UPDATE ref.classes SET academic_year_id = $2 WHERE school_id = $1`, k.school, year); err != nil {
		t.Fatal(err)
	}
	if v := defaultVersion(); v == nil || v.String() != ver2 {
		t.Fatalf("default of the year: %v", v)
	}
	// Edge case: a closed class archives its courses.
	if err := c.classSynced(ctx, t, ids.New(), class2, k.school, 3, k.year, "closed"); err != nil {
		t.Fatal(err)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM lms.courses WHERE class_id = $1 AND status = 'archived'`, class2); n != 1 {
		t.Fatal("closed class not archived")
	}
}

func pathBody(title, mode string, versions ...string) map[string]any {
	nodes := make([]any, len(versions))
	for i, v := range versions {
		nodes[i] = map[string]any{"position": i + 1, "content_ref": map[string]any{"content_type": "activity", "content_version_id": v}}
	}
	return map[string]any{"title": title, "unlock_mode": mode, "nodes": nodes}
}

// F040-AC-06…AC-12 (US2; quickstart 5–9): a mastery_gate path pinned to published activities (draft 409, platform not
// assigned 403), the template of the subject for a course without own path, node states for the student, manual unlock
// with a reason and audit, direct update with If-Match keeping node ids, hidden courses left out, R6.
func TestLMSLearningPaths(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.lmsSetup(ctx, t)
	authztest.Cover(t, "listLearningPaths", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createLearningPath", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateLearningPath", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "unlockPathNodeForStudent", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listMyLearningPaths", authztest.CrossTenant, authztest.RoleDenied)
	courseID := k.courses[k.s.qc.subject]
	a1 := c.activityVersion(ctx, t, k.s.activityID)
	qc2 := k.s.qc
	qc2.lo = k.s.qc.lo2
	act2 := c.createActivity(t, qlcmA, activityBody(qc2, "quiz", quizConfig(k.s.questions...)))
	c.publishActivity(ctx, t, act2, 1)
	a2 := c.activityVersion(ctx, t, act2)
	draft := c.createActivity(t, qlcmA, activityBody(k.s.qc, "quiz", quizConfig(k.s.questions...)))
	var draftVersion string
	if err := c.platform.QueryRowContext(ctx, `SELECT activity_version_id FROM content.activity_versions WHERE activity_id = $1`, draft).
		Scan(&draftVersion); err != nil {
		t.Fatal(err)
	}
	paths := "/lms/courses/" + courseID + "/paths"
	if r := c.do(t, http.MethodPost, paths, qlcmA, idem(), pathBody("Lộ trình", "mastery_gate", a1, draftVersion)); r.status != 409 ||
		r.str("code") != "CONTENT_NOT_PUBLISHED" {
		t.Fatalf("draft node: %d %s", r.status, r.raw)
	}
	// Platform content not assigned to the tenant: a platform activity of cm.noidung.
	platQ, _ := c.question(t, cmNoidung, k.s.qc, map[string]any{"owner_scope": "platform"})
	platQV := c.publishQuestion(ctx, t, platQ, 1)
	platBody := activityBody(k.s.qc, "quiz", quizConfig(platQV))
	platBody["owner_scope"] = "platform"
	plat := c.createActivity(t, cmNoidung, platBody)
	if _, err := c.platform.ExecContext(ctx, `UPDATE content.activity_versions SET status = 'published', published_at = now() WHERE activity_id = $1`,
		plat); err != nil {
		t.Fatal(err)
	}
	var platVersion string
	_ = c.platform.QueryRowContext(ctx, `SELECT activity_version_id FROM content.activity_versions WHERE activity_id = $1`, plat).Scan(&platVersion)
	if r := c.do(t, http.MethodPost, paths, qlcmA, idem(), pathBody("Lộ trình", "mastery_gate", a1, platVersion)); r.status != 403 ||
		r.str("code") != "RESOURCE_NOT_ASSIGNED" {
		t.Fatalf("platform node: %d %s", r.status, r.raw)
	}
	// AC-08: without an own path, the template of the subject and grade.
	tpl := ids.New()
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO lms.learning_paths (path_id, tenant_id, owner_scope, subject_id, grade_level, title,
		unlock_mode) VALUES ($1, NULL, 'platform', $2, 3, 'Lộ trình mẫu', 'free')`, tpl, k.s.qc.subject); err != nil {
		t.Fatal(err)
	}
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO lms.path_nodes (node_id, tenant_id, path_id, position, content_type, content_id,
		content_version_id) VALUES ($1, NULL, $2, 1, 'activity', $3, $4)`, ids.New(), tpl, k.s.activityID, a1); err != nil {
		t.Fatal(err)
	}
	if ps := must(t, c.do(t, http.MethodGet, paths, adA, nil, nil), 200, "template").items("data"); len(ps) != 1 || ps[0]["is_template"] != true {
		t.Fatalf("template: %v", ps)
	}
	if my := must(t, c.do(t, http.MethodGet, "/learning/my-learning-paths", hsAn, nil, nil), 200, "my template").items("items"); len(my) != 1 ||
		my[0]["is_template"] != true {
		t.Fatalf("my template: %v", my)
	}
	// AC-06: create (Idempotency-Key replay), nodes pinned, audit.
	key := idem()
	cr := must(t, c.do(t, http.MethodPost, paths, qlcmA, key, pathBody("Lộ trình Toán 3A", "mastery_gate", a1, a2)), http.StatusCreated, "create path")
	pathID := cr.str("path_id")
	nodes := cr.items("nodes")
	if len(nodes) != 2 || cr.str("unlock_mode") != "mastery_gate" || cr.header.Get("ETag") != `W/"1"` {
		t.Fatalf("path: %s", cr.raw)
	}
	if again := must(t, c.do(t, http.MethodPost, paths, qlcmA, key, pathBody("Lộ trình Toán 3A", "mastery_gate", a1, a2)), http.StatusCreated, "replay"); again.str("path_id") != pathID {
		t.Fatal("replay")
	}
	if r := c.do(t, http.MethodPost, paths, qlcmA, idem(), pathBody("Thứ hai", "free", a1)); r.status != 409 {
		t.Fatalf("second path: %d", r.status)
	}
	node2 := nodes[1]["node_id"].(string)
	// AC-09, AC-11: node 2 locked until MATH.G3.NUM.MUL is developing.
	state := func(sub string) []string {
		my := must(t, c.do(t, http.MethodGet, "/learning/my-learning-paths", sub, nil, nil), 200, "my paths").items("items")
		if len(my) != 1 || my[0]["path_id"] != pathID || my[0]["is_template"] != false {
			t.Fatalf("my paths of %s: %v", sub, my)
		}
		var out []string
		for _, n := range my[0]["nodes"].([]any) {
			out = append(out, n.(map[string]any)["state"].(string))
		}
		return out
	}
	if s := state(hsAn); s[0] != "unlocked" || s[1] != "locked" {
		t.Fatalf("an: %v", s)
	}
	// tenancyguard:ok TEST fixture under learning_platform: hs.binh has MATH.G3.NUM.MUL developing
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO learning.objective_masteries (objective_mastery_id, tenant_id, school_id, student_id,
		objective_id, value, level, evidence_count, distinct_days, last_evidence_at, last_evidence_date, model_version)
		VALUES ($1, $2, $3, $4, $5, 0.500, 'developing', 3, 1, now(), current_date, 'we-1.0')`, ids.New(), c.tenantA, k.school,
		c.users[hsBinh].UserID, k.s.qc.lo); err != nil {
		t.Fatal(err)
	}
	if s := state(hsBinh); s[1] != "unlocked" {
		t.Fatalf("binh after developing: %v", s)
	}
	// AC-10: manual unlock (reason, class, audit).
	unlock := func(student, reason string) resp {
		return c.do(t, http.MethodPost, paths+"/"+pathID+"/nodes/"+node2+"/students/"+student+":unlock", qlcmA, idem(), map[string]any{"reason": reason})
	}
	if r := unlock(c.users[hsAn].UserID.String(), "   "); r.status != 422 {
		t.Fatalf("no reason: %d", r.status)
	}
	if r := unlock(c.users[hsDung].UserID.String(), "Ôn lại"); r.status != 404 {
		t.Fatalf("student outside the class: %d", r.status)
	}
	if r := unlock(c.users[hsAn].UserID.String(), "Em đã ôn ở lớp"); r.status != 204 {
		t.Fatalf("unlock: %d %s", r.status, r.raw)
	}
	if s := state(hsAn); s[1] != "unlocked" {
		t.Fatalf("an after unlock: %v", s)
	}
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' = 'path_node.unlocked' AND envelope->'payload'->>'reason' = 'Em đã ôn ở lớp'`); n != 1 {
		t.Fatal("audit path_node.unlocked")
	}
	// AC-06: update directly with If-Match; nodes keep their ids by content; a stale version is 412.
	up := must(t, c.do(t, http.MethodPatch, paths+"/"+pathID, qlcmA, ifm(`W/"1"`), pathBody("Lộ trình mới", "sequential", a2, a1)), 200, "update")
	if un := up.items("nodes"); up.str("title") != "Lộ trình mới" || un[0]["node_id"] != node2 || up.header.Get("ETag") != `W/"2"` {
		t.Fatalf("update: %s", up.raw)
	}
	if r := c.do(t, http.MethodPatch, paths+"/"+pathID, qlcmA, ifm(`W/"1"`), pathBody("x", "free", a1)); r.status != 412 {
		t.Fatalf("stale path: %d", r.status)
	}
	if r := c.do(t, http.MethodPatch, paths+"/"+pathID, qlcmOther, ifm(`W/"2"`), pathBody("x", "free", a1)); r.status != 403 {
		t.Fatalf("other subject: %d", r.status)
	}
	// AC-12: a lesson of the schedule tomorrow whose objective has no evidence → R6 after a recompute.
	tomorrow := c.clk.Now().Add(24 * time.Hour).Format(time.DateOnly)
	must(t, c.do(t, http.MethodPost, "/lms/schedules", adA, idem(), map[string]any{"course_id": courseID, "date": tomorrow, "kind": "lesson",
		"ref_id": act2}), http.StatusCreated, "lesson item")
	if err := c.uow.Do(tenancy.WithScope(ctx, scopeOf(t, c.tenantA)), func(ctx context.Context) error {
		return c.recs.Recompute(ctx, c.users[hsAn].UserID, nil)
	}); err != nil {
		t.Fatal(err)
	}
	rec := must(t, c.do(t, http.MethodGet, "/learning/recommendations/me", hsAn, nil, nil), 200, "recommendations")
	found := false
	for _, it := range rec.items("items") {
		if it["rule_code"] == "R6" && it["target_id"] == act2 && it["priority"] == float64(4) && it["kind"] == "activity" {
			found = true
		}
	}
	if !found {
		// R6 has priority 4: with three higher recommendations it may not be shown; check the stored one.
		if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.rm_student_recommendations WHERE student_id = $1 AND rule_code = 'R6'
			AND target_id = $2 AND status = 'active'`, c.users[hsAn].UserID, act2); n != 1 {
			t.Fatalf("R6: %s", rec.raw)
		}
	}
	// AC-11: a hidden course is left out of the student's paths.
	must(t, c.do(t, http.MethodPatch, "/lms/courses/"+courseID, adA, ifm(`W/"1"`), map[string]any{"status": "hidden"}), 200, "hide")
	if my := must(t, c.do(t, http.MethodGet, "/learning/my-learning-paths", hsAn, nil, nil), 200, "hidden").items("items"); len(my) != 0 {
		t.Fatalf("hidden course: %v", my)
	}
	if r := c.do(t, http.MethodGet, "/learning/my-learning-paths", adA, nil, nil); r.status != 403 {
		t.Fatalf("staff on a student route: %d", r.status)
	}
	if r := c.do(t, http.MethodGet, "/learning/my-learning-paths", hsAn, hdr{"X-Tenant-Id": c.tenantB.String()}, nil); r.status != 403 ||
		r.str("code") != "TENANT_MISMATCH" {
		t.Fatalf("tenant mismatch: %d", r.status)
	}
}
