// Package minigame is the mini-game part of module game (F073, MINIGAME_ENGINE §1–§3, §6): the config of a version as the
// game reads it (start, display config, play limits), the publish rules that need no database and the reward
// eligibility of a play. A config holds no question (M-1): the questions come from the Question Bank of module learning
// through question_source; the compatibility of question types and the grading belong to module learning (plugin
// minigame). No I/O.
package minigame

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
)

// Types are the 11 minigame_type values of minigame-config.schema.json (MINIGAME_ENGINE §2).
var Types = []string{"multiple_choice", "true_false", "fill_in_blank", "drag_and_drop", "matching", "ordering", "calculation", "puzzle",
	"word_game", "science_simulation", "timed_challenge"}

// TimedChallenge is the type whose items are prefetched and that never resumes.
const TimedChallenge = "timed_challenge"

// Context types of a play (game.minigame_plays.context_type).
const (
	ContextFreePlay   = "free_play"
	ContextQuestStep  = "quest_step"
	ContextAssignment = "assignment"
)

// Play statuses.
const (
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusExpired    = "expired"
	StatusAbandoned  = "abandoned"
)

// ErrTranslationIncomplete is a name or instructions without vi or en (TRANSLATION_INCOMPLETE, F073-AC-05).
var ErrTranslationIncomplete = errors.New("minigame: name and instructions need vi and en")

// ErrInvalid is a config the release refuses (VALIDATION_FAILED); FieldError says which field.
var ErrInvalid = errors.New("minigame: invalid config")

// FieldError is a validation failure of one field (path, no value).
type FieldError struct {
	Field  string
	Reason string
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Reason }
func (e *FieldError) Unwrap() error { return ErrInvalid }

// Text is a bilingual text.
type Text struct {
	VI string `json:"vi"`
	EN string `json:"en"`
}

// Complete reports both languages.
func (t Text) Complete() bool { return strings.TrimSpace(t.VI) != "" && strings.TrimSpace(t.EN) != "" }

// Bonus is time_bonus or streak_bonus (only the switch matters to module game).
type Bonus struct {
	Enabled bool `json:"enabled"`
}

// Scoring is the scoring block.
type Scoring struct {
	PointsPerCorrect int    `json:"points_per_correct"`
	TimeBonus        *Bonus `json:"time_bonus,omitempty"`
	StreakBonus      *Bonus `json:"streak_bonus,omitempty"`
}

// Feedback is feedback_policy.
type Feedback struct {
	Timing            string `json:"timing"`
	Display           string `json:"feedback_display,omitempty"`
	ShowExplanation   *bool  `json:"show_explanation,omitempty"`
	ShowCorrectAnswer *bool  `json:"show_correct_answer,omitempty"`
}

// Limits is the limits block.
type Limits struct {
	MaxPlaysPerDay *int  `json:"max_plays_per_day,omitempty"`
	Resumable      *bool `json:"resumable,omitempty"`
}

// Presentation is the presentation block (scene variant, theme, music, mascot).
type Presentation struct {
	SceneVariant      string `json:"scene_variant,omitempty"`
	Theme             string `json:"theme,omitempty"`
	BackgroundMusicID string `json:"background_music_id,omitempty"`
	MascotNPCID       string `json:"mascot_npc_id,omitempty"`
}

// Rule is the selection rule of a question source.
type Rule struct {
	GradeLevels   []int       `json:"grade_levels,omitempty"`
	SubjectID     *uuid.UUID  `json:"subject_id,omitempty"`
	TopicIDs      []uuid.UUID `json:"topic_ids,omitempty"`
	ObjectiveIDs  []uuid.UUID `json:"objective_ids,omitempty"`
	DifficultyMin *int        `json:"difficulty_min,omitempty"`
	DifficultyMax *int        `json:"difficulty_max,omitempty"`
	QuestionTypes []string    `json:"question_types"`
}

// QuestionSource is question_source (kept as authored; module learning resolves it).
type QuestionSource struct {
	Kind          string     `json:"kind"`
	QuestionSetID *uuid.UUID `json:"question_set_id,omitempty"`
	Version       *int       `json:"version,omitempty"`
	Rule          *Rule      `json:"rule,omitempty"`
}

// Config is one version of minigame-config.schema.json as module game reads it.
type Config struct {
	ConfigID           uuid.UUID       `json:"minigame_config_id"`
	Code               string          `json:"code"`
	Type               string          `json:"minigame_type"`
	Name               Text            `json:"name"`
	Instructions       *Text           `json:"instructions,omitempty"`
	GradeLevels        []int           `json:"grade_levels"`
	SubjectID          uuid.UUID       `json:"subject_id"`
	ObjectiveIDs       []uuid.UUID     `json:"objective_ids,omitempty"`
	Source             QuestionSource  `json:"question_source"`
	Difficulty         json.RawMessage `json:"difficulty"`
	TimeLimitSeconds   *int            `json:"time_limit_seconds,omitempty"`
	PerItemTimeSeconds *int            `json:"per_item_time_seconds,omitempty"`
	QuestionCount      int             `json:"question_count"`
	Scoring            Scoring         `json:"scoring"`
	Feedback           Feedback        `json:"feedback_policy"`
	Limits             Limits          `json:"limits"`
	RewardRuleID       *uuid.UUID      `json:"reward_rule_id,omitempty"`
	Presentation       *Presentation   `json:"presentation,omitempty"`
}

// Parse reads a config payload (authored or stored).
func Parse(raw []byte) (Config, error) {
	var c Config
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, &FieldError{Field: "payload", Reason: "not a mini-game config object"}
	}
	return c, nil
}

