// Package party is the bounded context party of module competition (F100, PARTY_SYSTEM): the Party aggregate with its
// active members, the invitations and the policy rules. Pure Go: no I/O, the clock and presence come from the caller.
package party

import (
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Party types (PARTY_SYSTEM §4).
const (
	TypeFriend          = "friend"
	TypeStudyGroup      = "study_group"
	TypeQuest           = "quest"
	TypeCompetitionTeam = "competition_team"
	TypeCustom          = "custom"
)

// Visibility of a party (PARTY_SYSTEM §3).
const (
	VisibilityPrivate = "private"
	VisibilityFriends = "friends"
	VisibilityClass   = "class"
)

// Invitation policies.
const (
	InviteLeaderOnly = "leader_only"
	InviteMembers    = "members"
)

// Party status (PARTY_SYSTEM §13.1); in_activity is written by F101.
const (
	StatusOpen       = "open"
	StatusInActivity = "in_activity"
	StatusDisbanded  = "disbanded"
)

// Member roles.
const (
	RoleLeader = "leader"
	RoleMember = "member"
)

// Reasons a membership ends (party_members.left_reason).
const (
	LeftLeft      = "left"
	LeftKicked    = "kicked"
	LeftDisbanded = "disbanded"
	LeftTimeout   = "timeout"
	LeftPolicy    = "policy"
)

// Reasons of party.leader_transferred.
const (
	TransferManual     = "manual"
	TransferLeaderLeft = "leader_left"
)

// Reasons of party.disbanded.
const (
	DisbandLeader     = "leader_disbanded"
	DisbandAllLeft    = "all_left"
	DisbandModeration = "moderation"
)

// Platform bounds of max_members (PARTY_SYSTEM §3: configurable 2–8, hard platform limit 8) and of the name.
const (
	MinMembers  = 2
	MaxMembers  = 8
	NameMaxLen  = 40
	defaultName = "Nhóm của "
	fallback    = "Nhóm mới"
)

var types = map[string]bool{TypeFriend: true, TypeStudyGroup: true, TypeQuest: true, TypeCompetitionTeam: true, TypeCustom: true}
var visibilities = map[string]bool{VisibilityPrivate: true, VisibilityFriends: true, VisibilityClass: true}
var invitePolicies = map[string]bool{InviteLeaderOnly: true, InviteMembers: true}

// ValidType reports a known party type.
func ValidType(t string) bool { return types[t] }

// DefaultName is the generated name "Nhóm của <nickname>" (≤ 40 characters); without a nickname "Nhóm mới".
// The server stores the Vietnamese string (research R2).
func DefaultName(nickname string) string {
	nickname = strings.TrimSpace(nickname)
	if nickname == "" {
		return fallback
	}
	name := defaultName + nickname
	if utf8.RuneCountInString(name) > NameMaxLen {
		name = string([]rune(name)[:NameMaxLen])
	}
	return name
}

// CleanName trims a name typed by a student and checks 1–40 characters.
func CleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	n := utf8.RuneCountInString(name)
	if n == 0 {
		return "", ValidationError{Field: "name", Reason: "REQUIRED", Message: "must not be empty"}
	}
	if n > NameMaxLen {
		return "", ValidationError{Field: "name", Reason: "MAX_LENGTH", Message: "at most 40 characters"}
	}
	return name, nil
}

// Member is an active membership.
type Member struct {
	StudentID    uuid.UUID
	SchoolID     uuid.UUID
	Role         string
	JoinedAt     time.Time
	InvitationID *uuid.UUID
}

// Party is the aggregate: settings and the active members ordered by joined_at.
type Party struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	SchoolID         uuid.UUID
	Name             string
	LeaderID         uuid.UUID
	Type             string
	Visibility       string
	InvitationPolicy string
	MaxMembers       int
	GradeBand        string
	Status           string
	StateVersion     int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DisbandedAt      *time.Time
	Members          []Member
}

// Settings are the fields of createParty / updatePartySettings (nil: unchanged / default).
type Settings struct {
	Name             *string
	Visibility       *string
	InvitationPolicy *string
	MaxMembers       *int
}

