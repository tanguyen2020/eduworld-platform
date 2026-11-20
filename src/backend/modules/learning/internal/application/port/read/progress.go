package read

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/mastery"
)

// MasteryModelConfig is the active mastery model version and its parameters (learning.mastery_model_configs).
type MasteryModelConfig struct {
	ModelVersion string
	Params       json.RawMessage
}

// ObjectiveMastery is the stored mastery of one objective of a student (value before decay).
type ObjectiveMastery struct {
	ObjectiveID uuid.UUID
	State       mastery.State
}

// ProgressTotals are the sums of progress_daily of a student (LEARNING_ANALYTICS §4).
type ProgressTotals struct {
	LessonsCompleted int
	ItemsAnswered    int
	ItemsCorrect     int
	ActiveSeconds    int64
}

// StudentRef is the projected student (ref.students) of the session tenant.
type StudentRef struct {
	StudentID  uuid.UUID
	SchoolID   uuid.UUID
	GradeLevel int
}

// ProgressReadModel reads mastery and progress in the scope of ctx (tenant + deleted_at filter, RLS T).
type ProgressReadModel interface {
	ActiveModel(ctx context.Context) (MasteryModelConfig, error)
	// Masteries lists the masteries of a student, of one subject when subjectID is set (objective → topic → subject).
	Masteries(ctx context.Context, studentID uuid.UUID, subjectID *uuid.UUID) ([]ObjectiveMastery, error)
	Totals(ctx context.Context, studentID uuid.UUID, subjectID *uuid.UUID) (ProgressTotals, error)
	// Student returns the projected student of the session tenant (nil when unknown there).
	Student(ctx context.Context, studentID uuid.UUID) (*StudentRef, error)
}
