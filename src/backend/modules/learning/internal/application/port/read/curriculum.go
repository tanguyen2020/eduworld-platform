package read

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/curriculum"
)

// TopicFilter selects topics.
type TopicFilter struct {
	VersionIDs []uuid.UUID // required (one or more versions)
	SubjectID  *uuid.UUID
	GradeLevel *int
}

// ObjectiveFilter selects learning objectives; at least one of VersionIDs, SectionID or IDs is set.
type ObjectiveFilter struct {
	VersionIDs []uuid.UUID
	IDs        []uuid.UUID
	TopicID    *uuid.UUID
	SubjectID  *uuid.UUID
	GradeLevel *int
	SectionID  *uuid.UUID
	YccdRef    *string
}

// TextbookFilter selects textbooks.
type TextbookFilter struct {
	SeriesID   *uuid.UUID
	SubjectID  *uuid.UUID
	GradeLevel *int
}

// CurriculumReadModel reads the curriculum structure with the scope of ctx (entx filter: tenant rows and
// assigned platform rows, never deleted rows).
type CurriculumReadModel interface {
	// Curricula lists the visible curricula (ownerScope "" = both) with their current and latest version refs.
	Curricula(ctx context.Context, ownerScope string) ([]dto.Curriculum, error)
	Curriculum(ctx context.Context, id uuid.UUID) (dto.Curriculum, error)
	// Versions lists the versions of a curriculum, newest first, with their subject × grade pairs.
	Versions(ctx context.Context, curriculumID uuid.UUID) ([]dto.CurriculumVersion, error)
	Version(ctx context.Context, curriculumID uuid.UUID, versionNo int) (dto.CurriculumVersion, error)
	// VersionsByID returns the visible versions of ids.
	VersionsByID(ctx context.Context, ids []uuid.UUID) ([]dto.CurriculumVersion, error)
	Subjects(ctx context.Context, gradeLevel *int) ([]dto.Subject, error)
	Topics(ctx context.Context, f TopicFilter) ([]dto.Topic, error)
	Objectives(ctx context.Context, f ObjectiveFilter) ([]dto.Objective, error)
	Competencies(ctx context.Context, versionID uuid.UUID) ([]dto.Competency, error)
	Mappings(ctx context.Context, versionID uuid.UUID) (curriculum.MappingSet, error)
	TextbookSeries(ctx context.Context) ([]dto.TextbookSeries, error)
	Textbooks(ctx context.Context, f TextbookFilter) ([]dto.Textbook, error)
	Sections(ctx context.Context, textbookID uuid.UUID) ([]dto.TextbookSection, error)
	ImportJob(ctx context.Context, id uuid.UUID) (dto.ImportJob, error)
}

// SchoolCurriculumSettings reads the default curriculum version of a school (lms.school_curriculum_settings,
// owner F040) and where a principal studies (ref.students).
type SchoolCurriculumSettings interface {
	// DefaultVersion returns the version set for the school (and grade, preferred over the all-grade row), or
	// nil when the school has no setting or the table does not exist yet (F040).
	DefaultVersion(ctx context.Context, schoolID uuid.UUID, gradeLevel *int) (*uuid.UUID, error)
	// StudentSchool returns the school and grade of a student of the projection, or nil.
	StudentSchool(ctx context.Context, studentID uuid.UUID) (*uuid.UUID, *int, error)
}
