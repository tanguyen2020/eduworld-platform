package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/challenge"
)

// ChallengeRow is a published challenge version (current or pinned).
type ChallengeRow struct {
	ChallengeID uuid.UUID
	VersionID   uuid.UUID
	VersionNo   int
	Version     challenge.Version
}

// ChallengeFilter is the database filter of listGameChallenges (F076-AC-01): the challenges assigned to the tenant
// whose current version serves the grade, is playable solo, is open at Now, of the type/area given; keyset on
// challenge_id after After, at most Limit rows (the caller asks limit + 1).
type ChallengeFilter struct {
	Tenant uuid.UUID
	Grade  int
	Type   string
	AreaID string
	Now    time.Time
	After  *uuid.UUID
	Limit  int
}

// ChallengeUsage is what a student did on a challenge in a period (attempts counted, last failure, first clear).
type ChallengeUsage struct {
	Attempts     int
	LastFailedAt *time.Time
	LastFailedOn string     // difficulty of the last failed attempt
	Cleared      bool       // a succeeded attempt exists (any version, any period)
	Active       *uuid.UUID // the in_progress attempt of the student for this challenge (active_attempt_id, delta F076)
}

// ChallengeContent reads the published challenges, their tenant assignment and the attempts of a student in the
// transaction of ctx (read or write): explicit tenant and soft-delete conditions (R27).
type ChallengeContent interface {
	// AvailableChallenges is the keyset page of ChallengeFilter (database paging, never sliced in memory).
	AvailableChallenges(ctx context.Context, f ChallengeFilter) ([]ChallengeRow, error)
	// Assigned reports a challenge assigned to tenant (active assignment).
	Assigned(ctx context.Context, tenant, challengeID uuid.UUID) (bool, error)
	// CurrentChallenge returns the current version of a challenge, nil when unknown or unpublished.
	CurrentChallenge(ctx context.Context, challengeID uuid.UUID) (*ChallengeRow, error)
	// ChallengeVersion returns a version by id (pinned versions stay readable after a new publish).
	ChallengeVersion(ctx context.Context, versionID uuid.UUID) (*ChallengeRow, error)
	// KnownChallenges returns which of ids are published challenges (references of other content: live-ops events).
	KnownChallenges(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error)
	// Usage returns the usage of a challenge by a student in tenant for a period.
	Usage(ctx context.Context, tenant, student, challengeID uuid.UUID, period string) (ChallengeUsage, error)
	// Attempt returns an attempt of tenant (nil when unknown).
	Attempt(ctx context.Context, tenant, id uuid.UUID) (*challenge.Attempt, error)
}

// ChallengeLearningAttempt is the request of createAttempt for a challenge activity (facade learning, mode challenge).
type ChallengeLearningAttempt struct {
	StudentID          uuid.UUID
	ActivityID         uuid.UUID
	ChallengeAttemptID uuid.UUID
	DifficultyMin      int
	DifficultyMax      int
	GradeLevel         int
	ObjectiveIDs       []uuid.UUID
	QuestionCount      *int
	TimeLimitSeconds   *int
	GameSessionID      *uuid.UUID
}

// ChallengeLearning is createAttempt of the learning facade for a challenge activity (x-allowed-callers: game; deadline,
// TenantScope, original Principal and Idempotency-Key through pkg/modcall). ErrLearningUnavailable when it cannot
// answer.
type ChallengeLearning interface {
	CreateChallengeAttempt(ctx context.Context, in ChallengeLearningAttempt) (LearningAttemptCreated, error)
}

// ChallengeRealtime pushes challenge.attempt.updated to the connections of a student (best effort after commit).
type ChallengeRealtime interface {
	ChallengeAttemptUpdated(ctx context.Context, student uuid.UUID, payload map[string]any, at time.Time) error
}

// ChallengeRoom is a room of the shared room host (pkg/roomhost) for an attempt in progress: announced at start (an
// owner node takes it and fires its timer at the due time), closed when the attempt ends otherwise. Best effort: the
// job challenge-deadline ends a due attempt from PostgreSQL when Valkey is down (F076-AC-19).
type ChallengeRoom interface {
	Open(ctx context.Context, a *challenge.Attempt, due time.Time) error
	Close(ctx context.Context, attemptID uuid.UUID) error
}
