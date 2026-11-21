// Package channel is the channel aggregate of module chat (F115, CHAT_SYSTEM §2, §4): a conversation space of one
// tenant with its members. Pure Go: no I/O, no grade condition (ADR-0020, R15).
package channel

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Type is channel_type.
type Type string

// Channel types (DATABASE_DESIGN §2.12); no class or study_group channel since ADR-0095 (and no grade channel).
const (
	World  Type = "world"
	Area   Type = "area"
	Match  Type = "match"
	Party  Type = "party"
	Team   Type = "team"
	Direct Type = "direct"
)

// Types lists every channel type.
var Types = []Type{World, Area, Match, Party, Team, Direct}

// Valid reports a known channel type.
func (t Type) Valid() bool {
	for _, x := range Types {
		if x == t {
			return true
		}
	}
	return false
}

// AlwaysOn reports the gameplay channel types that tenant/school settings cannot turn off (ADR-0020 §6).
func (t Type) AlwaysOn() bool { return t == Match || t == Party || t == Team }

// Implicit reports world/area: membership is implicit (no channel_members, F120).
func (t Type) Implicit() bool { return t == World || t == Area }

// Status of a channel.
const (
	StatusOpen   = "open"
	StatusClosed = "closed"
)

// Member roles.
const (
	RoleMember = "member"
	RoleLeader = "leader"
)

// Errors of the aggregate.
var (
	ErrType     = errors.New("channel: unknown or unsupported channel type")
	ErrScope    = errors.New("channel: scope_ref does not match the channel type")
	ErrNoTenant = errors.New("channel: tenant is required")
)

// Channel is one conversation space.
type Channel struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	SchoolID   *uuid.UUID
	Type       Type
	IsGameplay bool
	ScopeRef   map[string]string
	Status     string
	LastSeq    int64
	OpenedAt   time.Time
	ClosedAt   *time.Time
}

// Gameplay reports match, party and team bound to a match (DATABASE_DESIGN §2.12 CHECK).
func Gameplay(t Type, scope map[string]string) bool {
	return t == Match || t == Party || (t == Team && scope["match_id"] != "")
}

// New builds an open channel. world/area are implicit channels of F120 and refused here.
func New(id, tenant uuid.UUID, school *uuid.UUID, t Type, scope map[string]string, now time.Time) (Channel, error) {
	switch {
	case tenant == uuid.Nil:
		return Channel{}, ErrNoTenant
	case !t.Valid() || t.Implicit():
		return Channel{}, ErrType
	}
	if id == uuid.Nil {
		return Channel{}, ErrScope
	}
	for k, v := range scope {
		if len(k) > 64 || len(v) > 64 {
			return Channel{}, ErrScope
		}
	}
	return Channel{ID: id, TenantID: tenant, SchoolID: school, Type: t, IsGameplay: Gameplay(t, scope), ScopeRef: scope,
		Status: StatusOpen, OpenedAt: now.UTC()}, nil
}

// Open reports an open channel (sending into a closed one is CHAT_CHANNEL_CLOSED, F117).
func (c Channel) Open() bool { return c.Status == StatusOpen }

// Member is one student in one channel.
type Member struct {
	ChannelID uuid.UUID
	UserID    uuid.UUID
	TenantID  uuid.UUID
	SchoolID  *uuid.UUID
	Role      string
	JoinedAt  time.Time
	LeftAt    *time.Time
}

// Active reports a member who has not left.
func (m Member) Active() bool { return m.LeftAt == nil }

// Members is the member list of a channel.
type Members []Member

// Active returns the active member of user, if any.
func (ms Members) Active(user uuid.UUID) (Member, bool) {
	for _, m := range ms {
		if m.UserID == user && m.Active() {
			return m, true
		}
	}
	return Member{}, false
}

// ActiveIDs returns the ids of the active members.
func (ms Members) ActiveIDs() []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ms))
	for _, m := range ms {
		if m.Active() {
			out = append(out, m.UserID)
		}
	}
	return out
}
