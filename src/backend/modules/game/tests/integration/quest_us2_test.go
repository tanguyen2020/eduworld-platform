//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

func body(p played, kv ...string) map[string]any {
	m := map[string]any{"game_session_id": p.session}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return m
}

// completeMainA completes main.ch0.awakening in main_hub (the presence of p must be in main_hub).
func (e *env) completeMainA(t *testing.T, p played) {
	t.Helper()
	s := p.s
	if r := e.accept(t, s, p.d.token, questMainA, ""); r.status != http.StatusOK {
		t.Fatalf("accept: %d %s", r.status, r.raw)
	}
	for _, rep := range []struct {
		step string
		b    map[string]any
	}{{"meet_cloudy", body(p, "npc_id", "npc.cloudy")}, {"read_sign", body(p, "interactable_id", "quest.hub.welcome_sign")},
		{"light_lanterns", body(p, "interactable_id", "quest.hub.lantern_east")}, {"light_lanterns", body(p, "interactable_id", "quest.hub.lantern_west")},
		{"report_cloudy", body(p, "npc_id", "npc.cloudy")}} {
		if r := e.report(t, s, p.d.token, questMainA, rep.step, "", rep.b); r.status != http.StatusOK {
			t.Fatalf("report %s: %d %s", rep.step, r.status, r.raw)
		}
	}
}

