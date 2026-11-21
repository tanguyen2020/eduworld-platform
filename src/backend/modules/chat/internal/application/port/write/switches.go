package write

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
)

// SwitchChangeRecord is one row of chat.moderation_switch_changes (before/after of an effective change).
type SwitchChangeRecord struct {
	ID         uuid.UUID
	TenantID   *uuid.UUID
	Before     moderation.Switch
	After      moderation.Switch
	Reason     *string
	ExpiresAt  *time.Time
	ActorID    *uuid.UUID
	ApprovedBy *uuid.UUID
	RequestID  *uuid.UUID
	Source     string
	At         time.Time
}

// Sources of a switch change.
const (
	SourceManual          = "manual"
	SourceApprovedRequest = "approved_request"
	SourceAutoExpire      = "auto_expire"
)

// SwitchLevel addresses one level: the platform (TenantID nil) or a tenant.
type SwitchLevel struct {
	TenantID *uuid.UUID
}

// ExpiredRequest is a pending request past its approval window.
type ExpiredRequest struct {
	ID       uuid.UUID
	TenantID *uuid.UUID
}

// SwitchRepository writes the moderation switches in the transaction (F116 US3). tenant nil addresses the platform
// row (platform scope); a tenant its own row (tenant scope or a cross-tenant scope of exactly that tenant).
type SwitchRepository interface {
	// LockSwitch returns the live configured switch of the level locked until the end of the transaction (nil: not
	// configured, every step on).
	LockSwitch(ctx context.Context, tenant *uuid.UUID) (*read.SwitchRow, error)
	// SaveSwitch writes the configuration of the level (insert when cur is nil, otherwise update of cur when its
	// version did not change — ErrStale) and returns the stored row. Only switchable steps are stored (a legacy pii
	// key is dropped).
	SaveSwitch(ctx context.Context, tenant *uuid.UUID, cur *read.SwitchRow, next moderation.Switch, reason *string, actor *uuid.UUID,
		now time.Time) (*read.SwitchRow, error)
	InsertChange(ctx context.Context, c SwitchChangeRecord) error
	InsertRequest(ctx context.Context, r read.SwitchRequestRow) error
	// LockRequest returns the live request id of the level locked until the end of the transaction (nil: unknown).
	LockRequest(ctx context.Context, tenant *uuid.UUID, id uuid.UUID) (*read.SwitchRequestRow, error)
	// DecideRequest sets the status of a pending request (approved with its approver, expired, rejected).
	DecideRequest(ctx context.Context, id uuid.UUID, status string, approver *uuid.UUID, now time.Time) error
	// ExpiredSwitches returns the levels whose disabled values lapsed at now (system job: every tenant).
	ExpiredSwitches(ctx context.Context, now time.Time, limit int) ([]SwitchLevel, error)
	// ExpiredRequests returns the requests pending_approval past request_expires_at at now (system job: every tenant).
	ExpiredRequests(ctx context.Context, now time.Time, limit int) ([]ExpiredRequest, error)
}
