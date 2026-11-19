package contract

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/plugins"
)

// answerKeys are the fields that never reach a client before grading (F030-AC-02, ADR-0016).
var answerKeys = []string{"answer_key", "scoring_rule", "accepted_answers", "correct_option_id", "correct_option_ids", "explanation",
	"feedback", "option_feedback", "hints", "misconception_tag", "min_response_ms", "question_source", "pass_threshold", "evaluation"}

func fullConfig(t *testing.T) activity.Config {
	t.Helper()
	raw := `{"content":{"question_source":{"kind":"questions","questions":[{"question_version_id":"` + uuid.NewString() + `"}]},
	 "timed":{"total_seconds":60}},"interaction":{"navigation":"free","per_item_time_limit_seconds":30,"hints_enabled":false},
	 "evaluation":{"strategies":["auto"],"max_score":10,"pass_threshold":0.6},
	 "feedback":{"timing":"after_submit","show_explanation":true,"show_correct_answer":false},"completion_rule":{"mode":"graded","min_score_ratio":0.5}}`
	c, err := activity.ParseConfig(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// F030-AC-23 (T039): every registered plugin passes the shared contract — the render payload holds no answer, scoring
// or feedback data, Evaluate is deterministic and idempotent, the completion rule is defined.
func TestPluginContract(t *testing.T) {
	for _, p := range plugins.All() {
		info := p.Info()
		t.Run(string(info.Type), func(t *testing.T) {
			if !info.Type.InRelease() || info.MinClientVersion == "" || !strings.HasPrefix(info.SchemaURL, "https://schemas.eduworld.vn/content/activities/") ||
				len(info.Strategies) == 0 || info.SchemaVersion < 1 {
				t.Fatalf("info: %+v", info)
			}
			v := activity.Version{ActivityID: uuid.New(), VersionID: uuid.New(), VersionNo: 1, Type: info.Type, Config: fullConfig(t)}
			b, _ := json.Marshal(p.BuildRenderPayload(v))
			var doc any
			_ = json.Unmarshal(b, &doc)
			keys := map[string]bool{}
			keysOf(doc, keys)
			for _, k := range answerKeys {
				if keys[k] {
					t.Fatalf("render payload leaks %q: %s", k, b)
				}
			}
			items := []activity.ItemResult{{Position: 1, MaxScore: 1, Score: 1, IsCorrect: true, Answered: true},
				{Position: 2, MaxScore: 2, Score: 0.5, Answered: true}, {Position: 3, MaxScore: 1}}
			first := p.Evaluate(v, items)
			if first.Score != 1.5 || first.MaxScore != 4 || first.CorrectCount != 1 || first.Passed == nil || *first.Passed {
				t.Fatalf("evaluate: %+v", first)
			}
			if again := p.Evaluate(v, items); !reflect.DeepEqual(first, again) {
				t.Fatalf("evaluate is not idempotent: %+v / %+v", first, again)
			}
			if p.CompletionRule(v).Mode == "" {
				t.Fatal("completion rule")
			}
			if len(p.AllowedQuestionTypes(v.Config)) == 0 {
				t.Fatal("allowed question types")
			}
		})
	}
}
