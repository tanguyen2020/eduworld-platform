package read

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
)

// CourseFilter selects courses (listCourses).
type CourseFilter struct {
	SchoolID       *uuid.UUID
	ClassID        *uuid.UUID
	SubjectID      *uuid.UUID
	AcademicYearID *uuid.UUID
	Status         string
	ClassIDs       []uuid.UUID // the active classes of a student
}

// ScheduleFilter selects schedule items of courses in a date range (inclusive, YYYY-MM-DD).
type ScheduleFilter struct {
	CourseIDs []uuid.UUID
	ClassID   *uuid.UUID
	From, To  string
}

// LMSReadModel reads the LMS of F040 in the scope of ctx (tenant + deleted_at, RLS; platform templates of learning
// paths are readable by every tenant).
type LMSReadModel interface {
	Courses(ctx context.Context, f CourseFilter) ([]dto.CourseView, error)
	Course(ctx context.Context, id uuid.UUID) (*dto.CourseView, error)
	// CoursePath is the active own path of a course, or the platform template of its textbook series (or of its subject
	// and grade when the course has no series).
	CoursePath(ctx context.Context, c dto.CourseView) (*dto.LearningPathView, error)
	Schedule(ctx context.Context, f ScheduleFilter) ([]dto.ScheduleItemView, error)
	StudyGroups(ctx context.Context, schoolID uuid.UUID, classID *uuid.UUID, grade *int) ([]dto.StudyGroupView, error)
	StudyGroup(ctx context.Context, schoolID, id uuid.UUID) (*dto.StudyGroupView, error)
	CurriculumSettings(ctx context.Context, schoolID uuid.UUID, yearID *uuid.UUID) (dto.CurriculumSettingsView, error)
	// StudentClasses lists the classes a student is actively enrolled in.
	StudentClasses(ctx context.Context, studentID uuid.UUID) ([]uuid.UUID, error)
	// CompletedContent reports which content items (activity, question set, lesson) a student completed.
	CompletedContent(ctx context.Context, studentID uuid.UUID, contentIDs []uuid.UUID) (map[uuid.UUID]bool, error)
	// UnlockedNodes reports the nodes the school unlocked for a student.
	UnlockedNodes(ctx context.Context, studentID uuid.UUID, nodeIDs []uuid.UUID) (map[uuid.UUID]bool, error)
}
