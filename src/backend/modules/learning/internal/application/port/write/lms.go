package write

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// CourseRow is one row of lms.courses (F040 data-model §1).
type CourseRow struct {
	ID                  uuid.UUID
	TenantID            uuid.UUID
	SchoolID            uuid.UUID
	ClassID             uuid.UUID
	SubjectID           uuid.UUID
	Grade               int
	AcademicYearID      uuid.UUID
	AcademicYear        string
	CurriculumVersionID uuid.UUID
	TextbookSeriesID    *uuid.UUID
	Status              string
	Version             int64
}

// CurriculumSettingRow is one row of lms.school_curriculum_settings.
type CurriculumSettingRow struct {
	ID                  uuid.UUID
	TenantID            uuid.UUID
	SchoolID            uuid.UUID
	AcademicYearID      uuid.UUID
	Grade               *int
	CurriculumVersionID uuid.UUID
	SetBy               uuid.UUID
	UpdatedAt           time.Time
}

// VersionAccess is a published-or-not content (or curriculum) version as the tenant of the session sees it: Visible
// when the tenant owns it or it is platform content assigned to the tenant; Platform when it is platform content.
type VersionAccess struct {
	ItemID       uuid.UUID
	VersionID    uuid.UUID
	Status       string
	Platform     bool
	Visible      bool
	Title        string
	SubjectID    *uuid.UUID
	ObjectiveIDs []uuid.UUID
	TopicIDs     []uuid.UUID
}

// NodeRow is one row of lms.path_nodes.
type NodeRow struct {
	ID               uuid.UUID
	Position         int
	ContentType      string
	ContentID        uuid.UUID
	ContentVersionID uuid.UUID
	Title            string
	ObjectiveIDs     []uuid.UUID
	UnlockRule       json.RawMessage
}

// PathRow is one row of lms.learning_paths with its live nodes in position order.
type PathRow struct {
	ID               uuid.UUID
	TenantID         *uuid.UUID
	CourseID         *uuid.UUID
	SchoolID         *uuid.UUID
	TextbookSeriesID *uuid.UUID
	SubjectID        *uuid.UUID
	Grade            *int
	Title            string
	UnlockMode       string
	Status           string
	By               uuid.UUID
	Version          int64
	UpdatedAt        time.Time
	Nodes            []NodeRow
}

// ScheduleRow is one row of lms.schedule_items.
type ScheduleRow struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	SchoolID      uuid.UUID
	CourseID      uuid.UUID
	Date          string // YYYY-MM-DD
	Period        *int
	Kind          string
	RefType       *string
	RefID         *uuid.UUID
	Note          *string
	SourceEventID *uuid.UUID
	CreatedBy     *uuid.UUID
	Version       int64
}

// StudyGroupRow is one row of lms.study_groups with its live members.
type StudyGroupRow struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	SchoolID  uuid.UUID
	ClassID   *uuid.UUID
	Grade     int
	Name      string
	Members   []uuid.UUID
	CreatedBy uuid.UUID
	UpdatedBy uuid.UUID
	Version   int64
	UpdatedAt time.Time
}

// ClassRef is a class of the projection ref.classes of the session tenant.
type ClassRef struct {
	ClassID        uuid.UUID
	SchoolID       uuid.UUID
	Grade          int
	AcademicYearID *uuid.UUID
}

// LMSStore persists the LMS of F040 in the TenantScope of ctx (RLS T, soft-delete only) and reads what its commands
// check: the class and student projections, curriculum and content versions visible to the tenant.
type LMSStore interface {
	// Courses.
	InsertCourse(ctx context.Context, r CourseRow) (bool, error)
	Course(ctx context.Context, id uuid.UUID) (*CourseRow, error)
	UpdateCourseStatus(ctx context.Context, id uuid.UUID, status string, ifVersion int64) (bool, error)
	ArchiveClassCourses(ctx context.Context, classID uuid.UUID) (int, error)

	// Curriculum of the school.
	CurriculumSettings(ctx context.Context, schoolID uuid.UUID) ([]CurriculumSettingRow, int64, error)
	ReplaceCurriculumSettings(ctx context.Context, schoolID uuid.UUID, years []uuid.UUID, rows []CurriculumSettingRow, by uuid.UUID) error
	SettingFor(ctx context.Context, schoolID, yearID uuid.UUID, grade int) (*uuid.UUID, error)
	CurriculumVersion(ctx context.Context, versionID uuid.UUID) (*VersionAccess, error)
	LatestCurriculumVersion(ctx context.Context, grade int) (*uuid.UUID, error)
	SubjectsOfGrade(ctx context.Context, versionID uuid.UUID, grade int) ([]uuid.UUID, error)

	// Learning paths.
	ContentVersion(ctx context.Context, contentType string, versionID uuid.UUID) (*VersionAccess, error)
	ActivePath(ctx context.Context, courseID uuid.UUID) (*PathRow, error)
	Path(ctx context.Context, id uuid.UUID) (*PathRow, error)
	InsertPath(ctx context.Context, p PathRow) error
	// UpdatePath saves the path and replaces its nodes when the row version is still ifVersion (a node with the same
	// content keeps its node_id; removed nodes are soft-deleted).
	UpdatePath(ctx context.Context, p PathRow, ifVersion int64) (bool, error)
	InsertUnlock(ctx context.Context, tenant, school, pathID, nodeID, studentID uuid.UUID, reason string, by uuid.UUID) (bool, error)

	// Schedule.
	InsertSchedule(ctx context.Context, r ScheduleRow) (bool, error)
	Schedule(ctx context.Context, id uuid.UUID) (*ScheduleRow, error)
	UpdateSchedule(ctx context.Context, r ScheduleRow, ifVersion int64) (bool, error)
	DeleteSchedule(ctx context.Context, id uuid.UUID, by uuid.UUID) (bool, error)

	// Study groups.
	InsertGroup(ctx context.Context, g StudyGroupRow) error
	Group(ctx context.Context, schoolID, id uuid.UUID) (*StudyGroupRow, error)
	UpdateGroup(ctx context.Context, g StudyGroupRow, added, removed []uuid.UUID, ifVersion int64) (bool, error)
	DeleteGroup(ctx context.Context, g StudyGroupRow, by uuid.UUID) error

	// Projections (ref.*).
	Class(ctx context.Context, classID uuid.UUID) (*ClassRef, error)
	// ActiveStudents keeps the students active in the school; InClass keeps those with an active enrollment in the class.
	ActiveStudents(ctx context.Context, schoolID uuid.UUID, ids []uuid.UUID) ([]uuid.UUID, error)
	InClass(ctx context.Context, classID uuid.UUID, ids []uuid.UUID) ([]uuid.UUID, error)

	// UpcomingLessons lists the lesson items of the schedule of the active courses of a student between two dates whose
	// reference is a lesson or an activity visible to the tenant, with its current version (rule R6).
	UpcomingLessons(ctx context.Context, studentID uuid.UUID, from, to string) ([]UpcomingContent, error)
}

// UpcomingContent is a scheduled lesson or activity of a course of a student.
type UpcomingContent struct {
	ContentType  string
	ContentID    uuid.UUID
	SubjectID    uuid.UUID
	Title        string
	ObjectiveIDs []uuid.UUID
	Date         string
}
