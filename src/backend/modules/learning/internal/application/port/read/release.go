package read

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// VersionFilter selects the versions of an item: status ("" = any), ids (nil = any) and the keyset page.
type VersionFilter struct {
	Status string
	IDs    []uuid.UUID
	Seek   Seek
}

// ReleaseReadModel reads the version history, releases and usages of learning items with the scope of ctx (F026 US5, US7).
type ReleaseReadModel interface {
	// Versions is one keyset page of the versions of an item, newest first (version_no DESC, id DESC), filtered in SQL.
	Versions(ctx context.Context, t contentmodel.ContentType, itemID uuid.UUID, f VersionFilter) ([]dto.VersionSummary, error)
	// Releases lists content_releases of an item, newest first, before the release id after (nil = first page).
	Releases(ctx context.Context, t contentmodel.ContentType, itemID uuid.UUID, after *uuid.UUID, limit int) ([]dto.ReleaseRecord, error)
	// QuestionUsages is one keyset page of the question set versions holding a version of the question: set items in
	// id order after `after`, at most limit items read (items outside the scope are skipped); returns the id of the last
	// item read and whether more items follow (API_DESIGN §5).
	QuestionUsages(ctx context.Context, questionID uuid.UUID, after *uuid.UUID, limit int) (usages []dto.ContentUsageView, last *uuid.UUID, more bool, err error)
}
