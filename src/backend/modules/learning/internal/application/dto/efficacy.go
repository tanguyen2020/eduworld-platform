package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ResearchAgreementView is ResearchAgreement (F035).
type ResearchAgreementView struct {
	AgreementID  uuid.UUID  `json:"agreement_id"`
	TenantID     uuid.UUID  `json:"tenant_id"`
	AgreementRef string     `json:"agreement_ref"`
	Covers       []string   `json:"covers"`
	AgreedOn     string     `json:"agreed_on"`
	Status       string     `json:"status"`
	RecordedBy   uuid.UUID  `json:"recorded_by"`
	RevokedAt    *time.Time `json:"revoked_at"`
	RevokedBy    *uuid.UUID `json:"revoked_by"`
	RevokeReason *string    `json:"revoke_reason"`
	CreatedAt    time.Time  `json:"created_at"`
}

// EfficacyStudyView is EfficacyStudy.
type EfficacyStudyView struct {
	StudyID      uuid.UUID   `json:"study_id"`
	Code         string      `json:"code"`
	Title        string      `json:"title"`
	SubjectID    uuid.UUID   `json:"subject_id"`
	GradeLevel   int         `json:"grade_level"`
	ObjectiveIDs []uuid.UUID `json:"objective_ids"`
	FormA        uuid.UUID   `json:"form_a_blueprint_version_id"`
	FormB        uuid.UUID   `json:"form_b_blueprint_version_id"`
	MinWeeks     int         `json:"min_intervention_weeks"`
	Status       string      `json:"status"`
	ClassCount   int         `json:"class_count"`
	PairCount    int         `json:"pair_count"`
	TenantIDs    []uuid.UUID `json:"tenant_ids"`
	CreatedBy    uuid.UUID   `json:"created_by"`
	CreatedAt    time.Time   `json:"created_at"`
	ActivatedAt  *time.Time  `json:"activated_at"`
	CompletedAt  *time.Time  `json:"completed_at"`
	Version      int64       `json:"version"`
}

// EfficacyStudyClassView is EfficacyStudyClass.
type EfficacyStudyClassView struct {
	StudyID          uuid.UUID  `json:"study_id"`
	ClassID          uuid.UUID  `json:"class_id"`
	TenantID         uuid.UUID  `json:"tenant_id"`
	SchoolID         uuid.UUID  `json:"school_id"`
	ClassName        string     `json:"class_name,omitempty"`
	GradeLevel       int        `json:"grade_level"`
	PairNo           *int       `json:"pair_no"`
	Arm              *string    `json:"arm"`
	AssignmentMethod string     `json:"assignment_method"`
	FormOrder        *string    `json:"form_order"`
	Status           string     `json:"status"`
	ParticipantCount int        `json:"participant_count"`
	WithdrawnAt      *time.Time `json:"withdrawn_at"`
}

// EfficacyWaveClassView is one class of EfficacyWave.classes (counts only, never scores).
type EfficacyWaveClassView struct {
	ClassID        uuid.UUID `json:"class_id"`
	TenantID       uuid.UUID `json:"tenant_id"`
	ExamSessionID  uuid.UUID `json:"exam_session_id"`
	Form           string    `json:"form"`
	SessionStatus  string    `json:"session_status"`
	InvitedCount   int       `json:"invited_count"`
	CompletedCount int       `json:"completed_count"`
	AbsentCount    int       `json:"absent_count"`
}

// EfficacyWaveView is EfficacyWave.
type EfficacyWaveView struct {
	WaveID          uuid.UUID               `json:"wave_id"`
	StudyID         uuid.UUID               `json:"study_id"`
	Kind            string                  `json:"kind"`
	Form            *string                 `json:"form"`
	WindowStart     time.Time               `json:"window_start"`
	WindowEnd       time.Time               `json:"window_end"`
	DurationSeconds int                     `json:"duration_seconds"`
	Status          string                  `json:"status"`
	OpenedAt        *time.Time              `json:"opened_at"`
	ClosedAt        *time.Time              `json:"closed_at"`
	Classes         []EfficacyWaveClassView `json:"classes"`
}

// EfficacyWithdrawalView is EfficacyWithdrawal.
type EfficacyWithdrawalView struct {
	WithdrawalID              uuid.UUID  `json:"withdrawal_id"`
	TenantID                  uuid.UUID  `json:"tenant_id"`
	StudyID                   *uuid.UUID `json:"study_id"`
	Scope                     string     `json:"scope"`
	AffectedCount             int        `json:"affected_count"`
	CancelledExamSessionCount int        `json:"cancelled_exam_session_count"`
	CreatedAt                 time.Time  `json:"created_at"`
}

// EfficacyReportView is EfficacyReport.
type EfficacyReportView struct {
	ReportID        uuid.UUID       `json:"report_id"`
	StudyID         uuid.UUID       `json:"study_id"`
	Status          string          `json:"status"`
	DescriptiveOnly bool            `json:"descriptive_only"`
	Excluded        []uuid.UUID     `json:"excluded_question_version_ids"`
	Results         json.RawMessage `json:"results"`
	Error           *string         `json:"error"`
	RequestedBy     uuid.UUID       `json:"requested_by"`
	CreatedAt       time.Time       `json:"created_at"`
	StartedAt       *time.Time      `json:"started_at"`
	FinishedAt      *time.Time      `json:"finished_at"`
}

// ExperimentArmView is ExperimentArm.
type ExperimentArmView struct {
	Code          string          `json:"code"`
	AllocationPct int             `json:"allocation_pct"`
	IsControl     bool            `json:"is_control"`
	ModelVersion  string          `json:"model_version,omitempty"`
	RuleParams    json.RawMessage `json:"rule_params,omitempty"`
}

// LearningExperimentView is LearningExperiment.
type LearningExperimentView struct {
	ExperimentID uuid.UUID           `json:"experiment_id"`
	Code         string              `json:"code"`
	Kind         string              `json:"kind"`
	Unit         string              `json:"unit"`
	Arms         []ExperimentArmView `json:"arms"`
	TenantIDs    []uuid.UUID         `json:"tenant_ids"`
	StudyID      *uuid.UUID          `json:"study_id"`
	Status       string              `json:"status"`
	StopReason   *string             `json:"stop_reason"`
	StartAt      time.Time           `json:"start_at"`
	EndAt        time.Time           `json:"end_at"`
	Revision     int                 `json:"revision"`
	OwnerID      uuid.UUID           `json:"owner_id"`
	CreatedAt    time.Time           `json:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at"`
}

// ExperimentResultsView is ExperimentResults.
type ExperimentResultsView struct {
	ExperimentID      uuid.UUID        `json:"experiment_id"`
	ComputedAt        time.Time        `json:"computed_at"`
	GuardrailBreached bool             `json:"guardrail_breached"`
	Arms              []map[string]any `json:"arms"`
}
