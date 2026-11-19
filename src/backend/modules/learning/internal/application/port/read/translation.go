package read

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
)

// TranslationJobReadModel reads content.translation_jobs with the scope of ctx.
type TranslationJobReadModel interface {
	Job(ctx context.Context, id uuid.UUID) (dto.TranslationJobView, error)
}
