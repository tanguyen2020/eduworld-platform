package write

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ---------- F044 US1: asynchronous exports (learning.report_jobs)

// ReportParams are the parameters of a job (learning.report_jobs.params): the request plus the scope of the requester
// applied by the worker (SubjectIDs nil = every subject of the school; ScopeSchoolIDs nil = every school of the tenant).
type ReportParams struct {
	SchoolID       *uuid.UUID  `json:"school_id,omitempty"`
	ClassID        *uuid.UUID  `json:"class_id,omitempty"`
	CourseID       *uuid.UUID  `json:"course_id,omitempty"`
	AssignmentID   *uuid.UUID  `json:"assignment_id,omitempty"`
	SubjectID      *uuid.UUID  `json:"subject_id,omitempty"`
	Period         string      `json:"period,omitempty"`
	AcademicYear   int         `json:"academic_year,omitempty"`
	From           string      `json:"from,omitempty"`
	To             string      `json:"to,omitempty"`
	SchoolIDs      []uuid.UUID `json:"school_ids,omitempty"`
	TenantIDs      []uuid.UUID `json:"tenant_ids,omitempty"`
	GradeLevel     *int        `json:"grade_level,omitempty"`
	SubjectIDs     []uuid.UUID `json:"scope_subject_ids,omitempty"`
	ScopeSchoolIDs []uuid.UUID `json:"scope_school_ids,omitempty"`
}

// ReportJobRow is one row of learning.report_jobs.
type ReportJobRow struct {
	ID             uuid.UUID
	TenantID       *uuid.UUID
	Scope          string
	Type           string
	Format         string
	Params         ReportParams
	RequestedBy    uuid.UUID
	Status         string
	RowCount       *int
	StorageKey     *string
	Error          *string
	IdempotencyKey string
	RunCount       int
	StartedAt      *time.Time
	FinishedAt     *time.Time
	FileExpiresAt  *time.Time
	CreatedAt      time.Time
}

// ReportJobStore stores the jobs in the tenant of the session (platform jobs: learning_platform).
type ReportJobStore interface {
	InsertReportJob(ctx context.Context, j ReportJobRow) error
	// ReportJobByKey returns the job of the requester created with the Idempotency-Key.
	ReportJobByKey(ctx context.Context, requestedBy uuid.UUID, key string) (ReportJobRow, bool, error)
	// ActiveReportJobs counts the queued and running jobs of the requester (dashboard_export_max_concurrent_per_user).
	ActiveReportJobs(ctx context.Context, requestedBy uuid.UUID) (int, error)
}

// ReportJobQueue is the work queue of the worker (every tenant, tenancy.Bypass platform_job): queued jobs and running jobs
// left without progress are claimed with FOR UPDATE SKIP LOCKED.
type ReportJobQueue interface {
	ClaimReportJobs(ctx context.Context, staleBefore time.Time, limit int) ([]ReportJobRow, error)
	FinishReportJob(ctx context.Context, j ReportJobRow) error
	ExpireReportJobs(ctx context.Context, now time.Time) (int, error)
}

// ---------- F044 US4/US5: item analysis and difficulty calibration

// QuestionPValues reads the measured p_value of question versions (content.question_stats rows with enough data, every
// grade pooled by first answers) for the variant balancer of F044.
type QuestionPValues interface {
	PValues(ctx context.Context, versionIDs []uuid.UUID) (map[uuid.UUID]float64, error)
}

// AnalysedVersion is a question version whose answers changed since the last run, with what item analysis needs.
type AnalysedVersion struct {
	QuestionID        uuid.UUID
	QuestionVersionID uuid.UUID
	TenantID          *uuid.UUID // nil = platform question
	Type              string
	Difficulty        int
	Render            json.RawMessage
	AnswerKey         json.RawMessage
}

