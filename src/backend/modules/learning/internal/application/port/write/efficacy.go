package write

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
)

// AgreementRow is one row of learning.research_agreements (T).
type AgreementRow struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	Ref          string
	Covers       []string
	AgreedOn     time.Time
	Status       string
	RevokedAt    *time.Time
	RevokedBy    *uuid.UUID
	RevokeReason *string
	RecordedBy   uuid.UUID
	Version      int64
	CreatedAt    time.Time
}

// StudyRow is one row of learning.efficacy_studies (G).
type StudyRow struct {
	ID             uuid.UUID
	Code           string
	Title          string
	SubjectID      uuid.UUID
	GradeLevel     int
	ObjectiveIDs   []uuid.UUID
	FormA, FormB   uuid.UUID
	MinWeeks       int
	Status         string
	CreatedBy      uuid.UUID
	ActivatedAt    *time.Time
	CompletedAt    *time.Time
	IdempotencyKey string
	Version        int64
	CreatedAt      time.Time
}

// StudyClassRow is one row of learning.efficacy_study_classes (T).
type StudyClassRow struct {
	StudyID      uuid.UUID
	ClassID      uuid.UUID
	TenantID     uuid.UUID
	SchoolID     uuid.UUID
	GradeLevel   int
	PairNo       *int
	Arm          *string
	Method       string
	FormOrder    *string
	Status       string
	WithdrawnAt  *time.Time
	WithdrawalID *uuid.UUID
	AddedBy      uuid.UUID
	Deleted      bool // a soft-deleted row (removed class), revived when the class is added again
}

// ParticipantRow is one row of learning.efficacy_participants (T).
type ParticipantRow struct {
	StudyID         uuid.UUID
	StudentID       uuid.UUID
	TenantID        uuid.UUID
	SchoolID        uuid.UUID
	ClassID         uuid.UUID
	Arm             string
	FormOrder       string
	Status          string
	ExclusionReason *string
	EnrolledAt      time.Time
	WithdrawnAt     *time.Time
}

// WaveRow is one row of learning.efficacy_waves (G).
type WaveRow struct {
	ID             uuid.UUID
	StudyID        uuid.UUID
	Kind           string
	Form           *string
	WindowStart    time.Time
	WindowEnd      time.Time
	DurationSec    int
	Status         string
	OpenedAt       *time.Time
	ClosedAt       *time.Time
	CreatedBy      uuid.UUID
	IdempotencyKey string
	Version        int64
	CreatedAt      time.Time
}

// WaveSessionRow is one row of learning.efficacy_wave_sessions (T).
type WaveSessionRow struct {
	WaveID        uuid.UUID
	ClassID       uuid.UUID
	TenantID      uuid.UUID
	SchoolID      uuid.UUID
	ExamSessionID uuid.UUID
	Form          string
	Status        string
}

// WithdrawalRow is one row of learning.efficacy_withdrawals (T, append-only).
type WithdrawalRow struct {
	ID                uuid.UUID
	TenantID          uuid.UUID
	StudyID           *uuid.UUID
	Scope             string
	SchoolID          *uuid.UUID
	ClassID           *uuid.UUID
	StudentIDs        []uuid.UUID
	Reason            string
	SchoolRequestRef  *string
	Affected          int
	CancelledSessions int
	RequestedBy       uuid.UUID
	IdempotencyKey    string
	CreatedAt         time.Time
}

// SnapshotRow is one row of learning.efficacy_mastery_snapshots (T): M_eff of a participant on an objective.
type SnapshotRow struct {
	StudyID       uuid.UUID
	StudentID     uuid.UUID
	ObjectiveID   uuid.UUID
	TenantID      uuid.UUID
	MEff          float64
	Level         string
	ModelVersion  string
	EvidenceCount int
	TakenAt       time.Time
}

// EfficacyReportRow is one row of learning.efficacy_reports (G).
type EfficacyReportRow struct {
	ID              uuid.UUID
	StudyID         uuid.UUID
	Status          string
	Excluded        []uuid.UUID // params.excluded_question_version_ids
	Results         json.RawMessage
	InputsHash      []byte
	DescriptiveOnly bool
	Error           *string
	RequestedBy     uuid.UUID
	IdempotencyKey  string
	StartedAt       *time.Time
	FinishedAt      *time.Time
	CreatedAt       time.Time
}

