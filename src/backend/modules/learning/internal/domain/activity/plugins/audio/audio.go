// Package audio is the plugin of activity_type audio_lesson (F066, AUDIO_LANGUAGE_LEARNING, ADR-0048): an Ogg Vorbis
// listening lesson with segments, auto-graded listening/dictation questions graded by the question bank scorer (each
// question is an item of the attempt), a play limit and a completion rule on the plausibly heard part. It shares the
// media timeline of video_lesson (domain/activity/audiolesson.Config.Timeline); there is no speaking or recording path
// (ADR-0059, ADR-0072).
package audio

import (
	"slices"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/audiolesson"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// SchemaURL is the $id of specs/schemas/activities/audio-lesson.schema.json.
const SchemaURL = "https://schemas.eduworld.vn/content/activities/audio-lesson.schema.json"

// MinClientVersion is the oldest game client with the audio_lesson renderer (F066).
const MinClientVersion = "0.1.0"

// QuestionTypes are the auto-graded question types of a listening lesson (AUDIO_LANGUAGE_LEARNING §2; ADR-0072).
var QuestionTypes = []string{"single_choice", "multiple_choice", "true_false", "fill_in_blank", "short_answer", "numeric", "matching",
	"ordering", "drag_drop", "image_based", "interactive"}

// Plugin is the audio_lesson plugin.
type Plugin struct{}

var (
	_ activity.Plugin       = Plugin{}
	_ activity.Timeline     = Plugin{}
	_ activity.PublishCheck = Plugin{}
	_ activity.MediaCheck   = Plugin{}
)

// New returns the audio_lesson plugin.
func New() activity.Plugin { return Plugin{} }

// Info implements activity.Plugin.
func (Plugin) Info() activity.Info {
	return activity.Info{Type: activity.AudioLesson, Label: "Audio lesson", SchemaURL: SchemaURL, SchemaVersion: 1,
		Strategies: []string{"completion", "auto"}, MinClientVersion: MinClientVersion, Status: activity.StatusEnabled}
}

func config(c activity.Config) (audiolesson.Config, error) { return audiolesson.Parse(c.Raw) }

// ValidateConfig implements activity.Plugin (F066-AC-01) without the duration of the audio (see ValidateMedia).
func (p Plugin) ValidateConfig(c activity.Config) contentmodel.Violations {
	return p.ValidateMedia(c, 0)
}

// ValidateMedia implements activity.MediaCheck: segments and question times inside the probed audio.
func (Plugin) ValidateMedia(c activity.Config, durationMS int64) contentmodel.Violations {
	ac, err := config(c)
	if err != nil {
		return contentmodel.Violations{{Field: "/config/content", Reason: "cannot be read"}}
	}
	return ac.Validate(durationMS)
}

// ValidatePublish implements activity.PublishCheck: a transcript is required to publish.
func (Plugin) ValidatePublish(c activity.Config) contentmodel.Violations {
	ac, err := config(c)
	if err != nil {
		return contentmodel.Violations{{Field: "/config/content", Reason: "cannot be read"}}
	}
	return ac.ValidatePublish()
}

// AllowedQuestionTypes implements activity.Plugin: every auto-graded type.
func (Plugin) AllowedQuestionTypes(activity.Config) []string { return slices.Clone(QuestionTypes) }

// BuildRenderPayload implements activity.Plugin (audiolesson.RenderPayload; never an answer).
func (Plugin) BuildRenderPayload(v activity.Version) map[string]any {
	ac, err := config(v.Config)
	if err != nil {
		return map[string]any{}
	}
	return audiolesson.RenderPayload(ac)
}

// Evaluate implements activity.Plugin: the sum of the question items (a lesson without question is scored by its
// completion in the engine, like a video); passed follows min_score_ratio, else pass_threshold.
func (Plugin) Evaluate(v activity.Version, items []activity.ItemResult) activity.Result {
	r := activity.QuestionBased{}.Evaluate(v, items)
	if th := v.Config.CompletionRule.MinScoreRatio; th != nil && r.MaxScore > 0 {
		passed := r.Score/r.MaxScore >= *th
		r.Passed = &passed
	}
	if len(v.Config.Evaluation.Strategies) == 0 {
		r.Strategies = []string{"completion", "auto"}
	}
	return r
}

// CompletionRule implements activity.Plugin (default min_watched_ratio, CONFIG_DEFAULTS video_completion).
func (Plugin) CompletionRule(v activity.Version) activity.CompletionRule {
	r := v.Config.CompletionRule
	if r.Mode == "" {
		r.Mode = "min_watched_ratio"
	}
	return r
}

// CueQuestions implements activity.Timeline: the questions by at_ms, then declaration order.
func (Plugin) CueQuestions(v activity.Version) ([]uuid.UUID, error) {
	ac, err := config(v.Config)
	if err != nil {
		return nil, err
	}
	return ac.QuestionIDs(), nil
}

// MediaAssetIDs implements activity.Timeline.
func (Plugin) MediaAssetIDs(v activity.Version) ([]uuid.UUID, error) {
	ac, err := config(v.Config)
	if err != nil {
		return nil, err
	}
	return ac.AssetIDs(), nil
}
