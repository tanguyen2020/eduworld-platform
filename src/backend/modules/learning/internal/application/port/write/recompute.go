package write

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/mastery"
)

// RecomputeRun is a row of learning.mastery_recompute_runs (F031 US5).
type RecomputeRun struct {
	ID           uuid.UUID
	ModelVersion string
	Status       string // running, completed, failed, activated
	Report       json.RawMessage
	StartedBy    *uuid.UUID
	StartedAt    time.Time
	FinishedAt   *time.Time
}

// MasteryPair is a (student, objective) of a tenant.
type MasteryPair struct {
	TenantID    uuid.UUID
	SchoolID    uuid.UUID
	StudentID   uuid.UUID
	ObjectiveID uuid.UUID
}

// ShadowRow is a row of learning.objective_mastery_shadows: the state of a pair under the model of a run.
type ShadowRow struct {
	RunID        uuid.UUID
	Pair         MasteryPair
	State        mastery.State
	Level        mastery.Level
	CurrentLevel mastery.Level
	ModelVersion string
}

// RecomputeStore persists model versions, recompute runs and shadow rows (explicit SQL; platform rows under
// tenancy.Bypass(platform_job), shadow rows in the TenantScope of their tenant).
type RecomputeStore interface {
	// ModelByVersion returns a model configuration (nil when unknown).
	ModelByVersion(ctx context.Context, version string) (*ModelConfig, error)
	// InsertModel adds a draft model version (false when the version exists).
	InsertModel(ctx context.Context, c ModelConfig) (bool, error)
	// SetModelStatus moves a model version from draft/shadow to status.
	SetModelStatus(ctx context.Context, version, status string) error
	// ActivateModel retires the active version and activates version (exactly one active, F031-AC-12).
	ActivateModel(ctx context.Context, version string, by uuid.UUID, at time.Time) error
	InsertRun(ctx context.Context, r RecomputeRun) error
	FinishRun(ctx context.Context, id uuid.UUID, status string, report json.RawMessage, at time.Time) error
	// Run returns a run (nil when unknown); MarkRunActivated records its activation.
	Run(ctx context.Context, id uuid.UUID) (*RecomputeRun, error)
	MarkRunActivated(ctx context.Context, id, by uuid.UUID, at time.Time) error
	// Pairs lists every live (student, objective) mastery of every tenant; ShadowPairs the pairs of a run.
	Pairs(ctx context.Context) ([]MasteryPair, error)
	ShadowPairs(ctx context.Context, runID uuid.UUID) ([]MasteryPair, error)
	InsertShadow(ctx context.Context, r ShadowRow) error
}
