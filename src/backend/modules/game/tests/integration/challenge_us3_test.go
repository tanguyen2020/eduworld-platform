//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// learningIDs returns the learning attempts linked to a challenge attempt.
func (e *env) learningIDs(ctx context.Context, t *testing.T, attempt string) []string {
	t.Helper()
	var raw []byte
	if err := e.platform.QueryRowContext(ctx, `SELECT learning_attempt_ids FROM game.challenge_attempts WHERE challenge_attempt_id = $1`,
		attempt).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var out []string
	_ = json.Unmarshal(raw, &out)
	return out
}

// F076-AC-08, AC-09, AC-12 (TC-CHL-003, TC-SVC-010): nextChallengeActivity opens a learning attempt of the student of
// the session through createAttempt (mode challenge, context challenge_attempt, the band of the level, the grade of the
// student, the objectives; one item at the per-item time of the level for survival); the answer carries no answer key;
// the same Idempotency-Key gives the same learning attempt; module learning down is 503 with Retry-After and changes
// nothing, a retry with the same key gives the attempt of that key; an unpublished activity is 422 ITEM_REF_INVALID.
func TestChallengeNextActivity(t *testing.T) {
	ctx := context.Background()
	e := setupChallenges(ctx, t)
	an := e.studentOfGrade(ctx, t, e.tenantA, e.schoolA, 3)
	st := e.startChallenge(t, an, chlSurvival, "hard", "")
	attempt := st.str("challenge_attempt_id")
	k := newKey()
	n := e.nextActivity(t, an, attempt, k)
	if n.status != http.StatusOK || n.body["done"] != false {
		t.Fatalf("next: %d %s", n.status, n.raw)
	}
	la := learningAttemptOf(t, n)
	a := n.body["attempt"].(map[string]any)
	if a["mode"] != "challenge" || a["context_type"] != "challenge_attempt" || a["context_id"] != attempt || a["activity_id"] != chlActivity1 {
		t.Fatalf("attempt: %s", n.raw)
	}
	calls, keys := e.learning.recorded()
	c := calls[len(calls)-1]
	if string(c.Mode) != "challenge" || string(c.Context.ContextType) != "challenge_attempt" || c.Context.ContextId.String() != attempt ||
		c.StudentId != an.id || *c.Activity.DifficultyMin != 3 || *c.Activity.DifficultyMax != 4 || *c.Activity.GradeLevel != 3 ||
		len(*c.Activity.ObjectiveIds) != 1 || (*c.Activity.ObjectiveIds)[0].String() != chlObjective || *c.Activity.QuestionCount != 1 ||
		*c.Activity.TimeLimitSeconds != 37 || keys[len(keys)-1] != k {
		t.Fatalf("createAttempt: %+v key %s", c, keys[len(keys)-1])
	}
	if keys := answerKeys(n.raw); len(keys) > 0 {
		t.Fatalf("answer data in the response: %v", keys)
	}
	if again := e.nextActivity(t, an, attempt, k); learningAttemptOf(t, again) != la {
		t.Fatalf("same key: %s", again.raw)
	}
	if ids := e.learningIDs(ctx, t, attempt); !slices.Equal(ids, []string{la}) {
		t.Fatalf("linked: %v", ids)
	}
	// AC-12: module learning down → 503 + Retry-After, nothing linked; the same key later gives the attempt of that key.
	e.learning.setFail(errors.New("learning down"), 0)
	k2 := newKey()
	down := e.nextActivity(t, an, attempt, k2)
	if down.status != http.StatusServiceUnavailable || down.code() != string(problem.Unavailable) || down.header.Get("Retry-After") == "" {
		t.Fatalf("learning down: %d %s", down.status, down.raw)
	}
	if ids := e.learningIDs(ctx, t, attempt); len(ids) != 1 {
		t.Fatalf("challenge changed: %v", ids)
	}
	if r := e.challengeAttempt(t, an, attempt); r.str("status") != "in_progress" {
		t.Fatalf("status: %s", r.raw)
	}
	e.learning.setFail(nil, 0)
	retry := e.nextActivity(t, an, attempt, k2)
	if retry.status != http.StatusOK || learningAttemptOf(t, retry) == la {
		t.Fatalf("retry: %s", retry.raw)
	}
	_, keys = e.learning.recorded()
	if keys[len(keys)-1] != k2 || keys[len(keys)-2] != k2 {
		t.Fatal("the same key is forwarded on retry")
	}
	// The second activity of the list, then the first again (survival cycles).
	if a := retry.body["attempt"].(map[string]any); a["activity_id"] != chlActivity2 {
		t.Fatalf("second activity: %s", retry.raw)
	}
	e.learning.setFail(problem.New(problem.ContentNotPublished, "not published"), 0)
	if r := e.nextActivity(t, an, attempt, ""); r.status != http.StatusUnprocessableEntity || r.code() != string(problem.ItemRefInvalid) {
		t.Fatalf("unpublished activity: %d %s", r.status, r.raw)
	}
	e.learning.setFail(nil, 0)
}

