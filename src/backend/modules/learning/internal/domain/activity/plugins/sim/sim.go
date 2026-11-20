// Package sim is the plugin of activity_type simulation (F066, SIMULATION_ENGINE, ADR-0052): a packaged Godot template
// parameterized by the CMS, runs verified on the server by the deterministic checker of the template
// (domain/activity/simulation), conclusion questions graded by the question bank scorer. The run command and the
// goal evidence are handled by the attempt engine for Simulated plugins.
package sim

import (
	"slices"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/simulation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// SchemaURL is the $id of specs/schemas/activities/simulation.schema.json.
const SchemaURL = "https://schemas.eduworld.vn/content/activities/simulation.schema.json"

// MinClientVersion is the oldest game client with the simulation renderer (F066).
const MinClientVersion = "0.1.0"

// QuestionTypes are the auto-graded types of a conclusion question (kind simulation_question; ADR-0072).
var QuestionTypes = []string{"single_choice", "multiple_choice", "true_false", "fill_in_blank", "short_answer", "numeric", "matching",
	"ordering", "drag_drop", "image_based", "interactive"}

// Plugin is the simulation plugin.
type Plugin struct{}

var (
	_ activity.Plugin    = Plugin{}
	_ activity.Simulated = Plugin{}
)

// New returns the simulation plugin.
func New() activity.Plugin { return Plugin{} }

// Info implements activity.Plugin.
func (Plugin) Info() activity.Info {
	return activity.Info{Type: activity.Simulation, Label: "Simulation", SchemaURL: SchemaURL, SchemaVersion: 1,
		Strategies: []string{"deterministic_check", "auto"}, MinClientVersion: MinClientVersion, Status: activity.StatusEnabled}
}

func config(c activity.Config) (simulation.Config, error) { return simulation.Parse(c.Raw) }

// ValidateConfig implements activity.Plugin (F066-AC-09).
func (Plugin) ValidateConfig(c activity.Config) contentmodel.Violations {
	sc, err := config(c)
	if err != nil {
		return contentmodel.Violations{{Field: "/config/content", Reason: "cannot be read"}}
	}
	return sc.Validate()
}

// AllowedQuestionTypes implements activity.Plugin.
func (Plugin) AllowedQuestionTypes(activity.Config) []string { return slices.Clone(QuestionTypes) }

// BuildRenderPayload implements activity.Plugin: without the goal checks; seed and runs_used are added by the engine.
func (Plugin) BuildRenderPayload(v activity.Version) map[string]any {
	sc, err := config(v.Config)
	if err != nil {
		return map[string]any{}
	}
	d, _ := sc.Template()
	return simulation.RenderPayload(sc, d, 0, 0)
}

// Evaluate implements activity.Plugin for the questions only (EvaluateRuns adds the goals).
func (p Plugin) Evaluate(v activity.Version, items []activity.ItemResult) activity.Result {
	r, _ := p.EvaluateRuns(v, items, nil, false)
	return r
}

// EvaluateRuns implements activity.Simulated (F066-AC-15, AC-16; Clarifications 2026-11-20 [Proposed]): one point per
// goal met in the last verified run plus the conclusion questions; completed when that run met every goal, or — for a
// template without checker — when the attempt has a run at all. passed follows min_score_ratio, else pass_threshold.
func (Plugin) EvaluateRuns(v activity.Version, items []activity.ItemResult, last *activity.RunResult, anyRun bool) (activity.Result, bool) {
	r := activity.QuestionBased{}.Evaluate(v, items)
	sc, _ := config(v.Config)
	completed := false
	if last != nil && last.Verifiable {
		r.MaxScore += float64(len(sc.Content.Goals))
		all := len(last.GoalsMet) > 0
		for _, met := range last.GoalsMet {
			if met {
				r.Score++
			}
			all = all && met
		}
		completed = all
	} else if d, ok := sc.Template(); ok && d.CheckerAvailable {
		r.MaxScore += float64(len(sc.Content.Goals))
	} else {
		completed = anyRun
	}
	var th *float64
	if v.Config.CompletionRule.MinScoreRatio != nil {
		th = v.Config.CompletionRule.MinScoreRatio
	} else if v.Config.Evaluation.PassThreshold != nil {
		th = v.Config.Evaluation.PassThreshold
	}
	if th != nil && r.MaxScore > 0 {
		passed := r.Score/r.MaxScore >= *th
		r.Passed = &passed
	}
	if len(v.Config.Evaluation.Strategies) == 0 {
		r.Strategies = []string{"deterministic_check", "auto"}
	}
	return r, completed
}

// CompletionRule implements activity.Plugin (default goals_met).
func (Plugin) CompletionRule(v activity.Version) activity.CompletionRule {
	r := v.Config.CompletionRule
	if r.Mode == "" {
		r.Mode = "goals_met"
	}
	return r
}

// SimulationQuestions implements activity.Simulated.
func (Plugin) SimulationQuestions(v activity.Version) ([]uuid.UUID, error) {
	sc, err := config(v.Config)
	if err != nil {
		return nil, err
	}
	return sc.QuestionIDs(), nil
}

// Seed implements activity.Simulated (research R5): fixed → from the version id; per_attempt → the random seed of the
// attempt within simulation.MaxSeed.
func (Plugin) Seed(v activity.Version, random int64) int64 {
	sc, err := config(v.Config)
	if err == nil && sc.Seeding() == simulation.SeedFixed {
		return simulation.FixedSeed(v.VersionID)
	}
	return simulation.ClientSeed(random)
}
