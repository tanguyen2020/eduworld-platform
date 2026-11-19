package write

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// SagaRun is one row of platform.saga_runs (F016 T006, shared with F045).
type SagaRun struct {
	ID             uuid.UUID
	Type           string
	TenantID       *uuid.UUID
	Subject        json.RawMessage
	Status         string // running | compensating | succeeded | blocked | failed
	RequestedBy    *uuid.UUID
	IdempotencyKey string
	Attempts       int
	NextAttemptAt  *time.Time
	LastError      string
	StartedAt      time.Time
	FinishedAt     *time.Time
	Steps          []SagaStep
	Version        int64
}

// SagaStep is one row of platform.saga_steps.
type SagaStep struct {
	No       int
	Service  string
	Action   string
	Status   string // pending | done | blocked | nothing_to_do | failed | compensated
	Attempts int
	Response json.RawMessage
	Error    string
}

// SagaRepository persists saga runs (platform scope, identity_platform).
type SagaRepository interface {
	// Create inserts the run and its steps; when (type, idempotency key) exists it returns the existing run
	// (created = false).
	Create(ctx context.Context, r SagaRun) (run SagaRun, created bool, err error)
	// Lock loads a run with its steps FOR UPDATE.
	Lock(ctx context.Context, id uuid.UUID) (*SagaRun, error)
	// SaveStep writes the status, attempts, response and error of one step.
	SaveStep(ctx context.Context, sagaID uuid.UUID, s SagaStep) error
	// SaveRun writes status, attempts, next attempt, last error and finished_at (version check).
	SaveRun(ctx context.Context, r *SagaRun) error
	// Due returns ids of running/compensating runs whose next attempt is due.
	Due(ctx context.Context, now time.Time, limit int) ([]uuid.UUID, error)
	// Stuck counts running/compensating runs started before the threshold (alert SagaStuck).
	Stuck(ctx context.Context, before time.Time) (int, error)
}
