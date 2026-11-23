package minigame_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/plugins"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/plugins/minigame"
)

// leaks are the keys that never reach a client (F073-AC-07, ADR-0016, R6): answers, correctness, scoring rules, option
// feedback, explanations, the question source and the bonus amounts of the game rules.
var leaks = []string{"answer_key", "correct", "correct_option_id", "correct_option_ids", "accepted_answers", "scoring_rule", "feedback",
	"option_feedback", "explanation", "hints", "question_source", "rule", "question_types", "wrong_penalty", "max_bonus", "bonus",
	"completion_rule", "result_calculation", "pass_threshold", "evaluation", "reward_rule_id"}

// displayKeys are the properties of MiniGameDisplayConfig (additionalProperties: false in the public API).
var displayKeys = []string{"name", "instructions", "grade_levels", "question_count", "time_limit_seconds", "per_item_time_seconds",
	"feedback_timing", "feedback_display", "show_explanation", "show_correct_answer", "scoring", "presentation", "resumable", "prefetch_items"}

func keysOf(v any, out map[string]bool) {
	switch t := v.(type) {
	case map[string]any:
		for k, e := range t {
			out[k] = true
			keysOf(e, out)
		}
	case []any:
		for _, e := range t {
			keysOf(e, out)
		}
	}
}

func wrapperConfig(t *testing.T, raw string) activity.Config {
	t.Helper()
	c, err := activity.ParseConfig(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// F073-AC-14 (T024, TC-ACT-001): the minigame plugin passes the shared plugin contract for the 11 mini-game types —
// registered, Info complete, ValidateConfig of the activity wrapper, allowed question types inside the compatibility
// table, render payload and display config without answer or scoring data (closed shape), Evaluate deterministic and
// idempotent, a completion rule.
func TestMinigamePluginContract(t *testing.T) {
	var p activity.Plugin
	for _, x := range plugins.All() {
		if x.Info().Type == activity.Minigame {
			p = x
		}
	}
	if p == nil {
		t.Fatal("minigame is not registered in plugins.All")
	}
	if _, ok := p.(activity.GameScored); !ok {
		t.Fatal("minigame is not GameScored")
	}
	info := p.Info()
	if info.MinClientVersion == "" || info.SchemaURL != minigame.SchemaURL || info.SchemaVersion != 1 || len(info.Strategies) == 0 ||
		info.Status != activity.StatusEnabled || !strings.HasPrefix(info.SchemaURL, "https://schemas.eduworld.vn/content/activities/") {
		t.Fatalf("info: %+v", info)
	}
	wrapper := `{"content":{"minigame_config_id":"` + uuid.NewString() + `","minigame_config_version":null},
	 "interaction":{"feedback_display":"end_of_round"},"evaluation":{"strategies":["auto"],"max_score":10},
	 "feedback":{"timing":"immediate"},"completion_rule":{"mode":"graded"}}`
	if vs := p.ValidateConfig(wrapperConfig(t, wrapper)); len(vs) != 0 {
		t.Fatalf("valid wrapper: %v", vs)
	}
	bad := `{"content":{"minigame_config_id":"` + uuid.NewString() + `"},"interaction":{"feedback_display":"end_of_game"},
	 "evaluation":{"strategies":["auto"],"max_score":10},"feedback":{"timing":"after_close"},"completion_rule":{"mode":"graded"}}`
	if vs := p.ValidateConfig(wrapperConfig(t, bad)); len(vs) != 2 {
		t.Fatalf("feedback_display without immediate, after_close: %v", vs)
	}
	if vs := p.ValidateConfig(wrapperConfig(t, `{"content":{},"feedback":{"timing":"immediate"}}`)); len(vs) != 1 {
		t.Fatalf("no config id: %v", vs)
	}
	items := []activity.ItemResult{{Position: 1, MaxScore: 1, Score: 1, IsCorrect: true, Answered: true}, {Position: 2, MaxScore: 1, Answered: true}}
	for _, mt := range minigame.Types {
		t.Run(mt, func(t *testing.T) {
			for _, grade := range []int{3, 7} {
				v := version(snapshot(mt, nil), grade)
				if allowed := p.AllowedQuestionTypes(v.Config); len(allowed) == 0 || !reflect.DeepEqual(allowed, minigame.Allowed(mt)) {
					t.Fatalf("allowed: %v", allowed)
				}
				b, _ := json.Marshal(p.BuildRenderPayload(v))
				var doc map[string]any
				if err := json.Unmarshal(b, &doc); err != nil {
					t.Fatal(err)
				}
				keys := map[string]bool{}
				keysOf(doc, keys)
				for _, k := range leaks {
					if keys[k] {
						t.Fatalf("render payload leaks %q: %s", k, b)
					}
				}
				if doc["minigame_type"] != mt || doc["minigame_config_version_id"] != v.Config.Pinned.VersionID.String() || len(doc) != 3 {
					t.Fatalf("payload: %s", b)
				}
				cfg := doc["config"].(map[string]any)
				for k := range cfg {
					if !slices.Contains(displayKeys, k) {
						t.Fatalf("display config has %q (closed shape): %s", k, b)
					}
				}
				scoring := cfg["scoring"].(map[string]any)
				if len(scoring) != 3 || scoring["time_bonus_enabled"] != (grade > 5) || scoring["streak_bonus_enabled"] != true {
					t.Fatalf("scoring of grade %d: %v", grade, scoring)
				}
				if cfg["prefetch_items"] != (mt == minigame.TimedChallenge) || cfg["resumable"] != (mt != minigame.TimedChallenge) {
					t.Fatalf("prefetch/resumable: %v", cfg)
				}
				first := p.Evaluate(v, items)
				if again := p.Evaluate(v, items); !reflect.DeepEqual(first, again) || first.Score != 1 || first.MaxScore != 2 {
					t.Fatalf("evaluate: %+v / %+v", first, again)
				}
				if p.CompletionRule(v).Mode == "" {
					t.Fatal("completion rule")
				}
			}
		})
	}
	// Before a version is pinned (CMS preview of a wrapper activity): only the wrapped configuration id.
	if payload := p.BuildRenderPayload(activity.Version{Config: wrapperConfig(t, wrapper)}); len(payload) != 1 || payload["minigame_config_id"] == nil {
		t.Fatalf("unpinned payload: %v", payload)
	}
	if len(p.AllowedQuestionTypes(wrapperConfig(t, wrapper))) != 11 {
		t.Fatal("unpinned allowed types")
	}
}
