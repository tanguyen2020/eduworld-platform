package write

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/exam/paper"
)

// ExamSessionRow is one row of assessment.exam_sessions (F042 data-model §1).
type ExamSessionRow struct {
	ID                 uuid.UUID
	TenantID           uuid.UUID
	SchoolID           uuid.UUID
	SubjectID          uuid.UUID
	GradeLevel         *int
	AssignmentID       *uuid.UUID
	Title              string
	Type               string
	IsOfficial         bool
	BlueprintID        uuid.UUID
	BlueprintVersionID uuid.UUID
	WindowStart        time.Time
	WindowEnd          time.Time
	DurationSec        int
	Target             json.RawMessage
	Sections           json.RawMessage
	Audience           string
	Strategy           string
	VariantCount       int
	FeedbackTiming     string
	ResultRelease      string
	ShowCorrect        bool
	ShowExplanations   bool
	LateStartPolicy    string
	Accommodations     json.RawMessage
	MaxAttempts        int
	ScorePolicy        string
	QuestionCount      int
	TotalPoints        float64
	Status             string
	CreatedBy          uuid.UUID
	CreatedByRole      string
	OpenedAt           *time.Time
	ClosedAt           *time.Time
	ReleasedAt         *time.Time
	ReleasedBy         *uuid.UUID
	CancelledAt        *time.Time
	CancelReason       *string
	Version            int64
	CreatedAt          time.Time
	// EfficacyWaveID is the efficacy wave of a session created by F035 (no exam.* event, no mastery evidence, never
	// released, never shown to students as such).
	EfficacyWaveID *uuid.UUID
	// Navigation is the blueprint navigation copied at creation (free | sequential_no_return, F064 T004).
	Navigation string
}

// ExamPaperRow is one row of assessment.exam_papers (immutable).
type ExamPaperRow struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	SessionID        uuid.UUID
	VariantNo        int
	StudentID        *uuid.UUID
	AttemptNo        *int
	Seed             int64
	Items            []paper.Item
	TotalPoints      float64
	ExpectedP        *float64
	BalanceWarning   bool // F044: the balancer could not reach max_mean_p_diff
	GeneratorVersion string
}

// ExamParticipationRow is one row of assessment.exam_participations (the audience snapshot of a session).
type ExamParticipationRow struct {
	SessionID            uuid.UUID
	StudentID            uuid.UUID
	TenantID             uuid.UUID
	SchoolID             uuid.UUID
	Status               string
	AttemptsUsed         int
	LastAttemptID        *uuid.UUID
	EffectiveDurationSec int
	ExtraTimePercent     int
	Version              int64
}

// ExamAttemptRef is an exam attempt found by a job (every tenant).
type ExamAttemptRef struct {
	AttemptID uuid.UUID
	TenantID  uuid.UUID
}

// ExamAttemptInfo are the exam columns of an attempt.
type ExamAttemptInfo struct {
	PaperID      *uuid.UUID
	DeviceToken  *string
	SubmitReason *string
}

