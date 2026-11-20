package read

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/videolesson"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/attempt"
)

// AttemptFilter filters the attempts of a student (listMyActivityAttempts).
type AttemptFilter struct {
	Mode         string
	Status       string
	ActivityType string
	After        *uuid.UUID
	Limit        int
}

// SavedAnswer is the last answer of an item (resume and review).
type SavedAnswer struct {
	Position  int
	Response  json.RawMessage
	IsCorrect *bool
	Score     *float64
	IsScored  bool
}

// AttemptReadModel reads attempts in the scope of ctx (tenant + deleted_at filter, RLS T).
type AttemptReadModel interface {
	Attempt(ctx context.Context, id uuid.UUID) (attempt.Attempt, error)
	// List returns the attempts of a student, newest first (keyset after an attempt id).
	List(ctx context.Context, studentID uuid.UUID, f AttemptFilter) ([]attempt.Attempt, error)
	// LastAnswers returns the newest answer of every answered position.
	LastAnswers(ctx context.Context, attemptID uuid.UUID) ([]SavedAnswer, error)
	// Evaluation returns the effective evaluation of an attempt (nil before grading).
	Evaluation(ctx context.Context, attemptID uuid.UUID) (*dto.EvaluationView, error)
	// ActivityVersion returns an activity and one of its versions by id (the version pinned by an attempt).
	ActivityVersion(ctx context.Context, activityID, versionID uuid.UUID) (dto.Doc, dto.DocVersion, error)
}

// MediaProgressReadModel reads the progress of the media attempts (assessment.video_progress_live, F065).
type MediaProgressReadModel interface {
	// Progress returns the progress of an attempt (nil when none).
	Progress(ctx context.Context, attemptID uuid.UUID) (*videolesson.Progress, error)
}

// SimulationRunReadModel reads the runs of a simulation attempt (assessment.simulation_runs_live, F066).
type SimulationRunReadModel interface {
	// RunCount is the number of runs stored for an attempt (runs_used).
	RunCount(ctx context.Context, attemptID uuid.UUID) (int, error)
}