// EfficacyClassRef is a class of the identity projection (ref.classes).
type EfficacyClassRef struct {
	ClassID    uuid.UUID
	TenantID   uuid.UUID
	SchoolID   uuid.UUID
	GradeLevel int
}

// WithdrawScope selects the participants of a withdrawal (zero values = no restriction).
type WithdrawScope struct {
	TenantID   uuid.UUID
	StudyID    *uuid.UUID // nil = every draft/active study of the tenant
	SchoolID   *uuid.UUID
	ClassID    *uuid.UUID
	StudentIDs []uuid.UUID
}

// EfficacyStore persists the study (F035 US2–US4, US6). Every method runs in the transaction of ctx: G tables have no
// tenant; T tables filter the tenant given (or the session tenant), deleted_at IS NULL on every read.
type EfficacyStore interface {
	// Agreements.
	InsertAgreement(ctx context.Context, a AgreementRow) error
	Agreement(ctx context.Context, tenant, id uuid.UUID, lock bool) (*AgreementRow, error)
	AgreementByRef(ctx context.Context, tenant uuid.UUID, ref string) (*AgreementRow, error)
	Agreements(ctx context.Context, tenant uuid.UUID) ([]AgreementRow, error)
	UpdateAgreement(ctx context.Context, a AgreementRow, prev int64) (bool, error)
	TenantIsQA(ctx context.Context, tenant uuid.UUID) (bool, error)

	// Studies (G).
	InsertStudy(ctx context.Context, s StudyRow) error
	Study(ctx context.Context, id uuid.UUID, lock bool) (*StudyRow, error)
	StudyByKey(ctx context.Context, createdBy uuid.UUID, key string) (*StudyRow, error)
	StudyCodeTaken(ctx context.Context, code string, except uuid.UUID) (bool, error)
	UpdateStudy(ctx context.Context, s StudyRow, prev int64) (bool, error)

	// Classes.
	Class(ctx context.Context, tenant, classID uuid.UUID) (*EfficacyClassRef, error)
	StudyClass(ctx context.Context, studyID, classID uuid.UUID, withDeleted bool) (*StudyClassRow, error)
	StudyClasses(ctx context.Context, studyID uuid.UUID) ([]StudyClassRow, error)
	// ClassInOtherStudy reports the class enrolled in another draft or active study.
	ClassInOtherStudy(ctx context.Context, tenant, classID, studyID uuid.UUID) (bool, error)
	UpsertStudyClass(ctx context.Context, c StudyClassRow) error
	UpdateStudyClass(ctx context.Context, c StudyClassRow) error
	RemoveStudyClass(ctx context.Context, c StudyClassRow, by uuid.UUID) error
	// ClassStudents lists the active students of a class (ref.enrollments, ref.students status active: anonymized or
	// processing-restricted accounts are not active).
	ClassStudents(ctx context.Context, tenant, classID uuid.UUID) ([]uuid.UUID, error)

	// Waves.
	InsertWave(ctx context.Context, w WaveRow) error
	Wave(ctx context.Context, id uuid.UUID, lock bool) (*WaveRow, error)
	WaveByKey(ctx context.Context, createdBy uuid.UUID, key string) (*WaveRow, error)
	Waves(ctx context.Context, studyID uuid.UUID) ([]WaveRow, error)
	UpdateWave(ctx context.Context, w WaveRow, prev int64) (bool, error)
	InsertWaveSession(ctx context.Context, s WaveSessionRow) error
	WaveSessions(ctx context.Context, waveID uuid.UUID) ([]WaveSessionRow, error)
	CancelWaveSession(ctx context.Context, s WaveSessionRow) error
	// DueWaves lists the scheduled waves whose window started and the open waves whose window ended.
	DueWaves(ctx context.Context, now time.Time, limit int) ([]WaveRow, error)

	// Participants.
	InsertParticipants(ctx context.Context, ps []ParticipantRow) (int, error)
	Participants(ctx context.Context, studyID uuid.UUID, classID *uuid.UUID, status string) ([]ParticipantRow, error)
	// Withdraw moves the active participants of the scope to withdrawn and returns them.
	Withdraw(ctx context.Context, s WithdrawScope, at time.Time, withdrawalID uuid.UUID) ([]ParticipantRow, error)
	// MarkClassesWithdrawn marks the enrolled classes of the scope withdrawn (scope tenant, school or class).
	MarkClassesWithdrawn(ctx context.Context, s WithdrawScope, at time.Time, withdrawalID uuid.UUID) ([]StudyClassRow, error)
	// ExcludeStudent excludes a student from every study of the tenant (anonymized, processing_restricted).
	ExcludeStudent(ctx context.Context, tenant, studentID uuid.UUID, reason string, at time.Time) (int, error)

	// Withdrawals.
	InsertWithdrawal(ctx context.Context, w WithdrawalRow) error
	WithdrawalByKey(ctx context.Context, requestedBy uuid.UUID, key string) (*WithdrawalRow, error)
	// WithdrawnStudents lists the students withdrawn by name from a study (or from every study) of a tenant.
	WithdrawnStudents(ctx context.Context, tenant, studyID uuid.UUID) ([]uuid.UUID, error)

	// Snapshots.
	InsertSnapshots(ctx context.Context, rows []SnapshotRow) error
	// SoftDeleteSnapshots soft-deletes the snapshots of students (all studies when studyID is nil) with a reason.
	SoftDeleteSnapshots(ctx context.Context, tenant uuid.UUID, studyID *uuid.UUID, students []uuid.UUID, reason string, by *uuid.UUID) (int, error)

	// Reports (G).
	InsertReport(ctx context.Context, r EfficacyReportRow) error
	ReportByKey(ctx context.Context, requestedBy uuid.UUID, key string) (*EfficacyReportRow, error)
	ClaimReports(ctx context.Context, staleBefore time.Time, limit int) ([]EfficacyReportRow, error)
	FinishReport(ctx context.Context, r EfficacyReportRow) error

	// StudentMasteries reads the live mastery rows of students on objectives (active model, tenant given).
	StudentMasteries(ctx context.Context, tenant uuid.UUID, students, objectives []uuid.UUID) ([]MasteryRow, error)
	// ActiveModel returns the active mastery model configuration.
	ActiveModel(ctx context.Context) (ModelConfig, error)

	// SessionTenant makes tenant the session tenant of the transaction (infra.current_tenant_id()) for the exam-engine
	// stores called while a platform job or a System Admin command writes into that tenant (bypass efficacy_study).
	SessionTenant(ctx context.Context, tenant uuid.UUID) error
}