func answerKeys(raw string) []string {
	var out []string
	for _, k := range []string{"correct_option_ids", "answer_key", "scoring", "correct_answer", "is_correct"} {
		if json.Valid([]byte(raw)) && containsKey(raw, k) {
			out = append(out, k)
		}
	}
	return out
}

func containsKey(raw, k string) bool {
	var v any
	_ = json.Unmarshal([]byte(raw), &v)
	var rec func(x any) bool
	rec = func(x any) bool {
		switch t := x.(type) {
		case map[string]any:
			for key, e := range t {
				if key == k || rec(e) {
					return true
				}
			}
		case []any:
			for _, e := range t {
				if rec(e) {
					return true
				}
			}
		}
		return false
	}
	return rec(v)
}

// challengeRewardRules are the reward rules of challenge.completed: 50 XP × reward_multiplier for a success, 100 XP and
// 20 Xu × multiplier for the first clear (F074 caps apply).
func challengeRewardRules() []command.ContentItem {
	rule := func(id, code string, conditions map[string]any, grants ...any) command.ContentItem {
		m := map[string]any{"schema_version": 1, "reward_rule_id": id, "code": code, "owner_scope": "platform", "tenant_id": nil,
			"source_code": "challenge.completed", "priority": 100, "trigger": map[string]any{"event_type": "challenge.completed"},
			"grants": grants, "caps": map[string]any{"xp_per_day": 1000}}
		if conditions != nil {
			m["conditions"] = conditions
		}
		b, _ := json.Marshal(m)
		return command.ContentItem{Type: write.ItemRewardRule, Payload: b}
	}
	return []command.ContentItem{
		rule("0193a5f4-6c1e-7b2a-9d1f-0000000f7601", "xp.challenge_completed", nil,
			map[string]any{"type": "xp", "amount": 50, "scale_by": "reward_multiplier"}),
		rule("0193a5f4-6c1e-7b2a-9d1f-0000000f7602", "xp.challenge_first_clear", map[string]any{"first_completion_only": true},
			map[string]any{"type": "xp", "amount": 100}),
	}
}

