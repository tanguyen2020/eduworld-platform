package read

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/mastery"
)

// ExperimentRow is a row of learning.experiments.
type ExperimentRow = write.ExperimentRow

// LiveArm is the arm of a student in a live mastery_model experiment that is not the control: the model of the arm and
// the mastery states computed with it (F035-AC-24).
type LiveArm struct {
	ExperimentCode string
	ArmCode        string
	ModelVersion   string
	Params         json.RawMessage
	States         map[uuid.UUID]mastery.State
}

// ExperimentReadModel reads the experiments (F035 US5).
type ExperimentReadModel interface {
	Experiments(ctx context.Context, status, kind string) ([]ExperimentRow, error)
	Experiment(ctx context.Context, id uuid.UUID) (*ExperimentRow, error)
	LatestResult(ctx context.Context, id uuid.UUID) (json.RawMessage, *time.Time, bool, error)
}

// ArmReader reads the live arm of a student for the student experience (getMyMastery); nil = no experiment.
type ArmReader interface {
	LiveArm(ctx context.Context, tenant, student uuid.UUID) (*LiveArm, error)
}
