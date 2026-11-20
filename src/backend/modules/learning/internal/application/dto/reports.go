package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ReportJobView is ReportJob (F044 US1): download_url only for the creator of a succeeded job, valid until
// download_expires_at (dashboard_export_link_ttl_minutes).
type ReportJobView struct {
	ReportID          uuid.UUID  `json:"report_id"`
	Status            string     `json:"status"`
	ReportType        string     `json:"report_type"`
	Format            string     `json:"format"`
	RowCount          *int       `json:"row_count,omitempty"`
	DownloadURL       *string    `json:"download_url,omitempty"`
	DownloadExpiresAt *time.Time `json:"download_expires_at,omitempty"`
	Error             *string    `json:"error,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	FinishedAt        *time.Time `json:"finished_at,omitempty"`
}

// CourseResultColumn is one graded assignment of the course results.
type CourseResultColumn struct {
	AssignmentID uuid.UUID `json:"assignment_id"`
	Title        string    `json:"title"`
	DueAt        time.Time `json:"due_at"`
	ScorePolicy  string    `json:"score_policy"`
}

// CourseResultScore is the effective self-graded score of a student on one assignment (10-point scale, score_scale).
type CourseResultScore struct {
	AssignmentID uuid.UUID `json:"assignment_id"`
	Score10      *float64  `json:"score_10"`
	Status       string    `json:"status"`
	TooFast      bool      `json:"too_fast"`
}

// CourseResultRow is one student of the course results.
type CourseResultRow struct {
	StudentID uuid.UUID           `json:"student_id"`
	Scores    []CourseResultScore `json:"scores"`
}

// CourseResultsView is CourseResults (F044 US2): no XP, rating, competition points nor ranking.
type CourseResultsView struct {
	CourseID uuid.UUID            `json:"course_id"`
	Columns  []CourseResultColumn `json:"columns"`
	Rows     []CourseResultRow    `json:"rows"`
}

// ReferenceItem is the reference level of one student.
type ReferenceItem struct {
	StudentID             uuid.UUID   `json:"student_id"`
	ClassID               uuid.UUID   `json:"class_id"`
	LevelCode             string      `json:"level_code"`
	CoverageRatio         float64     `json:"coverage_ratio"`
	ProficientRatio       float64     `json:"proficient_ratio"`
	MasteredRatio         float64     `json:"mastered_ratio"`
	OfficialScore         *float64    `json:"official_score"`
	RapidGuessRatio       float64     `json:"rapid_guess_ratio"`
	LowReliabilityExamIDs []uuid.UUID `json:"low_reliability_exam_ids"`
	ProfileVersion        int         `json:"profile_version"`
}

// RegulationReferenceReportView is RegulationReferenceReport (F044 US3): always with the disclaimer.
type RegulationReferenceReportView struct {
	SchoolID     uuid.UUID       `json:"school_id"`
	SubjectID    uuid.UUID       `json:"subject_id"`
	AcademicYear int             `json:"academic_year"`
	Period       string          `json:"period"`
	Disclaimer   string          `json:"disclaimer"`
	Items        []ReferenceItem `json:"items"`
}

// QuestionStatsItem is the item analysis of one question version and grade (grade_level 0 = every grade pooled).
type QuestionStatsItem struct {
	QuestionVersionID       uuid.UUID          `json:"question_version_id"`
	VersionNo               int                `json:"version_no"`
	GradeLevel              int                `json:"grade_level"`
	FirstAttemptCount       int                `json:"first_attempt_count"`
	ExcludedRapidGuessCount int                `json:"excluded_rapid_guess_count"`
	WithHintCount           int                `json:"with_hint_count"`
	Sufficient              bool               `json:"sufficient"`
	PValue                  *float64           `json:"p_value"`
	PValueWithHint          *float64           `json:"p_value_with_hint"`
	Discrimination          *float64           `json:"discrimination"`
	DistractorRates         map[string]float64 `json:"distractor_rates"`
	AvgTimeMS               int                `json:"avg_time_ms"`
	RapidGuessRate          float64            `json:"rapid_guess_rate"`
	HintUsageRate           float64            `json:"hint_usage_rate"`
	Flags                   []string           `json:"flags"`
	ComputedAt              time.Time          `json:"computed_at"`
}

// QuestionStatsView is QuestionStats (F044 US4).
type QuestionStatsView struct {
	QuestionID uuid.UUID           `json:"question_id"`
	ReadOnly   bool                `json:"read_only"`
	Items      []QuestionStatsItem `json:"items"`
}

// CalibrationProposalView is CalibrationProposal (F044 US5).
type CalibrationProposalView struct {
	ProposalID         uuid.UUID      `json:"proposal_id"`
	QuestionID         uuid.UUID      `json:"question_id"`
	BasedOnVersionID   uuid.UUID      `json:"based_on_version_id"`
	CurrentDifficulty  int            `json:"current_difficulty"`
	ProposedDifficulty int            `json:"proposed_difficulty"`
	PValue             float64        `json:"p_value"`
	SampleSize         int            `json:"sample_size"`
	GradeLevel         int            `json:"grade_level"`
	GradeDistribution  map[string]any `json:"grade_distribution"`
	Status             string         `json:"status"`
	DecidedBy          *uuid.UUID     `json:"decided_by,omitempty"`
	DecidedAt          *time.Time     `json:"decided_at,omitempty"`
	CreatedVersionID   *uuid.UUID     `json:"created_version_id,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
}

// ExamSessionReportView is ExamSessionReport (F044 US6): no answer of a named student.
type ExamSessionReportView struct {
	ExamSessionID      uuid.UUID       `json:"exam_session_id"`
	IsOfficial         bool            `json:"is_official"`
	Status             string          `json:"status"`
	Computed           bool            `json:"computed"`
	ComputedAt         *time.Time      `json:"computed_at,omitempty"`
	StudentCount       int             `json:"student_count"`
	ScoreDistribution  []int           `json:"score_distribution"`
	Mean               *float64        `json:"mean"`
	Median             *float64        `json:"median"`
	PassRate           *float64        `json:"pass_rate"`
	ReliabilityWarning bool            `json:"reliability_warning"`
	VariantGap         bool            `json:"variant_gap"`
	Variants           json.RawMessage `json:"variants"`
	Items              json.RawMessage `json:"items"`
	ProblemItems       json.RawMessage `json:"problem_items"`
}