// F076-AC-10, AC-11 (TC-CHL-001, TC-CHL-006, SC-005): activity.graded updates score/lives only from the graded result;
// a replayed event changes nothing; the end writes exactly one challenge.completed (status, score, difficulty,
// reward_multiplier, first_clear, objectives) and pushes challenge.attempt.updated; the Reward Engine rewards a success
// scaled by the level, the first clear once, a failure nothing (no XP loss), a replayed completion nothing more.
func TestChallengeProgressAndRewards(t *testing.T) {
	ctx := context.Background()
	e := setupChallenges(ctx, t)
	if _, err := e.publish(ctx, t, append(progressionSeed(t), challengeRewardRules()...), "rules"); err != nil {
		t.Fatal(err)
	}
	dung := e.studentOfGrade(ctx, t, e.tenantA, e.schoolA2, 7)
	at := e.clock.Now()
	// Survival hard: 3 lives, every round wrong → failed (lives_depleted) at the third.
	sv := e.startChallenge(t, dung, chlSurvival, "hard", "").str("challenge_attempt_id")
	for i := 1; i <= 3; i++ {
		la := learningAttemptOf(t, e.nextActivity(t, dung, sv, ""))
		ev := uuid.New()
		e.challengeGraded(ctx, t, dung, la, 0, 1, at, ev)
		e.challengeGraded(ctx, t, dung, la, 0, 1, at, uuid.New()) // replay with a new event id
		r := e.challengeAttempt(t, dung, sv)
		if i < 3 && (r.str("status") != "in_progress" || r.body["lives_left"] != float64(3-i)) {
			t.Fatalf("round %d: %s", i, r.raw)
		}
		if i == 3 && (r.str("status") != "failed" || r.body["lives_left"] != float64(0) || r.body["activities_completed"] != float64(3)) {
			t.Fatalf("lives depleted: %s", r.raw)
		}
	}
	failed := e.completedEvents(ctx, t, sv)
	if len(failed) != 1 || failed[0]["status"] != "failed" || failed[0]["end_reason"] != "lives_depleted" || failed[0]["difficulty"] != "hard" ||
		failed[0]["reward_multiplier"] != 1.6 || failed[0]["first_clear"] != false || failed[0]["score"] != float64(0) {
		t.Fatalf("challenge.completed (failed): %+v", failed)
	}
	if r := e.nextActivity(t, dung, sv, ""); r.status != http.StatusUnprocessableEntity || r.code() != string(problem.InvalidStateTransition) {
		t.Fatalf("next after the end: %d %s", r.status, r.raw)
	}
	e.mustDeliver(ctx, t, e.progressionUC(), e.envelope("challenge.completed", "game", dung.tenant, failed[0]))
	if rows := e.ledger(ctx, t, dung); len(rows) != 0 {
		t.Fatalf("a failure earns nothing and loses nothing: %+v", rows)
	}
	// Weekly (no time limit) normal ×1.3: 5 correct answers → succeeded, first clear.
	e.clock.Add(2 * time.Minute)
	wk := e.startChallenge(t, dung, chlWeekly7, "normal", "")
	if wk.status != http.StatusCreated || wk.body["deadline_at"] != nil {
		t.Fatalf("weekly: %s", wk.raw)
	}
	w := wk.str("challenge_attempt_id")
	e.challengeGraded(ctx, t, dung, learningAttemptOf(t, e.nextActivity(t, dung, w, "")), 3, 3, e.clock.Now(), uuid.New())
	e.challengeGraded(ctx, t, dung, learningAttemptOf(t, e.nextActivity(t, dung, w, "")), 2, 3, e.clock.Now(), uuid.New())
	if r := e.challengeAttempt(t, dung, w); r.str("status") != "succeeded" || r.body["score"] != float64(5) {
		t.Fatalf("succeeded: %s", r.raw)
	}
	done := e.completedEvents(ctx, t, w)
	if len(done) != 1 || done[0]["status"] != "succeeded" || done[0]["first_clear"] != true || done[0]["reward_multiplier"] != 1.3 ||
		done[0]["end_reason"] != "goal_reached" || len(done[0]["objective_ids"].([]any)) != 1 {
		t.Fatalf("challenge.completed (succeeded): %+v", done)
	}
	ev := e.envelope("challenge.completed", "game", dung.tenant, done[0])
	e.mustDeliver(ctx, t, e.progressionUC(), ev)
	e.mustDeliver(ctx, t, e.progressionUC(), e.envelope("challenge.completed", "game", dung.tenant, done[0])) // replayed completion
	rows := e.ledger(ctx, t, dung)
	if len(rows) != 2 || sumAmounts(rows) != 65+100 || rows[0].sourceType != "challenge" {
		t.Fatalf("success 50 × 1.3 + first clear 100, once: %+v", rows)
	}
	// A second success of the same challenge: no first clear again.
	e.clock.Add(time.Minute)
	w2 := e.startChallenge(t, dung, chlWeekly7, "normal", "").str("challenge_attempt_id")
	e.challengeGraded(ctx, t, dung, learningAttemptOf(t, e.nextActivity(t, dung, w2, "")), 5, 5, e.clock.Now(), uuid.New())
	again := e.completedEvents(ctx, t, w2)
	if len(again) != 1 || again[0]["first_clear"] != false {
		t.Fatalf("second clear: %+v", again)
	}
	e.mustDeliver(ctx, t, e.progressionUC(), e.envelope("challenge.completed", "game", dung.tenant, again[0]))
	for _, r := range e.ledger(ctx, t, dung)[2:] {
		if r.amount < 0 || r.ruleID.UUID.String() == "0193a5f4-6c1e-7b2a-9d1f-0000000f7602" {
			t.Fatalf("first clear twice or a negative amount: %+v", r)
		}
	}
	// challenge.attempt.updated pushes after commit, ending with the end state.
	var statuses []string
	for _, m := range e.challengePush.all() {
		if m["challenge_attempt_id"] == uuid.MustParse(w) {
			statuses = append(statuses, m["status"].(string))
		}
	}
	if len(statuses) < 2 || statuses[len(statuses)-1] != "succeeded" {
		t.Fatalf("pushes: %v", statuses)
	}
	if n := e.checkEventSchemas(ctx, t); n["challenge.started"] < 3 || n["challenge.completed"] < 3 {
		t.Fatalf("event schemas: %v", n)
	}
}
