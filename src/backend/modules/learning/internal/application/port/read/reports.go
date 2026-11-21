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

// ---------- reference levels (F044 US3)

// ReferenceLevel is one row of the reference report.
type ReferenceLevel struct {
	StudentID             uuid.UUID
	GradeLevel            *int // grade of the student when the level was computed (ADR-0095)
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
	ReferenceLevels(ctx context.Context, schoolID, subjectID uuid.UUID, year int, period string, grade *int, after uuid.UUID,
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
	AssignmentScope(ctx context.Context, id uuid.UUID) (school, subject uuid.UUID, ok bool, err error)
}