// FirstAnswer is the first scored answer (attempt_no 1, is_scored) of a student to a version in one attempt.
type FirstAnswer struct {
	GradeLevel   int
	Correct      bool
	Score        float64
	MaxScore     float64
	AttemptScore float64 // score ratio of the whole attempt
	ResponseMS   *int
	RapidGuess   bool
	HintsUsed    int
	Response     json.RawMessage
}

// QuestionStatRow is one row of content.question_stats.
type QuestionStatRow struct {
	TenantID           *uuid.UUID
	QuestionID         uuid.UUID
	QuestionVersionID  uuid.UUID
	GradeLevel         int
	FirstAttempts      int
	ExcludedRapidGuess int
	WithHint           int
	PValue             *float64
	PValueWithHint     *float64
	Discrimination     *float64
	DistractorRates    map[string]float64
	AvgTimeMS          int
	RapidGuessRate     float64
	HintUsageRate      float64
	Flags              []string
	ComputedAt         time.Time
}

// ItemAnalysisStore reads the answers and writes the statistics (job item-analysis-nightly, every tenant under
// tenancy.Bypass platform_job: a platform question pools the answers of every tenant).
type ItemAnalysisStore interface {
	ChangedVersions(ctx context.Context, since time.Time, limit int) ([]AnalysedVersion, error)
	FirstAnswers(ctx context.Context, versionID uuid.UUID) ([]FirstAnswer, error)
	UpsertQuestionStats(ctx context.Context, rows []QuestionStatRow) error
}

// CalibrationCandidate is the current published version of a question with its pooled statistics of one grade.
type CalibrationCandidate struct {
	QuestionID        uuid.UUID
	QuestionVersionID uuid.UUID
	TenantID          *uuid.UUID
	Difficulty        int
	GradeLevel        int
	PValue            float64
	SampleSize        int
	Grades            map[string]any // per grade: n, p_value
}

// CalibrationState is what earlier proposals of a question say (pending, dismissal streak, suppression).
type CalibrationState struct {
	Pending               bool
	ConsecutiveDismissals int
	SuppressedUntil       *time.Time
}

// ProposalRow is one row of content.difficulty_calibration_proposals.
type ProposalRow struct {
	ID                    uuid.UUID
	TenantID              *uuid.UUID
	QuestionID            uuid.UUID
	BasedOnVersionID      uuid.UUID
	CurrentDifficulty     int
	ProposedDifficulty    int
	PValue                float64
	SampleSize            int
	GradeLevel            int
	Grades                map[string]any
	Status                string
	DecidedBy             *uuid.UUID
	DecidedAt             *time.Time
	CreatedVersionID      *uuid.UUID
	ConsecutiveDismissals int
	SuppressedUntil       *time.Time
	CreatedAt             time.Time
	Version               int64
}

// CalibrationStore keeps the proposals.
type CalibrationStore interface {
	// CalibrationCandidates returns the current versions of the questions with ≥ minN first answers in one grade
	// (the grade with the most answers), every tenant (platform_job).
	CalibrationCandidates(ctx context.Context, minN int) ([]CalibrationCandidate, error)
	CalibrationState(ctx context.Context, questionID uuid.UUID) (CalibrationState, error)
	InsertProposal(ctx context.Context, p ProposalRow) error
	// Proposal reads a proposal visible in the session, locked (FOR UPDATE) when lock.
	Proposal(ctx context.Context, id uuid.UUID, lock bool) (ProposalRow, error)
	UpdateProposal(ctx context.Context, p ProposalRow) error
}

// ---------- F044 US6: exam quality

// ExamSheet is the graded attempt of one student of a session (latest graded attempt of the participation).
type ExamSheet struct {
	StudentID uuid.UUID
	VariantNo int
	Score     float64
	MaxScore  float64
	Items     []ExamSheetItem
}

// ExamSheetItem is the final result of one item of an attempt with its first answer.
type ExamSheetItem struct {
	QuestionVersionID uuid.UUID
	Score, MaxScore   float64
	Correct           bool
	RapidGuess        bool
	ResponseMS        *int
	Response          json.RawMessage
}

