package write

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// SimulationRun is one row of assessment.simulation_runs (F066-AC-11…AC-14): immutable once written.
type SimulationRun struct {
	ID                 uuid.UUID
	TenantID           uuid.UUID
	SchoolID           uuid.UUID
	AttemptID          uuid.UUID
	StudentID          uuid.UUID
	RunNo              int
	TemplateCode       string
	TemplateVersion    int
	Seed               int64
	ActionLog          json.RawMessage
	ClientOutcome      json.RawMessage
	VerifiedOutcome    json.RawMessage
	GoalsMet           json.RawMessage
	VerificationStatus string
	RejectCode         string
	IdempotencyKey     string
	CreatedAt          time.Time
}

// Integrity signals of the simulation runs (ANTI_CHEAT §6: signals, never an automatic penalty).
const (
	SignalSimulationLogInvalid      = "simulation_log_invalid"
	SignalSimulationOutcomeMismatch = "simulation_outcome_mismatch"
)

// SimulationRunStore persists the runs of the simulation attempts in the transaction of ctx (TenantScope; RLS T).
type SimulationRunStore interface {
	// Runs returns the runs of an attempt by run_no.
	Runs(ctx context.Context, attemptID uuid.UUID) ([]SimulationRun, error)
	// InsertRun records a run; the same Idempotency-Key or run_no twice is problem CONFLICT.
	InsertRun(ctx context.Context, r SimulationRun) error
}
