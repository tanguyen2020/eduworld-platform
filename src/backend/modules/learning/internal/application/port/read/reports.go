package read

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
)

// ReportSpec is what an export reads (F044 US1): the type, the tenant (nil = cross-tenant export of System Admin) and the
// parameters with the scope of the requester.
type ReportSpec struct {
	TenantID    *uuid.UUID
	Type        string
	Params      write.ReportParams
	MinCellSize int // analytics_min_cell_size (cross-tenant summaries)
}

// ReportBatch is one batch of rows (keyset pagination: Next = "" when the last batch was read).
type ReportBatch struct {
	Columns []string
	Rows    [][]string
	Next    string
}

// ReportReader reads the rows of an export on the dashboard session (read replica, role learning_dashboard; learning_platform
// for cross-tenant rollups), in batches of at most limit rows; only rows in the scope of the spec.
type ReportReader interface {
	Prepare(ctx context.Context, timeout time.Duration) error
	CountReportRows(ctx context.Context, s ReportSpec) (int64, error)
	ReportRows(ctx context.Context, s ReportSpec, after string, limit int) (ReportBatch, error)
}

// ReportJobReader reads a job of the requester (getLmsReport, getTenantReport, getPlatformReport).
type ReportJobReader interface {
	ReportJob(ctx context.Context, id uuid.UUID) (write.ReportJobRow, bool, error)
}

// ---------- course results (F044 US2)

// CourseRef is the course of a results table.
type CourseRef struct {
	CourseID, SchoolID, SubjectID, ClassID uuid.UUID
}

// GradedAssignment is one column of the course results (assignment mode graded).
type GradedAssignment struct {
	AssignmentID uuid.UUID
	Title        string
	DueAt        time.Time
	ScorePolicy  string
}

// ResultCell is the submission of a student to a graded assignment.
type ResultCell struct {
	StudentID       uuid.UUID
	AssignmentID    uuid.UUID
	Status          string
	Score, MaxScore *float64
	RapidGuessCount int
}

// CourseResultsReader reads the course results table (lms.courses, lms.assignments, lms.assignment_submissions).
type CourseResultsReader interface {
	Course(ctx context.Context, id uuid.UUID) (*CourseRef, error)
	GradedAssignments(ctx context.Context, courseID uuid.UUID) ([]GradedAssignment, error)
	// CourseStudents returns up to limit+1 active students of the course class after the student id after.
	CourseStudents(ctx context.Context, classID uuid.UUID, after uuid.UUID, limit int) ([]uuid.UUID, error)
	ResultCells(ctx context.Context, courseID uuid.UUID, students []uuid.UUID) ([]ResultCell, error)
}

// ---------- reference levels (F044 US3)

// ReferenceLevel is one row of the reference report.
type ReferenceLevel struct {
	StudentID             uuid.UUID
	ClassID               uuid.UUID
	LevelCode             string
	Coverage              float64
	Proficient, Mastered  float64
	OfficialScore         *float64
	RapidGuessRatio       float64
	LowReliabilityExamIDs []uuid.UUID
	ProfileCode           string
	ProfileVersion        int
	ComputedAt            time.Time
}

// ReferenceReader reads learning.rm_student_subject_reference_levels and the profiles.
type ReferenceReader interface {
	ReferenceLevels(ctx context.Context, schoolID, subjectID uuid.UUID, year int, period string, classID *uuid.UUID, after uuid.UUID,
		limit int) ([]ReferenceLevel, error)
	ProfileDisclaimer(ctx context.Context, code string, version int) (map[string]string, error)
}

// ---------- item analysis and calibration (F044 US4, US5)

// QuestionOwner is the owner of a question visible in the session (tenant row or assigned platform row).
type QuestionOwner struct {
	QuestionID uuid.UUID
	TenantID   *uuid.UUID
}

// QuestionStat is one row of content.question_stats with its version number.
type QuestionStat struct {
	write.QuestionStatRow
	VersionNo int
}

// QualityReader reads the statistics and the proposals visible in the session.
type QualityReader interface {
	Question(ctx context.Context, id uuid.UUID) (*QuestionOwner, error)
	QuestionStats(ctx context.Context, questionID uuid.UUID) ([]QuestionStat, error)
	// Proposals returns up to limit+1 proposals of the session tenant (platform: tenant_id NULL) with the status, after
	// the proposal id after.
	Proposals(ctx context.Context, platform bool, status string, after uuid.UUID, limit int) ([]write.ProposalRow, error)
}

// ---------- exam session report (F044 US6)

// ExamQualityReader reads the session and its quality report.
type ExamQualityReader interface {
	ExamSession(ctx context.Context, id uuid.UUID) (*write.ExamSessionRow, error)
	ExamQuality(ctx context.Context, id uuid.UUID) (*write.ExamQualityRow, error)
	ExamPapersExpected(ctx context.Context, id uuid.UUID) (map[int]*float64, error)
}

// ReportScopeReader resolves the school and subject of the object an LMS export is about (scope checks of createLmsReport).
type ReportScopeReader interface {
	Course(ctx context.Context, id uuid.UUID) (*CourseRef, error)
	AssignmentScope(ctx context.Context, id uuid.UUID) (school, subject uuid.UUID, ok bool, err error)
	ClassSchool(ctx context.Context, id uuid.UUID) (school uuid.UUID, ok bool, err error)
}
