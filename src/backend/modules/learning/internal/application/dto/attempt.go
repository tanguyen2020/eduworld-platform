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
}
