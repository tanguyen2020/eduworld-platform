package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// Doc is the item of a composed content type of F024 (question set, lesson, learning material, assessment
// blueprint): owner, release pointer, latest version number, clone origin.
type Doc struct {
	Type               contentmodel.ContentType
	ID                 uuid.UUID
	Owner              contentmodel.Owner
	CurrentVersionID   *uuid.UUID
	LatestVersionNo    int
	RowVersion         int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
	ClonedFromID       *uuid.UUID
	ClonedFromTenantID *uuid.UUID
	ClonedAt           *time.Time
	ClonedBy           *uuid.UUID
}

// SetItem is one fixed question of a question set version (content.question_set_items).
type SetItem struct {
	Position          int
	QuestionID        uuid.UUID
	QuestionVersionID uuid.UUID
	MaxScore          *float64
}

// DocVersion is one version of a composed content type: shared metadata in columns, the type-specific document in
// Body (definition, blocks, sections…).
type DocVersion struct {
	Type                contentmodel.ContentType
	ID                  uuid.UUID
	ItemID              uuid.UUID
	VersionNo           int
	Status              contentmodel.Status
	Title               json.RawMessage
	Summary             json.RawMessage
	SubjectID           *uuid.UUID
	CurriculumVersionID *uuid.UUID
	GradeLevels         []int
	TopicIDs            []uuid.UUID
	ObjectiveIDs        []uuid.UUID
	MediaRefs           []uuid.UUID
	WorkedExampleIDs    []string
	Body                json.RawMessage
	ContentLanguage     string
	AuthorID            uuid.UUID
	ReviewerID          *uuid.UUID
	PublishedAt         *time.Time
	ChangeNote          *string
	BasedOnID           *uuid.UUID
	Owner               contentmodel.Owner
	RowVersion          int64
	UpdatedAt           time.Time
	Items               []SetItem // question sets only
}
