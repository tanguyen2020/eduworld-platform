// Package write holds the write-context ports of module chat (ADR-0087): repositories load and save inside
// entx.UnitOfWork; events and audit go to the outbox of the same transaction.
package write

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/social"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// UnitOfWork runs fn in one write transaction (entx.UnitOfWork); InTenantTx uses an explicit scope (jobs, consumers).
type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
	InTenantTx(ctx context.Context, s tenancy.Scope, fn func(ctx context.Context) error) error
}

// AfterCommit registers fn to run after the commit of the transaction of ctx (cache invalidation, best effort).
type AfterCommit func(ctx context.Context, fn func(ctx context.Context)) error

// Event is a domain event written to the outbox. TenantID nil takes the session tenant of the scope; Platform marks a
// platform event (tenant_id NULL, e.g. a change of the platform catalogue).
type Event struct {
	Type         string
	SubjectType  string
	SubjectID    string
	PartitionKey string
	TenantID     *uuid.UUID
	Platform     bool
	OccurredAt   time.Time
	Payload      any
}

// EventPublisher writes events into the outbox of the transaction.
type EventPublisher interface {
	Publish(ctx context.Context, e Event) error
}

// Auditor records audit events in the transaction (audit.recorded via the outbox).
type Auditor interface {
	Record(ctx context.Context, e audit.Event) error
}

// Card is the public card of a student (ref.player_profiles): never a real name, school or class (R16).
type Card struct {
	UserID   uuid.UUID
	Nickname *string
	Level    int
	Avatar   json.RawMessage
}

// Directory reads the identity and game projections inside the transaction (ref.tenants, ref.students,
// ref.player_profiles).
type Directory interface {
	// Tenant returns the projected tenant, nil when not projected yet.
	Tenant(ctx context.Context, tenant uuid.UUID) (*social.Tenant, error)
	// Student returns the projected student of tenant, nil when unknown in that tenant.
	Student(ctx context.Context, tenant, student uuid.UUID) (*social.Student, error)
	// Cards returns the public cards of the students of tenant (missing profiles are absent).
	Cards(ctx context.Context, tenant uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]Card, error)
}

// ErrCodeTaken: the generated friend code is already live in the tenant (retry with another code).
var ErrCodeTaken = errors.New("social: friend code taken")

