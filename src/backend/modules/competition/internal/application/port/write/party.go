// Package write holds the write-context ports of the party use cases of module competition (ADR-0087): repositories
// load and save inside entx.UnitOfWork; events and audit go to the outbox of the same transaction.
package write

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// UnitOfWork runs fn in one write transaction (entx.UnitOfWork); InTenantTx uses an explicit scope (jobs, consumers).
type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
	InTenantTx(ctx context.Context, s tenancy.Scope, fn func(ctx context.Context) error) error
}

// AfterCommit registers fn to run after the commit of the transaction of ctx (realtime push, Valkey state).
type AfterCommit func(ctx context.Context, fn func(ctx context.Context)) error

// Event is a domain event written to the outbox. TenantID nil takes the session tenant of the scope.
type Event struct {
	Type         string
	SubjectType  string
	SubjectID    string
	PartitionKey string
	TenantID     *uuid.UUID
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

// Tenant is the projected tenant (ref.tenants): its type and the cross-school social setting (ADR-0054).
type Tenant struct {
	Type        string
	CrossSchool bool
}

// Directory reads the projections of module competition (ref.*) in the transaction of ctx: data of other modules come
// from events only (R13, R29).
type Directory interface {
	// Tenant returns the projected tenant, nil when not projected yet.
	Tenant(ctx context.Context, tenant uuid.UUID) (*Tenant, error)
	// Subject returns the policy view of a student of tenant at now, nil when unknown in that tenant.
	Subject(ctx context.Context, tenant, student uuid.UUID, now time.Time) (*party.Subject, error)
	// Friends reports a live friendship of a and b.
	Friends(ctx context.Context, tenant, a, b uuid.UUID) (bool, error)
	// Blocked reports a live block between student and any of others, either direction.
	Blocked(ctx context.Context, tenant, student uuid.UUID, others []uuid.UUID) (bool, error)
	// Overrides returns the party_policies rules of the school and of the classes (live rows only).
	Overrides(ctx context.Context, tenant, school uuid.UUID, classes []uuid.UUID) ([]party.Override, error)
	// Cards returns the public cards of the students of tenant (missing profiles are absent).
	Cards(ctx context.Context, tenant uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]port.Card, error)
}

// ErrAlreadyInParty: the student already holds an active membership (unique party_members_one_active_party_uq).
var ErrAlreadyInParty = errors.New("party: student already in an active party")

// ErrPendingExists: a pending invitation of the party to the same student exists (party_invitations_pending_uq).
var ErrPendingExists = errors.New("party: pending invitation exists")

// History is a row of multiplayer.party_histories (ids and enums only, no PII).
type History struct {
	TenantID  uuid.UUID
	SchoolID  uuid.UUID
	PartyID   uuid.UUID
	EventType string
	ActorID   *uuid.UUID
	Data      map[string]any
	At        time.Time
}

// PendingFilter selects pending invitations; nil fields do not filter. Between matches either direction.
type PendingFilter struct {
	PartyID     *uuid.UUID
	InviterID   *uuid.UUID
	InviteeID   *uuid.UUID
	Student     *uuid.UUID    // inviter or invitee
	Between     *[2]uuid.UUID // a → b or b → a
	ExceptParty *uuid.UUID    // not this party
	Expired     *time.Time    // expires_at ≤ this instant
}

