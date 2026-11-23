package minigame

import (
	"slices"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// SchemaURL is the $id of specs/schemas/activities/minigame.schema.json.
const SchemaURL = "https://schemas.eduworld.vn/content/activities/minigame.schema.json"

// MinClientVersion is the oldest game client with the minigame renderer (MiniGameHost + MiniGameRegistry, F073).
const MinClientVersion = "0.1.0"

// Plugin is the minigame plugin.
type Plugin struct{}

var (
	_ activity.Plugin     = Plugin{}
	_ activity.GameScored = Plugin{}
)

// New returns the minigame plugin.
func New() activity.Plugin { return Plugin{} }

// Info implements activity.Plugin.
func (Plugin) Info() activity.Info {
	return activity.Info{Type: activity.Minigame, Label: "Mini-game", SchemaURL: SchemaURL, SchemaVersion: 1, Strategies: []string{"auto"},
		MinClientVersion: MinClientVersion, Status: activity.StatusEnabled}
}

// RulesOf returns the configuration pinned by an attempt (false outside an attempt or for an unreadable snapshot).
func RulesOf(c activity.Config) (Rules, bool) {
	if c.Pinned == nil {
		return Rules{}, false
	}
	r, err := Parse(c.Pinned.Snapshot)
	return r, err == nil
}

// ValidateConfig implements activity.Plugin for a minigame learning activity (activities/minigame.schema.json): the
// wrapped configuration is set, feedback_display only with immediate feedback, no after_close outside an assignment or
// exam window. The configuration and the question set override are checked against the projection by the use case
// (F073-AC-15).
func (Plugin) ValidateConfig(c activity.Config) contentmodel.Violations {
	var out contentmodel.Violations
	ac, err := ParseActivity(c.Raw)
	if err != nil {
		return contentmodel.Violations{{Field: "/config/content/minigame_config_id", Reason: "is required", Required: true}}
	}
	if ac.MinigameConfigVersion != nil && *ac.MinigameConfigVersion < 1 {
		out = append(out, contentmodel.Violation{Field: "/config/content/minigame_config_version", Reason: "must be at least 1"})
	}
	if ac.FeedbackDisplay != "" && c.Feedback.Timing != "immediate" {
		out = append(out, contentmodel.Violation{Field: "/config/interaction/feedback_display", Reason: "needs feedback timing immediate"})
	}
	if c.Feedback.Timing == "after_close" {
		out = append(out, contentmodel.Violation{Field: "/config/feedback/timing", Reason: "after_close needs an assignment or exam window (F041, F042)"})
	}
	return out
}

// AllowedQuestionTypes implements activity.Plugin: the column of the compatibility table of the pinned configuration,
// every auto-graded type before a configuration is pinned.
func (Plugin) AllowedQuestionTypes(c activity.Config) []string {
	if r, ok := RulesOf(c); ok {
		return Allowed(r.Type)
	}
	return slices.Clone(AutoGraded)
}

func display(v activity.Version) Display {
	d := Display{Timing: v.Config.Feedback.Timing, ShowExplanation: v.Config.Feedback.ShowExplanation,
		ShowCorrectAnswer: v.Config.Feedback.ShowCorrectAnswer, TimeLimitSeconds: v.TimeLimitSeconds, Grade: v.StudentGrade}
	if ac, err := ParseActivity(v.Config.Raw); err == nil {
		d.FeedbackDisplay = ac.FeedbackDisplay
	}
	return d
}

// BuildRenderPayload implements activity.Plugin: {minigame_type, minigame_config_version_id, config} with the display
// configuration of the pinned version (F073-AC-07); before a version is pinned only the wrapped configuration id.
func (Plugin) BuildRenderPayload(v activity.Version) map[string]any {
	r, ok := RulesOf(v.Config)
	if !ok {
		if ac, err := ParseActivity(v.Config.Raw); err == nil {
			return map[string]any{"minigame_config_id": ac.MinigameConfigID}
		}
		return map[string]any{}
	}
	return r.RenderPayload(v.Config.Pinned.VersionID, display(v))
}

// Evaluate implements activity.Plugin: the learning score of result_calculation (sum_question_scores, or correct_ratio
// as correct items over items) from the final item results — never the game score; passed against pass_threshold of
// a learning activity when set. Idempotent.
func (Plugin) Evaluate(v activity.Version, items []activity.ItemResult) activity.Result {
	r := activity.QuestionBased{}.Evaluate(v, items)
	rules, ok := RulesOf(v.Config)
	if !ok || rules.ResultCalculation.LearningScore != LearningCorrectRatio {
		return r
	}
	r.Score, r.MaxScore, r.CorrectCount = rules.LearningScore(items)
	r.Passed = nil
	if th := v.Config.Evaluation.PassThreshold; th != nil && r.MaxScore > 0 {
		passed := r.Score/r.MaxScore >= *th
		r.Passed = &passed
	}
	return r
}

// CompletionRule implements activity.Plugin (the completion of a play is CompletionMet; the activity is completed when
// graded).
func (Plugin) CompletionRule(v activity.Version) activity.CompletionRule {
	if v.Config.CompletionRule.Mode == "" {
		return activity.CompletionRule{Mode: "graded"}
	}
	return v.Config.CompletionRule
}

// ScoreAnswer implements activity.GameScored with the rules of the pinned configuration (MinigameScorer).
func (Plugin) ScoreAnswer(v activity.Version, in activity.GameAnswer) activity.GamePoints {
	r, ok := RulesOf(v.Config)
	if !ok {
		return activity.GamePoints{Score: max(in.Score, 0), Streak: max(in.Streak, 0)}
	}
	return r.Points(in, v.StudentGrade)
}

// CompletionMet implements activity.GameScored (completion_rule of the pinned configuration; every item answered
// without one).
func (Plugin) CompletionMet(v activity.Version, items []activity.ItemResult) bool {
	r, ok := RulesOf(v.Config)
	if !ok {
		r = Rules{CompletionRule: CompletionRule{Type: CompleteAll}}
	}
	return r.CompletionMet(items)
}