// ExamStore persists the exam sessions of F042 in the TenantScope of ctx (RLS T, soft-delete only); the Due*/Expired*
// reads run under tenancy.Bypass(platform_job) for the jobs exam-window and exam.autosubmit.
type ExamStore interface {
	InsertSession(ctx context.Context, s ExamSessionRow) error
	// Session reads a live session; lock = SELECT … FOR UPDATE.
	Session(ctx context.Context, id uuid.UUID, lock bool) (*ExamSessionRow, error)
	// SessionOfAssignment reads the live session of an assignment.
	SessionOfAssignment(ctx context.Context, assignmentID uuid.UUID) (*ExamSessionRow, error)
	// UpdateSession saves status, release and cancel fields when the row version is prev.
	UpdateSession(ctx context.Context, s ExamSessionRow, prev int64) (bool, error)
	InsertPapers(ctx context.Context, ps []ExamPaperRow) error
	// Papers lists the shared variants of a session (student_id NULL) by variant.
	Papers(ctx context.Context, sessionID uuid.UUID) ([]ExamPaperRow, error)
	// StudentPaper reads the per_student paper of an attempt number.
	StudentPaper(ctx context.Context, sessionID, studentID uuid.UUID, attemptNo int) (*ExamPaperRow, error)
	Paper(ctx context.Context, id uuid.UUID) (*ExamPaperRow, error)
	InsertParticipations(ctx context.Context, ps []ExamParticipationRow) error
	Participation(ctx context.Context, sessionID, studentID uuid.UUID, lock bool) (*ExamParticipationRow, error)
	UpdateParticipation(ctx context.Context, p ExamParticipationRow) error
	// SetAccommodations replaces the personal extra times of a session when its version is prev (putExamAccommodations).
	SetAccommodations(ctx context.Context, sessionID uuid.UUID, acc json.RawMessage, prev int64) (bool, error)
	// SetParticipationTime stores the extra time and effective duration of one student of a session; false when the
	// student is not in the session's audience.
	SetParticipationTime(ctx context.Context, sessionID, studentID uuid.UUID, extraPercent, effectiveSec int) (bool, error)
	// MarkAbsent sets absent on the participations not started of a session; returns how many.
	MarkAbsent(ctx context.Context, sessionID uuid.UUID) (int, error)
	// SetAttemptExam stores the paper and the active device (hash) of an exam attempt.
	SetAttemptExam(ctx context.Context, attemptID, paperID uuid.UUID, deviceToken *string) error
	// SetAttemptDevice moves an exam attempt to another device (take over).
	SetAttemptDevice(ctx context.Context, attemptID uuid.UUID, deviceToken *string) error
	AttemptExam(ctx context.Context, attemptID uuid.UUID) (ExamAttemptInfo, error)
	// SetSubmitReason overrides submit_reason of a submitted exam attempt (auto_timeout, window_closed).
	SetSubmitReason(ctx context.Context, attemptID uuid.UUID, reason string) error
	// OpenAttempts lists the in-progress attempts of a session.
	OpenAttempts(ctx context.Context, sessionID uuid.UUID) ([]uuid.UUID, error)
	// SubmittedAttempts lists the submitted (not graded) attempts of a session.
	SubmittedAttempts(ctx context.Context, sessionID uuid.UUID) ([]uuid.UUID, error)
	// GradedAttempts lists the graded attempts of a session holding a question version (regrade).
	GradedAttempts(ctx context.Context, sessionID, questionVersionID uuid.UUID) ([]uuid.UUID, error)
	// DueToOpen / DueToClose list the sessions whose window started / ended (every tenant).
	DueToOpen(ctx context.Context, now time.Time, limit int) ([]DueAssignment, error)
	DueToClose(ctx context.Context, now time.Time, limit int) ([]DueAssignment, error)
	// ExpiredAttempts lists the in-progress exam attempts past deadline_at + grace (every tenant, FOR UPDATE SKIP LOCKED).
	ExpiredAttempts(ctx context.Context, cutoff time.Time, limit int) ([]ExamAttemptRef, error)
	// LatestAnswers returns the latest saved response of every position of an attempt (append-only answers).
	LatestAnswers(ctx context.Context, attemptID uuid.UUID) (map[int]json.RawMessage, error)
	// LatestResponseMS returns response_ms of the latest answer of every position (learning event question.answered).
	LatestResponseMS(ctx context.Context, attemptID uuid.UUID) (map[int]int64, error)
	// ExamObjectiveResults sums correct/total items per objective of the student's graded exam attempts since a time whose
	// results students may see (R5).
	ExamObjectiveResults(ctx context.Context, studentID uuid.UUID, since time.Time) ([]ExamObjectiveResult, error)
}

// ExamObjectiveResult is the share of correct items of one objective in released exams.
type ExamObjectiveResult struct {
	ObjectiveID uuid.UUID
	Correct     int
	Total       int
}
