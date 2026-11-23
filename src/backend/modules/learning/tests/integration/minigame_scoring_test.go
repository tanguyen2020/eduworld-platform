//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// mgAnswer sends a response to position of a mini-game attempt with the forged client fields of the tests.
func (c *curEnv) mgAnswer(t *testing.T, sub string, id uuid.UUID, position int, option string, extra map[string]any) resp {
	t.Helper()
	return c.answer(t, sub, id.String(), position, map[string]any{"option_id": option}, extra)
}

// F073-AC-07, AC-08, AC-09, AC-10 (T019, TC-LRN-007, TC-GAM-001…003): a mini-game play opened by module game is graded
// on the server; the game points come from the server interval (served → answered), the streak and the grade only — the
// forged is_correct, score, game_score and client_elapsed_ms of the client never count; grade 7 earns the speed bonus and
// pays the wrong penalty, grade 3 neither; the learning score, game score and completion are separate, stored and in
// activity.graded (minigame block), never in the learning events (mastery evidence).
func TestMinigameScoring(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil)
	c.user(hsGiang, authztest.Student(c.tenantA).PrincipalData, false)
	c.enroll(ctx, t, hsGiang, 7)
	cfg := mgSnapshot(uuid.New(), 1, "multiple_choice", selectionSource(s.qc.subject, "single_choice"), 3, nil)
	if !c.project(ctx, t, cfg) {
		t.Fatal("projection")
	}
	right, wrong := correctOption()
	forged := func(elapsed int) map[string]any {
		return map[string]any{"is_correct": true, "score": 1, "game_score": 9999, "game_points": 9999, "client_elapsed_ms": elapsed}
	}

	// Grade 7.
	created, err := c.mgStart(ctx, hsGiang, cfg, learningapi.AttemptModeMinigame)
	if err != nil || created.QuestionCount != 3 || created.DeadlineAt == nil {
		t.Fatalf("start: %+v %v", created, err)
	}
	g := c.getAttempt(t, hsGiang, created.AttemptId)
	if g.str("activity_type") != "minigame" || g.str("mode") != "minigame" || g.str("context_type") != "minigame_session" ||
		g.str("render", "render_payload", "minigame_type") != "multiple_choice" ||
		g.str("render", "render_payload", "minigame_config_version_id") != cfg.versionID.String() ||
		g.str("render", "render_payload", "config", "scoring", "time_bonus_enabled") != "true" || len(answerKeysIn(g.raw)) > 0 ||
		strings.Contains(g.raw, "question_source") || strings.Contains(g.raw, "wrong_penalty") || strings.Contains(g.raw, "max_bonus") {
		t.Fatalf("attempt of grade 7: %s", g.raw)
	}
	// 40 s on the server (bonus 25 between 20 s and 60 s) although the client claims 1 ms; a forged is_correct is ignored.
	c.clk.Advance(40 * time.Second)
	r := must(t, c.mgAnswer(t, hsGiang, created.AttemptId, 1, right, forged(1)), http.StatusOK, "answer 1")
	if mgNum(r, "game_points") != 125 || mgNum(r, "streak") != 1 || r.str("is_correct") != "true" {
		t.Fatalf("answer 1: %s", r.raw)
	}
	c.clk.Advance(5 * time.Second)
	r = must(t, c.mgAnswer(t, hsGiang, created.AttemptId, 2, wrong, forged(1)), http.StatusOK, "answer 2 (wrong, forged correct)")
	if mgNum(r, "game_points") != -10 || mgNum(r, "streak") != 0 || r.str("is_correct") != "false" {
		t.Fatalf("answer 2: %s", r.raw)
	}
	c.clk.Advance(70 * time.Second)
	r = must(t, c.mgAnswer(t, hsGiang, created.AttemptId, 3, right, forged(999999)), http.StatusOK, "answer 3")
	if mgNum(r, "game_points") != 100 || mgNum(r, "streak") != 1 {
		t.Fatalf("answer 3: %s", r.raw)
	}
	sub := must(t, c.do(t, http.MethodPost, "/learning/activity-attempts/"+created.AttemptId.String()+":submit", hsGiang, idem(),
		map[string]any{"game_score": 9999, "score": 3}), http.StatusOK, "submit")
	if mgNum(sub, "evaluation", "score") != 2 || mgNum(sub, "evaluation", "max_score") != 3 || mgNum(sub, "evaluation", "game_score") != 215 ||
		sub.str("evaluation", "completion_met") != "false" {
		t.Fatalf("submit: %s", sub.raw)
	}
	if again := c.getAttempt(t, hsGiang, created.AttemptId); mgNum(again, "evaluation", "game_score") != 215 || again.str("evaluation", "completion_met") != "false" {
		t.Fatalf("evaluation on a later read: %s", again.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempts WHERE attempt_id = $1 AND game_score = 215 AND score = 2
		AND source_kind = 'minigame_config' AND minigame_config_version_id = $2 AND activity_id IS NULL`, created.AttemptId, cfg.versionID); n != 1 {
		t.Fatal("stored learning score, game score and pinned version")
	}
	graded := c.outboxPayloads(ctx, t, "activity.graded", "attempt_id", created.AttemptId.String())
	if len(graded) != 1 {
		t.Fatalf("activity.graded: %v", graded)
	}
	mg, _ := graded[0]["minigame"].(map[string]any)
	if mg == nil || mg["game_score"] != float64(215) || mg["completion_met"] != false || mg["minigame_type"] != "multiple_choice" ||
		mg["minigame_config_id"] != cfg.configID.String() || mg["minigame_config_version_id"] != cfg.versionID.String() ||
		mg["context_type"] != "minigame_session" || graded[0]["score"] != float64(2) || graded[0]["max_score"] != float64(3) {
		t.Fatalf("activity.graded minigame block: %v", graded[0])
	}
	// The mastery evidence (learning events) never carries the game score (F073-AC-10, AC-MG-006).
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.learning_events WHERE payload->>'attempt_id' = $1`, created.AttemptId.String()); n < 4 {
		t.Fatalf("learning events: %d", n)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.learning_events WHERE payload->>'attempt_id' = $1
		AND (payload::text LIKE '%game_score%' OR payload::text LIKE '%game_points%')`, created.AttemptId.String()); n != 0 {
		t.Fatal("a learning event carries the game score")
	}

	// Grade 3: no speed bonus, no wrong penalty (F073-AC-09); every item answered completes a complete_all config.
	cfg3 := mgSnapshot(uuid.New(), 1, "multiple_choice", selectionSource(s.qc.subject, "single_choice"), 3, func(m map[string]any) {
		m["completion_rule"] = map[string]any{"type": "complete_all"}
	})
	c.project(ctx, t, cfg3)
	a3, err := c.mgStart(ctx, hsAn, cfg3, learningapi.AttemptModeQuest)
	if err != nil {
		t.Fatal(err)
	}
	if g := c.getAttempt(t, hsAn, a3.AttemptId); g.str("render", "render_payload", "config", "scoring", "time_bonus_enabled") != "false" ||
		g.str("context_type") != "quest_step" {
		t.Fatalf("attempt of grade 3: %s", g.raw)
	}
	c.clk.Advance(time.Second)
	if r := must(t, c.mgAnswer(t, hsAn, a3.AttemptId, 1, right, nil), http.StatusOK, "g3 answer 1"); mgNum(r, "game_points") != 100 {
		t.Fatalf("grade 3 fast: %s", r.raw)
	}
	c.clk.Advance(time.Second)
	if r := must(t, c.mgAnswer(t, hsAn, a3.AttemptId, 2, wrong, nil), http.StatusOK, "g3 answer 2"); mgNum(r, "game_points") != 0 || mgNum(r, "streak") != 0 {
		t.Fatalf("grade 3 wrong: %s", r.raw)
	}
	must(t, c.mgAnswer(t, hsAn, a3.AttemptId, 3, right, nil), http.StatusOK, "g3 answer 3")
	sub = must(t, c.do(t, http.MethodPost, "/learning/activity-attempts/"+a3.AttemptId.String()+":submit", hsAn, idem(), nil), http.StatusOK, "g3 submit")
	if mgNum(sub, "evaluation", "game_score") != 200 || sub.str("evaluation", "completion_met") != "true" {
		t.Fatalf("grade 3 submit: %s", sub.raw)
	}

	// Deferred feedback: the game points would reveal the correctness, so none is returned before the submit.
	late := mgSnapshot(uuid.New(), 1, "multiple_choice", selectionSource(s.qc.subject, "single_choice"), 3, func(m map[string]any) {
		m["feedback_policy"] = map[string]any{"timing": "after_submit"}
	})
	c.project(ctx, t, late)
	al, err := c.mgStart(ctx, hsGiang, late, learningapi.AttemptModeMinigame)
	if err != nil {
		t.Fatal(err)
	}
	if r := must(t, c.mgAnswer(t, hsGiang, al.AttemptId, 1, right, nil), http.StatusOK, "deferred"); r.body["game_points"] != nil || r.body["is_correct"] != nil {
		t.Fatalf("deferred feedback leaks: %s", r.raw)
	}
	// Another student never acts on the play; tenant B never sees it.
	if r := c.mgAnswer(t, hsBinh, al.AttemptId, 2, right, nil); r.status != http.StatusNotFound {
		t.Fatalf("another student: %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodGet, "/learning/activity-attempts/"+al.AttemptId.String(), hsDung, nil, nil); r.status != http.StatusNotFound {
		t.Fatalf("tenant B: %d", r.status)
	}
	// A version that is not projected yet: 409 CONFLICT (module game answers 503 Retry-After); no attempt.
	missing := mgSnapshot(uuid.New(), 1, "multiple_choice", selectionSource(s.qc.subject, "single_choice"), 3, nil)
	if _, err := c.mgStart(ctx, hsGiang, missing, learningapi.AttemptModeMinigame); code(err) != problem.Conflict {
		t.Fatalf("projection not ready: %v", err)
	}
	// A pool shorter than question_count: QUESTION_POOL_INSUFFICIENT, no attempt.
	big := mgSnapshot(uuid.New(), 1, "multiple_choice", selectionSource(s.qc.subject, "single_choice"), 4, nil)
	c.project(ctx, t, big)
	before := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempts WHERE activity_type = 'minigame'`)
	if _, err := c.mgStart(ctx, hsGiang, big, learningapi.AttemptModeMinigame); code(err) != problem.QuestionPoolInsufficient {
		t.Fatalf("pool insufficient: %v", err)
	}
	if after := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempts WHERE activity_type = 'minigame'`); after != before {
		t.Fatal("an attempt was created without enough questions")
	}
}

// F073-AC-11 (T020, TC-EXM-001): an answer after deadline_at + deadline_grace is 422 ATTEMPT_DEADLINE_PASSED (the code of
// the ProblemCode catalog); the attempt-lifecycle job expires the play and grades the answered part with its game score
// and completion; a later answer is refused.
func TestMinigameDeadline(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil)
	cfg := mgSnapshot(uuid.New(), 1, "timed_challenge", selectionSource(s.qc.subject, "single_choice", "true_false"), 3, func(m map[string]any) {
		m["time_limit_seconds"] = 10
		m["completion_rule"] = map[string]any{"type": "complete_all"}
	})
	c.project(ctx, t, cfg)
	a, err := c.mgStart(ctx, hsAn, cfg, learningapi.AttemptModeMinigame)
	if err != nil || a.DeadlineAt == nil || a.DeadlineAt.Sub(a.StartedAt) != 10*time.Second {
		t.Fatalf("start: %+v %v", a, err)
	}
	right, _ := correctOption()
	c.clk.Advance(2 * time.Second)
	must(t, c.mgAnswer(t, hsAn, a.AttemptId, 1, right, nil), http.StatusOK, "in time")
	c.clk.Advance(11 * time.Second) // 13 s: past deadline_at + deadline_grace (2 s)
	if r := c.mgAnswer(t, hsAn, a.AttemptId, 2, right, nil); r.status != http.StatusUnprocessableEntity || r.str("code") != "ATTEMPT_DEADLINE_PASSED" {
		t.Fatalf("late answer: %d %s", r.status, r.raw)
	}
	rep, err := c.att.RunLifecycle(ctx, 100)
	if err != nil || rep.AutoSubmitted < 1 {
		t.Fatalf("lifecycle: %+v %v", rep, err)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempts WHERE attempt_id = $1 AND status = 'graded'
		AND submit_reason = 'timeout' AND score = 1 AND max_score = 3 AND game_score = 100`, a.AttemptId); n != 1 {
		t.Fatal("expired play graded on its answered part")
	}
	g := c.getAttempt(t, hsAn, a.AttemptId)
	if mgNum(g, "evaluation", "game_score") != 100 || g.str("evaluation", "completion_met") != "false" {
		t.Fatalf("evaluation after timeout: %s", g.raw)
	}
	graded := c.outboxPayloads(ctx, t, "activity.graded", "attempt_id", a.AttemptId.String())
	if len(graded) != 1 || graded[0]["minigame"] == nil || graded[0]["minigame"].(map[string]any)["completion_met"] != false {
		t.Fatalf("activity.graded after timeout: %v", graded)
	}
	if r := c.mgAnswer(t, hsAn, a.AttemptId, 3, right, nil); r.status != http.StatusConflict {
		t.Fatalf("answer after grading: %d %s", r.status, r.raw)
	}
}
