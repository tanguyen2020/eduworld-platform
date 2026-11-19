package read

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// ReleaseReadModel reads the version history, releases and usages of learning items with the scope of ctx (F026 US5, US7).
type ReleaseReadModel interface {
	// Versions lists the versions of an item, newest first.
	Versions(ctx context.Context, t contentmodel.ContentType, itemID uuid.UUID) ([]dto.VersionSummary, error)
	// Releases lists content_releases of an item, newest first, before the release id after (nil = first page).
	Releases(ctx context.Context, t contentmodel.ContentType, itemID uuid.UUID, after *uuid.UUID, limit int) ([]dto.ReleaseRecord, error)
	// QuestionUsages lists the question set versions holding a version of the question.
	QuestionUsages(ctx context.Context, questionID uuid.UUID) ([]dto.ContentUsageView, error)
}
