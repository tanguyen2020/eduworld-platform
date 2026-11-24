package read

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// OncallShiftRow is one on-call shift of the Safety Team (chat.moderation_oncall_shifts, F124): platform data, no tenant.
type OncallShiftRow struct {
	ID        uuid.UUID
	Moderator uuid.UUID
	Role      string
	Start     time.Time
	End       time.Time
	Note      *string
	CreatedBy *uuid.UUID
	CreatedAt time.Time
	Version   int64
}

// ShiftFilter is the query of listModerationOncallShifts: shifts meeting [From, To), one role, keyset (starts_at, shift_id).
type ShiftFilter struct {
	From, To   *time.Time
	Role       string
	AfterStart *time.Time
	AfterID    *uuid.UUID
	Limit      int
}

// OncallReader reads the on-call shifts (one page: Limit+1 rows from the database, never sliced in memory).
type OncallReader interface {
	ListShifts(ctx context.Context, f ShiftFilter) ([]OncallShiftRow, error)
}

// GrantRow is a break-glass request/grant (chat.break_glass_grants, F124): never message content.
type GrantRow struct {
	ID                  uuid.UUID
	TenantID            uuid.UUID
	ScopeType           string
	ScopeID             uuid.UUID
	WindowFrom          time.Time
	WindowTo            time.Time
	DurationMinutes     int
	Reason              string
	Status              string
	RequestedBy         uuid.UUID
	RequestedAt         time.Time
	DecidedBy           *uuid.UUID
	DecidedAt           *time.Time
	DecisionNote        *string
	ExpiresAt           *time.Time
	RevokedBy           *uuid.UUID
	RevokedAt           *time.Time
	RevokeReason        *string
	MessagesViewedCount int
	Version             int64
}

// GrantFilter is the query of listBreakGlassGrants: keyset (requested_at, grant_id) descending.
type GrantFilter struct {
	TenantIDs   []uuid.UUID
	Status      string
	RequestedBy *uuid.UUID // the requester's own grants (platform.break_glass.request only)
	BeforeAt    *time.Time
	BeforeID    *uuid.UUID
	Limit       int
}

// BreakGlassMessage is one message in the scope of a grant (original content; shown to the requester only).
type BreakGlassMessage struct {
	MessageID       uuid.UUID
	ChannelID       uuid.UUID
	Seq             int64
	SenderID        uuid.UUID
	SenderSchoolID  *uuid.UUID
	Kind            string
	Body            *string
	BodyDisplay     *string
	QuickMessageID  *string
	Revision        int
	ModerationState string
	RecalledAt      *time.Time
	CreatedAt       time.Time
}

// MessageFilter is the scope and page of listBreakGlassMessages: the messages of the target tenant in [From, To) of one
// channel or sent by one user, keyset (created_at, message_id) ascending.
type MessageFilter struct {
	TenantID  uuid.UUID
	ScopeType string
	ScopeID   uuid.UUID
	From, To  time.Time
	AfterAt   *time.Time
	AfterID   *uuid.UUID
	Limit     int
}

// BreakGlassReader reads grants and the messages of a grant (callers run it under tenancy.Bypass(ctx, "break_glass")).
type BreakGlassReader interface {
	ListGrants(ctx context.Context, f GrantFilter) ([]GrantRow, error)
	Grant(ctx context.Context, id uuid.UUID) (*GrantRow, error)
	Messages(ctx context.Context, f MessageFilter) ([]BreakGlassMessage, error)
}
