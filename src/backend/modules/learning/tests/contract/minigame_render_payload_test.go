package contract

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/plugins/minigame"
)

// minigameLeaks are the keys a mini-game payload never carries (F073-AC-07, AC-MG-001, R6): answers, correctness,
// scoring rules, option feedback, explanations, the question source and the hidden game rules.
var minigameLeaks = []string{"answer_key", "correct", "correct_option_id", "correct_option_ids", "accepted_answers", "scoring_rule", "feedback",
	"option_feedback", "explanation", "hints", "question_source", "question_types", "wrong_penalty", "max_bonus", "completion_rule",
	"result_calculation", "reward_rule_id", "limits"}

func minigameSnapshot(mt string) json.RawMessage {
	b, _ := json.Marshal(map[string]any{"schema_version": 1, "minigame_config_id": uuid.NewString(), "minigame_config_version_id": uuid.NewString(),
		"code": "math.g3.contract", "version": 1, "status": "published", "owner_scope": "platform", "minigame_type": mt,
		"name": map[string]string{"vi": "Trò chơi", "en": "Game"}, "instructions": map[string]string{"vi": "Chơi", "en": "Play"},
		"grade_levels": []int{3, 7}, "subject_id": uuid.NewString(),
		"question_source": map[string]any{"kind": "selection_rule", "rule": map[string]any{"question_types": minigame.Allowed(mt)}},
		"difficulty":      2, "time_limit_seconds": 120, "per_item_time_seconds": 20, "question_count": 6,
		"scoring": map[string]any{"points_per_correct": 100, "wrong_penalty": 5,
			"time_bonus":   map[string]any{"enabled": true, "max_bonus": 50, "full_bonus_within_ms": 5000},
			"streak_bonus": map[string]any{"enabled": true, "every_n_correct": 3, "bonus": 30}},
		"result_calculation": map[string]any{"learning_score": "sum_question_scores", "game_score": "points_plus_bonus"},
		"feedback_policy":    map[string]any{"timing": "immediate", "feedback_display": "end_of_round"},
		"completion_rule":    map[string]any{"type": "min_correct_ratio", "threshold": 0.7},
		"limits":             map[string]any{"max_plays_per_day": 5, "retry_policy": "new_questions"},
		"reward_rule_id":     uuid.NewString(),
		"presentation":       map[string]any{"scene_variant": mt + ".default", "theme": "garden", "background_music_id": "music_light", "mascot_npc_id": "npc.ong_so"}})
	return b
}

func publicSchemas(t *testing.T) openapi3.Schemas {
	t.Helper()
	_, file, _, _ := runtime.Caller(0) // src/backend/modules/learning/tests/contract → repository root
	path := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "..", "specs", "openapi", "eduworld-api.v1.yaml")
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	doc, err := loader.LoadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return doc.Components.Schemas
}

// F073-AC-07 (T018, TC-ACT-001, TC-LRN-007, SC-001): for the 11 mini-game types and both grade bands, the render payload
// {minigame_type, minigame_config_version_id, config} holds no answer, correctness, scoring rule, option feedback nor
// hidden game rule anywhere, and config is a valid closed MiniGameDisplayConfig of the public API (additionalProperties
// false); time_bonus_enabled is false for grades 1–5.
func TestMinigameRenderPayloadContract(t *testing.T) {
	schemas := publicSchemas(t)
	display, typ := schemas["MiniGameDisplayConfig"], schemas["MiniGameType"]
	if display == nil || typ == nil {
		t.Fatal("MiniGameDisplayConfig / MiniGameType missing in the public API")
	}
	p := minigame.New()
	for _, mt := range minigame.Types {
		for _, grade := range []int{2, 5, 6, 9} {
			g := grade
			v := activity.Version{Type: activity.Minigame, StudentGrade: &g, Config: activity.Config{Feedback: activity.Feedback{Timing: "immediate"},
				Pinned: &activity.Pinned{VersionID: uuid.New(), Snapshot: minigameSnapshot(mt)}}}
			b, _ := json.Marshal(p.BuildRenderPayload(v))
			var doc map[string]any
			if err := json.Unmarshal(b, &doc); err != nil {
				t.Fatal(err)
			}
			keys := map[string]bool{}
			keysOf(doc, keys)
			for _, k := range minigameLeaks {
				if keys[k] {
					t.Fatalf("%s grade %d: payload leaks %q: %s", mt, grade, k, b)
				}
			}
			if err := typ.Value.VisitJSON(doc["minigame_type"]); err != nil {
				t.Fatalf("%s: minigame_type: %v", mt, err)
			}
			cfg, _ := doc["config"].(map[string]any)
			if err := display.Value.VisitJSON(cfg); err != nil {
				t.Fatalf("%s grade %d: MiniGameDisplayConfig: %v (%s)", mt, grade, err, b)
			}
			if bonus := cfg["scoring"].(map[string]any)["time_bonus_enabled"]; bonus != (grade > 5) {
				t.Fatalf("%s grade %d: time_bonus_enabled %v", mt, grade, bonus)
			}
		}
	}
}
