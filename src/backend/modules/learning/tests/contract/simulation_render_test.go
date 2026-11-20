package contract

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/attemptview"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/plugins/sim"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/attempt"
)

// F066-AC-10 (T024, TC-ACT-001): the render payload of simulation, with the seed and runs_used of the attempt, matches
// simulation.schema.json#/$defs/render (closed) and never holds a goal check, a question version or an answer; the
// fixed seed is stable per version; Evaluate is idempotent.
func TestSimulationRenderPayload(t *testing.T) {
	raw := `{"content":{"simulation_kind":"physics","template_code":"physics.lever","min_template_version":1,
	 "parameters":{"arm_length_m":1,"slots_per_side":4,"mass_1_kg":2,"mass_2_kg":1},
	 "goals":[{"goal_id":"balance","description":{"vi":"Cân bằng đòn bẩy"},"check":{"metric":"balanced","operator":"eq","value":1}}],
	 "steps":[{"step_id":"place","instruction":{"vi":"Đặt vật"}}],
	 "questions":[{"question_version_id":"` + uuid.NewString() + `"}],"seed_policy":"fixed"},
	 "interaction":{"max_runs":5},
	 "evaluation":{"strategies":["deterministic_check","auto"],"max_score":2},"feedback":{"timing":"immediate"},
	 "completion_rule":{"mode":"goals_met"}}`
	cfg, err := activity.ParseConfig(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	p := sim.New()
	v := activity.Version{ActivityID: uuid.New(), VersionID: uuid.New(), VersionNo: 1, Type: activity.Simulation, Config: cfg}
	if vs := p.ValidateConfig(cfg); len(vs) != 0 {
		t.Fatalf("valid config refused: %v", vs)
	}
	seed := p.(activity.Simulated).Seed(v, 1<<62)
	if again := p.(activity.Simulated).Seed(v, 7); seed != again || seed < 0 || seed > 1<<53 {
		t.Fatalf("fixed seed: %d %d", seed, again)
	}
	view := dto.AttemptView{Render: &dto.ActivityRenderView{RenderPayload: p.BuildRenderPayload(v)}}
	view = attemptview.WithSimulation(view, attempt.Attempt{ActivityType: "simulation", Seed: seed}, 2)
	b, _ := json.Marshal(view.Render.RenderPayload)
	for _, leak := range append([]string{"check", "operator", "question_version_id", "metric"}, answerKeys...) {
		if strings.Contains(string(b), `"`+leak+`"`) {
			t.Fatalf("render payload leaks %q: %s", leak, b)
		}
	}
	inst, _ := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err := renderSchema(t, "simulation.schema.json").Validate(inst); err != nil {
		t.Fatalf("render payload does not match $defs/render: %v\n%s", err, b)
	}
	items := []activity.ItemResult{{Position: 1, MaxScore: 1, Score: 1, IsCorrect: true, Answered: true}}
	r1, done := p.(activity.Simulated).EvaluateRuns(v, items, &activity.RunResult{Verifiable: true, GoalsMet: []bool{true}}, true)
	r2, _ := p.(activity.Simulated).EvaluateRuns(v, items, &activity.RunResult{Verifiable: true, GoalsMet: []bool{true}}, true)
	if !done || r1.Score != 2 || r1.MaxScore != 2 || r1.Score != r2.Score {
		t.Fatalf("evaluate: %+v %+v %v", r1, r2, done)
	}
	if r, done := p.(activity.Simulated).EvaluateRuns(v, items, nil, false); done || r.MaxScore != 2 || r.Score != 1 {
		t.Fatalf("no verified run: %+v %v", r, done)
	}
}
