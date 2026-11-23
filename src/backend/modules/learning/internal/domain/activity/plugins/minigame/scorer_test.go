package minigame_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/plugins/minigame"
)

// snapshot is a MiniGameConfigVersion of type mt (the example of specs/examples/minigame.math-g3-matching.json with a
// speed bonus of 50 within 20 s, a streak bonus of 30 every 3 correct answers, a wrong penalty of 10, 180 s for 3 items).
func snapshot(mt string, edit func(m map[string]any)) json.RawMessage {
	m := map[string]any{"schema_version": 1, "minigame_config_id": uuid.NewString(), "minigame_config_version_id": uuid.NewString(),
		"code": "math.g3.test", "version": 1, "status": "published", "owner_scope": "platform", "minigame_type": mt,
		"name": map[string]string{"vi": "Lật thẻ", "en": "Card match"}, "instructions": map[string]string{"vi": "Ghép cặp", "en": "Match the pairs"},
		"grade_levels": []int{3}, "subject_id": uuid.NewString(),
		"question_source": map[string]any{"kind": "selection_rule", "rule": map[string]any{"question_types": minigame.Allowed(mt)}},
		"difficulty":      2, "time_limit_seconds": 180, "per_item_time_seconds": nil, "question_count": 3,
		"scoring": map[string]any{"points_per_correct": 100, "wrong_penalty": 10,
			"time_bonus":   map[string]any{"enabled": true, "max_bonus": 50, "full_bonus_within_ms": 20000},
			"streak_bonus": map[string]any{"enabled": true, "every_n_correct": 3, "bonus": 30}},
		"result_calculation": map[string]any{"learning_score": "sum_question_scores", "game_score": "points_plus_bonus"},
		"feedback_policy":    map[string]any{"timing": "immediate", "show_explanation": true, "show_correct_answer": true},
		"completion_rule":    map[string]any{"type": "min_correct_ratio", "threshold": 0.7},
		"limits":             map[string]any{"max_plays_per_day": 5, "retry_policy": "new_questions", "resumable": true},
		"reward_rule_id":     uuid.NewString(),
		"presentation":       map[string]any{"scene_variant": "matching.market_cards", "theme": "market_day"}}
	if edit != nil {
		edit(m)
	}
	b, _ := json.Marshal(m)
	return b
}

func version(snap json.RawMessage, grade int) activity.Version {
	g := grade
	var gp *int
	if grade > 0 {
		gp = &g
	}
	return activity.Version{VersionID: uuid.New(), Type: activity.Minigame, StudentGrade: gp,
		Config: activity.Config{Feedback: activity.Feedback{Timing: "immediate"}, Pinned: &activity.Pinned{VersionID: uuid.New(), Snapshot: snap}}}
}

