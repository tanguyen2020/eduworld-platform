package write

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
)

// OncallStore writes the on-call shifts and reads the rotation state of the p1 cases (F124 US3).
type OncallStore interface {
	// LockRole serialises the writers of one role in the transaction (pg_advisory_xact_lock), so the overlap check holds.
	LockRole(ctx context.Context, role string) error
	// Overlapping reports a live shift of role sharing an instant with [start, end).
	Overlapping(ctx context.Context, role string, start, end time.Time) (bool, error)
	InsertShift(ctx context.Context, s read.OncallShiftRow) error
	// LockShift returns a live shift for update (nil when unknown or deleted).
	LockShift(ctx context.Context, id uuid.UUID) (*read.OncallShiftRow, error)
	SoftDeleteShift(ctx context.Context, id, by uuid.UUID, at time.Time) error
	// CurrentShift returns the live shift of role covering at (the earliest started one), nil when none.
	CurrentShift(ctx context.Context, role string, at time.Time) (*read.OncallShiftRow, error)
	// UnclaimedP1 lists the open p1 cases of every tenant (work list of oncall-escalator; caller under tenancy.Bypass).
	UnclaimedP1(ctx context.Context, limit int) ([]read.CaseRow, error)
}

// BreakGlassStore writes the break-glass grants (F124 US4) in the scope of the target tenant.
type BreakGlassStore interface {
	InsertGrant(ctx context.Context, g read.GrantRow) error
	// LockGrant returns a live grant of tenant for update (nil when unknown there).
	LockGrant(ctx context.Context, tenant, id uuid.UUID) (*read.GrantRow, error)
	UpdateGrant(ctx context.Context, g read.GrantRow, actor *uuid.UUID) error
	// AddViewed adds n to messages_viewed_count.
	AddViewed(ctx context.Context, tenant, id uuid.UUID, n int) error
	// ExpiredGrants lists the approved grants past expires_at of every tenant (caller under tenancy.Bypass).
	ExpiredGrants(ctx context.Context, now time.Time, limit int) ([]read.GrantRow, error)
}
