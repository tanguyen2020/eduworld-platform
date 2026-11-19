package read

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// DocFilter selects the items of a CMS list of a composed content type (F024-AC-17): full-text q (accent and case
// insensitive), metadata filters, newest version status; keyset after After.
type DocFilter struct {
	Q           string
	SubjectID   *uuid.UUID
	GradeLevel  *int
	TopicID     *uuid.UUID
	ObjectiveID *uuid.UUID
	Status      string
	// ActivityType filters activity versions on their generated column activity_type (F030).
	ActivityType string
	After        *uuid.UUID
	Limit        int
}

// DocHit is an item with its newest version.
type DocHit struct {
	Doc     dto.Doc
	Version dto.DocVersion
}

// DocReadModel reads the composed content types with the scope of ctx.
type DocReadModel interface {
	Doc(ctx context.Context, t contentmodel.ContentType, id uuid.UUID) (dto.Doc, error)
	Version(ctx context.Context, t contentmodel.ContentType, itemID uuid.UUID, versionNo int) (dto.DocVersion, error)
	// Latest returns the newest version of an item.
	Latest(ctx context.Context, t contentmodel.ContentType, itemID uuid.UUID) (dto.DocVersion, error)
	// Search lists items (newest first) whose newest version matches f.
	Search(ctx context.Context, t contentmodel.ContentType, f DocFilter) ([]DocHit, error)
	// Items returns the fixed items of question set versions.
	Items(ctx context.Context, versionIDs []uuid.UUID) (map[uuid.UUID][]dto.SetItem, error)
}
