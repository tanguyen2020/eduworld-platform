//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F030-AC-01…AC-07 (US1, quickstart 1–5, 15): start pins the version and the items, no answer data reaches the
// client, an open practice attempt is resumed, Idempotency-Key and attempt_start_rate, game_player and owner checks,
// tenant isolation, history of the student only.
func TestAttemptStart(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil)
	authztest.Cover(t, "startActivityAttempt", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getLearningActivity", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listMyActivityAttempts", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getActivityAttempt", authztest.CrossTenant, authztest.RoleDenied)
	key := idem()
	r := must(t, c.start(t, hsAn, s.activityID, key, nil), http.StatusCreated, "start")
	id := r.str("attempt_id")
	if r.str("status") != "in_progress" || r.str("mode") != "practice" || r.str("feedback_timing") != "immediate" ||
		r.str("display_language") != "vi-VN" || r.str("server_time") == "" || r.str("render", "activity_type") != "quiz" ||
		r.str("activity_version_id") == "" || r.header.Get("Content-Language") != "vi" {
		t.Fatalf("start: %s", r.raw)
	}
	items := r.items("items")
	if len(items) != 3 {
		t.Fatalf("items: %s", r.raw)
	}
	for i, it := range items {
		if it["question_version_id"] != s.questions[i] || it["type"] != "single_choice" || it["position"] != float64(i+1) {
			t.Fatalf("item %d: %v", i, it)
		}
	}
	if leaked := answerKeysIn(r.raw); len(leaked) > 0 {
		t.Fatalf("answer data in the attempt: %v", leaked)
	}
	// AC-05: same key replays; AC-04: another key resumes the open practice attempt.
	again := must(t, c.start(t, hsAn, s.activityID, key, nil), http.StatusCreated, "replay")
	if again.str("attempt_id") != id || again.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %s", again.raw)
	}
	if other := must(t, c.start(t, hsAn, s.activityID, nil, nil), http.StatusCreated, "resume"); other.str("attempt_id") != id {
		t.Fatalf("open practice not resumed: %s", other.raw)
	}
	noKey := c.do(t, http.MethodPost, "/learning/activities/"+s.activityID+"/attempts", hsAn, nil, map[string]any{})
	if noKey.status != http.StatusPreconditionRequired || noKey.str("code") != "IDEMPOTENCY_KEY_REQUIRED" {
		t.Fatalf("no key: %d %s", noKey.status, noKey.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempts WHERE student_id = $1`, c.users[hsAn].UserID); n != 1 {
		t.Fatalf("%d attempts", n)
	}
	// AC-03: another tenant, an unpublished activity.
	if r := c.start(t, hsDung, s.activityID, nil, nil); r.status != http.StatusNotFound {
		t.Fatalf("tenant B: %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodGet, "/learning/activities/"+s.activityID, hsDung, nil, nil); r.status != http.StatusNotFound {
		t.Fatalf("tenant B activity: %d", r.status)
	}
	draft := c.createActivity(t, qlcmA, activityBody(s.qc, "quiz", quizConfig(s.questions...)))
	if r := c.start(t, hsAn, draft, nil, nil); r.status != http.StatusConflict || r.str("code") != "CONTENT_NOT_PUBLISHED" {
		t.Fatalf("draft: %d %s", r.status, r.raw)
	}
	act := must(t, c.do(t, http.MethodGet, "/learning/activities/"+s.activityID, hsAn, nil, nil), http.StatusOK, "activity")
	if act.str("title") != "Luyện bảng nhân" || act.str("min_client_version") != "0.1.0" || len(answerKeysIn(act.raw)) > 0 {
		t.Fatalf("activity render: %s", act.raw)
	}
	// AC-06: staff on the portal, another student of the same tenant.
	if r := c.start(t, qlcmA, s.activityID, nil, nil); r.status != http.StatusForbidden || r.str("code") != "GAME_ACCESS_STUDENT_ONLY" {
		t.Fatalf("staff: %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodGet, "/learning/activity-attempts/"+id, hsBinh, nil, nil); r.status != 404 || r.str("code") != "ATTEMPT_NOT_FOUND" {
		t.Fatalf("other student: %d %s", r.status, r.raw)
	}
	if r := c.answer(t, hsBinh, id, 1, map[string]any{"option_id": "o_3b2e"}, nil); r.status != 404 {
		t.Fatalf("other student answers: %d", r.status)
	}
	if r := c.do(t, http.MethodGet, "/learning/activity-attempts/"+id, hsDung, nil, nil); r.status != 404 {
		t.Fatalf("tenant B reads: %d", r.status)
	}
	// AC-07: history of the student only, with filters.
	mine := must(t, c.do(t, http.MethodGet, "/learning/activity-attempts?status=in_progress&activity_type=quiz", hsAn, nil, nil), 200, "mine")
	if ds := mine.items("data"); len(ds) != 1 || ds[0]["attempt_id"] != id || ds[0]["title"] != "Luyện bảng nhân" {
		t.Fatalf("mine: %s", mine.raw)
	}
	if r := must(t, c.do(t, http.MethodGet, "/learning/activity-attempts?status=graded", hsAn, nil, nil), 200, "graded"); len(r.items("data")) != 0 {
		t.Fatalf("graded filter: %s", r.raw)
	}
	if r := must(t, c.do(t, http.MethodGet, "/learning/activity-attempts", hsBinh, nil, nil), 200, "binh"); len(r.items("data")) != 0 {
		t.Fatalf("binh sees: %s", r.raw)
	}
	// activity.started: learning event and outbox.
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.learning_events WHERE event_type = 'activity.started' AND idempotency_key = $1
		AND context = 'practice' AND content_version_id IS NOT NULL`, "start:"+id); n != 1 {
		t.Fatalf("learning event activity.started: %d", n)
	}
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'activity.started'
		AND envelope->'payload'->>'attempt_id' = $1`, id); n != 1 {
		t.Fatalf("outbox activity.started: %d", n)
	}
	// attempt_start_rate (429): a limit of 1 start per hour, the second start (mini-game: no resume) is refused.
	limited := c.att
	limited.StartRate = 1
	binh := c.studentOf(ctx, hsBinh)
	aid := uuid.MustParse(s.activityID)
	if _, err := limited.StartActivity(binh, command.StartActivityInput{ActivityID: aid, Mode: "minigame"}); err != nil {
		t.Fatal(err)
	}
	_, err := limited.StartActivity(binh, command.StartActivityInput{ActivityID: aid, Mode: "minigame"})
	if code(err) != problem.RateLimited {
		t.Fatalf("start rate: %v", err)
	}
}

// F030-AC-08…AC-15 (US2, quickstart 6–12): server grading with feedback, client fields ignored, response errors, answer
// limit, one submit (replay, other key), events, review, retake on a new version.
func TestAttemptRespondSubmitReview(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil)
	authztest.Cover(t, "submitActivityResponse", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "submitActivityAttempt", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getActivityAttemptReview", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "pauseActivityAttempt", authztest.CrossTenant, authztest.RoleDenied)
	right, wrong := correctOption()
	id := must(t, c.start(t, hsAn, s.activityID, nil, nil), http.StatusCreated, "start").str("attempt_id")
	// AC-08, AC-09: the client claims a score; the server grades.
	r := must(t, c.answer(t, hsAn, id, 1, map[string]any{"option_id": right}, map[string]any{"score": 100, "is_correct": false,
		"duration_ms": 1, "max_score": 100, "client_elapsed_ms": 2500}), http.StatusOK, "answer 1")
	if r.body["accepted"] != true || r.body["is_correct"] != true || r.body["score"] != float64(1) || r.str("feedback", "option_id") != right ||
		r.str("feedback", "text") == "" {
		t.Fatalf("answer 1: %s", r.raw)
	}
	// AC-10: wrong shape, unknown position, retries are unscored and bounded (g1_3: 3 tries).
	if r := c.answer(t, hsAn, id, 2, map[string]any{"option_ids": []string{right}}, nil); r.status != 422 || r.str("code") != "INVALID_RESPONSE" {
		t.Fatalf("invalid response: %d %s", r.status, r.raw)
	}
	if r := c.answer(t, hsAn, id, 99, map[string]any{"option_id": right}, nil); r.status != 422 || r.str("code") != "VALIDATION_FAILED" {
		t.Fatalf("position 99: %d %s", r.status, r.raw)
	}
	for i := 0; i < 2; i++ {
		must(t, c.answer(t, hsAn, id, 1, map[string]any{"option_id": wrong}, nil), http.StatusOK, "retry")
	}
	if r := c.answer(t, hsAn, id, 1, map[string]any{"option_id": right}, nil); r.status != 409 || r.str("code") != "ANSWER_LIMIT_REACHED" {
		t.Fatalf("answer limit: %d %s", r.status, r.raw)
	}
	must(t, c.answer(t, hsAn, id, 2, map[string]any{"option_id": wrong}, nil), http.StatusOK, "answer 2")
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempt_answers WHERE attempt_id = $1 AND is_scored`, id); n != 2 {
		t.Fatalf("scored answers: %d", n)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempt_answers WHERE attempt_id = $1 AND NOT is_scored AND attempt_no > 1`, id); n != 2 {
		t.Fatalf("unscored retries: %d", n)
	}
	// AC-15: no review before grading.
	if r := c.do(t, http.MethodGet, "/learning/activity-attempts/"+id+"/review", hsAn, nil, nil); r.status != http.StatusConflict {
		t.Fatalf("review before grading: %d", r.status)
	}
	// AC-12, AC-13: one submit; the same key replays, another key is refused; the score never moves.
	key := idem()
	sub := must(t, c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+":submit", hsAn, key, nil), http.StatusOK, "submit")
	if sub.str("status") != "graded" || sub.str("evaluation", "source") != "system" || sub.body["evaluation"].(map[string]any)["score"] != float64(1) ||
		sub.body["evaluation"].(map[string]any)["max_score"] != float64(3) || sub.str("submitted_at") == "" {
		t.Fatalf("submit: %s", sub.raw)
	}
	if again := must(t, c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+":submit", hsAn, key, nil), http.StatusOK, "replay"); again.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("submit replay: %s", again.raw)
	}
	if r := c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+":submit", hsAn, idem(), nil); r.status != 409 || r.str("code") != "ATTEMPT_ALREADY_SUBMITTED" {
		t.Fatalf("second submit: %d %s", r.status, r.raw)
	}
	// AC-11: no response nor pause after the submit.
	if r := c.answer(t, hsAn, id, 3, map[string]any{"option_id": right}, nil); r.status != 409 || r.str("code") != "ATTEMPT_NOT_IN_PROGRESS" {
		t.Fatalf("answer after submit: %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+":pause", hsAn, idem(), nil); r.status != 409 {
		t.Fatalf("pause after submit: %d", r.status)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempts WHERE attempt_id = $1 AND status = 'graded' AND score = 1 AND max_score = 3
		AND duration_ms >= 0 AND submit_reason = 'manual' AND client_elapsed_ms IS NULL`, id); n != 1 {
		t.Fatal("graded attempt row")
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.evaluations WHERE attempt_id = $1 AND effective AND status = 'final'`, id); n != 1 {
		t.Fatal("one effective evaluation")
	}
	for typ, want := range map[string]int{"activity.started": 1, "question.answered": 4, "activity.submitted": 1, "activity.graded": 1} {
		if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.learning_events WHERE event_type = $1 AND payload->>'attempt_id' = $2`,
			typ, id); n != want {
			t.Fatalf("learning events %s: %d, want %d", typ, n, want)
		}
	}
	for _, typ := range []string{"activity.submitted", "activity.graded"} {
		if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = $1 AND envelope->'payload'->>'attempt_id' = $2
			AND (envelope->'payload'->>'score')::numeric = 1`, typ, id); n != 1 {
			t.Fatalf("outbox %s: %d", typ, n)
		}
	}
	// AC-15: the review after grading.
	rv := must(t, c.do(t, http.MethodGet, "/learning/activity-attempts/"+id+"/review", hsAn, nil, nil), http.StatusOK, "review")
	ri := rv.items("items")
	if len(ri) != 3 || ri[0]["is_correct"] != true || ri[0]["score"] != float64(1) || ri[1]["is_correct"] != false || ri[2]["is_correct"] != nil ||
		rv.body["score"] != float64(1) {
		t.Fatalf("review: %s", rv.raw)
	}
	if fb, _ := ri[1]["feedback"].(map[string]any); fb == nil || fb["option_id"] != wrong {
		t.Fatalf("review feedback: %v", ri[1])
	}
	if weak, _ := rv.body["weak_objectives"].([]any); len(weak) != 1 || weak[0] != s.qc.lo {
		t.Fatalf("weak objectives: %s", rv.raw)
	}
	if r := c.do(t, http.MethodGet, "/learning/activity-attempts/"+id+"/review", hsBinh, nil, nil); r.status != 404 {
		t.Fatalf("review of another student: %d", r.status)
	}
	// AC-14: a new release does not touch the old attempt; the retake pins the new version.
	staff := authztest.Ctx(ctx, authzPrincipal(c, qlcmA))
	if _, err := c.dcmd.NewVersion(staff, command.NewDocVersionInput{Type: contentmodel.TypeActivity, ID: uuid.MustParse(s.activityID), FromVersion: 1}); err != nil {
		t.Fatal(err)
	}
	c.publishActivity(ctx, t, s.activityID, 2)
	retake := must(t, c.start(t, hsAn, s.activityID, nil, nil), http.StatusCreated, "retake")
	old := must(t, c.do(t, http.MethodGet, "/learning/activity-attempts/"+id, hsAn, nil, nil), http.StatusOK, "old attempt")
	if retake.str("attempt_id") == id || retake.str("activity_version_id") == old.str("activity_version_id") || old.str("status") != "graded" ||
		old.str("evaluation", "score") != "1" {
		t.Fatalf("retake %s / old %s", retake.raw, old.raw)
	}
}

// F030-AC-16…AC-19 (US3, quickstart 8, 13, 14): pause and resume on the pinned version, pause time out of the duration,
// abandon after the resume window, deadline: 410 after the grace, 422 on submit, timeout auto-submit by the job.
func TestAttemptPauseResumeExpire(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil)
	right, _ := correctOption()
	id := must(t, c.start(t, hsAn, s.activityID, nil, nil), http.StatusCreated, "start").str("attempt_id")
	must(t, c.answer(t, hsAn, id, 1, map[string]any{"option_id": right}, nil), http.StatusOK, "answer 1")
	p := must(t, c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+":pause", hsAn, idem(), nil), http.StatusOK, "pause")
	if p.str("paused_at") == "" {
		t.Fatalf("pause: %s", p.raw)
	}
	c.clk.Advance(30 * time.Second)
	g := must(t, c.do(t, http.MethodGet, "/learning/activity-attempts/"+id, hsAn, nil, nil), http.StatusOK, "resume read")
	saved := g.items("saved_responses")
	if len(saved) != 1 || saved[0]["position"] != float64(1) || len(g.items("items")) != 3 || g.str("paused_at") == "" ||
		g.items("items")[0]["question_version_id"] != s.questions[0] {
		t.Fatalf("resume read: %s", g.raw)
	}
	must(t, c.answer(t, hsAn, id, 2, map[string]any{"option_id": right}, nil), http.StatusOK, "answer after the pause")
	must(t, c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+":submit", hsAn, idem(), nil), http.StatusOK, "submit")
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempts WHERE attempt_id = $1 AND paused_ms = 30000 AND duration_ms = 0`, id); n != 1 {
		t.Fatal("the pause is excluded from the duration")
	}
	// AC-18: abandoned after attempt_resume_window; 410 afterwards.
	ab := must(t, c.start(t, hsBinh, s.activityID, nil, nil), http.StatusCreated, "binh").str("attempt_id")
	c.clk.Advance(6 * time.Minute)
	if rep, err := c.att.RunLifecycle(ctx, 100); err != nil || rep.Paused != 1 {
		t.Fatalf("idle pause: %+v %v", rep, err)
	}
	c.clk.Advance(25 * time.Hour)
	if rep, err := c.att.RunLifecycle(ctx, 100); err != nil || rep.Abandoned != 1 {
		t.Fatalf("abandon: %+v %v", rep, err)
	}
	if r := c.do(t, http.MethodGet, "/learning/activity-attempts/"+ab, hsBinh, nil, nil); r.status != http.StatusGone || r.str("code") != "ATTEMPT_EXPIRED" {
		t.Fatalf("abandoned read: %d %s", r.status, r.raw)
	}
	if r := c.answer(t, hsBinh, ab, 1, map[string]any{"option_id": right}, nil); r.status != http.StatusGone {
		t.Fatalf("abandoned answer: %d", r.status)
	}
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'activity.abandoned'
		AND envelope->'payload'->>'attempt_id' = $1 AND envelope->'payload'->>'reason' = 'expired'`, ab); n != 1 {
		t.Fatalf("activity.abandoned: %d", n)
	}
	// AC-19: a timed quiz.
	timedBody := activityBody(s.qc, "quiz", quizConfig(s.questions...))
	timedBody["time_limit_seconds"] = 60
	timed := c.createActivity(t, qlcmA, timedBody)
	c.publishActivity(ctx, t, timed, 1)
	tid := must(t, c.start(t, hsAn, timed, nil, nil), http.StatusCreated, "timed").str("attempt_id")
	empty := must(t, c.start(t, hsBinh, timed, nil, nil), http.StatusCreated, "timed empty").str("attempt_id")
	must(t, c.answer(t, hsAn, tid, 1, map[string]any{"option_id": right}, nil), http.StatusOK, "timed answer")
	c.clk.Advance(61 * time.Second)
	must(t, c.answer(t, hsAn, tid, 2, map[string]any{"option_id": right}, nil), http.StatusOK, "within the grace")
	c.clk.Advance(2 * time.Second)
	if r := c.answer(t, hsAn, tid, 3, map[string]any{"option_id": right}, nil); r.status != http.StatusGone {
		t.Fatalf("after the grace: %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodPost, "/learning/activity-attempts/"+tid+":submit", hsAn, idem(), nil); r.status != 422 || r.str("code") != "ATTEMPT_DEADLINE_PASSED" {
		t.Fatalf("late submit: %d %s", r.status, r.raw)
	}
	rep, err := c.att.RunLifecycle(ctx, 100)
	if err != nil || rep.AutoSubmitted != 1 || rep.Expired != 1 {
		t.Fatalf("expire: %+v %v", rep, err)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempts WHERE attempt_id = $1 AND status = 'graded' AND submit_reason = 'timeout'
		AND score = 2 AND duration_ms = 60000`, tid); n != 1 {
		t.Fatal("timeout submit")
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempts WHERE attempt_id = $1 AND status = 'expired' AND score IS NULL`, empty); n != 1 {
		t.Fatal("expired without answers is not graded")
	}
}
