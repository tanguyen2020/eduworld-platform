package write

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// AccountLock is one lock of an account by one source (identity.account_locks, F124).
type AccountLock struct {
	ID            uuid.UUID
	TenantID      *uuid.UUID
	UserID        uuid.UUID
	Source        string
	SourceRefID   *uuid.UUID // sanction_id of a moderation lock
	CaseID        *uuid.UUID // id only, never content
	Reason        *string
	CreatedBy     *uuid.UUID
	CreatedAt     time.Time
	ReleasedAt    *time.Time
	SourceEventID *uuid.UUID
}

// AccountLockRepository reads and writes the locks of accounts (F124, ACCOUNT_PROVISIONING §6) in the transaction of ctx.
type AccountLockRepository interface {
	// ActiveSources returns the sources of the locks of user not released (one entry per lock).
	ActiveSources(ctx context.Context, user uuid.UUID) ([]string, error)
	// Add inserts a lock; false when an equivalent lock already exists (same sanction, or an active lock of the same
	// source without reference).
	Add(ctx context.Context, l AccountLock) (bool, error)
	// ReleaseSource releases every active lock of user from source; it returns how many were released.
	ReleaseSource(ctx context.Context, user uuid.UUID, source string, by *uuid.UUID, reason string, event *uuid.UUID, at time.Time) (int, error)
	// BySanction returns the lock of a sanction (released or not), nil when none.
	BySanction(ctx context.Context, sanction uuid.UUID) (*AccountLock, error)
	// ReleaseSanction releases the lock of a sanction; false when it was already released.
	ReleaseSanction(ctx context.Context, sanction uuid.UUID, event *uuid.UUID, at time.Time) (bool, error)
}