// New builds the party of a leader (createParty, F100-AC-01): status open, the leader as only member.
func New(id, tenant, school, leader uuid.UUID, typ, band, name string, s Settings, defaultMax int, now time.Time) (*Party, error) {
	if !types[typ] {
		return nil, ValidationError{Field: "party_type", Reason: "INVALID", Message: "unknown party type"}
	}
	p := &Party{ID: id, TenantID: tenant, SchoolID: school, Name: name, LeaderID: leader, Type: typ, Visibility: VisibilityPrivate,
		InvitationPolicy: InviteLeaderOnly, MaxMembers: defaultMax, GradeBand: band, Status: StatusOpen, CreatedAt: now, UpdatedAt: now,
		Members: []Member{{StudentID: leader, SchoolID: school, Role: RoleLeader, JoinedAt: now}}}
	if s.Visibility != nil {
		p.Visibility = *s.Visibility
	}
	if s.InvitationPolicy != nil {
		p.InvitationPolicy = *s.InvitationPolicy
	}
	if s.MaxMembers != nil {
		p.MaxMembers = *s.MaxMembers
	}
	if err := p.validSettings(MaxMembers); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Party) validSettings(capacity int) error {
	switch {
	case !visibilities[p.Visibility]:
		return ValidationError{Field: "visibility", Reason: "INVALID", Message: "unknown visibility"}
	case !invitePolicies[p.InvitationPolicy]:
		return ValidationError{Field: "invitation_policy", Reason: "INVALID", Message: "unknown invitation policy"}
	case p.MaxMembers < MinMembers || p.MaxMembers > MaxMembers:
		return ValidationError{Field: "max_members", Reason: "INVALID", Message: "must be between 2 and 8"}
	case p.MaxMembers > capacity:
		return ValidationError{Field: "max_members", Reason: "INVALID", Message: "above the limit of the school policy"}
	}
	return nil
}

// Active reports a party that is not disbanded.
func (p *Party) Active() bool { return p.Status != StatusDisbanded }

// Member returns the active membership of student.
func (p *Party) Member(student uuid.UUID) (Member, bool) {
	for _, m := range p.Members {
		if m.StudentID == student {
			return m, true
		}
	}
	return Member{}, false
}

// MemberIDs returns the active members in joined_at order.
func (p *Party) MemberIDs() []uuid.UUID {
	out := make([]uuid.UUID, len(p.Members))
	for i, m := range p.Members {
		out[i] = m.StudentID
	}
	return out
}

// Full reports max_members reached.
func (p *Party) Full() bool { return len(p.Members) >= p.MaxMembers }

func (p *Party) bump(now time.Time) {
	p.StateVersion++
	p.UpdatedAt = now
}

// RequireMember refuses anyone who is not an active member of an active party (404 for the caller).
func (p *Party) RequireMember(student uuid.UUID) error {
	if !p.Active() {
		return ErrDisbanded
	}
	if _, ok := p.Member(student); !ok {
		return ErrNotMember
	}
	return nil
}

// RequireLeader refuses a member that is not the leader (PARTY_NOT_LEADER) and a non-member (404).
func (p *Party) RequireLeader(student uuid.UUID) error {
	if err := p.RequireMember(student); err != nil {
		return err
	}
	if p.LeaderID != student {
		return ErrNotLeader
	}
	return nil
}

// CanInvite checks the invitation_policy for inviter (F100-AC-08): the leader, or any member when the policy is
// members.
func (p *Party) CanInvite(inviter uuid.UUID) error {
	if err := p.RequireMember(inviter); err != nil {
		return err
	}
	if p.LeaderID != inviter && p.InvitationPolicy != InviteMembers {
		return ErrInviteNotAllowed
	}
	return nil
}

// Add adds an active member (accept, join — F100-AC-11, AC-16).
func (p *Party) Add(m Member, now time.Time) error {
	if !p.Active() {
		return ErrDisbanded
	}
	if _, ok := p.Member(m.StudentID); ok {
		return ErrAlreadyMember
	}
	if p.Full() {
		return ErrFull
	}
	m.Role = RoleMember
	p.Members = append(p.Members, m)
	p.bump(now)
	return nil
}

// NextLeader is the leader after the leader left: the online member who joined first, else the member who joined first
// (F100-AC-17, PARTY_SYSTEM §9). except is the leaving leader.
func (p *Party) NextLeader(except uuid.UUID, online map[uuid.UUID]bool) (uuid.UUID, bool) {
	ms := append([]Member(nil), p.Members...)
	sort.SliceStable(ms, func(i, j int) bool { return ms[i].JoinedAt.Before(ms[j].JoinedAt) })
	var first uuid.UUID
	for _, m := range ms {
		if m.StudentID == except {
			continue
		}
		if online[m.StudentID] {
			return m.StudentID, true
		}
		if first == uuid.Nil {
			first = m.StudentID
		}
	}
	return first, first != uuid.Nil
}

