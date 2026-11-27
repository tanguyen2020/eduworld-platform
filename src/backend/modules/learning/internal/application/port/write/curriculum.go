package write

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/curriculum"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/trash"
)

// CurriculumStore persists the curriculum structure (F021) inside entx.UnitOfWork with the scope of ctx: rows
// invisible to the scope do not exist (problem NotFound); an If-Match row version that differs is problem
// VersionMismatch; a duplicate unique key is problem Conflict; a change of a version that is not draft, caught by
// the database trigger, is problem ContentNotEditable.
type CurriculumStore interface {
	InsertCurriculum(ctx context.Context, c dto.Curriculum) error
	Curriculum(ctx context.Context, id uuid.UUID) (dto.Curriculum, error)
	// CurriculumByCode returns the live curriculum with code owned by owner, or nil.
	CurriculumByCode(ctx context.Context, owner contentmodel.Owner, code string) (*dto.Curriculum, error)
	// UpdateCurriculum saves name, framework, current_version_id and latest_version_no when the row version is ifRow.
	UpdateCurriculum(ctx context.Context, c dto.Curriculum, ifRow int64) (dto.Curriculum, error)

	InsertVersion(ctx context.Context, v dto.CurriculumVersion) error
	Version(ctx context.Context, curriculumID uuid.UUID, versionNo int) (dto.CurriculumVersion, error)
	VersionByID(ctx context.Context, id uuid.UUID) (dto.CurriculumVersion, error)
	Versions(ctx context.Context, curriculumID uuid.UUID) ([]dto.CurriculumVersion, error)
	// UpdateVersion saves label, effective range, change note, status, reviewer and published_at.
	UpdateVersion(ctx context.Context, v dto.CurriculumVersion, ifRow int64) (dto.CurriculumVersion, error)

	SubjectGrades(ctx context.Context, versionID uuid.UUID) ([]dto.SubjectGrade, error)
	InsertSubjectGrade(ctx context.Context, v dto.CurriculumVersion, sg dto.SubjectGrade) error
	DeleteSubjectGrade(ctx context.Context, id uuid.UUID) error

	InsertTopic(ctx context.Context, t dto.Topic) error
	Topic(ctx context.Context, id uuid.UUID) (dto.Topic, error)
	Topics(ctx context.Context, versionID uuid.UUID) ([]dto.Topic, error)
	UpdateTopic(ctx context.Context, t dto.Topic, ifRow int64) (dto.Topic, error)
	DeleteTopic(ctx context.Context, id uuid.UUID, ifRow int64, reason string) error

	InsertObjective(ctx context.Context, o dto.Objective) error
	Objective(ctx context.Context, id uuid.UUID) (dto.Objective, error)
	Objectives(ctx context.Context, versionID uuid.UUID) ([]dto.Objective, error)
	UpdateObjective(ctx context.Context, o dto.Objective, ifRow int64) (dto.Objective, error)
	DeleteObjective(ctx context.Context, id uuid.UUID, ifRow int64, reason string) error

	InsertCompetency(ctx context.Context, c dto.Competency) error
	Competency(ctx context.Context, id uuid.UUID) (dto.Competency, error)
	Competencies(ctx context.Context, versionID uuid.UUID) ([]dto.Competency, error)
	UpdateCompetency(ctx context.Context, c dto.Competency, ifRow int64) (dto.Competency, error)

	Mappings(ctx context.Context, versionID uuid.UUID) (curriculum.MappingSet, error)
	// ReplaceMappings soft-deletes the live mappings of the version and inserts set.
	ReplaceMappings(ctx context.Context, v dto.CurriculumVersion, set curriculum.MappingSet) error

	// RecordTransition appends content.content_transitions (content_type curriculum).
	RecordTransition(ctx context.Context, v dto.CurriculumVersion, from, to contentmodel.Status, actor uuid.UUID, note string) error
	// RecordRelease appends content.content_releases (item_type curriculum, action publish) and returns its id.
	RecordRelease(ctx context.Context, r Release) (uuid.UUID, error)
}

