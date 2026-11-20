package read

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/mastery"
)

// WrongQuestionRecord is one active entry of the review list of a student (no answer, no explanation).
type WrongQuestionRecord struct {
	EntryID           uuid.UUID
	QuestionVersionID uuid.UUID
	SubjectID         *uuid.UUID
	ObjectiveIDs      []uuid.UUID
	WrongCount        int
	LastWrongAt       time.Time
	NextDueAt         time.Time
}

// RecommendationRecord is one active recommendation of a student.
type RecommendationRecord struct {
	ID           uuid.UUID
	RuleCode     string
	Kind         string
	TargetType   string
	TargetID     uuid.UUID
	SubjectID    *uuid.UUID
	Grade        *int
	ObjectiveIDs []uuid.UUID
	Reason       string
	ReasonCode   string
	ReasonParams json.RawMessage
	Priority     int
	GeneratedAt  time.Time
	ExpiresAt    time.Time
}

// MasteryDetail is the mastery of one objective of a student with its curriculum metadata (getMyMastery).
type MasteryDetail struct {
	ObjectiveID uuid.UUID
	Code        string
	Name        string
	TopicID     *uuid.UUID
	SubjectID   *uuid.UUID
	State       mastery.State
}

// SelfStudyReadModel reads the self-study read models of F032 in the scope of ctx (tenant + deleted_at, RLS T).
type SelfStudyReadModel interface {
	// WrongQuestions lists the active review entries of a student (one subject or all), soonest due first.
	WrongQuestions(ctx context.Context, studentID uuid.UUID, subjectID *uuid.UUID) ([]WrongQuestionRecord, error)
	// Recommendations lists the active, not expired recommendations of a student (one subject or all) by priority.
	Recommendations(ctx context.Context, studentID uuid.UUID, subjectID *uuid.UUID, now time.Time) ([]RecommendationRecord, error)
	// MasteryDetails lists the masteries of a student with the objective metadata (one subject and/or grade).
	MasteryDetails(ctx context.Context, studentID uuid.UUID, subjectID *uuid.UUID, grade *int) ([]MasteryDetail, error)
}

// ReasonCatalog renders the reason of a recommendation in a language (vi/en, ADR-0056) from its code and parameters.
type ReasonCatalog interface {
	Text(code, lang string, params map[string]string) string
}