// ExamQualityRow is one row of assessment.exam_quality_reports.
type ExamQualityRow struct {
	SessionID          uuid.UUID
	TenantID           uuid.UUID
	SchoolID           uuid.UUID
	StudentCount       int
	Variants           json.RawMessage
	VariantGap         bool
	ProblemItems       json.RawMessage
	Items              json.RawMessage
	Distribution       []int
	Mean, Median       *float64
	PassRate           *float64
	ReliabilityWarning bool
	ComputedAt         time.Time
}

// ExamQualityStore reads the graded sheets of a closed session and writes its quality report.
type ExamQualityStore interface {
	ExamSessionForQuality(ctx context.Context, id uuid.UUID) (ExamSessionRow, error)
	ExamSheets(ctx context.Context, id uuid.UUID) ([]ExamSheet, error)
	// OptionMisconceptions returns, per question version, the misconception tag of every option that has one.
	OptionMisconceptions(ctx context.Context, versionIDs []uuid.UUID) (map[uuid.UUID]map[string]string, error)
	UpsertExamQuality(ctx context.Context, r ExamQualityRow) error
}

// ---------- F044 US3: reference levels TT27/TT22

// AssessmentProfileRow is one row of learning.assessment_profiles.
type AssessmentProfileRow struct {
	ID          uuid.UUID
	TenantID    *uuid.UUID
	Code        string
	Version     int
	GradeLevels []int
	Rules       json.RawMessage
	Disclaimer  map[string]string
	Status      string
	CreatedAt   time.Time
}

// ReferenceStudent is a student × subject × class to grade for a period, with the objectives of the period scope.
type ReferenceStudent struct {
	SchoolID, ClassID, StudentID, SubjectID uuid.UUID
	GradeLevel                              int
	Objectives                              []uuid.UUID
}

// ObjectiveLevel is the mastery of one objective of a student.
type ObjectiveLevel struct {
	ObjectiveID   uuid.UUID
	Level         string
	EvidenceCount int
}

// ReferenceLevelRow is one row of learning.rm_student_subject_reference_levels.
type ReferenceLevelRow struct {
	TenantID, SchoolID, ClassID, StudentID, SubjectID uuid.UUID
	AcademicYear                                      int
	Period                                            string
	ProfileID                                         uuid.UUID
	ProfileCode                                       string
	ProfileVersion                                    int
	LevelCode                                         string
	InScope                                           int
	Coverage, Proficient, Mastered                    float64
	OfficialScore                                     *float64
	LowReliabilityExamIDs                             []uuid.UUID
	RapidGuessRatio                                   float64
	ComputedAt                                        time.Time
}

// ReferenceStore computes the inputs and keeps the reference levels (tenant of the session).
type ReferenceStore interface {
	ActiveProfiles(ctx context.Context) ([]AssessmentProfileRow, error)
	// ReferenceStudents returns the active students of the tenant with their class (grade 1–9) and, per subject of their
	// courses, the objectives of the scope: the first share (curriculum order) of the objectives of the grade × subject.
	ReferenceStudents(ctx context.Context, share float64) ([]ReferenceStudent, error)
	ObjectiveLevels(ctx context.Context, studentID uuid.UUID, objectives []uuid.UUID) ([]ObjectiveLevel, error)
	// OfficialScores returns the average 10-point score of the official sessions of the subject closed in [from, to] and the
	// sessions with a low-reliability warning.
	OfficialScores(ctx context.Context, studentID, subjectID uuid.UUID, from, to time.Time) (*float64, []uuid.UUID, error)
	RapidGuessRatio(ctx context.Context, studentID, subjectID uuid.UUID, from, to time.Time) (float64, error)
	UpsertReferenceLevels(ctx context.Context, rows []ReferenceLevelRow) error
	InsertProfileVersion(ctx context.Context, p AssessmentProfileRow) error
	SupersedeProfile(ctx context.Context, id uuid.UUID) error
}
