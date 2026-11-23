package minigame

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"
)

// Question source kinds of minigame-config.schema.json.
const (
	SourceQuestionSet   = "question_set"
	SourceSelectionRule = "selection_rule"
)

// Rules is a published MiniGameConfigVersion (minigame-config.schema.json) as the plugin reads it from the snapshot of the
// projection: the configuration holds no question and no answer (F073-AC-03).
type Rules struct {
	MinigameConfigID        uuid.UUID         `json:"minigame_config_id"`
	MinigameConfigVersionID uuid.UUID         `json:"minigame_config_version_id"`
	Version                 int               `json:"version"`
	Type                    string            `json:"minigame_type"`
	Name                    map[string]string `json:"name"`
	Instructions            map[string]string `json:"instructions,omitempty"`
	GradeLevels             []int             `json:"grade_levels"`
	SubjectID               *uuid.UUID        `json:"subject_id,omitempty"`
	QuestionSource          Source            `json:"question_source"`
	TimeLimitSeconds        *int              `json:"time_limit_seconds,omitempty"`
	PerItemTimeSeconds      *int              `json:"per_item_time_seconds,omitempty"`
	QuestionCount           int               `json:"question_count"`
	Scoring                 Scoring           `json:"scoring"`
	ResultCalculation       ResultCalculation `json:"result_calculation"`
	FeedbackPolicy          FeedbackPolicy    `json:"feedback_policy"`
	CompletionRule          CompletionRule    `json:"completion_rule"`
	Limits                  Limits            `json:"limits"`
	Presentation            Presentation      `json:"presentation"`
	RewardRuleID            *uuid.UUID        `json:"reward_rule_id,omitempty"`
}

// Source is question_source: a question set (current release or a fixed version number) or a selection rule.
type Source struct {
	Kind          string     `json:"kind"`
	QuestionSetID *uuid.UUID `json:"question_set_id,omitempty"`
	Version       *int       `json:"version,omitempty"`
	Rule          *Rule      `json:"rule,omitempty"`
}

// Rule is the selection rule of a source (filters of the published pool).
type Rule struct {
	GradeLevels             []int       `json:"grade_levels,omitempty"`
	SubjectID               *uuid.UUID  `json:"subject_id,omitempty"`
	TopicIDs                []uuid.UUID `json:"topic_ids,omitempty"`
	ObjectiveIDs            []uuid.UUID `json:"objective_ids,omitempty"`
	DifficultyMin           *int        `json:"difficulty_min,omitempty"`
	DifficultyMax           *int        `json:"difficulty_max,omitempty"`
	QuestionTypes           []string    `json:"question_types,omitempty"`
	ExcludeRecentlySeenDays *int        `json:"exclude_recently_seen_days,omitempty"`
}

// Scoring is the game score configuration (points per correct answer, speed bonus, streak bonus, wrong penalty).
type Scoring struct {
	PointsPerCorrect int          `json:"points_per_correct"`
	WrongPenalty     int          `json:"wrong_penalty,omitempty"`
	TimeBonus        *TimeBonus   `json:"time_bonus,omitempty"`
	StreakBonus      *StreakBonus `json:"streak_bonus,omitempty"`
}

// TimeBonus is the speed bonus (never for grades 1–5, F073-AC-09).
type TimeBonus struct {
	Enabled           bool `json:"enabled"`
	MaxBonus          int  `json:"max_bonus,omitempty"`
	FullBonusWithinMS int  `json:"full_bonus_within_ms,omitempty"`
}

// StreakBonus is the bonus of every every_n_correct consecutive correct answers.
type StreakBonus struct {
	Enabled       bool `json:"enabled"`
	EveryNCorrect int  `json:"every_n_correct,omitempty"`
	Bonus         int  `json:"bonus,omitempty"`
}

// Result calculations.
const (
	LearningSumQuestionScores = "sum_question_scores"
	LearningCorrectRatio      = "correct_ratio"
	GamePointsPlusBonus       = "points_plus_bonus"
	GamePointsOnly            = "points_only"
)

// ResultCalculation tells how the learning score and the game score are computed.
type ResultCalculation struct {
	LearningScore string `json:"learning_score"`
	GameScore     string `json:"game_score"`
}

// FeedbackPolicy is the feedback of the configuration (FeedbackTiming; feedback_display only with immediate).
type FeedbackPolicy struct {
	Timing            string `json:"timing"`
	FeedbackDisplay   string `json:"feedback_display,omitempty"`
	ShowExplanation   *bool  `json:"show_explanation,omitempty"`
	ShowCorrectAnswer *bool  `json:"show_correct_answer,omitempty"`
}

// Completion rule types.
const (
	CompleteAll      = "complete_all"
	MinCorrectRatio  = "min_correct_ratio"
	MinLearningScore = "min_learning_score"
)

// CompletionRule is completion_rule (the threshold of min_correct_ratio is a ratio, of min_learning_score a learning score).
type CompletionRule struct {
	Type      string   `json:"type"`
	Threshold *float64 `json:"threshold,omitempty"`
}

// Limits are the play limits (max_plays_per_day is applied by module game, F073-AC-19).
type Limits struct {
	MaxPlaysPerDay *int   `json:"max_plays_per_day,omitempty"`
	RetryPolicy    string `json:"retry_policy,omitempty"`
	Resumable      *bool  `json:"resumable,omitempty"`
}

