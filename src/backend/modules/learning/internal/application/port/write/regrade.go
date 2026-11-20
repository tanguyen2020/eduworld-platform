package write

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/evaluation"
)

// RegradeRequest is a row of assessment.regrade_requests (F031 US4).
type RegradeRequest struct {
	ID                 uuid.UUID
	TenantID           *uuid.UUID // nil = platform request of content_admin
	SubjectID          *uuid.UUID
	Scope              string
	QuestionVersionID  *uuid.UUID
	ActivityVersionID  *uuid.UUID
	CorrectedVersionID *uuid.UUID
	Action             evaluation.Action
	Reason             string
	Status             string
	Affected           int
	ReEvaluated        int
	RequestedBy        uuid.UUID
	RequestedRole      string
	IdempotencyKey     string
	CreatedAt          time.Time
	StartedAt          *time.Time
	FinishedAt         *time.Time
}

// Target returns the version the request is about.
func (r RegradeRequest) Target() uuid.UUID {
	if r.QuestionVersionID != nil {
		return *r.QuestionVersionID
	}
	if r.ActivityVersionID != nil {
		return *r.ActivityVersionID
	}
	return uuid.Nil
}

// VersionInfo describes a question or activity version seen from the scope of ctx.
type VersionInfo struct {
	ID        uuid.UUID
	ItemID    uuid.UUID // question_id or activity_id
	TenantID  *uuid.UUID
	SubjectID *uuid.UUID
	Status    string
}

// EffectiveEvaluation is the effective evaluation of an attempt with its item results.
type EffectiveEvaluation struct {
	ID       uuid.UUID
	Status   string
	Score    float64
	MaxScore float64
	Items    []evaluation.Item
}

// AffectedAttempt is a graded attempt touched by a request.
type AffectedAttempt struct {
	ID       uuid.UUID
	TenantID uuid.UUID
}

// RegradeStore persists regrade requests and reads what the re-evaluation needs (explicit SQL in the transaction of
// ctx: TenantScope, PlatformScope or tenancy.Bypass(re_evaluation) for the job).
type RegradeStore interface {
	// InsertRequest inserts a queued request; false when a queued/running request exists for the same version and tenant.
	InsertRequest(ctx context.Context, r RegradeRequest) (bool, error)
	// RequestByKey returns the request of requestedBy with an Idempotency-Key (nil when none).
	RequestByKey(ctx context.Context, requestedBy uuid.UUID, key string) (*RegradeRequest, error)
	// ClaimQueued marks the oldest queued request running and returns it (nil when none; FOR UPDATE SKIP LOCKED).
	ClaimQueued(ctx context.Context, now time.Time) (*RegradeRequest, error)
	// FinishRequest records the end of a request.
	FinishRequest(ctx context.Context, id uuid.UUID, status string, affected, done int, lastError string, now time.Time) error
	// QuestionVersion and ActivityVersion read a version visible in the scope of ctx (nil when invisible).
	QuestionVersion(ctx context.Context, id uuid.UUID) (*VersionInfo, error)
	ActivityVersion(ctx context.Context, id uuid.UUID) (*VersionInfo, error)
	// AffectedAttempts lists the graded attempts (mode ≠ exam) of the request: tenant of the request, or every tenant.
	AffectedAttempts(ctx context.Context, r RegradeRequest) ([]AffectedAttempt, error)
	// Effective returns the effective evaluation of an attempt (nil when none).
	Effective(ctx context.Context, attemptID uuid.UUID) (*EffectiveEvaluation, error)
	// Supersede turns the effective evaluation id into superseded (problem EVALUATION_NOT_EDITABLE when it is not final).
	Supersede(ctx context.Context, id uuid.UUID) error
	// LastScoredResponse returns the response of the last scored answer of a position (false when unanswered).
	LastScoredResponse(ctx context.Context, attemptID uuid.UUID, position int) (json.RawMessage, bool, error)
}
