//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F072-AC-14 (T026, TC-LRN-002): startQuestActivity calls createAttempt of module learning (mode quest, context quest_step
// with the progress, version, step and game session, the activity of the quest version) with the Idempotency-Key of
// the request, links the attempt to the step and answers 201 Attempt; a locked learning step is 422; the same key
// answers the same attempt.
func TestQuestStartActivity(t *testing.T) {
	ctx := context.Background()
	e := setupQuests(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	tok := e.gtoken(an)
	acc := e.accept(t, an, tok, learnQuest, "")
	if r := e.startActivity(t, an, tok, learnQuest, "solve", ""); r.status != http.StatusUnprocessableEntity || r.code() != string(problem.InvalidStateTransition) {
		t.Fatalf("locked learning step: %d %s", r.status, r.raw)
	}
	if r := e.startActivity(t, an, tok, learnQuest, "meet", ""); r.code() != string(problem.StepPreconditionFailed) {
		t.Fatalf("start a frame step: %d %s", r.status, r.raw)
	}
	p := e.play(t, an, "learning_zone.math")
	if r := e.report(t, an, p.d.token, learnQuest, "meet", "", body(p, "npc_id", "npc.mr_numbers")); r.status != http.StatusOK {
		t.Fatalf("meet: %d %s", r.status, r.raw)
	}
	k := newKey()
	r := e.startActivity(t, an, tok, learnQuest, "solve", k)
	if r.status != http.StatusCreated || r.body["mode"] != "quest" || r.body["context_type"] != "quest_step" || r.body["context_id"] != acc.body["quest_progress_id"] ||
		r.body["activity_id"] != testContent || r.body["status"] != "in_progress" {
		t.Fatalf("start: %d %s", r.status, r.raw)
	}
	calls, keys := e.learning.recorded()
	if len(calls) != 1 || keys[0] != k {
		t.Fatalf("createAttempt calls: %v keys %v", calls, keys)
	}
	c := calls[0]
	if string(c.Mode) != "quest" || string(c.Activity.ContentType) != "activity" || c.Activity.ContentId.String() != testContent ||
		string(c.Context.ContextType) != "quest_step" || c.Context.ContextId.String() != acc.body["quest_progress_id"] || c.Context.StepId == nil ||
		*c.Context.StepId != "solve" || c.Context.QuestVersionId == nil || c.Context.QuestVersionId.String() != acc.body["quest_version_id"] || c.StudentId != an.id {
		t.Fatalf("createAttempt request: %+v", c)
	}
	again := e.startActivity(t, an, tok, learnQuest, "solve", k)
	if again.status != http.StatusCreated || again.body["attempt_id"] != r.body["attempt_id"] {
		t.Fatalf("replay: %d %s", again.status, again.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.quest_step_attempts WHERE student_id = $1 AND step_id = 'solve'`, an.id); n != 1 {
		t.Fatalf("step attempts: %d", n)
	}
}

// F072-AC-15, AC-16 (T028, TC-WLD-006, TC-SVC-003, TC-LRN-007): activity.graded of a linked attempt below the pass rule
// leaves the step active and pushes the hint; a passing attempt completes it and opens the next; a replayed graded
// event changes nothing; a client report of a learning step is 409 STEP_PRECONDITION_FAILED; an attempt of another
// context is ignored.
func TestQuestActivityResult(t *testing.T) {
	ctx := context.Background()
	e := setupQuests(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	tok := e.gtoken(an)
	e.accept(t, an, tok, learnQuest, "")
	first := e.startActivityAfterMeet(ctx, t, an, tok)
	if r := e.report(t, an, tok, learnQuest, "solve", "", map[string]any{"game_session_id": uuid.NewString()}); r.status != http.StatusConflict {
		t.Fatalf("report a learning step (session check first): %d %s", r.status, r.raw)
	}
	s, _ := e.sessionOf(ctx, t, an)
	if r := e.report(t, an, tok, learnQuest, "solve", "", map[string]any{"game_session_id": s}); r.code() != string(problem.StepPreconditionFailed) {
		t.Fatalf("report a learning step: %d %s", r.status, r.raw)
	}
	if err := e.graded(ctx, t, an, first.body["attempt_id"].(string), 1, 3, uuid.New()); err != nil {
		t.Fatal(err)
	}
	if _, steps := e.progressRow(ctx, t, an, learnQuest); steps["solve"].Status != "active" {
		t.Fatalf("not passed: %v", steps)
	}
	pushes := e.questPush.all()
	last := pushes[len(pushes)-1]
	if last["passed"] != false || last["hint"] == nil {
		t.Fatalf("push of a failed attempt: %v", last)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.quest_step_attempts WHERE attempt_id = $1 AND status = 'not_passed'`, first.body["attempt_id"]); n != 1 {
		t.Fatalf("step attempt not_passed: %d", n)
	}
	second := e.startActivity(t, an, tok, learnQuest, "solve", "")
	if second.status != http.StatusCreated || second.body["attempt_id"] == first.body["attempt_id"] {
		t.Fatalf("retry: %d %s", second.status, second.raw)
	}
	if err := e.graded(ctx, t, an, uuid.NewString(), 3, 3, uuid.New()); err != nil {
		t.Fatalf("attempt of another context: %v", err)
	}
	if err := e.graded(ctx, t, an, second.body["attempt_id"].(string), 3, 3, uuid.New()); err != nil {
		t.Fatal(err)
	}
	if _, steps := e.progressRow(ctx, t, an, learnQuest); steps["solve"].Status != "completed" || steps["back"].Status != "active" ||
		steps["apples"].Status != "active" {
		t.Fatalf("passed: %v", steps)
	}
	n := e.events(ctx, t, "quest.step_completed")
	if err := e.graded(ctx, t, an, second.body["attempt_id"].(string), 3, 3, uuid.New()); err != nil {
		t.Fatal(err)
	}
	if e.events(ctx, t, "quest.step_completed") != n {
		t.Fatal("replayed activity.graded processed twice")
	}
	sc := e.payloads(ctx, t, "quest.step_completed")
	if sc[len(sc)-1]["attempt_id"] != second.body["attempt_id"] {
		t.Fatalf("quest.step_completed of the learning step: %v", sc[len(sc)-1])
	}
}

// sessionOf returns the active game session id of s.
func (e *env) sessionOf(ctx context.Context, t *testing.T, s student) (string, bool) {
	t.Helper()
	var id string
	err := e.platform.QueryRowContext(ctx, `SELECT game_session_id::text FROM game.game_sessions WHERE student_id = $1 AND ended_at IS NULL`, s.id).Scan(&id)
	return id, err == nil
}

// F072-AC-17, AC-18 (T029, TC-SVC-004, TC-SVC-010): max_attempts_per_day of the pass rule (2 in the test quest) → 409
// QUEST_STEP_ATTEMPT_LIMIT, again possible the next day; module learning failing or slower than its deadline → 503
// with Retry-After and no change of the quest progress.
func TestQuestActivityLimitsAndFailures(t *testing.T) {
	ctx := context.Background()
	e := setupQuests(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	tok := e.gtoken(an)
	e.accept(t, an, tok, learnQuest, "")
	if r := e.startActivityAfterMeet(ctx, t, an, tok); r.status != http.StatusCreated {
		t.Fatalf("first: %d %s", r.status, r.raw)
	}
	if r := e.startActivity(t, an, tok, learnQuest, "solve", ""); r.status != http.StatusCreated {
		t.Fatalf("second: %d %s", r.status, r.raw)
	}
	lim := e.startActivity(t, an, tok, learnQuest, "solve", "")
	if lim.status != http.StatusConflict || lim.code() != string(problem.QuestStepAttemptLimit) {
		t.Fatalf("limit: %d %s", lim.status, lim.raw)
	}
	e.clock.Add(24 * time.Hour)
	if r := e.startActivity(t, an, tok, learnQuest, "solve", ""); r.status != http.StatusCreated {
		t.Fatalf("next day: %d %s", r.status, r.raw)
	}
	before := count(ctx, t, e.platform, `SELECT count(*) FROM game.quest_step_attempts WHERE student_id = $1`, an.id)
	_, steps0 := e.progressRow(ctx, t, an, learnQuest)
	e.learning.setFail(problem.New(problem.Internal, "learning down"), 0)
	r := e.startActivity(t, an, tok, learnQuest, "solve", "")
	if r.status != http.StatusServiceUnavailable || r.header.Get("Retry-After") == "" {
		t.Fatalf("learning error: %d %v %s", r.status, r.header, r.raw)
	}
	e.learning.setFail(nil, 3*time.Second) // longer than the modcall write deadline (2 s)
	if r := e.startActivity(t, an, tok, learnQuest, "solve", ""); r.status != http.StatusServiceUnavailable || r.header.Get("Retry-After") == "" {
		t.Fatalf("learning timeout: %d %s", r.status, r.raw)
	}
	e.learning.setFail(nil, 0)
	_, steps1 := e.progressRow(ctx, t, an, learnQuest)
	if after := count(ctx, t, e.platform, `SELECT count(*) FROM game.quest_step_attempts WHERE student_id = $1`, an.id); after != before ||
		len(steps1["solve"].AttemptIDs) != len(steps0["solve"].AttemptIDs) {
		t.Fatalf("progress changed by a failed start: %d → %d", before, after)
	}
}