// PartyRepository persists parties, memberships, invitations and the party history (multiplayer.*). Every method runs
// in the transaction of ctx under its TenantScope with explicit tenant_id and deleted_at conditions (R27).
type PartyRepository interface {
	// LockStudents serializes the membership writes of the given students (transaction advisory locks, sorted).
	LockStudents(ctx context.Context, ids ...uuid.UUID) error
	// ActivePartyOf returns the active party of student, uuid.Nil when none.
	ActivePartyOf(ctx context.Context, tenant, student uuid.UUID) (uuid.UUID, error)
	// Create inserts the party and its leader membership.
	Create(ctx context.Context, p *party.Party) error
	// Load returns the party with its active members (locked FOR UPDATE when lock), nil when unknown in tenant.
	Load(ctx context.Context, tenant, id uuid.UUID, lock bool) (*party.Party, error)
	// Save writes the settings, leader, status and state_version of p.
	Save(ctx context.Context, p *party.Party) error
	// AddMember inserts an active membership (ErrAlreadyInParty when the student is in another party).
	AddMember(ctx context.Context, p *party.Party, m party.Member) error
	// EndMember ends the active membership of student with reason.
	EndMember(ctx context.Context, tenant, partyID, student uuid.UUID, reason string, at time.Time) error
	// SetRoles writes the roles of the active members of p (demote first: one leader at a time).
	SetRoles(ctx context.Context, p *party.Party) error
	// AppendHistory appends a row to multiplayer.party_histories.
	AppendHistory(ctx context.Context, h History) error
	// KickedSince reports a membership of student in partyID that ended kicked after since.
	KickedSince(ctx context.Context, tenant, partyID, student uuid.UUID, since time.Time) (bool, error)
	// HistorySince returns the times of the history rows of actor with one of types after since, oldest first.
	HistorySince(ctx context.Context, tenant, actor uuid.UUID, types []string, since time.Time) ([]time.Time, error)
	// CreateInvitation inserts a pending invitation (ErrPendingExists on the pending unique index).
	CreateInvitation(ctx context.Context, i *party.Invitation) error
	// Invitation returns an invitation of tenant (locked FOR UPDATE when lock), nil when unknown.
	Invitation(ctx context.Context, tenant, id uuid.UUID, lock bool) (*party.Invitation, error)
	// SaveInvitation writes status, responded_at and end_reason of i.
	SaveInvitation(ctx context.Context, i *party.Invitation) error
	// Pending returns the pending invitations of tenant matching f, oldest first (locked FOR UPDATE).
	Pending(ctx context.Context, tenant uuid.UUID, f PendingFilter) ([]party.Invitation, error)
	// InvitesSince returns the creation times of the invitations sent by inviter after since, oldest first.
	InvitesSince(ctx context.Context, tenant, inviter uuid.UUID, since time.Time) ([]time.Time, error)
	// RejectionsSince returns the rejection times of the invitations from inviter to invitee after since, oldest first.
	RejectionsSince(ctx context.Context, tenant, inviter, invitee uuid.UUID, since time.Time) ([]time.Time, error)
	// AcceptedInvitation reports an accepted invitation of partyID to student that has not expired at now.
	AcceptedInvitation(ctx context.Context, tenant, partyID, student uuid.UUID, now time.Time) (bool, error)
	// MembershipsOf returns the active memberships of student (normally at most one) with their party ids.
	MembershipsOf(ctx context.Context, tenant, student uuid.UUID) ([]uuid.UUID, error)
}

// InvitationRef identifies a pending invitation of any tenant (job party-invitation-expiry).
type InvitationRef struct {
	TenantID     uuid.UUID
	InvitationID uuid.UUID
}

// ExpiryRepository is the platform side of the job party-invitation-expiry (SystemPlatformScope: every tenant).
type ExpiryRepository interface {
	// ExpiredPending lists up to limit pending invitations past their expiry.
	ExpiredPending(ctx context.Context, now time.Time, limit int) ([]InvitationRef, error)
}

// SanctionWriter keeps ref.sanctions (consumer competition.party_safety).
type SanctionWriter interface {
	// Sanction upserts a sanction (newer source_version only); returns whether a row changed.
	Sanction(ctx context.Context, tenant, sanction, user uuid.UUID, typ string, until *time.Time, active bool, version int64) (bool, error)
	// OwnerTenant returns the tenant owning a projected student whatever the session tenant (nil when unknown).
	OwnerTenant(ctx context.Context, student uuid.UUID) (*uuid.UUID, error)
}
