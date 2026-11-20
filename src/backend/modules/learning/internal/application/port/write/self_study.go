package write

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/retrywrong"
)

// WrongEntryRow is one row of learning.wrong_question_entries (F032 data-model §1): the review state of a question a
// student answered wrong at its first scored try.
type WrongEntryRow struct {
	ID                  uuid.UUID
	TenantID            uuid.UUID
	SchoolID            uuid.UUID
	StudentID           uuid.UUID
	QuestionID          uuid.UUID
	QuestionVersionID   uuid.UUID
	SubjectID           *uuid.UUID
	ObjectiveIDs        []uuid.UUID
	QuestionType        string
	Difficulty          int
	FirstWrongAttemptID uuid.UUID
	Entry               retrywrong.Entry
	// RetryAttemptID/RetryPosition: the item of the retry_wrong attempt that reviews this entry (a sibling or the
	// original question).
	RetryAttemptID *uuid.UUID
	RetryPosition  *int
	Version        int64
}

// WrongQuestionStore persists the review list in the TenantScope of ctx (RLS T, soft-delete only).
type WrongQuestionStore interface {
	// ActiveEntry returns the active entry of a student for a question (nil when none).
	ActiveEntry(ctx context.Context, studentID, questionID uuid.UUID) (*WrongEntryRow, error)
	// RetryEntry returns the active entry reviewed by an item of a retry_wrong attempt (nil when none).
	RetryEntry(ctx context.Context, attemptID uuid.UUID, position int) (*WrongEntryRow, error)
	// InsertEntry inserts an entry; false when an active entry of the question exists already.
	InsertEntry(ctx context.Context, r WrongEntryRow) (bool, error)
	// UpdateEntry saves the review state and the retry item when the row version is still r.Version.
	UpdateEntry(ctx context.Context, r WrongEntryRow) (bool, error)
	// DueEntries lists the active entries of a student due at now (one subject or every subject), oldest due first.
	DueEntries(ctx context.Context, studentID uuid.UUID, subjectID *uuid.UUID, now time.Time, limit int) ([]WrongEntryRow, error)
	// AttemptEntries lists the active entries whose first wrong answer is in an attempt (re-evaluation).
	AttemptEntries(ctx context.Context, attemptID uuid.UUID) ([]WrongEntryRow, error)
}

// RecommendationRow is one row of learning.rm_student_recommendations (F032 data-model §1).
type RecommendationRow struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	SchoolID         uuid.UUID
	StudentID        uuid.UUID
	RuleCode         string
	Kind             string
	TargetType       string
	TargetID         uuid.UUID
	SubjectID        *uuid.UUID
	Grade            *int
	ObjectiveIDs     []uuid.UUID
	Reason           string
	ReasonCode       string
	ReasonParams     json.RawMessage
	Priority         int
	Status           string
	SkipCount        int
	StartedAttemptID *uuid.UUID
	GeneratedAt      time.Time
	ExpiresAt        time.Time
	ClosedAt         *time.Time
	Version          int64
}

// RecommendationStore persists the recommendations of a student in the TenantScope of ctx.
type RecommendationStore interface {
	// ActiveRecommendations lists the active recommendations of a student.
	ActiveRecommendations(ctx context.Context, studentID uuid.UUID) ([]RecommendationRow, error)
	// BlockedTargets lists the targets (kind|target_type|target_id) a student skipped three times.
	BlockedTargets(ctx context.Context, studentID uuid.UUID, skips int) ([]string, error)
	// Recommendation returns one recommendation of a student (nil when none).
	Recommendation(ctx context.Context, studentID, id uuid.UUID) (*RecommendationRow, error)
	// RecommendationByAttempt returns the recommendation started by an attempt (nil when none).
	RecommendationByAttempt(ctx context.Context, attemptID uuid.UUID) (*RecommendationRow, error)
	InsertRecommendation(ctx context.Context, r RecommendationRow) error
	// UpdateRecommendation saves the mutable columns when the row version is still r.Version.
	UpdateRecommendation(ctx context.Context, r RecommendationRow) (bool, error)
	// RecomputeStudents lists the students of every tenant (ctx carries tenancy.Bypass(platform_job)) with evidence since
	// since, an active recommendation or an active review entry (job recommendation-nightly).
	RecomputeStudents(ctx context.Context, since time.Time, limit int) ([]StudentRef, error)
}

// StudentRef is a student of a tenant.
type StudentRef struct {
	TenantID  uuid.UUID
	StudentID uuid.UUID
}

// ObjectiveInfo is a learning objective of the curriculum visible to the tenant, with its topic, subject and grade.
type ObjectiveInfo struct {
	ID            uuid.UUID
	Code          string
	Name          string
	TopicID       uuid.UUID
	TopicName     string
	TopicOrder    int
	SubjectID     uuid.UUID
	Grade         int
	SortOrder     int
	Prerequisites []uuid.UUID
}

// ObjectiveFilter selects objectives: of a grade, a subject, topics or ids (empty = no condition); WithPrerequisites
// adds the direct prerequisites of the selected objectives.
type ObjectiveFilter struct {
	Grade             *int
	SubjectID         *uuid.UUID
	TopicIDs          []uuid.UUID
	ObjectiveIDs      []uuid.UUID
	WithPrerequisites bool
}

// SelfStudyReader reads what the self-study use cases need in the transaction of ctx (TenantScope of the student):
// curriculum objectives visible to the tenant, mastery rows, questions seen recently, diagnostics and learned topics.
type SelfStudyReader interface {
	Objectives(ctx context.Context, f ObjectiveFilter) ([]ObjectiveInfo, error)
	// StudentMasteries lists the mastery rows of a student (every objective).
	StudentMasteries(ctx context.Context, studentID uuid.UUID) ([]MasteryRow, error)
	// RecentQuestions lists the questions delivered to a student since t (avoid_recent).
	RecentQuestions(ctx context.Context, studentID uuid.UUID, since time.Time) ([]uuid.UUID, error)
	// LastDiagnostics returns the start of the latest diagnostic per subject and per topic of a student.
	LastDiagnostics(ctx context.Context, studentID uuid.UUID) (map[uuid.UUID]time.Time, error)
	// LearnedTopics returns the topics of a subject where a student has evidence, most recent evidence first (interleaving).
	LearnedTopics(ctx context.Context, studentID, subjectID uuid.UUID) ([]uuid.UUID, error)
	// DiagnosticOutcomes returns, per objective, the outcomes o ∈ [0, 1] of the last scored answer of each item of an
	// attempt; an item whose last scored answer is a rapid guess is left out (LEARNING_ANALYTICS §3.5 step 2).
	DiagnosticOutcomes(ctx context.Context, attemptID uuid.UUID) (map[uuid.UUID][]float64, error)
	// ReEvaluatedCorrect lists the question versions of the items an effective re-evaluation of an attempt counts as
	// correct or dropped (they leave the review list).
	ReEvaluatedCorrect(ctx context.Context, attemptID uuid.UUID) ([]uuid.UUID, error)
}
