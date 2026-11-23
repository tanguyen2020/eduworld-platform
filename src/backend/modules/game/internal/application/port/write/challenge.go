package write

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/challenge"
)

// ItemChallenge is the challenge item type of the release core (F076).
const ItemChallenge = "challenge"

// AttemptRef identifies an attempt found by a platform job.
type AttemptRef struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	StudentID uuid.UUID
}

// ChallengeStore writes the challenge attempts of a student in the tenant transaction of ctx (explicit tenant
// conditions; RLS as well).
type ChallengeStore interface {
	// LockStudent serializes the challenge writes of one student (pg_advisory_xact_lock).
	LockStudent(ctx context.Context, student uuid.UUID) error
	// ActiveOf returns the attempt in progress of a student in tenant (locked), nil when none.
	ActiveOf(ctx context.Context, tenant, student uuid.UUID) (*challenge.Attempt, error)
	// ByID returns an attempt of tenant (locked), nil when unknown.
	ByID(ctx context.Context, tenant, id uuid.UUID) (*challenge.Attempt, error)
	// ByLearningAttempt returns the attempt in progress that opened a learning attempt (locked), nil when none.
	ByLearningAttempt(ctx context.Context, tenant, attemptID uuid.UUID) (*challenge.Attempt, error)
	// Insert adds an attempt; false when the unique "one in progress per student" index refuses it.
	Insert(ctx context.Context, a *challenge.Attempt) (bool, error)
	// Save writes the changed state (optimistic version check).
	Save(ctx context.Context, a *challenge.Attempt) error
	// Cleared reports a succeeded attempt of the student on the challenge other than except.
	Cleared(ctx context.Context, tenant, student, challengeID, except uuid.UUID) (bool, error)
	// Due lists attempts in progress whose due time (deadline_at, or started_at + resume for no time limit) is before
	// before (platform scope, job challenge-deadline).
	Due(ctx context.Context, before time.Time, resume time.Duration, limit int) ([]AttemptRef, error)
	// ActiveOfStudentAnyTenant lists the attempts in progress of a student (tenant nil = every tenant: platform event).
	ActiveOfStudentAnyTenant(ctx context.Context, student uuid.UUID, tenant *uuid.UUID) ([]*challenge.Attempt, error)
}