// F072-AC-09, AC-10, AC-11, AC-28 (T020, T021, TC-WLD-006, TC-GAM-010): an interaction step is checked against the
// active game session, the presence in the area of the step and the ids of the step; a completed step opens the next one
// (quest.step_completed + quest.progress.updated); a wrong area, id or a lost presence is 409 STEP_PRECONDITION_FAILED
// (API_DESIGN §4.1) with no change, a locked or completed step 422 INVALID_STATE_TRANSITION; the same Idempotency-Key or
// the same interactable of a collect step counts once; an id longer than 64 characters is 422 VALIDATION_FAILED.
func TestQuestReportSteps(t *testing.T) {
	ctx := context.Background()
	e := setupQuests(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	p := e.play(t, an, "learning_zone.math")
	tok := p.d.token
	if r := e.accept(t, an, tok, questMainA, ""); r.status != http.StatusOK {
		t.Fatalf("accept: %d %s", r.status, r.raw)
	}
	if r := e.report(t, an, tok, questMainA, "meet_cloudy", "", body(p, "npc_id", "npc.cloudy")); r.status != http.StatusConflict ||
		r.code() != string(problem.StepPreconditionFailed) {
		t.Fatalf("wrong area: %d %s", r.status, r.raw)
	}
	if st, steps := e.progressRow(ctx, t, an, questMainA); st != "active" || steps["meet_cloudy"].Status != "active" {
		t.Fatalf("changed after a refused report: %s %v", st, steps)
	}
	if r := e.enter(t, an, p.d, p.session, "main_hub", ""); r.status != http.StatusOK {
		t.Fatalf("enter hub: %d %s", r.status, r.raw)
	}
	if r := e.report(t, an, tok, questMainA, "read_sign", "", body(p, "interactable_id", "quest.hub.welcome_sign")); r.status != http.StatusUnprocessableEntity ||
		r.code() != string(problem.InvalidStateTransition) {
		t.Fatalf("locked step: %d %s", r.status, r.raw)
	}
	if r := e.report(t, an, tok, questMainA, "meet_cloudy", "", body(p, "npc_id", "npc.postie")); r.code() != string(problem.StepPreconditionFailed) {
		t.Fatalf("npc of another step: %d %s", r.status, r.raw)
	}
	if r := e.report(t, an, tok, questMainA, "meet_cloudy", "", map[string]any{"game_session_id": newKey(), "npc_id": "npc.cloudy"}); r.status != http.StatusConflict ||
		r.code() != string(problem.Conflict) {
		t.Fatalf("unknown game session: %d %s", r.status, r.raw)
	}
	r := e.report(t, an, tok, questMainA, "meet_cloudy", "", body(p, "npc_id", "npc.cloudy", "area_id", "main_hub"))
	if r.status != http.StatusOK || stepStatus(r, "meet_cloudy") != "completed" || stepStatus(r, "read_sign") != "active" {
		t.Fatalf("meet: %d %s", r.status, r.raw)
	}
	if n := e.events(ctx, t, "quest.step_completed"); n != 1 {
		t.Fatalf("quest.step_completed: %d", n)
	}
	pushes := e.questPush.all()
	if len(pushes) != 1 || pushes[0]["step_id"] != "meet_cloudy" || !strings.Contains(strings.Join(anyStrings(pushes[0]["next_step_ids"]), ","), "read_sign") {
		t.Fatalf("quest.progress.updated: %v", pushes)
	}
	if r := e.report(t, an, tok, questMainA, "meet_cloudy", "", body(p, "npc_id", "npc.cloudy")); r.code() != string(problem.InvalidStateTransition) {
		t.Fatalf("completed step: %d %s", r.status, r.raw)
	}
	if r := e.report(t, an, tok, questMainA, "read_sign", "", body(p, "interactable_id", "quest.hub.lantern_east")); r.code() != string(problem.StepPreconditionFailed) {
		t.Fatalf("interactable of another step: %d %s", r.status, r.raw)
	}
	if r := e.report(t, an, tok, questMainA, "read_sign", "", body(p, "interactable_id", strings.Repeat("x", 65))); r.status != http.StatusUnprocessableEntity ||
		r.code() != string(problem.ValidationFailed) {
		t.Fatalf("65 characters: %d %s", r.status, r.raw)
	}
	if r := e.report(t, an, tok, questMainA, "read_sign", "", body(p, "interactable_id", "quest.hub.welcome_sign")); r.status != http.StatusOK {
		t.Fatalf("go_to: %d %s", r.status, r.raw)
	}
	// F072-AC-11: same key → same answer; same interactable with a new key → counted once.
	k := newKey()
	c1 := e.report(t, an, tok, questMainA, "light_lanterns", k, body(p, "interactable_id", "quest.hub.lantern_east"))
	c2 := e.report(t, an, tok, questMainA, "light_lanterns", k, body(p, "interactable_id", "quest.hub.lantern_east"))
	c3 := e.report(t, an, tok, questMainA, "light_lanterns", "", body(p, "interactable_id", "quest.hub.lantern_east"))
	cnt := func(r resp) any {
		return r.body["step_states"].(map[string]any)["light_lanterns"].(map[string]any)["count"]
	}
	if c1.status != http.StatusOK || c2.header.Get("Idempotent-Replayed") != "true" || cnt(c1) != float64(1) || cnt(c3) != float64(1) {
		t.Fatalf("collect once: %s / %s / %s", c1.raw, c2.raw, c3.raw)
	}
	// A lost presence (Valkey) is a precondition failure; entering again restores it.
	e.presence.lose(true)
	if r := e.report(t, an, tok, questMainA, "light_lanterns", "", body(p, "interactable_id", "quest.hub.lantern_west")); r.code() != string(problem.StepPreconditionFailed) {
		t.Fatalf("lost presence: %d %s", r.status, r.raw)
	}
	e.presence.lose(false)
	if r := e.report(t, an, tok, questMainA, "light_lanterns", "", body(p, "interactable_id", "quest.hub.lantern_west")); stepStatus(r, "light_lanterns") != "completed" {
		t.Fatalf("collect done: %d %s", r.status, r.raw)
	}
	done := e.report(t, an, tok, questMainA, "report_cloudy", "", body(p, "npc_id", "npc.cloudy"))
	if done.status != http.StatusOK || done.body["status"] != "completed" || e.events(ctx, t, "quest.completed") != 1 {
		t.Fatalf("completed: %d %s", done.status, done.raw)
	}
}

func anyStrings(v any) []string {
	var out []string
	if l, ok := v.([]string); ok {
		return l
	}
	if l, ok := v.([]any); ok {
		for _, x := range l {
			out = append(out, x.(string))
		}
	}
	return out
}

// F072-AC-12, AC-13 (T022, TC-WLD-006): a discover step advances from world.discovery_found only (a report is 409); the
// frame steps never produce learning evidence: no learning event in the outbox of module game and quest.completed of a
// quest without objectives carries an empty objective_ids (module learning records nothing).
func TestQuestDiscoverStep(t *testing.T) {
	ctx := context.Background()
	e := setupQuests(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	p := e.play(t, an, "main_hub")
	e.completeMainA(t, p)
	tok := p.d.token
	if r := e.accept(t, an, tok, questMainB, ""); r.status != http.StatusOK {
		t.Fatalf("chained quest after its prerequisite: %d %s", r.status, r.raw)
	}
	for _, rep := range []struct{ step, k, v string }{{"meet_postie", "npc_id", "npc.postie"}, {"check_board", "interactable_id", "board.school_assignments"}} {
		if r := e.report(t, an, tok, questMainB, rep.step, "", body(p, rep.k, rep.v)); r.status != http.StatusOK {
			t.Fatalf("%s: %d %s", rep.step, r.status, r.raw)
		}
	}
	if r := e.report(t, an, tok, questMainB, "find_spark_tree", "", body(p, "interactable_id", "discovery.hub.spark_tree")); r.code() != string(problem.StepPreconditionFailed) {
		t.Fatalf("report of a discover step: %d %s", r.status, r.raw)
	}
	e.discovered(ctx, t, an, "learning_zone.math", "discovery.math.old_well")
	if _, steps := e.progressRow(ctx, t, an, questMainB); steps["find_spark_tree"].Count != 0 {
		t.Fatalf("discovery of another area counted: %v", steps)
	}
	e.discovered(ctx, t, an, "main_hub", "discovery.hub.spark_tree")
	e.discovered(ctx, t, an, "main_hub", "discovery.hub.spark_tree")
	if _, steps := e.progressRow(ctx, t, an, questMainB); steps["find_spark_tree"].Status != "completed" || steps["report_cloudy"].Status != "active" {
		t.Fatalf("discover: %v", steps)
	}
	if r := e.report(t, an, tok, questMainB, "report_cloudy", "", body(p, "npc_id", "npc.cloudy")); r.body["status"] != "completed" {
		t.Fatalf("complete: %d %s", r.status, r.raw)
	}
	for _, typ := range []string{"question.answered", "activity.graded", "activity.submitted", "learning_objective.progressed"} {
		if n := e.events(ctx, t, typ); n != 0 {
			t.Fatalf("%s written by module game: %d", typ, n)
		}
	}
	for _, c := range e.payloads(ctx, t, "quest.completed") {
		if o, _ := c["objective_ids"].([]any); len(o) != 0 {
			t.Fatalf("frame quest with objectives: %v", c)
		}
	}
}
