// Package dto holds the data shapes shared by the write and read contexts of module learning (ports,
// commands, queries, presentation) without behaviour; rules live in the domain.
package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/curriculum"
)

// VersionRef is a short reference to a curriculum version.
type VersionRef struct {
	ID        uuid.UUID
	VersionNo int
	Label     string
	Status    contentmodel.Status
}

// Curriculum is a curriculum (curriculum.curricula) with its current and latest version.
type Curriculum struct {
	ID               uuid.UUID
	Code             string
	Name             map[string]string
	Framework        *string
	Owner            contentmodel.Owner
	CurrentVersionID *uuid.UUID
	LatestVersionNo  int
	RowVersion       int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Current          *VersionRef
	Latest           *VersionRef
	// Clone origin (ADR-0029), set once by the clone step.
	ClonedFromID       *uuid.UUID
	ClonedFromTenantID *uuid.UUID
	ClonedAt           *time.Time
	ClonedBy           *uuid.UUID
}

// SubjectGrade is a subject × grade pair of a version.
type SubjectGrade struct {
	ID             uuid.UUID
	SubjectID      uuid.UUID
	GradeLevel     int
	PeriodsPerYear *int
}

// CurriculumVersion is a version of a curriculum (curriculum.curriculum_versions).
type CurriculumVersion struct {
	ID              uuid.UUID
	CurriculumID    uuid.UUID
	VersionNo       int
	Label           string
	Status          contentmodel.Status
	EffectiveFrom   *time.Time
	EffectiveTo     *time.Time
	SupersedesID    *uuid.UUID
	BasedOnID       *uuid.UUID
	ChangeNote      *string
	ContentLanguage string
	AuthorID        uuid.UUID
	ReviewerID      *uuid.UUID
	PublishedAt     *time.Time
	Owner           contentmodel.Owner
	RowVersion      int64
	UpdatedAt       time.Time
	IsCurrent       bool
	SubjectGrades   []SubjectGrade
	Warnings        []curriculum.Warning
}

// Subject is an entry of the platform subject catalog.
type Subject struct {
	ID          uuid.UUID
	Code        string
	Name        map[string]string
	GradeLevels []int
	IconKey     *string
	RowVersion  int64
}

// Topic is a topic of a version.
type Topic struct {
	ID           uuid.UUID
	CurriculumID uuid.UUID
	VersionID    uuid.UUID
	SubjectID    uuid.UUID
	GradeLevel   int
	ParentID     *uuid.UUID
	Code         string
	Name         string
	SortOrder    int
	Owner        contentmodel.Owner
	RowVersion   int64
}

// Objective is a learning objective of a version.
type Objective struct {
	ID              uuid.UUID
	CurriculumID    uuid.UUID
	VersionID       uuid.UUID
	TopicID         uuid.UUID
	Code            string
	StableKey       string
	Description     string
	BloomLevel      *string
	YccdRef         *string
	CompetencyIDs   []uuid.UUID
	PrerequisiteIDs []uuid.UUID
	SortOrder       int
	Owner           contentmodel.Owner
	RowVersion      int64
	// Read side only: subject and grade of the topic.
	SubjectID  uuid.UUID
	GradeLevel int
}

// Competency is a competency or quality of a version.
type Competency struct {
	ID           uuid.UUID
	CurriculumID uuid.UUID
	VersionID    uuid.UUID
	Code         string
	Name         map[string]string
	Kind         string
	SubjectID    *uuid.UUID
	Owner        contentmodel.Owner
	RowVersion   int64
}

// TextbookSeries is a textbook series (metadata only).
type TextbookSeries struct {
	ID        uuid.UUID
	Code      string
	Name      string
	Publisher *string
}

// Textbook is a volume of a series for a subject × grade.
type Textbook struct {
	ID         uuid.UUID
	SeriesID   uuid.UUID
	SubjectID  uuid.UUID
	GradeLevel int
	Volume     int
}

// TextbookSection is a lesson of a textbook and the LOs mapped to it.
type TextbookSection struct {
	ID           uuid.UUID
	TextbookID   uuid.UUID
	Chapter      *string
	LessonNo     int
	Title        string
	PageRange    *string
	ObjectiveIDs []uuid.UUID
	RowVersion   int64
}

// ImportRowError is one rejected row of an import (ContentImportJob.errors).
type ImportRowError struct {
	Row     int    `json:"row"`
	Field   string `json:"field,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

// ImportJob is a content import job (content.import_jobs).
type ImportJob struct {
	ID             uuid.UUID
	Owner          contentmodel.Owner
	Kind           string
	AssetID        *uuid.UUID
	SourceRef      *string
	DryRun         bool
	Status         string
	CreatedCount   int
	RejectedCount  int
	Errors         []ImportRowError
	Result         map[string]any
	IdempotencyKey string
	CreatedBy      *uuid.UUID
	CreatedAt      time.Time
	FinishedAt     *time.Time
}
