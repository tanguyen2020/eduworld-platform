package read

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/analytics"
)

// RollupMeta is what a dashboard answer reports of the rows it read (DASHBOARD_DATA §8 rule 8): the oldest computed_at and
// whether a provisional row (is_final = false) was read.
type RollupMeta struct {
	Oldest  *time.Time
	Partial bool
}

// Merge adds the rows of o.
func (m *RollupMeta) Merge(o RollupMeta) {
	if o.Oldest != nil && (m.Oldest == nil || o.Oldest.Before(*m.Oldest)) {
		t := *o.Oldest
		m.Oldest = &t
	}
	m.Partial = m.Partial || o.Partial
}

// DashScope is the data scope of one dashboard query: always a tenant (R8), the schools of the caller's grants (nil = every
// school of the tenant), optionally a grade, the grades whose cells are visible (k-anonymity, FR-ANL-016: nil = every
// grade; aggregates read the visible grades only) and the subjects of a subject-limited grant (nil = every subject).
// ADR-0095: no class level.
type DashScope struct {
	TenantID   uuid.UUID
	SchoolIDs  []uuid.UUID
	GradeLevel *int
	Grades     []int
	SubjectIDs []uuid.UUID
}

// RollupTotals are the summed counters of the rollup rows of a scope and range.
type RollupTotals struct {
	RollupMeta
	Activities, Lessons, AccNum, AccDen, RapidNum, RapidDen, ActiveSeconds int64
	AssignDue, AssignCompleted, AssignOnTime                               int64
}

// SeriesPoint is one bucket of a school series.
type SeriesPoint struct {
	Bucket                                                         time.Time
	Final                                                          bool
	ActiveStudents, Activities, Lessons, AccNum, AccDen, ActiveSec int64
	AssignDue, AssignCompleted                                     int64
}

// DashStudent is a not deleted student of the session tenant (ref.students).
type DashStudent struct {
	StudentID, SchoolID uuid.UUID
}

// GradeSummary is one grade of a school (rm_school_grade_summary over the subjects of the scope): Students is the active
// students of the school × grade (size of the grade cells, k-anonymity), Active7d the students active over the rapid-guess
// window, CompNum/CompDen the assignment completion of the last 14 days.
type GradeSummary struct {
	GradeLevel       int
	Students         int
	Active7d         int
	CompNum, CompDen int64
	ComputedAt       time.Time
}

// ObjectiveStat is one school × grade × objective of rm_grade_objective_stats (Students = size of the grade).
type ObjectiveStat struct {
	GradeLevel    int
	ObjectiveID   uuid.UUID
	Code, YccdRef string
	LevelDist     []int
	WithEvidence  int
	Students      int
	ComputedAt    time.Time
}

// AttentionEntry is one student of rm_student_attention with at least one reason.
type AttentionEntry struct {
	StudentID  uuid.UUID
	GradeLevel *int
	Reasons    []string
	ComputedAt time.Time
}

// AssignmentRow is the progress of one open or recent assignment in one grade (grade of the students when targeted).
type AssignmentRow struct {
	AssignmentID, SubjectID uuid.UUID
	Title, Status           string
	DueAt                   time.Time
	GradeLevel              int
	Submitted, Total        int
	NotStarted, Missed      int
}

// SelfStudyFacts are the facts of the "self-study and recommendations" widget (rm_school_grade_summary).
type SelfStudyFacts struct {
	Students         int
	RecNum, RecDen   int64
	Improved         int
	ComputedAtOldest *time.Time
}

// StudentRow is one student of a grade with its progress over the range.
type StudentRow struct {
	StudentID                                                     uuid.UUID
	Activities, Lessons, AccNum, AccDen, RapidNum, RapidDen, ActS int64
	Reasons                                                       []string
}

// SubjectTotals are the progress counters of a student in one subject.
type SubjectTotals struct {
	SubjectID                                                     uuid.UUID
	Activities, Lessons, AccNum, AccDen, RapidNum, RapidDen, ActS int64
}

// TopicLevels is the level distribution of the objectives of a topic for a student.
type TopicLevels struct {
	SubjectID, TopicID uuid.UUID
	TopicName          string
	LevelDist          []int
}

// TrendRow is one weekly mastery snapshot.
type TrendRow struct {
	Week          time.Time
	SubjectID     uuid.UUID
	MeanMastery   float64
	MasteredCount int
}

// AssignmentHistory is one assignment submission of a student (no answer).
type AssignmentHistory struct {
	AssignmentID, SubjectID uuid.UUID
	Title, Status           string
	Score, MaxScore         *float64
	DueAt                   time.Time
	SubmittedAt             *time.Time
}

// RecommendationRow is one active recommendation of a student.
type RecommendationRow struct {
	RecommendationID     uuid.UUID
	RuleCode, ReasonCode string
	SubjectID            *uuid.UUID
}

