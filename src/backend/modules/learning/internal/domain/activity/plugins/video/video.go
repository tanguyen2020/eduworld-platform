// Package video is the plugin of activity_type video_lesson (F065, VIDEO_LEARNING, ADR-0048): a Theora video with
// chapters, WebVTT subtitles, embedded questions graded by the question bank scorer (each cue is an item of the
// attempt, research R1), optional branches, and a completion rule on the plausibly watched part. The pure rules are in
// domain/activity/videolesson; heartbeats and branch choices are handled by the attempt engine for Timeline plugins.
package video

import (
	"slices"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/videolesson"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// SchemaURL is the $id of specs/schemas/activities/video-lesson.schema.json.
const SchemaURL = "https://schemas.eduworld.vn/content/activities/video-lesson.schema.json"

// MinClientVersion is the oldest game client with the video_lesson renderer (F065).
const MinClientVersion = "0.1.0"

// CueQuestionTypes are the question types an embedded question may use: every auto-graded type (ADR-0072).
var CueQuestionTypes = []string{"single_choice", "multiple_choice", "true_false", "fill_in_blank", "short_answer", "numeric", "matching",
	"ordering", "drag_drop", "image_based", "interactive"}

// Plugin is the video_lesson plugin.
type Plugin struct{}

var (
	_ activity.Plugin   = Plugin{}
	_ activity.Timeline = Plugin{}
)

// New returns the video_lesson plugin.
func New() activity.Plugin { return Plugin{} }

// Info implements activity.Plugin.
func (Plugin) Info() activity.Info {
	return activity.Info{Type: activity.VideoLesson, Label: "Video lesson", SchemaURL: SchemaURL, SchemaVersion: 1,
		Strategies: []string{"completion", "auto"}, MinClientVersion: MinClientVersion, Status: activity.StatusEnabled}
}

func config(c activity.Config) (videolesson.Config, error) { return videolesson.Parse(c.Raw) }

// ValidateConfig implements activity.Plugin (F065-AC-01): the rules of videolesson.Config.Validate.
func (Plugin) ValidateConfig(c activity.Config) contentmodel.Violations {
	vc, err := config(c)
	if err != nil {
		return contentmodel.Violations{{Field: "/config/content", Reason: "cannot be read"}}
	}
	return vc.Validate()
}

// AllowedQuestionTypes implements activity.Plugin: an embedded question may be any auto-graded type.
func (Plugin) AllowedQuestionTypes(activity.Config) []string { return slices.Clone(CueQuestionTypes) }

// BuildRenderPayload implements activity.Plugin (videolesson.RenderPayload; never a target segment or an answer).
func (Plugin) BuildRenderPayload(v activity.Version) map[string]any {
	vc, err := config(v.Config)
	if err != nil {
		return map[string]any{}
	}
	return videolesson.RenderPayload(vc)
}

// Evaluate implements activity.Plugin: the score is the sum of the cue items (a video without cue scores 0 of 0);
// completion is decided by the engine with videolesson.Evaluate. passed follows min_score_ratio, else pass_threshold.
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

// CueQuestions implements activity.Timeline.
func (Plugin) CueQuestions(v activity.Version) ([]uuid.UUID, error) {
	vc, err := config(v.Config)
	if err != nil {
		return nil, err
	}
	out := []uuid.UUID{}
	for _, q := range vc.OrderedCues() {
		out = append(out, q.QuestionVersionID)
	}
	return out, nil
}

// MediaAssetIDs implements activity.Timeline.
func (Plugin) MediaAssetIDs(v activity.Version) ([]uuid.UUID, error) {
	vc, err := config(v.Config)
	if err != nil {
		return nil, err
	}
	return vc.AssetIDs(), nil
}
