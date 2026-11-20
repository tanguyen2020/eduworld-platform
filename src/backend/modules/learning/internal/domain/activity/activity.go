// Package activity is the Activity Registry of module learning (F030, ACTIVITY_ENGINE §3–§5, ADR-0048): the plugin
// interface every activity_type implements, the registry loaded at start-up, and the config of a version (content,
// interaction, evaluation, feedback, completion rule). The attempt engine never knows a concrete type: it calls the
// plugin of the version. No I/O: question content, grading and the JSON Schema check are ports of the application.
package activity

import (
	"encoding/json"
	"errors"
	"slices"
	"sort"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// Type is an activity_type (ActivityType of specs/schemas/common/enums.schema.json).
type Type string

// Activity types of the release (ACTIVITY_ENGINE §3); written_response, speaking and project are outside it (ADR-0072).
const (
	Quiz                Type = "quiz"
	InteractiveExercise Type = "interactive_exercise"
	Minigame            Type = "minigame"
	VideoLesson         Type = "video_lesson"
	AudioLesson         Type = "audio_lesson"
	Simulation          Type = "simulation"
)

var releaseTypes = []Type{Quiz, InteractiveExercise, Minigame, VideoLesson, AudioLesson, Simulation}

// InRelease reports a type of ActivityType.
func (t Type) InRelease() bool { return slices.Contains(releaseTypes, t) }

// Status is the registry status of a type (ActivityTypeInfo.status).
type Status string

// Registry statuses.
const (
	StatusEnabled  Status = "enabled"
	StatusPreview  Status = "preview"
	StatusDisabled Status = "disabled"
)

// Info describes a registered type (ActivityTypeInfo, F030-AC-22).
type Info struct {
	Type             Type
	Label            string
	SchemaURL        string
	SchemaVersion    int
	Strategies       []string
	MinClientVersion string
	Status           Status
}

// QuestionRef is a fixed question of a source.
type QuestionRef struct {
	QuestionVersionID uuid.UUID `json:"question_version_id"`
}

// Selection is a selection rule of a source (QUESTION_BANK §6).
type Selection struct {
	TopicIDs      []uuid.UUID `json:"topic_ids,omitempty"`
	ObjectiveIDs  []uuid.UUID `json:"objective_ids,omitempty"`
	DifficultyMin *int        `json:"difficulty_min,omitempty"`
	DifficultyMax *int        `json:"difficulty_max,omitempty"`
	QuestionTypes []string    `json:"question_types,omitempty"`
	Count         int         `json:"count"`
}

// Source kinds of a question-based activity.
const (
	SourceQuestions   = "questions"
	SourceQuestionSet = "question_set"
	SourceSelection   = "selection"
)

// QuestionSource is content.question_source (activity-base.schema.json#/$defs/questionSource).
type QuestionSource struct {
	Kind               string        `json:"kind"`
	Questions          []QuestionRef `json:"questions,omitempty"`
	QuestionSetID      *uuid.UUID    `json:"question_set_id,omitempty"`
	QuestionSetVersion *int          `json:"question_set_version,omitempty"`
	Selection          *Selection    `json:"selection,omitempty"`
}

// Timed is the timer of a timed quiz (server time).
type Timed struct {
	TotalSeconds int   `json:"total_seconds"`
	ShowTimer    *bool `json:"show_timer,omitempty"`
}

// Content is the content of a question-based activity.
type Content struct {
	QuestionSource       QuestionSource `json:"question_source"`
	AllowedQuestionTypes []string       `json:"allowed_question_types,omitempty"`
	Timed                *Timed         `json:"timed,omitempty"`
}

// Interaction is questionInteraction (shuffles, retries, hints, navigation).
type Interaction struct {
	ShuffleItems       *bool  `json:"shuffle_items,omitempty"`
	ShuffleOptions     *bool  `json:"shuffle_options,omitempty"`
	PerItemTimeLimit   *int   `json:"per_item_time_limit_seconds,omitempty"`
	AllowUnscoredRetry *bool  `json:"allow_unscored_retry,omitempty"`
	HintsEnabled       *bool  `json:"hints_enabled,omitempty"`
	Navigation         string `json:"navigation,omitempty"`
}

// Evaluation is the evaluation block (strategies, max_score, pass_threshold).
type Evaluation struct {
	Strategies    []string `json:"strategies"`
	MaxScore      float64  `json:"max_score"`
	PassThreshold *float64 `json:"pass_threshold,omitempty"`
}

// Feedback is the feedback block (FeedbackTiming of the activity, ACTIVITY_ENGINE §8.1).
type Feedback struct {
	Timing            string `json:"timing"`
	ShowExplanation   *bool  `json:"show_explanation,omitempty"`
	ShowCorrectAnswer *bool  `json:"show_correct_answer,omitempty"`
}

// CompletionRule is the completion rule ("hoàn thành" ≠ "đạt"); the watched ratio and required cues are the rule of the
// media types (activity-base.schema.json#/$defs/completionRule, F065).
type CompletionRule struct {
	Mode                string   `json:"mode"`
	MinScoreRatio       *float64 `json:"min_score_ratio,omitempty"`
	MinWatchedRatio     *float64 `json:"min_watched_ratio,omitempty"`
	RequireRequiredCues *bool    `json:"require_required_cues,omitempty"`
}

// Config is the config of an activity version (ActivityDefinitionInput.config).
type Config struct {
	Content        Content        `json:"content"`
	Interaction    Interaction    `json:"interaction"`
	Evaluation     Evaluation     `json:"evaluation"`
	Feedback       Feedback       `json:"feedback"`
	CompletionRule CompletionRule `json:"completion_rule"`
	// Raw is the config as stored: the plugins whose content is not question-based (video_lesson, audio_lesson,
	// simulation) read their own content and interaction from it.
	Raw json.RawMessage `json:"-"`
}

// ParseConfig decodes a config already checked against the JSON Schema of its type.
func ParseConfig(raw json.RawMessage) (Config, error) {
	var c Config
	if len(raw) == 0 {
		return c, errors.New("activity: empty config")
	}
	err := json.Unmarshal(raw, &c)
	c.Raw = raw
	return c, err
}

// Bool returns *b or def.
func Bool(b *bool, def bool) bool {
	if b == nil {
		return def
	}
	return *b
}

// Version is what a plugin reads of a published activity version.
type Version struct {
	ActivityID       uuid.UUID
	VersionID        uuid.UUID
	VersionNo        int
	Type             Type
	Config           Config
	TimeLimitSeconds *int
	ObjectiveIDs     []uuid.UUID
}

// ItemResult is the final result of one delivered item (ACTIVITY_ENGINE §7.2: the last scored answer, 0 when unanswered).
type ItemResult struct {
	Position  int
	MaxScore  float64
	Score     float64
	IsCorrect bool
	Answered  bool
}

// Result is the evaluation of an attempt computed by a plugin.
type Result struct {
	Score        float64
	MaxScore     float64
	CorrectCount int
	Passed       *bool
	Strategies   []string
}

// Plugin is the contract of an activity_type (ACTIVITY_ENGINE §5, ADR-0048). Question-based types (quiz,
// interactive_exercise) grade each response through the question bank scorer of the engine; the plugin decides which
// question types are allowed, what the client sees of the config and how the attempt is evaluated.
type Plugin interface {
	Info() Info
	// ValidateConfig checks what the JSON Schema of the type cannot express (on draft save and on publish).
	ValidateConfig(c Config) contentmodel.Violations
	// AllowedQuestionTypes are the question types this version accepts (the item of another type is refused).
	AllowedQuestionTypes(c Config) []string
	// BuildRenderPayload is the client part of the config: never an answer, a scoring rule or option feedback.
	BuildRenderPayload(v Version) map[string]any
	// Evaluate computes the evaluation from the item results, always automatically (ADR-0072); idempotent.
	Evaluate(v Version, items []ItemResult) Result
	// CompletionRule returns the completion rule of the version.
	CompletionRule(v Version) CompletionRule
}

// Timeline is implemented by the plugins of the media activities (video_lesson F065, audio_lesson F066): the items of an
// attempt are the embedded questions of the version in time order (never shuffled) and the attempt receives progress
// heartbeats (ACTIVITY_ENGINE §5, F065 research R1).
type Timeline interface {
	// CueQuestions are the question versions of the cues by at_ms (item position i+1 is cue i).
	CueQuestions(v Version) ([]uuid.UUID, error)
	// MediaAssetIDs are the media the version plays (start checks they are ready, the render signs their URLs).
	MediaAssetIDs(v Version) ([]uuid.UUID, error)
}

// PublishCheck is implemented by the plugins with a rule checked only at publish (audio_lesson: the transcript,
// F066-AC-01); a draft may still miss it.
type PublishCheck interface {
	ValidatePublish(c Config) contentmodel.Violations
}

// MediaCheck is implemented by the media plugins whose rules need the probed duration of the main media (audio_lesson:
// segments and question times inside the audio, F066-AC-01); durationMS is 0 when unknown.
type MediaCheck interface {
	ValidateMedia(c Config, durationMS int64) contentmodel.Violations
}

// RunResult is the last verified run of a simulation attempt (F066-AC-16): nil when the attempt has none.
type RunResult struct {
	// Verifiable is false for a template without checker (runs not_verifiable: no score, no evidence).
	Verifiable bool
	GoalsMet   []bool
}

// Simulated is implemented by the simulation plugin (F066): the items are its conclusion questions (never shuffled),
// the seed follows seed_policy, and the evaluation adds the goals of the last verified run.
type Simulated interface {
	SimulationQuestions(v Version) ([]uuid.UUID, error)
	// Seed is the seed of an attempt: the fixed seed of the version, else random bounded for the client.
	Seed(v Version, random int64) int64
	EvaluateRuns(v Version, items []ItemResult, last *RunResult, anyRun bool) (Result, bool)
}

// ErrNotSupported is returned for a type outside the release or without plugin (ACTIVITY_TYPE_NOT_SUPPORTED).
var ErrNotSupported = errors.New("activity: activity_type not supported")

// Registry holds the plugins loaded at start-up (ACTIVITY_ENGINE §5).
type Registry struct{ plugins map[Type]Plugin }

// NewRegistry registers plugins; a type outside the release or registered twice is an error.
func NewRegistry(ps ...Plugin) (*Registry, error) {
	r := &Registry{plugins: map[Type]Plugin{}}
	for _, p := range ps {
		t := p.Info().Type
		if !t.InRelease() {
			return nil, errors.New("activity: plugin for a type outside the release: " + string(t))
		}
		if _, dup := r.plugins[t]; dup {
			return nil, errors.New("activity: plugin registered twice: " + string(t))
		}
		r.plugins[t] = p
	}
	return r, nil
}

// Get returns the plugin of t.
func (r *Registry) Get(t Type) (Plugin, bool) {
	if r == nil {
		return nil, false
	}
	p, ok := r.plugins[t]
	return p, ok
}

// Resolve returns the plugin of a type string, ErrNotSupported otherwise (F030-AC-20).
func (r *Registry) Resolve(s string) (Plugin, error) {
	t := Type(s)
	if !t.InRelease() {
		return nil, ErrNotSupported
	}
	p, ok := r.Get(t)
	if !ok || p.Info().Status == StatusDisabled {
		return nil, ErrNotSupported
	}
	return p, nil
}

// Infos lists the registered types in name order (listActivityTypes).
func (r *Registry) Infos() []Info {
	out := make([]Info, 0, len(r.plugins))
	for _, p := range r.plugins {
		out = append(out, p.Info())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}

// MinClientVersion returns the min_client_version of a type ("" without plugin).
func (r *Registry) MinClientVersion(t string) string {
	if p, ok := r.Get(Type(t)); ok {
		return p.Info().MinClientVersion
	}
	return ""
}

// Missing returns the types among published that have no plugin (readiness, F030-AC-23).
func (r *Registry) Missing(published []string) []string {
	var out []string
	for _, t := range published {
		if _, ok := r.Get(Type(t)); !ok && !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	sort.Strings(out)
	return out
}
