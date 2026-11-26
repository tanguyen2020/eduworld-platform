//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// F042-AC-18…AC-25 (US3, TC-EXM-003…TC-EXM-006, TC-LRN-007, TC-LRN-008): one submit per attempt (same key replays,
// another key 409); the worker grades once on the pinned versions; results follow result_release (manual: released by
// lms.result.release once, idempotent, audited); the result has the scale and sections, the analysis the objectives,
// wrong items with correct options and explanation, 1–3 objectives to review; R5 is recommended; the results live in
// PostgreSQL only; a 100 % answer faster than suspicious_speed_flag is flagged without changing the score.
func TestExamResultRelease(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.examSetup(ctx, t, 8, nil)
	authztest.Cover(t, "getExamResult", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getExamAttemptAnalysis", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "releaseExamResults", authztest.CrossTenant, authztest.RoleDenied)
	// The blueprint releases manually with feedback after the close (blueprintBody).
	session := c.createSession(t, k, map[string]any{"type": "chapter"})
	a := must(t, c.startExam(t, hsAn, session, devA, nil), http.StatusCreated, "start An")
	id := a.str("attempt_id")
	// AC-18: submit once; the same key replays; another key is 409.
	examOK := metricSum(t, observability.ExamSubmissions(), map[string]string{"exam_type": "chapter", "result": "ok"})
	examDup := metricSum(t, observability.ExamSubmissions(), map[string]string{"exam_type": "chapter", "result": "duplicate"})
	examDur := metricSum(t, observability.ExamSubmitDuration(), map[string]string{"exam_type": "chapter"})
	key := idem()
	sub := must(t, c.answerExam(t, hsAn, a, devA, false, key), http.StatusOK, "submit")
	if sub.str("status") != "submitted" || sub.str("score") != "" {
		t.Fatalf("submit: %s", sub.raw)
	}
	replay := must(t, c.do(t, http.MethodPost, "/exams/attempts/"+id+":submit", hsAn, merge(key, hdr{"X-Exam-Device-Token": devA}), nil),
		http.StatusOK, "replay")
	if replay.header.Get("Idempotent-Replayed") != "true" || replay.str("attempt_id") != id {
		t.Fatalf("replay: %s", replay.raw)
	}
	if x := c.do(t, http.MethodPost, "/exams/attempts/"+id+":submit", hsAn, merge(idem(), hdr{"X-Exam-Device-Token": devA}), nil); x.status !=
		http.StatusConflict || x.str("code") != "EXAM_ALREADY_SUBMITTED" {
		t.Fatalf("second key: %d %s", x.status, x.raw)
	}
	// MONITORING §3: one ok submit with its duration, one duplicate, both under the exam type; the replay counts nothing.
	if got := metricSum(t, observability.ExamSubmissions(), map[string]string{"exam_type": "chapter", "result": "ok"}) - examOK; got != 1 {
		t.Fatalf("exam_submissions_total{ok} delta %v", got)
	}
	if got := metricSum(t, observability.ExamSubmissions(), map[string]string{"exam_type": "chapter", "result": "duplicate"}) - examDup; got != 1 {
		t.Fatalf("exam_submissions_total{duplicate} delta %v", got)
	}
	if got := metricSum(t, observability.ExamSubmitDuration(), map[string]string{"exam_type": "chapter"}) - examDur; got != 1 {
		t.Fatalf("exam_submit_duration_seconds count delta %v", got)
	}
	// AC-19: graded once by the worker from the pinned versions; every item wrong → 0 of 4.
	c.gradeExams(ctx, t, id, 2)
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'exam.graded'
		AND envelope->'payload'->>'attempt_id' = $1`, id); n != 1 {
		t.Fatalf("exam.graded: %d", n)
	}
	if ev := c.outboxPayloads(ctx, t, "exam.graded", "attempt_id", id); len(ev) != 1 || ev[0]["score"] != float64(0) || ev[0]["max_score"] != float64(4) ||
		ev[0]["results_visible_at"] != nil || ev[0]["result_release"] != "manual" {
		t.Fatalf("exam.graded payload: %v", ev)
	}
	for _, typ := range []string{"exam.started", "exam.submitted"} {
		if len(c.outboxPayloads(ctx, t, typ, "attempt_id", id)) != 1 {
			t.Fatalf("%s payload", typ)
		}
	}
	// AC-20: before the release the student gets 403 EXAM_RESULTS_NOT_RELEASED, nothing of the answers.
	for _, path := range []string{"/result", "/analysis"} {
		if x := c.do(t, http.MethodGet, "/exams/attempts/"+id+path, hsAn, nil, nil); x.status != http.StatusForbidden ||
			x.str("code") != "EXAM_RESULTS_NOT_RELEASED" || strings.Contains(x.raw, "explanation") {
			t.Fatalf("%s before release: %d %s", path, x.status, x.raw)
		}
	}
	// Staff in scope read it; another school no.
	must(t, c.do(t, http.MethodGet, "/exams/attempts/"+id+"/result", qlcmA, nil, nil), http.StatusOK, "staff result")
	if x := c.do(t, http.MethodGet, "/exams/attempts/"+id+"/result", adA2, nil, nil); x.status != http.StatusNotFound {
		t.Fatalf("other school result: %d", x.status)
	}
	// AC-21: release only once the window closed; lms.result.release; the key replays without a second event.
	if x := c.do(t, http.MethodPost, examSessionsPath+"/"+session+":release-results", qlcmA, idem(), nil); x.status != http.StatusConflict {
		t.Fatalf("release before close: %d %s", x.status, x.raw)
	}
	c.clk.Advance(2 * time.Hour)
	c.runExamWindows(ctx, t)
	if x := c.do(t, http.MethodGet, "/exams/attempts/"+id+"/result", hsAn, nil, nil); x.status != http.StatusForbidden {
		t.Fatalf("manual after close: %d", x.status)
	}
	if x := c.do(t, http.MethodPost, examSessionsPath+"/"+session+":release-results", noRoleA, idem(), nil); x.status != http.StatusForbidden ||
		x.str("code") != "PERMISSION_DENIED" {
		t.Fatalf("no permission: %d %s", x.status, x.raw)
	}
	rk := idem()
	rel := must(t, c.do(t, http.MethodPost, examSessionsPath+"/"+session+":release-results", qlcmA, rk, nil), http.StatusOK, "release")
	must(t, c.do(t, http.MethodPost, examSessionsPath+"/"+session+":release-results", qlcmA, rk, nil), http.StatusOK, "release replay")
	if rel.str("status") != "results_released" || len(c.outboxPayloads(ctx, t, "exam.results_released", "exam_session_id", session)) != 1 ||
		c.audits(ctx, t, "exam_session.results_released", &c.tenantA) != 1 {
		t.Fatalf("release: %s", rel.raw)
	}
	// AC-22: the result (score_scale 10, sections) and the analysis (objectives, wrong items, 1–3 objectives to review).
	res := must(t, c.do(t, http.MethodGet, "/exams/attempts/"+id+"/result", hsAn, nil, nil), http.StatusOK, "result")
	secs := res.items("section_scores")
	if res.str("score") != "0" || res.str("score_scaled") != "0" || len(secs) != 1 || secs[0]["section_id"] != "sec_tn" || secs[0]["max_score"] != float64(4) ||
		len(res.items("items")) != 2 || strings.Contains(res.raw, "rank") {
		t.Fatalf("result: %s", res.raw)
	}
	an := must(t, c.do(t, http.MethodGet, "/exams/attempts/"+id+"/analysis", hsAn, nil, nil), http.StatusOK, "analysis")
	los, wrong, sugg := an.items("objective_breakdown"), an.items("wrong_items"), an.items("review_suggestions")
	right, _ := correctOption()
	if len(los) != 1 || los[0]["objective_id"] != k.s.qc.lo || los[0]["correct"] != float64(0) || los[0]["total"] != float64(2) || len(wrong) != 2 ||
		wrong[0]["correct_option_ids"] == nil || !strings.Contains(an.raw, right) || len(sugg) != 1 || sugg[0]["objective_id"] != k.s.qc.lo {
		t.Fatalf("analysis: %s", an.raw)
	}
	// AC-23: R5 — the objective below 60 % with mastery below proficient is recommended (priority 2, exam_followup).
	if err := c.uow.Do(tenancy.WithScope(ctx, scopeOf(t, c.tenantA)), func(ctx context.Context) error {
		return c.recs.Recompute(ctx, c.users[hsAn].UserID, nil)
	}); err != nil {
		t.Fatal(err)
	}
	recs := must(t, c.do(t, http.MethodGet, "/learning/recommendations/me", hsAn, nil, nil), http.StatusOK, "recommendations")
	found := false
	for _, it := range recs.items("items") {
		if it["rule_code"] == "R5" && it["target_id"] == k.s.qc.lo && it["reason"] == "exam_followup" && it["priority"] == float64(2) {
			found = true
		}
	}
	if !found {
		t.Fatalf("R5: %s", recs.raw)
	}
	// AC-24: the result is PostgreSQL only (attempt, items, evaluation; no cache in the read path).
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.evaluations WHERE attempt_id = $1 AND effective`, id); n != 1 {
		t.Fatal("evaluation")
	}
	// AC-20 after_close: visible once the window closed; AC-25: a 100 % attempt of 10 items in 2 s is flagged, the score unchanged.
	ten := c.examSetupOn(ctx, t, k, 10, []any{k.loRule(10, 1)}, nil)
	fast := c.createSession(t, ten, map[string]any{"type": "chapter", "result_release": "after_close", "window_end": c.clk.Now().Add(time.Hour).
		Format(time.RFC3339), "window_start": c.clk.Now().Add(-time.Minute).Format(time.RFC3339)})
	f := must(t, c.startExam(t, hsBinh, fast, devB, nil), http.StatusCreated, "start Bình")
	for pos := range f.items("items") {
		must(t, c.saveExam(t, hsBinh, f.str("attempt_id"), pos+1, devB, map[string]any{"response": map[string]any{"option_id": right}}),
			http.StatusNoContent, "fast answer")
	}
	c.clk.Advance(2 * time.Second)
	must(t, c.do(t, http.MethodPost, "/exams/attempts/"+f.str("attempt_id")+":submit", hsBinh, merge(idem(), hdr{"X-Exam-Device-Token": devB}), nil),
		http.StatusOK, "fast submit")
	c.gradeExams(ctx, t, f.str("attempt_id"), 1)
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.integrity_signals WHERE attempt_id = $1 AND signal_type = 'suspicious_speed'`,
		f.str("attempt_id")); n != 1 {
		t.Fatalf("suspicious_speed: %d", n)
	}
	if x := c.do(t, http.MethodGet, "/exams/attempts/"+f.str("attempt_id")+"/result", hsBinh, nil, nil); x.status != http.StatusForbidden {
		t.Fatalf("after_close before close: %d", x.status)
	}
	c.clk.Advance(2 * time.Hour)
	c.runExamWindows(ctx, t)
	if x := must(t, c.do(t, http.MethodGet, "/exams/attempts/"+f.str("attempt_id")+"/result", hsBinh, nil, nil), http.StatusOK, "after close"); x.str("score") != "10" {
		t.Fatalf("fast score: %s", x.raw)
	}
}
