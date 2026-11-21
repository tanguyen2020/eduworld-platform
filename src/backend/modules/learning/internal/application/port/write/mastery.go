package write

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/mastery"
)

// ModelConfig is a row of learning.mastery_model_configs (ADR-0017): the parameters of a model version.
type ModelConfig struct {
	ID           uuid.UUID
	ModelVersion string
	Algorithm    string
	Params       json.RawMessage
	Status       string
}

// MasteryRow is a row of learning.objective_masteries: the stored state of (student, objective) and its row version.
type MasteryRow struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	SchoolID     uuid.UUID
	StudentID    uuid.UUID
	ObjectiveID  uuid.UUID
	State        mastery.State
	Level        mastery.Level
	ModelVersion string
	Version      int64
	// PriorAttemptID is prior_diagnostic_attempt_id: the diagnostic that set the prior (F032).
	PriorAttemptID *uuid.UUID
}

// HistoryRow is a row of learning.mastery_history (append-only).
type HistoryRow struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	StudentID     uuid.UUID
	ObjectiveID   uuid.UUID
	ValueBefore   *float64
	ValueAfter    float64
	LevelBefore   *mastery.Level
	LevelAfter    mastery.Level
	EvidenceCount int
	SourceEventID *uuid.UUID
	SourceKind    string // evidence, re_evaluation, recompute
	ModelVersion  string
	OccurredAt    time.Time
}

// EvidenceRow is a scored, non-rapid question.answered learning event bound to objectives, with what the model needs.
type EvidenceRow struct {
	EventID      uuid.UUID
	OccurredAt   time.Time
	ObjectiveIDs []uuid.UUID
	Mode         string
	QuestionType string
	Difficulty   int
	Raw, Max     float64
	HintsUsed    int
	OptionCount  int
	Repeat       bool
}

// AttemptFacts are the facts of a graded attempt the consumers need (student, school, mode, subject, duration).
type AttemptFacts struct {
	AttemptID  uuid.UUID
	StudentID  uuid.UUID
	SchoolID   uuid.UUID
	Mode       string
	SubjectID  uuid.UUID // uuid.Nil = no subject
	GradedAt   time.Time
	DurationMS int64
	// StartedAt and RecommendationID (F032): the start of the attempt and the recommendation it followed.
	StartedAt        time.Time
	RecommendationID *uuid.UUID
}

// ItemCounts are the first scored answers of an attempt (accuracy counts on the first answer, LEARNING_ANALYTICS §4).
type ItemCounts struct {
	Answered, Correct, RapidGuess, WithHint int
	// CorrectRapid are the correct answers that were rapid guesses (accuracy excludes them, F043-AC-05).
	CorrectRapid int
}

// MasteryStore persists mastery in the TenantScope of ctx (RLS T): masteries with optimistic row versions, the history.
type MasteryStore interface {
	// ActiveModel returns the configuration with status active (exactly one, F031-AC-12).
	ActiveModel(ctx context.Context) (ModelConfig, error)
	// Mastery returns the live row of (student, objective) or nil.
	Mastery(ctx context.Context, studentID, objectiveID uuid.UUID) (*MasteryRow, error)
	// InsertMastery inserts a new row; false when a row of (student, objective) appeared meanwhile.
	InsertMastery(ctx context.Context, r MasteryRow) (bool, error)
	// UpdateMastery writes r when the stored row version still is r.Version (bumped); false otherwise.
	UpdateMastery(ctx context.Context, r MasteryRow) (bool, error)
	AppendHistory(ctx context.Context, h HistoryRow) error
}

// EvidenceReader reads the learning record of module learning (same database, TenantScope of ctx).
type EvidenceReader interface {
	// Attempt returns the facts of an attempt (problem NOT_FOUND when invisible).
	Attempt(ctx context.Context, attemptID uuid.UUID) (AttemptFacts, error)
	// AttemptObjectives lists the objectives of the scored, non-rapid answers of an attempt.
	AttemptObjectives(ctx context.Context, attemptID uuid.UUID) ([]uuid.UUID, error)
	// ObjectiveEvidence lists every evidence of (student, objective) in occurred_at order.
	ObjectiveEvidence(ctx context.Context, studentID, objectiveID uuid.UUID) ([]EvidenceRow, error)
	// FirstAnswers counts the first scored answers of an attempt.
	FirstAnswers(ctx context.Context, attemptID uuid.UUID) (ItemCounts, error)
}

// ProgressDelta adds to the counters of learning.progress_daily (student × subject × day, metric_version 1).
type ProgressDelta struct {
	TenantID, StudentID, SchoolID, SubjectID uuid.UUID
	ClassID                                  *uuid.UUID
	// GradeLevel is the grade of the student on the day (ADR-0095); nil = read from ref.students.
	GradeLevel          *int
	Day                 time.Time // the calendar day (date part used)
	ActivitiesCompleted int
	Items               ItemCounts
	ActiveSeconds       int64
	LessonsCompleted    int
}

// ProgressStore upserts progress_daily counters in the TenantScope of ctx.
type ProgressStore interface {
	AddDaily(ctx context.Context, d ProgressDelta) error
}

// PartitionMaintenance keeps the monthly partitions of the partitioned learning tables (role learning_partition_admin,
// SECURITY DEFINER functions of the F031 migration).
type PartitionMaintenance interface {
	// EnsurePartitions creates the missing partitions of months [month of from, + months) and returns how many.
	EnsurePartitions(ctx context.Context, from time.Time, months int) (int, error)
	// MonthsAhead counts the consecutive monthly partitions of table from the month of from (that month included).
	MonthsAhead(ctx context.Context, table string, from time.Time) (int, error)
}

// PartitionedTables are the monthly partitioned tables of module learning.
var PartitionedTables = []string{"learning.learning_events", "assessment.attempt_answers", "learning.mastery_history", "learning.progress_daily"}
