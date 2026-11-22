package write

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
)

// ItemQuest is the quest item type of the release core (F072).
const ItemQuest = "quest"

// StepAttempt is one row of game.quest_step_attempts: a learning attempt opened for a quest step.
type StepAttempt struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	ProgressID       uuid.UUID
	StepID           string
	StudentID        uuid.UUID
	AttemptID        uuid.UUID
	ContentVersionID *uuid.UUID
	StartedOn        string // YYYY-MM-DD in the reset time zone (max_attempts_per_day)
	Status           string // in_progress, passed, not_passed
	CorrectCount     *int
	ItemCount        *int
	GradedAt         *time.Time
}

// Step attempt statuses.
const (
	AttemptInProgress = "in_progress"
	AttemptPassed     = "passed"
	AttemptNotPassed  = "not_passed"
)

// ProgressRef identifies an active progress found by a platform job.
type ProgressRef struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	StudentID uuid.UUID
}

// QuestStore writes the quest progress of a student in the tenant transaction of ctx (explicit tenant conditions).
type QuestStore interface {
	// LockStudent serializes the quest writes of one student (pg_advisory_xact_lock).
	LockStudent(ctx context.Context, student uuid.UUID) error
	// Active returns the active progress of the student on a quest (locked), nil when none.
	Active(ctx context.Context, tenant, student, questID uuid.UUID) (*quest.Progress, error)
	// ByID returns a progress of tenant (locked), nil when unknown.
	ByID(ctx context.Context, tenant, id uuid.UUID) (*quest.Progress, error)
	// ActiveOfStudent returns every active progress of a student (locked).
	ActiveOfStudent(ctx context.Context, tenant, student uuid.UUID) ([]*quest.Progress, error)
	// CountActive counts the active progress of a student.
	CountActive(ctx context.Context, tenant, student uuid.UUID) (int, error)
	// Insert adds a new progress; false when the unique "one active per quest" or "one per period" index refuses it.
	Insert(ctx context.Context, p *quest.Progress) (bool, error)
	// Save writes the status, step states and times (optimistic version check).
	Save(ctx context.Context, p *quest.Progress) error
	// InsertAttempt links a learning attempt to a step.
	InsertAttempt(ctx context.Context, a StepAttempt) error
	// AttemptByAttemptID returns the step attempt of a learning attempt (locked), nil when the attempt is not a quest step.
	AttemptByAttemptID(ctx context.Context, tenant, attemptID uuid.UUID) (*StepAttempt, error)
	// SaveAttempt writes the graded result of a step attempt.
	SaveAttempt(ctx context.Context, a StepAttempt) error
	// CountAttemptsOn counts the attempts started for a step on day.
	CountAttemptsOn(ctx context.Context, tenant, progressID uuid.UUID, stepID, day string) (int, error)
	// InsertGrant records the quests granted for a period; false when the period was already granted.
	InsertGrant(ctx context.Context, tenant, student uuid.UUID, kind, period string, questIDs []uuid.UUID, at time.Time) (bool, error)
	// Expiring lists active progress whose expires_at is before before (platform scope, job quest-period-expiry).
	Expiring(ctx context.Context, before time.Time, limit int) ([]ProgressRef, error)
	// Overdue lists active progress whose deadline_at is before before (platform scope, job quest-failure-timer).
	Overdue(ctx context.Context, before time.Time, limit int) ([]ProgressRef, error)
	// ActiveEndingBy lists active progress on versions whose availability ends before before (platform scope).
	ActiveEndingBy(ctx context.Context, before time.Time, limit int) ([]ProgressRef, error)
}

// AssignmentStore writes infra.tenant_resource_assignments under the platform scope (seed; saga F045 later).
type AssignmentStore interface {
	// Assign creates the active assignment of a resource to a tenant once; false when it already exists.
	Assign(ctx context.Context, resourceType string, resourceID, tenant, by uuid.UUID, at time.Time) (bool, error)
	// TenantByCode returns the projected tenant with code (nil when not projected yet).
	TenantByCode(ctx context.Context, code string) (*uuid.UUID, error)
}