func rules(t *testing.T, snap json.RawMessage) minigame.Rules {
	t.Helper()
	r, err := minigame.Parse(snap)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// F073-AC-08, AC-09 (T019 unit part): game points from the server interval and the streak only; grades 1–5 have no
// speed bonus and no wrong penalty whatever the configuration; points_only has no bonus; the score never goes below 0.
func TestScorer(t *testing.T) {
	p := minigame.Plugin{}
	snap := snapshot("matching", nil)
	g7, g3, unknown := version(snap, 7), version(snap, 3), version(snap, 0)
	// Grade 7: 100 + full speed bonus (≤ 20 s) on the first correct answer.
	got := p.ScoreAnswer(g7, activity.GameAnswer{Correct: true, Scored: true, ServerMS: 5000})
	if got.Points != 150 || got.Score != 150 || got.Streak != 1 {
		t.Fatalf("grade 7 fast: %+v", got)
	}
	// Linear between 20 s and 60 s (180 s / 3 items): at 40 s half of 50 = 25.
	if b := rules(t, snap).TimeBonus(40000); b != 25 {
		t.Fatalf("bonus at 40 s: %d", b)
	}
	if b := rules(t, snap).TimeBonus(60000); b != 0 {
		t.Fatalf("bonus at 60 s: %d", b)
	}
	// Third correct answer in a row: streak bonus 30.
	got = p.ScoreAnswer(g7, activity.GameAnswer{Correct: true, Scored: true, ServerMS: 90000, Streak: 2, Score: 200})
	if got.Points != 130 || got.Score != 330 || got.Streak != 3 {
		t.Fatalf("streak: %+v", got)
	}
	// Wrong answer of grade 7: penalty 10, streak reset; never below 0.
	if got = p.ScoreAnswer(g7, activity.GameAnswer{Correct: false, Scored: true, Streak: 3, Score: 330}); got.Points != -10 || got.Score != 320 || got.Streak != 0 {
		t.Fatalf("penalty: %+v", got)
	}
	if got = p.ScoreAnswer(g7, activity.GameAnswer{Correct: false, Scored: true, Score: 4}); got.Points != -4 || got.Score != 0 {
		t.Fatalf("floor 0: %+v", got)
	}
	// Grade 3 (and an unknown grade): no speed bonus, no penalty.
	for name, v := range map[string]activity.Version{"grade 3": g3, "unknown grade": unknown} {
		if got = p.ScoreAnswer(v, activity.GameAnswer{Correct: true, Scored: true, ServerMS: 1}); got.Points != 100 {
			t.Fatalf("%s fast: %+v", name, got)
		}
		if got = p.ScoreAnswer(v, activity.GameAnswer{Correct: false, Scored: true, Score: 100, Streak: 1}); got.Points != 0 || got.Score != 100 || got.Streak != 0 {
			t.Fatalf("%s wrong: %+v", name, got)
		}
		if got = p.ScoreAnswer(v, activity.GameAnswer{Correct: true, Scored: true, ServerMS: 1, Streak: 2, Score: 200}); got.Points != 130 {
			t.Fatalf("%s streak bonus kept: %+v", name, got)
		}
	}
	// An unscored retry changes nothing.
	if got = p.ScoreAnswer(g7, activity.GameAnswer{Correct: true, Scored: false, Streak: 1, Score: 150}); got.Points != 0 || got.Score != 150 || got.Streak != 1 {
		t.Fatalf("unscored: %+v", got)
	}
	// points_only: no bonus at all.
	only := version(snapshot("matching", func(m map[string]any) {
		m["result_calculation"] = map[string]any{"learning_score": "sum_question_scores", "game_score": "points_only"}
	}), 7)
	if got = p.ScoreAnswer(only, activity.GameAnswer{Correct: true, Scored: true, ServerMS: 1, Streak: 2}); got.Points != 100 {
		t.Fatalf("points_only: %+v", got)
	}
}

// F073-AC-10 (T019 unit part): learning score, game score and completion are separate; correct_ratio and the three
// completion rules.
func TestLearningScoreAndCompletion(t *testing.T) {
	p := minigame.Plugin{}
	items := []activity.ItemResult{{Position: 1, MaxScore: 2, Score: 2, IsCorrect: true, Answered: true},
		{Position: 2, MaxScore: 1, Score: 0.5, Answered: true}, {Position: 3, MaxScore: 1}}
	sum := version(snapshot("matching", nil), 3)
	if r := p.Evaluate(sum, items); r.Score != 2.5 || r.MaxScore != 4 || r.CorrectCount != 1 {
		t.Fatalf("sum: %+v", r)
	}
	ratio := version(snapshot("matching", func(m map[string]any) {
		m["result_calculation"] = map[string]any{"learning_score": "correct_ratio", "game_score": "points_plus_bonus"}
	}), 3)
	if r := p.Evaluate(ratio, items); r.Score != 1 || r.MaxScore != 3 || r.CorrectCount != 1 {
		t.Fatalf("correct_ratio: %+v", r)
	}
	if p.CompletionMet(sum, items) { // 1/3 < 0.7
		t.Fatal("min_correct_ratio met")
	}
	all := func(rule map[string]any) activity.Version {
		return version(snapshot("matching", func(m map[string]any) { m["completion_rule"] = rule }), 3)
	}
	if p.CompletionMet(all(map[string]any{"type": "complete_all"}), items) || !p.CompletionMet(all(map[string]any{"type": "complete_all"}), items[:2]) {
		t.Fatal("complete_all")
	}
	if !p.CompletionMet(all(map[string]any{"type": "min_learning_score", "threshold": 2.5}), items) ||
		p.CompletionMet(all(map[string]any{"type": "min_learning_score", "threshold": 3}), items) {
		t.Fatal("min_learning_score")
	}
}

// Snapshots the plugin cannot play are refused (the projection DLQs them).
func TestParseRefuses(t *testing.T) {
	for name, edit := range map[string]func(m map[string]any){
		"type":       func(m map[string]any) { m["minigame_type"] = "drag_drop" },
		"count":      func(m map[string]any) { m["question_count"] = 51 },
		"grades":     func(m map[string]any) { m["grade_levels"] = []int{} },
		"name en":    func(m map[string]any) { m["name"] = map[string]string{"vi": "x"} },
		"timing":     func(m map[string]any) { m["feedback_policy"] = map[string]any{"timing": "later"} },
		"completion": func(m map[string]any) { m["completion_rule"] = map[string]any{"type": "min_correct_ratio"} },
		"source":     func(m map[string]any) { m["question_source"] = map[string]any{"kind": "question_set"} },
		"learning": func(m map[string]any) {
			m["result_calculation"] = map[string]any{"learning_score": "max", "game_score": "points_only"}
		},
	} {
		if _, err := minigame.Parse(snapshot("matching", edit)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}