// FormVersion is a platform blueprint version used as a form of a study.
type FormVersion struct {
	VersionID   uuid.UUID
	ItemID      uuid.UUID
	Status      string
	SubjectID   *uuid.UUID
	GradeLevels []int
	Body        json.RawMessage
}

// InstrumentQuestion is a published platform question version as a form reads it.
type InstrumentQuestion struct {
	VersionID    uuid.UUID
	Efficacy     bool
	ObjectiveIDs []uuid.UUID
}

// EfficacyContent reads the platform instruments of a study (content.* tables, platform rows only).
type EfficacyContent interface {
	// FormVersion returns a live platform blueprint version (nil when unknown).
	FormVersion(ctx context.Context, versionID uuid.UUID) (*FormVersion, error)
	// InstrumentQuestions returns the published platform question versions among ids with their objectives.
	InstrumentQuestions(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]InstrumentQuestion, error)
	// InstrumentPool returns the published platform efficacy question versions among ids as selection candidates (the pool of an
	// efficacy exam session: the ordinary pools never hold them).
	InstrumentPool(ctx context.Context, ids []uuid.UUID) ([]selection.Candidate, error)
}

// ExamExclusions removes students from a scheduled exam session (F035-AC-18): their participations not started are
// soft-deleted (reason efficacy_withdrawn).
type ExamExclusions interface {
	ExcludeParticipations(ctx context.Context, sessionID uuid.UUID, students []uuid.UUID, by uuid.UUID) (int, error)
}
