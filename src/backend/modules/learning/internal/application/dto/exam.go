package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ExamAccommodationView is ExamAccommodation.
type ExamAccommodationView struct {
	StudentID        uuid.UUID `json:"student_id"`
	ExtraTimePercent int       `json:"extra_time_percent"`
}

// ExamSessionView is ExamSession (F042): staff see the policies and accommodations, a student sees its own effective
// duration and attempts used (never the accommodations of others).
type ExamSessionView struct {
	ExamSessionID           uuid.UUID               `json:"exam_session_id"`
	Title                   string                  `json:"title"`
	Type                    string                  `json:"type"`
	IsOfficial              bool                    `json:"is_official"`
	SubjectID               uuid.UUID               `json:"subject_id"`
	GradeLevel              *int                    `json:"grade_level,omitempty"`
	WindowStart             time.Time               `json:"window_start"`
	WindowEnd               time.Time               `json:"window_end"`
	DurationSeconds         int                     `json:"duration_seconds"`
	QuestionCount           int                     `json:"question_count"`
	MaxAttempts             int                     `json:"max_attempts"`
	AttemptsUsed            *int                    `json:"attempts_used,omitempty"`
	Status                  string                  `json:"status"`
	BlueprintVersionID      uuid.UUID               `json:"assessment_blueprint_version_id"`
	FeedbackTiming          string                  `json:"feedback_timing"`
	ResultRelease           string                  `json:"result_release"`
	ResultsReleasedAt       *time.Time              `json:"results_released_at,omitempty"`
	SchoolID                uuid.UUID               `json:"school_id"`
	CourseID                *uuid.UUID              `json:"course_id,omitempty"`
	AssignmentID            *uuid.UUID              `json:"assignment_id,omitempty"`
	Audience                string                  `json:"audience"`
	VariantStrategy         string                  `json:"variant_strategy"`
	VariantCount            int                     `json:"variant_count"`
	LateStartPolicy         string                  `json:"late_start_policy"`
	ScorePolicy             string                  `json:"score_policy"`
	ShowCorrectAnswers      bool                    `json:"show_correct_answers"`
	ShowExplanations        bool                    `json:"show_explanations"`
	Accommodations          []ExamAccommodationView `json:"accommodations,omitempty"`
	EffectiveDurationSecond *int                    `json:"effective_duration_seconds,omitempty"`
	ServerTime              time.Time               `json:"server_time"`
	CreatedAt               time.Time               `json:"created_at"`
	IsEfficacy              *bool                   `json:"is_efficacy,omitempty"`
	// Balance is the result of the variant balancer on createExamSession (F044-AC-22, staff only).
	Balance *ExamBalanceView `json:"balance,omitempty"`
	Version int64            `json:"-"`
}

// ExamPaperItemView is one item of ExamPaper.
type ExamPaperItemView struct {
	Position          int       `json:"position"`
	QuestionVersionID uuid.UUID `json:"question_version_id"`
	Points            float64   `json:"points"`
}

// ExamPaperSectionView is one section of ExamPaper.
type ExamPaperSectionView struct {
	SectionID string              `json:"section_id"`
	Title     string              `json:"title,omitempty"`
	Items     []ExamPaperItemView `json:"items"`
}

// ExamPaperView is ExamPaper.
type ExamPaperView struct {
	ExamPaperID    uuid.UUID              `json:"exam_paper_id"`
	VariantNo      int                    `json:"variant_no"`
	Seed           string                 `json:"seed"`
	ExpectedP      *float64               `json:"expected_p"`
	BalanceWarning bool                   `json:"balance_warning"`
	TotalPoints    float64                `json:"total_points"`
	Sections       []ExamPaperSectionView `json:"sections"`
}

// ExamSectionScoreView is ExamSectionScore.
type ExamSectionScoreView struct {
	SectionID string  `json:"section_id"`
	Title     string  `json:"title,omitempty"`
	Score     float64 `json:"score"`
	MaxScore  float64 `json:"max_score"`
}

// ExamObjectiveView is one row of ExamAttemptAnalysis.objective_breakdown.
type ExamObjectiveView struct {
	ObjectiveID  uuid.UUID `json:"objective_id"`
	Correct      int       `json:"correct"`
	Total        int       `json:"total"`
	MasteryLevel *string   `json:"mastery_level"`
}

// ExamWrongItemView is one row of ExamAttemptAnalysis.wrong_items.
type ExamWrongItemView struct {
	Position          int                 `json:"position"`
	QuestionVersionID uuid.UUID           `json:"question_version_id"`
	Question          *QuestionRenderView `json:"question,omitempty"`
	Response          json.RawMessage     `json:"response,omitempty"`
	CorrectOptionIDs  []string            `json:"correct_option_ids,omitempty"`
	Explanation       json.RawMessage     `json:"explanation,omitempty"`
	Feedback          *AnswerFeedbackView `json:"feedback,omitempty"`
	ObjectiveIDs      []uuid.UUID         `json:"objective_ids,omitempty"`
}

// ExamSuggestionView is one row of ExamAttemptAnalysis.review_suggestions.
type ExamSuggestionView struct {
	ObjectiveID  uuid.UUID  `json:"objective_id"`
	CorrectRatio float64    `json:"correct_ratio"`
	Kind         string     `json:"kind,omitempty"`
	RefID        *uuid.UUID `json:"ref_id,omitempty"`
	Title        string     `json:"title,omitempty"`
}

// ExamAnalysisView is ExamAttemptAnalysis.
type ExamAnalysisView struct {
	AttemptID          uuid.UUID              `json:"attempt_id"`
	ExamSessionID      uuid.UUID              `json:"exam_session_id"`
	Score              float64                `json:"score"`
	MaxScore           float64                `json:"max_score"`
	ScoreScaled        float64                `json:"score_scaled"`
	DurationMS         *int64                 `json:"duration_ms,omitempty"`
	SectionScores      []ExamSectionScoreView `json:"section_scores"`
	ObjectiveBreakdown []ExamObjectiveView    `json:"objective_breakdown"`
	WrongItems         []ExamWrongItemView    `json:"wrong_items"`
	ReviewSuggestions  []ExamSuggestionView   `json:"review_suggestions"`
}

// ExamSubmitView is AttemptResult of submitExamAttempt (F042-AC-18): the attempt is graded by the worker; the score is
// returned only once the results are visible.
type ExamSubmitView struct {
	AttemptID         uuid.UUID  `json:"attempt_id"`
	Status            string     `json:"status"`
	Score             *float64   `json:"score,omitempty"`
	MaxScore          *float64   `json:"max_score,omitempty"`
	CorrectCount      *int       `json:"correct_count,omitempty"`
	DurationMS        int64      `json:"duration_ms"`
	ResultAvailableAt *time.Time `json:"result_available_at,omitempty"`
}

// ExamBalanceView is the result of the variant balancer of independent_draw (F044-AC-22, EXAM_ENGINE §4.1): expected_p of
// every variant and the warning when max − min stays above max_mean_p_diff.
type ExamBalanceView struct {
	MaxMeanPDiff float64                  `json:"max_mean_p_diff"`
	Warning      bool                     `json:"warning"`
	Variants     []ExamBalanceVariantView `json:"variants"`
}

// ExamBalanceVariantView is the expected proportion of points of one variant.
type ExamBalanceVariantView struct {
	VariantNo int      `json:"variant_no"`
	ExpectedP *float64 `json:"expected_p"`
}