// Presentation is the visual variant of the scene (no game logic).
type Presentation struct {
	SceneVariant      string `json:"scene_variant,omitempty"`
	Theme             string `json:"theme,omitempty"`
	BackgroundMusicID string `json:"background_music_id,omitempty"`
	MascotNPCID       string `json:"mascot_npc_id,omitempty"`
}

var (
	feedbackTimings = []string{"immediate", "after_submit", "after_close", "never"}
	displays        = []string{"end_of_round", "end_of_game"}
)

// ErrInvalidSnapshot is returned for a snapshot that is not a MiniGameConfigVersion the plugin can play.
var ErrInvalidSnapshot = errors.New("minigame: invalid configuration snapshot")

func invalid(why string) error { return fmt.Errorf("%w: %s", ErrInvalidSnapshot, why) }

// Parse decodes and checks a snapshot (published by module game against minigame-config.schema.json; the projection
// refuses what the plugin cannot play).
func Parse(snapshot json.RawMessage) (Rules, error) {
	var r Rules
	if len(snapshot) == 0 {
		return r, invalid("empty")
	}
	if err := json.Unmarshal(snapshot, &r); err != nil {
		return r, invalid(err.Error())
	}
	switch {
	case !Valid(r.Type):
		return r, invalid("minigame_type")
	case r.Version < 1:
		return r, invalid("version")
	case r.QuestionCount < 1 || r.QuestionCount > 50:
		return r, invalid("question_count")
	case len(r.GradeLevels) == 0 || len(r.GradeLevels) > 9:
		return r, invalid("grade_levels")
	case r.Name["vi"] == "" || r.Name["en"] == "":
		return r, invalid("name")
	case !slices.Contains(feedbackTimings, r.FeedbackPolicy.Timing):
		return r, invalid("feedback_policy.timing")
	case r.FeedbackPolicy.FeedbackDisplay != "" && !slices.Contains(displays, r.FeedbackPolicy.FeedbackDisplay):
		return r, invalid("feedback_policy.feedback_display")
	case r.ResultCalculation.LearningScore != LearningSumQuestionScores && r.ResultCalculation.LearningScore != LearningCorrectRatio:
		return r, invalid("result_calculation.learning_score")
	case r.ResultCalculation.GameScore != GamePointsPlusBonus && r.ResultCalculation.GameScore != GamePointsOnly:
		return r, invalid("result_calculation.game_score")
	case r.Scoring.PointsPerCorrect < 0 || r.Scoring.WrongPenalty < 0:
		return r, invalid("scoring")
	}
	for _, g := range r.GradeLevels {
		if g < 1 || g > 9 {
			return r, invalid("grade_levels")
		}
	}
	switch r.CompletionRule.Type {
	case CompleteAll:
	case MinCorrectRatio, MinLearningScore:
		if r.CompletionRule.Threshold == nil || *r.CompletionRule.Threshold < 0 {
			return r, invalid("completion_rule.threshold")
		}
	default:
		return r, invalid("completion_rule.type")
	}
	switch r.QuestionSource.Kind {
	case SourceQuestionSet:
		if r.QuestionSource.QuestionSetID == nil {
			return r, invalid("question_source.question_set_id")
		}
	case SourceSelectionRule:
		if r.QuestionSource.Rule == nil {
			return r, invalid("question_source.rule")
		}
	default:
		return r, invalid("question_source.kind")
	}
	return r, nil
}

// ActivityContent is the content of a learning activity minigame (activities/minigame.schema.json): the wrapped
// configuration, its version number (nil = current at start, pinned by the attempt) and an optional question set
// replacing the question source of the configuration (the game rules are kept).
type ActivityContent struct {
	MinigameConfigID      uuid.UUID    `json:"minigame_config_id"`
	MinigameConfigVersion *int         `json:"minigame_config_version,omitempty"`
	QuestionSetOverride   *SetOverride `json:"question_set_override,omitempty"`
	// FeedbackDisplay is interaction.feedback_display ("" = the one of the configuration).
	FeedbackDisplay string `json:"-"`
}

// SetOverride is content.question_set_override.
type SetOverride struct {
	QuestionSetID      uuid.UUID `json:"question_set_id"`
	QuestionSetVersion *int      `json:"question_set_version,omitempty"`
}

// ParseActivity decodes the content and interaction of a minigame activity config.
func ParseActivity(raw json.RawMessage) (ActivityContent, error) {
	var c struct {
		Content     *ActivityContent `json:"content"`
		Interaction struct {
			FeedbackDisplay string `json:"feedback_display"`
		} `json:"interaction"`
	}
	if err := json.Unmarshal(raw, &c); err != nil || c.Content == nil || c.Content.MinigameConfigID == uuid.Nil {
		return ActivityContent{}, errors.New("minigame: the activity content has no minigame_config_id")
	}
	out := *c.Content
	out.FeedbackDisplay = c.Interaction.FeedbackDisplay
	return out, nil
}

// Effective returns the question source played by an attempt: the question set override of the activity, else the
// source of the configuration.
func (r Rules) Effective(override *SetOverride) Source {
	if override == nil {
		return r.QuestionSource
	}
	id := override.QuestionSetID
	return Source{Kind: SourceQuestionSet, QuestionSetID: &id, Version: override.QuestionSetVersion}
}