// Release is one row of content.content_releases.
type Release struct {
	Owner          contentmodel.Owner
	ItemType       string
	ItemID         uuid.UUID
	FromVersionID  *uuid.UUID
	ToVersionID    uuid.UUID
	ReleasedBy     uuid.UUID
	ReleasedAt     time.Time
	IdempotencyKey string
}

// CatalogStore persists the platform catalogs (subjects, textbooks): written only under PlatformScope.
type CatalogStore interface {
	InsertSubject(ctx context.Context, s dto.Subject) error
	Subject(ctx context.Context, id uuid.UUID) (dto.Subject, error)
	// SubjectsByCode returns the live subjects of codes by code.
	SubjectsByCode(ctx context.Context, codes []string) (map[string]dto.Subject, error)
	UpdateSubject(ctx context.Context, s dto.Subject, ifRow int64) (dto.Subject, error)

	InsertSeries(ctx context.Context, s dto.TextbookSeries) error
	Series(ctx context.Context, id uuid.UUID) (dto.TextbookSeries, error)
	InsertTextbook(ctx context.Context, t dto.Textbook) error
	Textbook(ctx context.Context, id uuid.UUID) (dto.Textbook, error)
	InsertSection(ctx context.Context, s dto.TextbookSection) error
	Section(ctx context.Context, id uuid.UUID) (dto.TextbookSection, error)
	UpdateSection(ctx context.Context, s dto.TextbookSection, ifRow int64) (dto.TextbookSection, error)
	// ReplaceSectionObjectives soft-deletes the links no longer listed and adds the new ones.
	ReplaceSectionObjectives(ctx context.Context, sectionID uuid.UUID, objectiveIDs []uuid.UUID) error
}

// ImportJobStore persists content.import_jobs.
type ImportJobStore interface {
	InsertImportJob(ctx context.Context, j dto.ImportJob) error
	ImportJob(ctx context.Context, id uuid.UUID) (dto.ImportJob, error)
	// ImportJobByKey returns the job of (created_by, idempotency_key) visible to the scope, or nil.
	ImportJobByKey(ctx context.Context, createdBy uuid.UUID, key string) (*dto.ImportJob, error)
	// UpdateImportJob saves status, counts, errors, result and finished_at.
	UpdateImportJob(ctx context.Context, j dto.ImportJob) error
	// QueuedImportJobs lists queued jobs of every owner, oldest first (ctx carries tenancy.Bypass(platform_job)).
	QueuedImportJobs(ctx context.Context, limit int) ([]dto.ImportJob, error)
}

// ImportItemStore persists content.import_items (F028): the questions of a lessons_docx job.
type ImportItemStore interface {
	InsertImportItems(ctx context.Context, items []dto.ImportItem) error
	// ImportItem returns the item no of a job visible to the scope (404 problem otherwise).
	ImportItem(ctx context.Context, importID uuid.UUID, no int) (dto.ImportItem, error)
	// UpdateImportItem saves status, issues and the created question.
	UpdateImportItem(ctx context.Context, it dto.ImportItem) error
}

// CurriculumTrashStore reads and restores soft-deleted curriculum rows (Thùng rác, resource_type curriculum:
// curricula, topics and objectives, F021-AC-10). ctx carries tenancy.Bypass(trash).
type CurriculumTrashStore interface {
	// SoftDeleteCurriculum soft-deletes a live curriculum visible to the scope.
	SoftDeleteCurriculum(ctx context.Context, id uuid.UUID, reason string) error
	// DeletedEntries lists deleted curricula, topics and objectives, newest deletion first, with their conflicts.
	DeletedEntries(ctx context.Context, f port.TrashFilter) ([]trash.Entry, error)
	// RestoreEntry clears deleted_* of one deleted curriculum, topic or objective: trash.ErrNotDeleted when it is
	// live, trash.ErrConflict when a live row holds its unique key.
	RestoreEntry(ctx context.Context, id uuid.UUID) (trash.Entry, error)
}