// SocialRepository persists the friend requests, friendships, friend codes, blocks and mutes (social.*). Every method
// runs in the transaction of ctx under its TenantScope with explicit tenant_id and deleted_at conditions (R27).
type SocialRepository interface {
	// LockUsers serializes the social writes of the given students (pg_advisory_xact_lock, sorted).
	LockUsers(ctx context.Context, ids ...uuid.UUID) error
	// Blocked reports a live block between a and b in either direction.
	Blocked(ctx context.Context, tenant, a, b uuid.UUID) (bool, error)
	// Friends reports a live friendship of the pair.
	Friends(ctx context.Context, tenant uuid.UUID, p social.Pair) (bool, error)
	// CountFriends counts the live friendships of user.
	CountFriends(ctx context.Context, tenant, user uuid.UUID) (int, error)
	// CountPending counts the live pending requests sent by requester at now.
	CountPending(ctx context.Context, tenant, requester uuid.UUID, now time.Time) (int, error)
	// CountSince counts the requests sent by requester since since (per-day limit; every status).
	CountSince(ctx context.Context, tenant, requester uuid.UUID, since time.Time) (int, error)
	// PendingFrom reports a live pending request from requester to addressee.
	PendingFrom(ctx context.Context, tenant, requester, addressee uuid.UUID, now time.Time) (bool, error)
	// ExpireStale marks expired the pending requests of the pair (either direction) past their expiry, so the unique
	// pending index does not refuse a new request before the job ran.
	ExpireStale(ctx context.Context, tenant, a, b uuid.UUID, now time.Time) error
	// CreateRequest inserts a pending request.
	CreateRequest(ctx context.Context, r *social.FriendRequest) error
	// Request returns a request of tenant (locked FOR UPDATE when lock), nil when unknown.
	Request(ctx context.Context, tenant, id uuid.UUID, lock bool) (*social.FriendRequest, error)
	// SaveRequest writes the status and responded_at of r.
	SaveRequest(ctx context.Context, r *social.FriendRequest) error
	// CancelPending cancels the live pending requests between a and b (both directions); returns how many.
	CancelPending(ctx context.Context, tenant, a, b uuid.UUID, now time.Time) (int, error)
	// CreateFriendship inserts the live friendship of the pair.
	CreateFriendship(ctx context.Context, tenant uuid.UUID, p social.Pair) error
	// RemoveFriendship soft-deletes the live friendship of the pair with reason; false when none.
	RemoveFriendship(ctx context.Context, tenant uuid.UUID, p social.Pair, reason string) (bool, error)
	// ReplaceCode soft-deletes the live friend code of owner.
	ReplaceCode(ctx context.Context, tenant, owner uuid.UUID) error
	// CreateCode inserts a live friend code (ErrCodeTaken when the code is live already).
	CreateCode(ctx context.Context, tenant, owner uuid.UUID, code string, expires time.Time) error
	// CodeOwner returns the owner of a live, unexpired code, nil when none.
	CodeOwner(ctx context.Context, tenant uuid.UUID, code string, now time.Time) (*uuid.UUID, error)
	// SetBlock creates (on) or soft-deletes (off) the block blocker → blocked; reports whether a row changed.
	SetBlock(ctx context.Context, tenant, blocker, blocked uuid.UUID, on bool) (bool, error)
	// SetMute creates (on) or soft-deletes (off) the mute muter → muted; reports whether a row changed.
	SetMute(ctx context.Context, tenant, muter, muted uuid.UUID, on bool) (bool, error)
	// CancelOfStudent cancels every live pending request sent or received by student (cleanup, edge case).
	CancelOfStudent(ctx context.Context, tenant, student uuid.UUID, now time.Time) (int, error)
}

// ExpiryRepository is the platform side of the job expire-friend-requests (PlatformScope: every tenant).
type ExpiryRepository interface {
	// ExpireRequests marks expired up to limit pending requests past their expiry; returns how many.
	ExpireRequests(ctx context.Context, now time.Time, limit int) (int, error)
	// ExpireCodes soft-deletes up to limit live friend codes past their expiry; returns how many.
	ExpireCodes(ctx context.Context, now time.Time, limit int) (int, error)
}

// ProjectionWriter writes the ref.* projections (consumers only, DATABASE_DESIGN §2.17).
type ProjectionWriter interface {
	OwnerTenant(ctx context.Context, table string, id uuid.UUID) (*uuid.UUID, error)
	Tenant(ctx context.Context, tenant uuid.UUID, code, name *string, typ, status string, isQA *bool, settings []byte, version int64) (bool, error)
	Student(ctx context.Context, student, tenant, school uuid.UUID, grade *int, status *string, version int64) (bool, error)
	StudentStatus(ctx context.Context, student uuid.UUID, tenant *uuid.UUID, status string) (bool, error)
	StudentDeleted(ctx context.Context, student, tenant uuid.UUID, deleted bool, at time.Time) (bool, error)
	PlayerProfile(ctx context.Context, student, tenant uuid.UUID, nickname *string, level int, avatar []byte, version int64) (bool, error)
	School(ctx context.Context, school, tenant uuid.UUID, status *string, settings []byte, version int64) (bool, error)
	PlatformSetting(ctx context.Context, key string, value []byte, version int64) (bool, error)
	// TenantRetention stores gameplay_chat_retention_days of a tenant (nil: inherit the platform; F117 research R4); ok
	// false while the tenant is not projected.
	TenantRetention(ctx context.Context, tenant uuid.UUID, days *int, version int64) (bool, error)
}
