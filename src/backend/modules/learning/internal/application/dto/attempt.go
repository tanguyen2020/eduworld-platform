package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Views of the attempt API (specs/openapi Attempt, AttemptSummary, ActivityRender, ActivityResponseResult, AttemptReview,
// F030): never an answer key, a scoring rule, an option feedback or an explanation before grading (ADR-0016).

// LocaleOf maps a content language key to the Locale of the API (ADR-0056).
func LocaleOf(lang string) string {
	if lang == "en" {
		return "en-US"
	}
	return "vi-VN"
}

// QuestionRenderView is QuestionRender.
type QuestionRenderView struct {
	Position          int            `json:"position"`
	QuestionVersionID uuid.UUID      `json:"question_version_id"`
	Type              string         `json:"type"`
	RenderPayload     map[string]any `json:"render_payload"`
	MaxScore          float64        `json:"max_score"`
	HintCount         int            `json:"hint_count"`
}

// SavedResponseView is one entry of Attempt.saved_responses.
type SavedResponseView struct {
	Position      int             `json:"position"`
	Response      json.RawMessage `json:"response"`
	HintsRevealed int             `json:"hints_revealed"`
}

// ActivityRenderView is ActivityRender.
type ActivityRenderView struct {
	ActivityID         uuid.UUID       `json:"activity_id"`
	ActivityVersionID  uuid.UUID       `json:"activity_version_id"`
	Version            int             `json:"version"`
	ActivityType       string          `json:"activity_type"`
	Title              string          `json:"title"`
	Instructions       string          `json:"instructions,omitempty"`
	TimeLimitSeconds   *int            `json:"time_limit_seconds,omitempty"`
	RenderPayload      map[string]any  `json:"render_payload"`
	MathAssets         json.RawMessage `json:"math_assets,omitempty"`
	MinClientVersion   string          `json:"min_client_version,omitempty"`
	ContentLanguage    string          `json:"content_language"`
	DisplayLanguage    string          `json:"display_language"`
	TranslationMissing bool            `json:"translation_missing"`
	// Media are the signed media of a media activity (video variants, subtitles; F065).
	Media []MediaRefView `json:"media,omitempty"`
}

// MediaRefView is one entry of ActivityRender.media.
type MediaRefView struct {
	AssetID   uuid.UUID  `json:"asset_id"`
	URL       string     `json:"url"`
	Kind      string     `json:"kind"`
	Variant   string     `json:"variant,omitempty"`
	Language  string     `json:"language,omitempty"`
	SegmentID string     `json:"segment_id,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// WatchedRangeView is one entry of ActivityProgressState.watched_ranges.
type WatchedRangeView struct {
	StartMS int64 `json:"start_ms"`
	EndMS   int64 `json:"end_ms"`
}

// ProgressStateView is ActivityProgressState (media attempts, F065).
type ProgressStateView struct {
	PositionMS    int64              `json:"position_ms"`
	WatchedRatio  float64            `json:"watched_ratio"`
	Completed     bool               `json:"completed"`
	WatchedRanges []WatchedRangeView `json:"watched_ranges"`
	SegmentID     string             `json:"segment_id,omitempty"`
	PendingCueIDs []string           `json:"pending_cue_ids"`
	// PlaysUsed and MaxPlays are the listening plays of an audio lesson (F066 research R1; 0 and null for a video).
	PlaysUsed int  `json:"plays_used"`
	MaxPlays  *int `json:"max_plays"`
}

// EvaluationView is Evaluation.
type EvaluationView struct {
	EvaluationID uuid.UUID `json:"evaluation_id"`
	AttemptID    uuid.UUID `json:"attempt_id"`
	Source       string    `json:"source"`
	Status       string    `json:"status"`
	Score        float64   `json:"score"`
	MaxScore     float64   `json:"max_score"`
	Passed       *bool     `json:"passed,omitempty"`
	Effective    bool      `json:"effective"`
	CreatedAt    time.Time `json:"created_at"`
}

// AttemptView is Attempt.
type AttemptView struct {
	AttemptID         uuid.UUID            `json:"attempt_id"`
	ActivityID        *uuid.UUID           `json:"activity_id,omitempty"`
	ActivityVersionID *uuid.UUID           `json:"activity_version_id,omitempty"`
	ActivityType      string               `json:"activity_type"`
	Mode              string               `json:"mode"`
	Status            string               `json:"status"`
	ContextType       string               `json:"context_type"`
	ContextID         uuid.UUID            `json:"context_id"`
	StartedAt         time.Time            `json:"started_at"`
	DeadlineAt        *time.Time           `json:"deadline_at,omitempty"`
	SubmittedAt       *time.Time           `json:"submitted_at,omitempty"`
	ServerTime        time.Time            `json:"server_time"`
	FeedbackTiming    string               `json:"feedback_timing"`
	PausedAt          *time.Time           `json:"paused_at,omitempty"`
	RecommendationID  *uuid.UUID           `json:"recommendation_id,omitempty"`
	DisplayLanguage   string               `json:"display_language"`
	Items             []QuestionRenderView `json:"items"`
	SavedResponses    []SavedResponseView  `json:"saved_responses"`
	Render            *ActivityRenderView  `json:"render,omitempty"`
	Evaluation        *EvaluationView      `json:"evaluation,omitempty"`
	// ResumePositionMS and Progress are set for the media activities (F065-AC-12).
	ResumePositionMS *int64             `json:"resume_position_ms,omitempty"`
	Progress         *ProgressStateView `json:"progress,omitempty"`
}

// AttemptSummaryView is AttemptSummary.
type AttemptSummaryView struct {
	AttemptID    uuid.UUID  `json:"attempt_id"`
	ActivityID   *uuid.UUID `json:"activity_id,omitempty"`
	ActivityType string     `json:"activity_type"`
	Title        string     `json:"title,omitempty"`
	Mode         string     `json:"mode"`
	Status       string     `json:"status"`
	StartedAt    time.Time  `json:"started_at"`
	SubmittedAt  *time.Time `json:"submitted_at,omitempty"`
	Score        *float64   `json:"score"`
	MaxScore     *float64   `json:"max_score"`
}

// AnswerFeedbackView is AnswerFeedback.
type AnswerFeedbackView struct {
	OptionID string `json:"option_id,omitempty"`
	Text     string `json:"text"`
}

// ResponseResultView is ActivityResponseResult.
type ResponseResultView struct {
	Accepted    bool                `json:"accepted"`
	Position    int                 `json:"position"`
	IsCorrect   *bool               `json:"is_correct"`
	Score       *float64            `json:"score"`
	Explanation string              `json:"explanation,omitempty"`
	Feedback    *AnswerFeedbackView `json:"feedback"`
	HintsUsed   int                 `json:"hints_used"`
	// NextSegmentID is the segment to play after a branch choice (F065-AC-24).
	NextSegmentID string `json:"next_segment_id,omitempty"`
}

// ReviewItemView is one item of AttemptReview.
type ReviewItemView struct {
	Position     int                 `json:"position"`
	Question     QuestionRenderView  `json:"question"`
	Response     json.RawMessage     `json:"response,omitempty"`
	Feedback     *AnswerFeedbackView `json:"feedback,omitempty"`
	HintsUsed    int                 `json:"hints_used"`
	IsCorrect    *bool               `json:"is_correct"`
	Score        *float64            `json:"score"`
	Explanation  json.RawMessage     `json:"explanation"`
	ObjectiveIDs []uuid.UUID         `json:"objective_ids"`
}

// ReviewView is AttemptReview.
type ReviewView struct {
	AttemptID      uuid.UUID        `json:"attempt_id"`
	Score          *float64         `json:"score"`
	MaxScore       *float64         `json:"max_score"`
	Items          []ReviewItemView `json:"items"`
	WeakObjectives []uuid.UUID      `json:"weak_objectives"`
	// Recommendations are the active study recommendations of the student about the weak objectives (F063-AC-24).
	Recommendations []ReviewRecommendationView `json:"recommendations,omitempty"`
}

// ReviewRecommendationView is AttemptReview.recommendations[].
type ReviewRecommendationView struct {
	Kind             string    `json:"kind"`
	RefID            uuid.UUID `json:"ref_id"`
	RecommendationID uuid.UUID `json:"recommendation_id"`
}
