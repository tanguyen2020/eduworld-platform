package dto

import (
	"time"

	"github.com/google/uuid"
)

// HintView is HintResult (revealNextHint, F032 US2).
type HintView struct {
	Position    int          `json:"position"`
	Level       int          `json:"level"`
	TotalLevels int          `json:"total_levels"`
	Hint        HintTextView `json:"hint"`
}

// HintTextView is the hint of one level (plain text in the language of the attempt).
type HintTextView struct {
	Text string `json:"text"`
}

// WrongQuestionView is WrongQuestion (listMyWrongQuestions, F032 US4): no answer, no explanation.
type WrongQuestionView struct {
	EntryID           uuid.UUID   `json:"-"`
	QuestionVersionID uuid.UUID   `json:"question_version_id"`
	SubjectID         *uuid.UUID  `json:"subject_id,omitempty"`
	ObjectiveIDs      []uuid.UUID `json:"objective_ids"`
	WrongCount        int         `json:"wrong_count"`
	LastWrongAt       time.Time   `json:"last_wrong_at"`
	NextReviewAt      time.Time   `json:"next_review_at"`
}

// RecommendationView is one item of RecommendationList (getMyRecommendations, F032 US6).
type RecommendationView struct {
	RecommendationID uuid.UUID   `json:"recommendation_id"`
	RuleCode         string      `json:"rule_code"`
	Kind             string      `json:"kind"`
	TargetType       string      `json:"target_type"`
	TargetID         uuid.UUID   `json:"target_id"`
	ObjectiveID      *uuid.UUID  `json:"objective_id,omitempty"`
	ObjectiveIDs     []uuid.UUID `json:"objective_ids"`
	SubjectID        *uuid.UUID  `json:"subject_id,omitempty"`
	GradeLevel       *int        `json:"grade_level,omitempty"`
	Reason           string      `json:"reason"`
	ReasonCode       string      `json:"reason_code"`
	ReasonText       string      `json:"reason_text"`
	Priority         int         `json:"priority"`
}

// RecommendationListView is RecommendationList.
type RecommendationListView struct {
	Items       []RecommendationView `json:"items"`
	GeneratedAt time.Time            `json:"generated_at"`
}

// MyMasteryView is MyMasteryList (getMyMastery, F032 US7).
type MyMasteryView struct {
	GradeBand string              `json:"grade_band"`
	Items     []MyMasteryItemView `json:"items"`
}

// MyMasteryItemView is one objective of MyMasteryList: Value only for grades 6–9.
type MyMasteryItemView struct {
	ObjectiveID    uuid.UUID  `json:"objective_id"`
	ObjectiveCode  string     `json:"objective_code,omitempty"`
	ObjectiveName  string     `json:"objective_name,omitempty"`
	SubjectID      *uuid.UUID `json:"subject_id,omitempty"`
	TopicID        *uuid.UUID `json:"topic_id,omitempty"`
	Level          string     `json:"level"`
	IconCode       string     `json:"icon_code"`
	Value          *float64   `json:"value"`
	EvidenceCount  int        `json:"evidence_count"`
	LastEvidenceAt *time.Time `json:"last_evidence_at,omitempty"`
}
