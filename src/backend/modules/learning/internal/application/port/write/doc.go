package write

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/trash"
)

// DocStore persists the composed content types of F024 (question_set, lesson, learning_material,
// assessment_blueprint) with the scope of ctx. The content type selects the table pair; errors follow QuestionStore.
type DocStore interface {
	InsertDoc(ctx context.Context, d dto.Doc) error
	Doc(ctx context.Context, t contentmodel.ContentType, id uuid.UUID) (dto.Doc, error)
	// UpdateDoc saves latest_version_no and current_version_id when the row version is ifRow.
	UpdateDoc(ctx context.Context, d dto.Doc, ifRow int64) (dto.Doc, error)
	SoftDeleteDoc(ctx context.Context, t contentmodel.ContentType, id uuid.UUID, reason string) error

	// InsertDocVersion inserts a version (and the items of a question set).
	InsertDocVersion(ctx context.Context, v dto.DocVersion) error
	DocVersion(ctx context.Context, t contentmodel.ContentType, itemID uuid.UUID, versionNo int) (dto.DocVersion, error)
	// DocVersions lists the versions of an item, newest first.
	DocVersions(ctx context.Context, t contentmodel.ContentType, itemID uuid.UUID) ([]dto.DocVersion, error)
	// UpdateDocVersion saves the content of a draft (and replaces the items of a question set).
	UpdateDocVersion(ctx context.Context, v dto.DocVersion, ifRow int64) (dto.DocVersion, error)

	// DeletedDocs and RestoreDoc serve the trash (ctx carries tenancy.Bypass(trash)).
	DeletedDocs(ctx context.Context, t contentmodel.ContentType, f port.TrashFilter) ([]trash.Entry, error)
	RestoreDoc(ctx context.Context, t contentmodel.ContentType, id uuid.UUID) (trash.Entry, error)
}