// CheckTranslations requires vi and en on name and instructions (F073-AC-05).
func (c Config) CheckTranslations() error {
	if !c.Name.Complete() || (c.Instructions != nil && !c.Instructions.Complete()) {
		return ErrTranslationIncomplete
	}
	return nil
}

// Validate checks what the JSON Schema leaves to the server: a known type, difficulty 1–5 (adaptive comes after the
// release, [Confirmed]), a question source of a known kind with a question set or a typed selection rule.
func (c Config) Validate() error {
	if !slices.Contains(Types, c.Type) {
		return &FieldError{Field: "minigame_type", Reason: "unknown mini-game type"}
	}
	var d int
	if err := json.Unmarshal(c.Difficulty, &d); err != nil || d < 1 || d > 5 {
		return &FieldError{Field: "difficulty", Reason: "1–5 (adaptive is not available in this release)"}
	}
	switch c.Source.Kind {
	case "question_set":
		if c.Source.QuestionSetID == nil {
			return &FieldError{Field: "question_source/question_set_id", Reason: "is required"}
		}
	case "selection_rule":
		if c.Source.Rule == nil || len(c.Source.Rule.QuestionTypes) == 0 {
			return &FieldError{Field: "question_source/rule/question_types", Reason: "is required"}
		}
	default:
		return &FieldError{Field: "question_source/kind", Reason: "question_set or selection_rule"}
	}
	if c.QuestionCount < 1 || c.QuestionCount > 50 {
		return &FieldError{Field: "question_count", Reason: "1–50"}
	}
	return nil
}

// ForGrade reports a config playable by a student of grade.
func (c Config) ForGrade(grade int) bool { return slices.Contains(c.GradeLevels, grade) }

// youngGrade is the upper grade without speed bonus (grades 1–5, LEARNING_MECHANICS §2, F073-AC-09).
const youngGrade = 5

func boolOr(b *bool, def bool) bool {
	if b == nil {
		return def
	}
	return *b
}

// Display is MiniGameDisplayConfig (F073-AC-07): what the client needs to play, never the question source, an answer, a
// scoring rule or option feedback. The speed bonus is shown off for grades 1–5 (and an unknown grade).
func (c Config) Display(grade *int) map[string]any {
	timeBonus := c.Scoring.TimeBonus != nil && c.Scoring.TimeBonus.Enabled && grade != nil && *grade > youngGrade
	grades := c.GradeLevels
	if grades == nil {
		grades = []int{}
	}
	out := map[string]any{
		"name": c.Name, "grade_levels": grades, "question_count": c.QuestionCount, "feedback_timing": c.Feedback.Timing,
		"show_explanation": boolOr(c.Feedback.ShowExplanation, true), "show_correct_answer": boolOr(c.Feedback.ShowCorrectAnswer, true),
		"scoring": map[string]any{"points_per_correct": c.Scoring.PointsPerCorrect, "time_bonus_enabled": timeBonus,
			"streak_bonus_enabled": c.Scoring.StreakBonus != nil && c.Scoring.StreakBonus.Enabled},
		"resumable":      c.Type != TimedChallenge && boolOr(c.Limits.Resumable, true),
		"prefetch_items": c.Type == TimedChallenge,
	}
	if c.Instructions != nil {
		out["instructions"] = *c.Instructions
	}
	if c.TimeLimitSeconds != nil {
		out["time_limit_seconds"] = *c.TimeLimitSeconds
	}
	if c.PerItemTimeSeconds != nil {
		out["per_item_time_seconds"] = *c.PerItemTimeSeconds
	}
	if c.Feedback.Display != "" && c.Feedback.Timing == "immediate" {
		out["feedback_display"] = c.Feedback.Display
	}
	if p := c.Presentation; p != nil {
		pres := map[string]any{}
		for k, v := range map[string]string{"scene_variant": p.SceneVariant, "theme": p.Theme, "background_music_id": p.BackgroundMusicID,
			"mascot_npc_id": p.MascotNPCID} {
			if v != "" {
				pres[k] = v
			}
		}
		out["presentation"] = pres
	}
	return out
}

// SourceCheck is the answer of module learning on the question source (validateMinigameQuestionSource).
type SourceCheck struct {
	Compatible     bool
	QuestionTypes  []string
	AvailableCount int
	Reasons        []string
}

// ErrSourceInvalid is a question source the release refuses (ACTIVITY_CONFIG_INVALID, F073-AC-02).
var ErrSourceInvalid = errors.New("minigame: question source not compatible")

// CheckSource turns the answer of module learning into the publish decision: compatible and enough questions.
func (c Config) CheckSource(s SourceCheck) error {
	if !s.Compatible {
		return fmt.Errorf("%w: %s", ErrSourceInvalid, strings.Join(s.Reasons, "; "))
	}
	if s.AvailableCount < c.QuestionCount {
		return fmt.Errorf("%w: %d questions available for question_count %d", ErrSourceInvalid, s.AvailableCount, c.QuestionCount)
	}
	return nil
}

// RewardsEligible is rewards_eligible of minigame.completed (F073-AC-19, data-model §3): the completion rule met and the
// plays of the day (this one included) within limits.max_plays_per_day; a play over the limit still counts as practice.
func (c Config) RewardsEligible(completionMet bool, playsToday int) bool {
	if !completionMet {
		return false
	}
	return c.Limits.MaxPlaysPerDay == nil || playsToday <= *c.Limits.MaxPlaysPerDay
}
