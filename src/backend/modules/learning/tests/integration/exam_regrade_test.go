//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
)

// F042-AC-28…AC-30 (US5, TC-LRN-015): a RegradeRequest of scope exam_session re-evaluates every graded attempt of the
// session holding the question (drop_item: the scale without the question; accept_all: full points), keeps the old
// evaluation as history, emits exam.regraded and attempt.re_evaluated per attempt; school_admin is 403, a second
// pending request 409, the key replays once, audit content.re_evaluated; the published question version is untouched.
func TestExamRegrade(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.examSetup(ctx, t, 1, nil)
	session := c.createSession(t, k, map[string]any{"type": "chapter", "result_release": "immediate", "feedback_timing": "after_submit"})
	an := must(t, c.startExam(t, hsAn, session, devA, nil), http.StatusCreated, "An")
	must(t, c.answerExam(t, hsAn, an, devA, true, idem()), http.StatusOK, "An submits")
	binh := must(t, c.startExam(t, hsBinh, session, devB, nil), http.StatusCreated, "Bình")
	must(t, c.answerExam(t, hsBinh, binh, devB, false, idem()), http.StatusOK, "Bình submits")
	c.gradeExams(ctx, t, an.str("attempt_id"), 1)
	c.gradeExams(ctx, t, binh.str("attempt_id"), 1)
	q := an.items("items")[0]["question_version_id"].(string)
	var body string
	// tenancyguard:ok TEST snapshot of the published question version under learning_platform
	if err := c.platform.QueryRowContext(ctx, `SELECT (answer_key || render_payload)::text FROM content.question_versions WHERE question_version_id = $1`, q).Scan(&body); err != nil {
		t.Fatal(err)
	}
	req := map[string]any{"scope": "exam_session", "exam_session_id": session, "question_version_id": q, "action": "drop_item",
		"reason": "Đáp án câu 1 bị sai trong đề"}
	// AC-29: school_admin has no learning_content.re_evaluate.
	if x := c.do(t, http.MethodPost, regradePath, adA, idem(), req); x.status != http.StatusForbidden {
		t.Fatalf("school_admin: %d %s", x.status, x.raw)
	}
	key := idem()
	r := must(t, c.do(t, http.MethodPost, regradePath, qlcmA, key, req), http.StatusAccepted, "regrade")
	if again := must(t, c.do(t, http.MethodPost, regradePath, qlcmA, key, req), http.StatusAccepted, "replay"); again.str("regrade_request_id") !=
		r.str("regrade_request_id") {
		t.Fatalf("replay: %s", again.raw)
	}
	if x := c.do(t, http.MethodPost, regradePath, qlcmA, idem(), req); x.status != http.StatusConflict {
		t.Fatalf("pending twice: %d %s", x.status, x.raw)
	}
	if x := c.do(t, http.MethodPost, regradePath, qlcmA, idem(), map[string]any{"scope": "exam_session", "exam_session_id": session,
		"question_version_id": k.s.questions[0], "action": "use_new_key", "corrected_version_id": q, "reason": "Đáp án mới của câu hỏi"}); x.status !=
		http.StatusUnprocessableEntity {
		t.Fatalf("use_new_key with another question: %d %s", x.status, x.raw)
	}
	if c.audits(ctx, t, "content.re_evaluated", &c.tenantA) != 1 {
		t.Fatal("request audit")
	}
	// AC-28: the worker re-evaluates both attempts once.
	if n, err := c.rgd.RunOnce(ctx); err != nil || n != 2 {
		t.Fatalf("run: %d %v", n, err)
	}
	if n, _ := c.rgd.RunOnce(ctx); n != 0 {
		t.Fatal("ran twice")
	}
	res := must(t, c.do(t, http.MethodGet, "/exams/attempts/"+an.str("attempt_id")+"/result", hsAn, nil, nil), http.StatusOK, "An result")
	if res.str("score") != "2" || res.str("max_score") != "2" || res.str("score_scaled") != "10" {
		t.Fatalf("An after drop_item: %s", res.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.evaluations WHERE attempt_id = $1 AND NOT effective AND score = 4`,
		an.str("attempt_id")); n != 1 {
		t.Fatal("the old evaluation is kept")
	}
	if ev := c.outboxPayloads(ctx, t, "exam.regraded", "exam_session_id", session); len(ev) != 1 || len(ev[0]["affected_attempt_ids"].([]any)) != 2 {
		t.Fatalf("exam.regraded: %v", ev)
	}
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'attempt.re_evaluated'
		AND envelope->'payload'->>'regrade_request_id' = $1`, r.str("regrade_request_id")); n != 2 {
		t.Fatalf("attempt.re_evaluated: %d", n)
	}
	// AC-30: the published version is unchanged.
	var after string
	// tenancyguard:ok TEST check under learning_platform
	if err := c.platform.QueryRowContext(ctx, `SELECT (answer_key || render_payload)::text FROM content.question_versions WHERE question_version_id = $1
		AND status = 'published'`, q).Scan(&after); err != nil || after != body {
		t.Fatalf("question version changed: %v", err)
	}
	// accept_all on the other question: Bình gets its points.
	q2 := an.items("items")[1]["question_version_id"].(string)
	must(t, c.do(t, http.MethodPost, regradePath, qlcmA, idem(), map[string]any{"scope": "exam_session", "exam_session_id": session,
		"question_version_id": q2, "action": "accept_all", "reason": "Câu 2 có hai đáp án đúng"}), http.StatusAccepted, "accept_all")
	if n, err := c.rgd.RunOnce(ctx); err != nil || n != 2 {
		t.Fatalf("accept_all run: %d %v", n, err)
	}
	if x := must(t, c.do(t, http.MethodGet, "/exams/attempts/"+binh.str("attempt_id")+"/result", hsBinh, nil, nil), http.StatusOK, "Bình"); x.str("score") != "2" {
		t.Fatalf("Bình after accept_all: %s", x.raw)
	}
}