// SummaryCell is one school × grade × subject (tenant summary) or tenant × grade × subject (platform summary) over the
// range: Students is the enrolled students of the school/tenant × grade (k-anonymity), ActiveSum the sum of the active
// students of its buckets.
type SummaryCell struct {
	TenantID                                    uuid.UUID
	SchoolID                                    *uuid.UUID
	GradeLevel                                  int
	SubjectID                                   uuid.UUID
	Students                                    int
	ActiveSum, Activities, AccNum, AccDen, ActS int64
	AssignDue, AssignCompleted                  int64
}

// DashboardReader is the query catalog of the dashboards (F043 T022, DASHBOARD_DATA §8 rule 6): predefined statements on
// rollups and read models only, run with the role learning_dashboard on the read replica (F043-AC-18). Every statement
// filters the tenant(s), the scope and a bounded range.
type DashboardReader interface {
	// Prepare sets the statement timeout of the read transaction (dashboard_statement_timeout).
	Prepare(ctx context.Context, timeout time.Duration) error
	SchoolExists(ctx context.Context, tenantID, schoolID uuid.UUID) (bool, error)
	Student(ctx context.Context, tenantID, studentID uuid.UUID) (*DashStudent, error)
	// SchoolTotals sums the rollup_school_* rows of the grain of rg; GradeTotals sums them per grade.
	SchoolTotals(ctx context.Context, sc DashScope, rg analytics.Range) (RollupTotals, error)
	GradeTotals(ctx context.Context, sc DashScope, rg analytics.Range) (map[int]RollupTotals, RollupMeta, error)
	// SchoolSeries returns one point per bucket of rollup_school_* (active students: all subjects of a grade, summed over grades).
	SchoolSeries(ctx context.Context, sc DashScope, rg analytics.Range) ([]SeriesPoint, RollupMeta, error)
	// ActiveStudents counts the distinct students of progress_daily in [from, to] (bounded window).
	ActiveStudents(ctx context.Context, sc DashScope, from, to time.Time) (int, error)
	// Grades lists the grades of the schools of the scope (one school) with their size and state facts.
	Grades(ctx context.Context, sc DashScope) ([]GradeSummary, error)
	ObjectiveStats(ctx context.Context, sc DashScope) ([]ObjectiveStat, error)
	Attention(ctx context.Context, sc DashScope, limit int) ([]AttentionEntry, int, error)
	Assignments(ctx context.Context, sc DashScope, now time.Time, limit int) ([]AssignmentRow, error)
	SelfStudy(ctx context.Context, sc DashScope) (SelfStudyFacts, error)
	// GradeStudents returns the active students of the grade GradeLevel of the school by student_id cursor with their progress.
	GradeStudents(ctx context.Context, sc DashScope, rg analytics.Range, after *uuid.UUID, limit int) ([]StudentRow, error)
	ProfileSubjects(ctx context.Context, sc DashScope, studentID uuid.UUID, rg analytics.Range) ([]SubjectTotals, RollupMeta, error)
	ProfileTopics(ctx context.Context, sc DashScope, studentID uuid.UUID) ([]TopicLevels, error)
	ProfileTrend(ctx context.Context, sc DashScope, studentID uuid.UUID, from, to time.Time) ([]TrendRow, error)
	ProfileAssignments(ctx context.Context, sc DashScope, studentID uuid.UUID, limit int) ([]AssignmentHistory, error)
	ProfileRecommendations(ctx context.Context, sc DashScope, studentID uuid.UUID, limit int) ([]RecommendationRow, error)
	// SchoolCells aggregates rollup_school_* per school × grade × subject of one tenant.
	SchoolCells(ctx context.Context, sc DashScope, rg analytics.Range) ([]SummaryCell, RollupMeta, error)
	// TenantCells aggregates rollup_tenant_* per tenant × grade × subject (cross-tenant scope).
	TenantCells(ctx context.Context, tenantIDs []uuid.UUID, grade *int, rg analytics.Range) ([]SummaryCell, RollupMeta, error)
	// Tenants lists the active tenants and those of them flagged is_qa (DASHBOARD_DATA §3.4).
	Tenants(ctx context.Context, ids []uuid.UUID) (active, qa []uuid.UUID, err error)
	// AvailableFrom is the earliest daily rollup bucket of the scope (rollup_school_daily of the schools, every school of
	// the tenant when SchoolIDs is nil); nil when the scope has no row.
	AvailableFrom(ctx context.Context, sc DashScope) (*time.Time, error)
	// ReplicaLag is the replay lag of the dashboard read replica (0 on a primary or a replica that replayed all it received).
	ReplicaLag(ctx context.Context) (time.Duration, error)
}