// Removal is the outcome of a member leaving: the new leader when the leader left, the disband when nobody is left.
type Removal struct {
	Member    Member
	NewLeader uuid.UUID // uuid.Nil: unchanged
	Disbanded bool
}

// Remove ends the membership of student (leave, kick, policy — F100-AC-17, AC-18, AC-25).
func (p *Party) Remove(student uuid.UUID, online map[uuid.UUID]bool, now time.Time) (Removal, error) {
	m, ok := p.Member(student)
	if !ok || !p.Active() {
		return Removal{}, ErrNotMember
	}
	r := Removal{Member: m}
	if p.LeaderID == student {
		if next, ok := p.NextLeader(student, online); ok {
			r.NewLeader = next
		}
	}
	kept := p.Members[:0]
	for _, x := range p.Members {
		if x.StudentID != student {
			kept = append(kept, x)
		}
	}
	p.Members = kept
	p.bump(now)
	if r.NewLeader != uuid.Nil {
		p.setLeader(r.NewLeader)
	}
	if len(p.Members) == 0 {
		p.disband(now)
		r.Disbanded = true
	}
	return r, nil
}

func (p *Party) setLeader(to uuid.UUID) {
	p.LeaderID = to
	for i := range p.Members {
		if p.Members[i].StudentID == to {
			p.Members[i].Role = RoleLeader
		} else {
			p.Members[i].Role = RoleMember
		}
	}
}

// Transfer gives the leader role to another member (F100-AC-19); the caller checked leadership and presence.
func (p *Party) Transfer(to uuid.UUID, now time.Time) error {
	if to == p.LeaderID {
		return ErrSelf
	}
	if _, ok := p.Member(to); !ok {
		return ErrTargetNotMember
	}
	p.setLeader(to)
	p.bump(now)
	return nil
}

func (p *Party) disband(now time.Time) {
	p.Status = StatusDisbanded
	at := now
	p.DisbandedAt = &at
	p.UpdatedAt = now
}

// Disband ends the party (F100-AC-20): every remaining member leaves (disbanded). Returns those members; false when
// the party was already disbanded (idempotent, no event).
func (p *Party) Disband(now time.Time) ([]Member, bool) {
	if !p.Active() {
		return nil, false
	}
	left := p.Members
	p.Members = nil
	p.bump(now)
	p.disband(now)
	return left, true
}

// Update applies updatePartySettings (F100-AC-21): max_members ≥ current members and ≤ capacity of the policy; returns
// the changed fields (empty: nothing changed, no event).
func (p *Party) Update(s Settings, capacity int, now time.Time) ([]string, error) {
	next := *p
	var changed []string
	if s.Name != nil && *s.Name != p.Name {
		next.Name = *s.Name
		changed = append(changed, "name")
	}
	if s.Visibility != nil && *s.Visibility != p.Visibility {
		next.Visibility = *s.Visibility
		changed = append(changed, "visibility")
	}
	if s.InvitationPolicy != nil && *s.InvitationPolicy != p.InvitationPolicy {
		next.InvitationPolicy = *s.InvitationPolicy
		changed = append(changed, "invitation_policy")
	}
	if s.MaxMembers != nil && *s.MaxMembers != p.MaxMembers {
		next.MaxMembers = *s.MaxMembers
		changed = append(changed, "max_members")
		if next.MaxMembers < len(p.Members) {
			return nil, ValidationError{Field: "max_members", Reason: "INVALID", Message: "below the current number of members"}
		}
		if err := next.validSettings(capacity); err != nil {
			return nil, err
		}
	} else if err := next.validSettings(MaxMembers); err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return nil, nil
	}
	*p = next
	p.bump(now)
	return changed, nil
}

// Reassign moves the active membership of from to into (user merge, F100 T042): same role and joined_at, the leader
// follows. Returns the new membership.
func (p *Party) Reassign(from, into uuid.UUID, now time.Time) (Member, error) {
	if !p.Active() {
		return Member{}, ErrDisbanded
	}
	if _, ok := p.Member(into); ok {
		return Member{}, ErrAlreadyMember
	}
	for i, m := range p.Members {
		if m.StudentID == from {
			m.StudentID = into
			p.Members[i] = m
			if p.LeaderID == from {
				p.LeaderID = into
			}
			p.bump(now)
			return m, nil
		}
	}
	return Member{}, ErrNotMember
}
