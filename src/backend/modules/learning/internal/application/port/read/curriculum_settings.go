package read

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
)

// CurriculumSettingsReadModel reads the curriculum versions a school uses (lms.school_curriculum_settings, F021
// FR-CUR-008) in the scope of ctx (tenant + deleted_at, RLS).
type CurriculumSettingsReadModel interface {
	CurriculumSettings(ctx context.Context, schoolID uuid.UUID, yearID *uuid.UUID) (dto.CurriculumSettingsView, error)
}
