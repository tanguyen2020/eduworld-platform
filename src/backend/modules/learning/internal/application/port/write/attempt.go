package write

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/attempt"
)

// AnswerRow is one row of assessment.attempt_answers (append-only, server times, F030-AC-08).
type AnswerRow struct {
	ID                uuid.UUID
	TenantID          uuid.UUID
	SchoolID          uuid.UUID
	AttemptID         uuid.UUID
	Position          int
	QuestionVersionID uuid.UUID
	Response          json.RawMessage
	IsCorrect         bool
	Score             float64
	AnsweredAt        time.Time
	ClientElapsedMS   *int64
	ResponseMS        int64
	AttemptNo         int
	IsScored          bool
	IdempotencyKey    string
}

// EvaluationRow is one row of assessment.evaluations.
type EvaluationRow struct {
	ID        uuid.UUID
	AttemptID uuid.UUID
	Source    string
	Status    string
	Score     float64
	MaxScore  float64
	Passed    *bool
	Details   json.RawMessage
	Effective bool
	// Re-evaluation (F031 US4): the request, the evaluation it replaces, the staff user and the reason (≤ 500).
	RegradeRequestID *uuid.UUID
	SupersedesID     *uuid.UUID
	TriggeredBy      *uuid.UUID
	Reason           *string
}

// DueAttempt is an in-progress attempt the lifecycle job must look at (idle, resume window or deadline).
type DueAttempt struct {
	ID       uuid.UUID
	TenantID uuid.UUID
}

// AttemptStore persists the Attempt aggregate (assessment.attempts, attempt_items, attempt_answers, evaluations) in
// the scope of ctx (TenantScope of the student; RLS T).
type AttemptStore interface {
	// InsertAttempt inserts an attempt and its items.
	InsertAttempt(ctx context.Context, a attempt.Attempt) error
	// Attempt loads an attempt with its items (problem ATTEMPT_NOT_FOUND when invisible).
	Attempt(ctx context.Context, id uuid.UUID) (attempt.Attempt, error)
	// AttemptByKey finds the attempt a student started with an Idempotency-Key (nil when none).
	AttemptByKey(ctx context.Context, studentID uuid.UUID, key string) (*attempt.Attempt, error)
	// OpenPractice finds the in-progress practice attempt of a student on a source version (nil when none).
	OpenPractice(ctx context.Context, studentID, sourceVersionID uuid.UUID) (*attempt.Attempt, error)
	// UpdateAttempt writes the mutable columns when the row version still is ifRow (problem VERSION_MISMATCH otherwise).
	UpdateAttempt(ctx context.Context, a attempt.Attempt, ifRow int64) error
	// UpdateItem writes answer_count, final_* and hints_revealed of an item (optimistic on its row version).
	UpdateItem(ctx context.Context, it attempt.Item) error
	InsertAnswer(ctx context.Context, r AnswerRow) error
	InsertEvaluation(ctx context.Context, r EvaluationRow) error
	// StartedSince counts the attempts a student started since t (attempt_start_rate).
	StartedSince(ctx context.Context, studentID uuid.UUID, since time.Time) (int, error)
	// Due lists in-progress attempts of every tenant (ctx carries tenancy.Bypass(platform_job)): not paused and inactive
	// since before idleBefore, inactive since before resumeBefore, or past deadlineBefore; oldest first.
	Due(ctx context.Context, idleBefore, resumeBefore, deadlineBefore time.Time, limit int) ([]DueAttempt, error)
}

// LearningEvent is one row of learning.learning_events (EVENT_DESIGN §5, learning-event.schema.json): written in the
// transaction of the change it records; a replay with the same idempotency key adds no row.
type LearningEvent struct {
	ID               uuid.UUID
	OccurredAt       time.Time
	Type             string
	StudentID        uuid.UUID
	TenantID         uuid.UUID
	SchoolID         uuid.UUID
	ClassID          *uuid.UUID
	GameSessionID    *uuid.UUID
	ActivityID       *uuid.UUID
	ContentID        *uuid.UUID
	ContentVersionID *uuid.UUID
	ObjectiveIDs     []uuid.UUID
	Context          string
	ActorType        string
	ActorID          *uuid.UUID // staff user of a staff event (nil: the student, or the zero UUID for system events)
	Payload          any
	IdempotencyKey   string
}

// LearningEventWriter appends learning events (INSERT … ON CONFLICT DO NOTHING on the idempotency key).
type LearningEventWriter interface {
	Write(ctx context.Context, e LearningEvent) error
}
