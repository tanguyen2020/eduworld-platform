package write

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/mastery"
)

// ExperimentRow is one row of learning.experiments (G).
type ExperimentRow struct {
	ID             uuid.UUID
	Code           string
	Kind           string
	Unit           string
	Arms           json.RawMessage
	TenantIDs      []uuid.UUID
	StudyID        *uuid.UUID
	Status         string
	StartAt        time.Time
	EndAt          time.Time
	StopReason     *string
	StopNote       *string
	Revision       int
	OwnerID        uuid.UUID
	IdempotencyKey string
	StartedAt      *time.Time
	StoppedAt      *time.Time
	Version        int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ExperimentAssignmentRow is one row of learning.experiment_assignments (T).
type ExperimentAssignmentRow struct {
	ExperimentID uuid.UUID
	StudentID    uuid.UUID
	TenantID     uuid.UUID
	UnitID       uuid.UUID
	Bucket       int
	ArmCode      string
	Status       string
	AssignedAt   time.Time
}

// ArmMetric is the primary metric and the counts of one arm (guardrail and results).
type ArmMetric struct {
	ArmCode        string
	Units          int
	Students       int
	Answers        int
	Correct        int
	LogLoss        *float64
	ActiveMinutes  float64
	ActiveStudents int
}

// ExperimentStore persists the A/B experiments (F035 US5): G rows without tenant, T rows with the tenant given.
type ExperimentStore interface {
	InsertExperiment(ctx context.Context, e ExperimentRow) error
	Experiment(ctx context.Context, id uuid.UUID, lock bool) (*ExperimentRow, error)
	ExperimentByKey(ctx context.Context, owner uuid.UUID, key string) (*ExperimentRow, error)
	ExperimentCodeTaken(ctx context.Context, code string) (bool, error)
	UpdateExperiment(ctx context.Context, e ExperimentRow, prev int64) (bool, error)
	// RunningExperiments lists the shadow and live experiments covering a tenant (every tenant when tenant is nil).
	RunningExperiments(ctx context.Context, tenant *uuid.UUID) ([]ExperimentRow, error)
	// ModelVersions lists the model_version of mastery_model_configs and the active one.
	ModelVersions(ctx context.Context) (map[string]bool, string, error)
	ModelConfig(ctx context.Context, version string) (*ModelConfig, error)
	// Assignment returns the assignment of a student (nil when none).
	Assignment(ctx context.Context, tenant, experimentID, studentID uuid.UUID) (*ExperimentAssignmentRow, error)
	InsertAssignment(ctx context.Context, a ExperimentAssignmentRow) (bool, error)
	// RemoveAssignments moves the active assignments of an experiment (of one tenant when tenant is not nil) to removed.
	RemoveAssignments(ctx context.Context, experimentID uuid.UUID, tenant *uuid.UUID) (int, error)
	// StudentCohort returns the unit id of the cohort (school, grade) of an active student (ref.students): the stable
	// md5(school_id:grade_level) UUID, ADR-0095; nil when the student is unknown.
	StudentCohort(ctx context.Context, tenant, studentID uuid.UUID) (*uuid.UUID, error)
	// InterventionCohorts lists the unit ids of the intervention cohorts of a study (same derivation).
	InterventionCohorts(ctx context.Context, studyID uuid.UUID) ([]uuid.UUID, error)
	// ArmState and UpsertArmState read and write the mastery of a student under the model of an arm.
	ArmStates(ctx context.Context, tenant, experimentID uuid.UUID, arm string, studentID uuid.UUID) (map[uuid.UUID]mastery.State, error)
	UpsertArmState(ctx context.Context, tenant, experimentID uuid.UUID, arm string, studentID, objectiveID uuid.UUID, s mastery.State,
		level mastery.Level, modelVersion string) error
	// ArmMetrics computes the metrics of the arms from the learning events tagged "<code>:<arm>" (first answers after delay).
	ArmMetrics(ctx context.Context, e ExperimentRow, delay time.Duration) ([]ArmMetric, error)
	InsertResult(ctx context.Context, experimentID uuid.UUID, at time.Time, results json.RawMessage, breached bool) error
	LatestResult(ctx context.Context, experimentID uuid.UUID) (json.RawMessage, *time.Time, bool, error)
	// DueExperiments lists the running experiments past end_at.
	DueExperiments(ctx context.Context, now time.Time) ([]ExperimentRow, error)
}
