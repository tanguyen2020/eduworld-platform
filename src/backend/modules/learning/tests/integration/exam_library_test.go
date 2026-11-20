//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// F042-AC-26, AC-27 (US4, TC-EXM-005): the library lists the open unofficial practice sessions of the student's
// audience only; a per_student paper is drawn at each start from the session, the student and the attempt number,
// another attempt is allowed by the type, the results are shown right after the grading.
func TestExamLibraryPerStudent(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.examSetup(ctx, t, 5, map[string]any{"strategy": "per_student"})
	authztest.Cover(t, "listExamLibrary", authztest.CrossTenant, authztest.RoleDenied)
	lib := c.createSession(t, k, map[string]any{"type": "practice_test", "audience": "library", "max_attempts": 2})
	official := c.createSession(t, k, map[string]any{"type": "chapter"})
	_ = official
	l := must(t, c.do(t, http.MethodGet, "/exams/library", hsAn, nil, nil), http.StatusOK, "library").items("data")
	if len(l) != 1 || l[0]["exam_session_id"] != lib || l[0]["result_release"] != "immediate" || l[0]["feedback_timing"] != "after_submit" ||
		l[0]["max_attempts"] != float64(2) {
		t.Fatalf("library: %v", l)
	}
	if l := must(t, c.do(t, http.MethodGet, "/exams/library?type=mock", hsAn, nil, nil), http.StatusOK, "mock").items("data"); len(l) != 0 {
		t.Fatalf("mock filter: %v", l)
	}
	if l := must(t, c.do(t, http.MethodGet, "/exams/library?grade_level=7", hsAn, nil, nil), http.StatusOK, "grade 7").items("data"); len(l) != 0 {
		t.Fatalf("grade filter: %v", l)
	}
	if l := must(t, c.do(t, http.MethodGet, "/exams/library", hsDung, nil, nil), http.StatusOK, "tenant B").items("data"); len(l) != 0 {
		t.Fatalf("tenant B library: %v", l)
	}
	if x := c.do(t, http.MethodGet, "/exams/library", adA, nil, nil); x.status != http.StatusForbidden {
		t.Fatalf("staff library: %d", x.status)
	}
	// AC-27: two attempts, each on its own paper, results right after the grading.
	var papers []string
	for i := range 2 {
		a := must(t, c.startExam(t, hsAn, lib, devA, nil), http.StatusCreated, "start")
		must(t, c.answerExam(t, hsAn, a, devA, true, idem()), http.StatusOK, "submit")
		c.gradeExams(ctx, t, a.str("attempt_id"), 1)
		r := must(t, c.do(t, http.MethodGet, "/exams/attempts/"+a.str("attempt_id")+"/result", hsAn, nil, nil), http.StatusOK, "immediate result")
		if r.str("score") != "4" || r.str("score_scaled") != "10" {
			t.Fatalf("result %d: %s", i, r.raw)
		}
		var p string
		if err := c.platform.QueryRowContext(ctx, `SELECT p.exam_paper_id::text FROM assessment.attempts a JOIN assessment.exam_papers p
			ON p.exam_paper_id = a.exam_paper_id WHERE a.attempt_id = $1 AND p.student_id = $2 AND p.attempt_no = $3`,
			a.str("attempt_id"), c.users[hsAn].UserID, i+1).Scan(&p); err != nil {
			t.Fatalf("per_student paper %d: %v", i, err)
		}
		papers = append(papers, p)
	}
	if papers[0] == papers[1] || count(ctx, t, c.platform, `SELECT count(DISTINCT seed) FROM assessment.exam_papers WHERE exam_session_id = $1`, lib) != 2 {
		t.Fatalf("papers: %v", papers)
	}
}

// F042-AC-31 (T051): with the adapter of F042, publishing a graded assignment on a blueprint creates its exam session in
// the same transaction (pinned version, the students and window of the assignment, the release of the assignment) and
// stores exam_session_id; cancelling the assignment cancels the session; a session that cannot be generated rolls the
// publish back.
func TestExamSessionOfAssignment(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.examSetup(ctx, t, 1, nil)
	body := c.asgBody(k.asgKit, map[string]any{"mode": "graded", "content_ref": map[string]any{"content_type": "assessment_blueprint",
		"content_id": k.blueprint}})
	r := must(t, c.createAsg(t, adA, body), http.StatusCreated, "graded blueprint assignment")
	p := must(t, c.publishAsg(t, adA, r), http.StatusOK, "publish")
	session := p.str("exam_session_id")
	if session == "" {
		t.Fatalf("no exam session: %s", p.raw)
	}
	asg := p.str("assignment_id")
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.exam_sessions s JOIN lms.assignments a ON a.assignment_id = s.assignment_id
		WHERE s.exam_session_id = $1 AND a.assignment_id = $2 AND s.blueprint_version_id = $3 AND s.window_start = a.open_at
		AND s.window_end = a.due_at AND s.result_release = a.result_release AND s.status = 'open'`, session, asg, k.version); n != 1 {
		t.Fatal("session of the assignment")
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.exam_participations WHERE exam_session_id = $1`, session); n != 2 {
		t.Fatalf("participations: %d", n)
	}
	must(t, c.startExam(t, hsAn, session, devA, nil), http.StatusCreated, "student starts the exam of the assignment")
	// Cancel → the session is cancelled (the attempt rule of F041 applies to activity attempts).
	other := must(t, c.publishAsg(t, adA, must(t, c.createAsg(t, adA, body), http.StatusCreated, "second")), http.StatusOK, "publish 2")
	must(t, c.do(t, http.MethodPost, assignmentsPath+"/"+other.str("assignment_id")+":cancel", adA, idem(), map[string]any{"reason": "Giao nhầm lớp"}),
		http.StatusOK, "cancel")
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.exam_sessions WHERE exam_session_id = $1 AND status = 'cancelled'`,
		other.str("exam_session_id")); n != 1 || c.audits(ctx, t, "exam_session.cancelled", &c.tenantA) != 1 {
		t.Fatal("cancelled session")
	}
	// A blueprint the pool cannot fill: the publish fails and nothing is kept.
	big := c.examSetupOn(ctx, t, k, 40, []any{k.loRule(20, 2)}, nil)
	draft := must(t, c.createAsg(t, adA, c.asgBody(k.asgKit, map[string]any{"mode": "graded", "content_ref": map[string]any{
		"content_type": "assessment_blueprint", "content_id": big.blueprint}})), http.StatusCreated, "big blueprint assignment")
	before := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.exam_sessions`)
	if x := c.publishAsg(t, adA, draft); x.status != http.StatusUnprocessableEntity {
		t.Fatalf("publish with a short pool: %d %s", x.status, x.raw)
	}
	if count(ctx, t, c.platform, `SELECT count(*) FROM assessment.exam_sessions`) != before ||
		count(ctx, t, c.platform, `SELECT count(*) FROM lms.assignments WHERE assignment_id = $1 AND status = 'draft' AND exam_session_id IS NULL`,
			draft.str("assignment_id")) != 1 {
		t.Fatal("publish not rolled back")
	}
}
