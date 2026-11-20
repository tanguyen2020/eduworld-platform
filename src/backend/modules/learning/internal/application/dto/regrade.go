package dto

import (
	"time"

	"github.com/google/uuid"
)

// RegradeRequestView is RegradeRequest of the API (createRegradeRequest, getRegradeRequest; F031 US4).
type RegradeRequestView struct {
	RegradeRequestID    uuid.UUID  `json:"regrade_request_id"`
	Scope               string     `json:"scope"`
	Action              string     `json:"action"`
	Status              string     `json:"status"`
	AffectedAttempts    int        `json:"affected_attempts"`
	ReEvaluatedAttempts int        `json:"re_evaluated_attempts"`
	Reason              string     `json:"reason"`
	RequestedBy         uuid.UUID  `json:"requested_by"`
	CreatedAt           time.Time  `json:"created_at"`
	StartedAt           *time.Time `json:"started_at,omitempty"`
	FinishedAt          *time.Time `json:"finished_at,omitempty"`
	QuestionVersionID   *uuid.UUID `json:"question_version_id,omitempty"`
	ActivityVersionID   *uuid.UUID `json:"activity_version_id,omitempty"`
	CorrectedVersionID  *uuid.UUID `json:"corrected_version_id,omitempty"`
	TenantID            *uuid.UUID `json:"tenant_id,omitempty"`
}
