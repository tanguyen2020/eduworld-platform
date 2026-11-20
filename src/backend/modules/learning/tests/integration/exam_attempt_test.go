//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/exam"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

const (
	devA = "device-aaaa-0001"
	devB = "device-bbbb-0002"
)

// F042-AC-09…AC-17 (US2, TC-EXM-001, TC-EXM-002, TC-LRN-007, TC-LRN-009, TC-TEN-001, TC-TEN-011): the student of the
// audience starts in the window with the variant hash(student, session) mod N and a server deadline (extra time
// included); render payloads without answers; resume keeps the deadline and the saved answers; one device unless taken
// over (audit); answers after the deadline + grace refused; the jobs auto-submit at the deadline and at the close (absent).
func TestExamAttemptFlow(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.examSetup(ctx, t, 5, map[string]any{"strategy": "independent_draw", "count": 3})
	authztest.Cover(t, "startExamAttempt", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "saveExamAnswer", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "submitExamAttempt", authztest.CrossTenant, authztest.RoleDenied)
	session := c.createSession(t, k, map[string]any{"accommodations": []any{map[string]any{"student_id": c.users[hsBinh].UserID,
		"extra_time_percent": 20}}})
	sid := uuid.MustParse(session)
	start := c.clk.Now()
	a := must(t, c.startExam(t, hsAn, session, devA, nil), http.StatusCreated, "start")
	id := a.str("attempt_id")
	// AC-09: mode exam, deadline = start + 900 s, server_time, the variant of the student.
	deadline, _ := time.Parse(time.RFC3339Nano, a.str("deadline_at"))
	if a.str("mode") != "exam" || a.str("context_type") != "exam_session" || a.str("server_time") == "" ||
		deadline.Sub(start).Round(time.Second) != 900*time.Second || len(a.items("items")) != 2 {
		t.Fatalf("start: %s", a.raw)
	}
	var variant int
	if err := c.platform.QueryRowContext(ctx, `SELECT p.variant_no FROM assessment.attempts a JOIN assessment.exam_papers p
		ON p.exam_paper_id = a.exam_paper_id WHERE a.attempt_id = $1`, id).Scan(&variant); err != nil ||
		variant != exam.Variant(c.users[hsAn].UserID, sid, 3) {
		t.Fatalf("variant %d: %v", variant, err)
	}
	// AC-10: no answer key, scoring, explanation, option feedback or hints in the payload.
	for _, leak := range []string{"answer_key", "scoring", "explanation", "feedback\"", "\"hints\"", "correct_option"} {
		if strings.Contains(a.raw, leak) {
			t.Fatalf("payload leaks %s: %s", leak, a.raw)
		}
	}
	// AC-05: Bình has 20 % more time.
	b := must(t, c.startExam(t, hsBinh, session, devB, nil), http.StatusCreated, "start Bình")
	if d, _ := time.Parse(time.RFC3339Nano, b.str("deadline_at")); d.Sub(start).Round(time.Second) != 1080*time.Second {
		t.Fatalf("Bình deadline: %s", b.str("deadline_at"))
	}
	// AC-12: saving returns nothing about correctness; hints are refused in an exam attempt.
	right, _ := correctOption()
	c.clk.Advance(30 * time.Second)
	if x := c.saveExam(t, hsAn, id, 1, devA, map[string]any{"response": map[string]any{"option_id": right}}); x.status != http.StatusNoContent ||
		x.raw != "" {
		t.Fatalf("save: %d %s", x.status, x.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempt_answers WHERE attempt_id = $1 AND is_correct IS NULL
		AND score IS NULL`, id); n != 1 {
		t.Fatalf("ungraded answers: %d", n)
	}
	if x := c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+"/items/1/hints:next", hsAn, idem(), nil); x.status < 400 {
		t.Fatalf("hint in an exam: %d", x.status)
	}
	if x := c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+":submit", hsAn, idem(), nil); x.status != http.StatusConflict {
		t.Fatalf("activity submit of an exam attempt: %d", x.status)
	}
	// AC-13: start again → the same attempt, same deadline, the saved answer.
	c.clk.Advance(time.Minute)
	again := must(t, c.startExam(t, hsAn, session, devA, nil), http.StatusCreated, "resume")
	if again.str("attempt_id") != id || !sameTime(again.str("deadline_at"), deadline) || len(again.items("saved_responses")) != 1 {
		t.Fatalf("resume: %s", again.raw)
	}
	// AC-14: another device → 409; take_over moves the attempt (audit); the old device is refused.
	if x := c.startExam(t, hsAn, session, devB, nil); x.status != http.StatusConflict || x.str("code") != "EXAM_ACTIVE_ON_OTHER_DEVICE" {
		t.Fatalf("other device: %d %s", x.status, x.raw)
	}
	if x := must(t, c.startExam(t, hsAn, session, devB, map[string]any{"take_over": true}), http.StatusCreated, "take over"); x.str("attempt_id") != id {
		t.Fatalf("take over: %s", x.raw)
	}
	if c.audits(ctx, t, "exam_attempt.device_switched", &c.tenantA) != 1 {
		t.Fatal("device switch audit")
	}
	if x := c.saveExam(t, hsAn, id, 2, devA, map[string]any{"response": map[string]any{"option_id": right}}); x.status != http.StatusConflict ||
		x.str("code") != "EXAM_ACTIVE_ON_OTHER_DEVICE" {
		t.Fatalf("old device: %d %s", x.status, x.raw)
	}
	// AC-17: another student's attempt 404, staff on a player route 403, another tenant 404.
	if x := c.saveExam(t, hsBinh, id, 2, devB, map[string]any{"response": map[string]any{"option_id": right}}); x.status != http.StatusNotFound {
		t.Fatalf("other student: %d", x.status)
	}
	if x := c.startExam(t, adA, session, "", nil); x.status != http.StatusForbidden {
		t.Fatalf("staff start: %d %s", x.status, x.raw)
	}
	if x := c.startExam(t, hsDung, session, "", nil); x.status != http.StatusNotFound {
		t.Fatalf("tenant B start: %d %s", x.status, x.raw)
	}
	if x := c.do(t, http.MethodPost, examSessionsPath+"/"+session+"/attempts", hsAn, merge(idem(), hdr{"X-Tenant-Id": c.tenantB.String()}),
		nil); x.status != http.StatusForbidden || x.str("code") != "TENANT_MISMATCH" {
		t.Fatalf("tenant header: %d %s", x.status, x.raw)
	}
	// AC-15: after deadline + 30 s grace → EXAM_TIME_OVER; within the grace an answer made before the deadline is taken.
	c.clk.Advance(deadline.Add(10 * time.Second).Sub(c.clk.Now()))
	before := deadline.Add(-time.Second).Format(time.RFC3339Nano)
	must(t, c.saveExam(t, hsAn, id, 2, devB, map[string]any{"response": map[string]any{"option_id": right}, "answered_at_client": before}),
		http.StatusNoContent, "offline answer in the grace")
	if x := c.saveExam(t, hsAn, id, 2, devB, map[string]any{"response": map[string]any{"option_id": right}}); x.status !=
		http.StatusUnprocessableEntity || x.str("code") != "EXAM_TIME_OVER" {
		t.Fatalf("answer after the deadline: %d %s", x.status, x.raw)
	}
	c.clk.Advance(deadline.Add(31 * time.Second).Sub(c.clk.Now()))
	if x := c.saveExam(t, hsAn, id, 2, devB, map[string]any{"response": map[string]any{"option_id": right}, "answered_at_client": before}); x.status !=
		http.StatusUnprocessableEntity || x.str("code") != "EXAM_TIME_OVER" {
		t.Fatalf("answer after the grace: %d %s", x.status, x.raw)
	}
	// AC-16: the job auto-submits An (auto_timeout) once, not Bình (extra time); then grades it.
	for range 2 {
		if _, err := c.exams.AutoSubmitExpired(ctx, 10); err != nil {
			t.Fatal(err)
		}
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempts WHERE attempt_id = $1 AND status = 'submitted'
		AND submit_reason = 'auto_timeout'`, id); n != 1 {
		t.Fatalf("auto-submit: %d", n)
	}
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'exam.submitted'
		AND envelope->'payload'->>'attempt_id' = $1`, id); n != 1 {
		t.Fatalf("exam.submitted: %d", n)
	}
	c.gradeExams(ctx, t, id, 2)
	// AC-11: the only attempt is used while the window is still open.
	if x := c.startExam(t, hsAn, session, devB, nil); x.status != http.StatusConflict || x.str("code") != "EXAM_MAX_ATTEMPTS" {
		t.Fatalf("max attempts: %d %s", x.status, x.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempts WHERE attempt_id = $1 AND status = 'graded'
		AND score = 4`, id); n != 1 {
		t.Fatalf("graded after timeout: %d", n)
	}
	// AC-16: at the close Bình's open attempt is submitted (window_closed); a student who never started is absent.
	only := c.createSession(t, k, map[string]any{"targets": []any{map[string]any{"target_type": "students",
		"student_ids": []string{c.users[hsAn].UserID.String(), c.users[hsBinh].UserID.String()}}}, "window_end": c.clk.Now().Add(20 * time.Minute).Format(time.RFC3339)})
	opened := must(t, c.startExam(t, hsBinh, only, devB, nil), http.StatusCreated, "Bình in the second session")
	c.clk.Advance(time.Hour)
	c.runExamWindows(ctx, t)
	c.runExamWindows(ctx, t)
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempts WHERE attempt_id = $1 AND status = 'submitted'
		AND submit_reason = 'window_closed'`, opened.str("attempt_id")); n != 1 {
		t.Fatalf("closed attempt: %d", n)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.exam_participations WHERE exam_session_id = $1 AND student_id = $2
		AND status = 'absent'`, only, c.users[hsAn].UserID); n != 1 {
		t.Fatal("absent")
	}
	if ev := c.outboxPayloads(ctx, t, "exam.closed", "exam_session_id", only); len(ev) != 1 || ev[0]["absent_count"] != float64(1) {
		t.Fatalf("exam.closed: %v", ev)
	}
	// AC-11: outside the window, not assigned, no attempt left.
	if x := c.startExam(t, hsAn, only, devA, nil); x.status != http.StatusForbidden || x.str("code") != "EXAM_WINDOW_CLOSED" {
		t.Fatalf("closed window: %d %s", x.status, x.raw)
	}
	later := c.createSession(t, k, map[string]any{"window_start": c.clk.Now().Add(time.Hour).Format(time.RFC3339),
		"window_end": c.clk.Now().Add(2 * time.Hour).Format(time.RFC3339), "targets": []any{map[string]any{"target_type": "students",
			"student_ids": []string{c.users[hsAn].UserID.String()}}}})
	if x := c.startExam(t, hsAn, later, devA, nil); x.status != http.StatusForbidden || x.str("code") != "EXAM_NOT_OPEN" {
		t.Fatalf("not open: %d %s", x.status, x.raw)
	}
	if x := c.startExam(t, hsBinh, later, devB, nil); x.status != http.StatusForbidden || x.str("code") != "EXAM_NOT_ASSIGNED" {
		t.Fatalf("not assigned: %d %s", x.status, x.raw)
	}
}

// sameTime compares an RFC 3339 time with t to the microsecond (the precision of PostgreSQL).
func sameTime(v string, t time.Time) bool {
	p, err := time.Parse(time.RFC3339Nano, v)
	return err == nil && p.Truncate(time.Microsecond).Equal(t.Truncate(time.Microsecond))
}
