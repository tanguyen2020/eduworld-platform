//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
)

// F073-AC-26 (T038, learning part): a timed_challenge delivers the whole batch of items at start (prefetch_items, not
// resumable) and still grades every answer on the server; a science_simulation item is an interactive question whose
// final state is graded by the interactive scorer (widget validator, action log replayed) like any response.
func TestMinigameExtendedTypes(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil)
	right, wrong := correctOption()

	timed := mgSnapshot(uuid.New(), 1, "timed_challenge", selectionSource(s.qc.subject, "single_choice", "true_false", "numeric"), 3, func(m map[string]any) {
		m["time_limit_seconds"] = 60
	})
	c.project(ctx, t, timed)
	a, err := c.mgStart(ctx, hsAn, timed, learningapi.AttemptModeMinigame)
	if err != nil || a.QuestionCount != 3 {
		t.Fatalf("timed_challenge: %+v %v", a, err)
	}
	g := c.getAttempt(t, hsAn, a.AttemptId)
	if len(g.items("items")) != 3 || g.str("render", "render_payload", "config", "prefetch_items") != "true" ||
		g.str("render", "render_payload", "config", "resumable") != "false" || len(answerKeysIn(g.raw)) > 0 {
		t.Fatalf("batch of a timed_challenge: %s", g.raw)
	}
	for i, opt := range []string{right, wrong, right} {
		r := must(t, c.mgAnswer(t, hsAn, a.AttemptId, i+1, opt, map[string]any{"is_correct": true}), http.StatusOK, "timed answer")
		if r.str("is_correct") != map[bool]string{true: "true", false: "false"}[opt == right] {
			t.Fatalf("answer %d graded by the server: %s", i+1, r.raw)
		}
	}

	// science_simulation: one interactive balance_scale question of tenant A.
	sim := advancedBodies(uuid.NewString())["interactive"]
	id := must(t, c.createQ(t, qlcmA, s.qc.body(sim, nil)), http.StatusCreated, "interactive").str("content_id")
	c.publishQuestion(ctx, t, id, 1)
	cfg := mgSnapshot(uuid.New(), 1, "science_simulation", selectionSource(s.qc.subject, "interactive"), 1, nil)
	c.project(ctx, t, cfg)
	play := func() uuid.UUID {
		p, err := c.mgStart(ctx, hsAn, cfg, learningapi.AttemptModeMinigame)
		if err != nil || p.QuestionCount != 1 {
			t.Fatalf("science_simulation: %+v %v", p, err)
		}
		if g := c.getAttempt(t, hsAn, p.AttemptId); g.items("items")[0]["type"] != "interactive" ||
			g.str("render", "render_payload", "minigame_type") != "science_simulation" {
			t.Fatalf("interactive item: %s", g.raw)
		}
		return p.AttemptId
	}
	// Final state: 8 on the right pan (left 5 + 3), the log replays to it.
	state := func(w int) map[string]any {
		return map[string]any{"state": map[string]any{"right_weights": []int{w}},
			"action_log": []any{map[string]any{"t_ms": 0, "action": "place", "args": map[string]any{"w": w}}}}
	}
	ok := play()
	if r := must(t, c.answer(t, hsAn, ok.String(), 1, state(8), map[string]any{"is_correct": false}), http.StatusOK, "balanced"); r.str("is_correct") != "true" ||
		mgNum(r, "score") != 1 {
		t.Fatalf("balanced state: %s", r.raw)
	}
	ko := play()
	if r := must(t, c.answer(t, hsAn, ko.String(), 1, state(4), map[string]any{"is_correct": true}), http.StatusOK, "unbalanced"); r.str("is_correct") != "false" ||
		mgNum(r, "game_points") != 0 {
		t.Fatalf("unbalanced state: %s", r.raw)
	}
	// A state the log does not replay to is a forged response: score 0 (F023-AC-11).
	forged := map[string]any{"state": map[string]any{"right_weights": []int{8}}, "action_log": []any{}}
	fp := play()
	if r := must(t, c.answer(t, hsAn, fp.String(), 1, forged, nil), http.StatusOK, "forged state"); r.str("is_correct") != "false" {
		t.Fatalf("forged state: %s", r.raw)
	}
}
