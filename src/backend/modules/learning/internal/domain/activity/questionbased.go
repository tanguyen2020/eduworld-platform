package activity

import (
	"math"
	"slices"
	"strconv"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// QuestionBased implements the plugin contract shared by the question-based types (quiz, interactive_exercise): the
// questions come from the question bank and are graded by its scorer; the plugin restricts the question types, builds
// the client view of the config and sums the item scores (ACTIVITY_ENGINE §7.2).
type QuestionBased struct {
	Meta Info
	// Types are the question types of the type (the schema enum of allowed_question_types and its default).
	Types []string
}

var _ Plugin = QuestionBased{}

// Info implements Plugin.
func (q QuestionBased) Info() Info { return q.Meta }

// AllowedQuestionTypes implements Plugin: allowed_question_types or every type of the activity type.
func (q QuestionBased) AllowedQuestionTypes(c Config) []string {
	if len(c.Content.AllowedQuestionTypes) > 0 {
		return slices.Clone(c.Content.AllowedQuestionTypes)
	}
	return slices.Clone(q.Types)
}

// ValidateConfig implements Plugin: allowed types of the activity type, a selection restricted to them, a timed quiz
// consistent with the activity time limit is checked by the use case, difficulty bounds ordered.
func (q QuestionBased) ValidateConfig(c Config) contentmodel.Violations {
	var out contentmodel.Violations
	add := func(f, r string) { out = append(out, contentmodel.Violation{Field: f, Reason: r}) }
	for i, t := range c.Content.AllowedQuestionTypes {
		if !slices.Contains(q.Types, t) {
			add("/config/content/allowed_question_types/"+strconv.Itoa(i), "is not a question type of "+string(q.Meta.Type))
		}
	}
	src := c.Content.QuestionSource
	if src.Selection != nil {
		s := src.Selection
		for i, t := range s.QuestionTypes {
			if !slices.Contains(q.AllowedQuestionTypes(c), t) {
				add("/config/content/question_source/selection/question_types/"+strconv.Itoa(i), "is not an allowed question type")
			}
		}
		if s.DifficultyMin != nil && s.DifficultyMax != nil && *s.DifficultyMin > *s.DifficultyMax {
			add("/config/content/question_source/selection/difficulty_min", "is greater than difficulty_max")
		}
	}
	seen := map[string]bool{}
	for i, r := range src.Questions {
		k := r.QuestionVersionID.String()
		if seen[k] {
			add("/config/content/question_source/questions/"+strconv.Itoa(i), "is listed twice")
		}
		seen[k] = true
	}
	if c.Feedback.Timing == "after_close" {
		add("/config/feedback/timing", "after_close needs an assignment or exam window (F041, F042)")
	}
	return out
}

// BuildRenderPayload implements Plugin: what the client needs to drive the activity (navigation, timer, retry and
// hint switches, feedback timing). The question source, evaluation and pass threshold stay on the server.
func (q QuestionBased) BuildRenderPayload(v Version) map[string]any {
	c := v.Config
	out := map[string]any{
		"navigation":           firstNonEmpty(c.Interaction.Navigation, "sequential"),
		"allow_unscored_retry": Bool(c.Interaction.AllowUnscoredRetry, true),
		"hints_enabled":        Bool(c.Interaction.HintsEnabled, true),
		"feedback_timing":      c.Feedback.Timing,
	}
	if c.Interaction.PerItemTimeLimit != nil {
		out["per_item_time_limit_seconds"] = *c.Interaction.PerItemTimeLimit
	}
	if c.Content.Timed != nil {
		out["timed"] = map[string]any{"total_seconds": c.Content.Timed.TotalSeconds, "show_timer": Bool(c.Content.Timed.ShowTimer, true)}
	}
	return out
}

// Evaluate implements Plugin: score = Σ final item score, max_score = Σ item max score; passed against pass_threshold
// (ratio of max_score) when set. Hints and rapid guesses never change the score (ACTIVITY_ENGINE §7.2).
func (q QuestionBased) Evaluate(v Version, items []ItemResult) Result {
	r := Result{Strategies: slices.Clone(v.Config.Evaluation.Strategies)}
	for _, it := range items {
		r.MaxScore += it.MaxScore
		if it.Answered {
			r.Score += it.Score
			if it.IsCorrect {
				r.CorrectCount++
			}
		}
	}
	r.Score, r.MaxScore = Round2(r.Score), Round2(r.MaxScore)
	if th := v.Config.Evaluation.PassThreshold; th != nil && r.MaxScore > 0 {
		passed := r.Score/r.MaxScore >= *th
		r.Passed = &passed
	}
	if len(r.Strategies) == 0 {
		r.Strategies = []string{"auto"}
	}
	return r
}

// CompletionRule implements Plugin (default: completed when submitted).
func (q QuestionBased) CompletionRule(v Version) CompletionRule {
	if v.Config.CompletionRule.Mode == "" {
		return CompletionRule{Mode: "submit"}
	}
	return v.Config.CompletionRule
}

// Round2 rounds to the numeric(7,2) of the attempt columns.
func Round2(f float64) float64 { return math.Round(f*100) / 100 }

func firstNonEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
