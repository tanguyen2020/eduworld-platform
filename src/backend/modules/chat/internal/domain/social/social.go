// Package social is the domain of the friends, blocks and mutes of module chat (F095, SOCIAL_SYSTEM §4–§5): pure rules,
// no I/O. The application layer loads the facts (projections, relations, counters) and asks these rules.
package social

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Request statuses (social.friend_requests.status).
const (
	StatusPending   = "pending"
	StatusAccepted  = "accepted"
	StatusRejected  = "rejected"
	StatusCancelled = "cancelled"
	StatusExpired   = "expired"
)

// Request sources.
const (
	SourceDirect     = "direct"
	SourceFriendCode = "friend_code"
)

// Reasons of friend.removed and of the soft-delete of a friendship.
const (
	RemovedByUser  = "removed"
	RemovedByBlock = "blocked"
)

// Tenant types (ref.tenants.tenant_type).
const (
	TenantSchool = "school"
	TenantUnit   = "unit"
	TenantCommon = "common"
)

// Pair is a friendship pair in its normalized order (user_low_id < user_high_id, by the byte order PostgreSQL uses
// for uuid).
type Pair struct{ Low, High uuid.UUID }

// NewPair normalizes a and b.
func NewPair(a, b uuid.UUID) Pair {
	if strings.Compare(a.String(), b.String()) > 0 {
		a, b = b, a
	}
	return Pair{Low: a, High: b}
}

// Other returns the member of p that is not me.
func (p Pair) Other(me uuid.UUID) uuid.UUID {
	if p.Low == me {
		return p.High
	}
	return p.Low
}

// FriendRequest is the aggregate of social.friend_requests.
type FriendRequest struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	RequesterID uuid.UUID
	AddresseeID uuid.UUID
	Status      string
	Source      string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	RespondedAt *time.Time
}

// ErrNotPending: the request is not a live pending request any more (accepted, rejected, cancelled, expired).
var ErrNotPending = errors.New("social: friend request is not pending")

// Live reports a pending request that has not expired at now.
func (r *FriendRequest) Live(now time.Time) bool {
	return r.Status == StatusPending && now.Before(r.ExpiresAt)
}

func (r *FriendRequest) respond(status string, now time.Time) error {
	if !r.Live(now) {
		return ErrNotPending
	}
	r.Status = status
	t := now
	r.RespondedAt = &t
	return nil
}

// Accept marks a live request accepted.
func (r *FriendRequest) Accept(now time.Time) error { return r.respond(StatusAccepted, now) }

// Reject marks a live request rejected (silent: no event, the requester keeps seeing pending until expiry).
func (r *FriendRequest) Reject(now time.Time) error { return r.respond(StatusRejected, now) }

// OutgoingStatus is the status the requester sees (SOCIAL_SYSTEM §4.1, F095-AC-06): a rejected request stays
// pending until it expires, so the requester never learns about the rejection.
func (r *FriendRequest) OutgoingStatus(now time.Time) string {
	switch {
	case (r.Status == StatusPending || r.Status == StatusRejected) && now.Before(r.ExpiresAt):
		return StatusPending
	case r.Status == StatusPending || r.Status == StatusRejected:
		return StatusExpired
	}
	return r.Status
}

// Student is the projected state of a student (ref.students) seen by the scope rules.
type Student struct {
	ID       uuid.UUID
	TenantID uuid.UUID
	SchoolID uuid.UUID
	Status   string
	Deleted  bool
}

// Active reports a student who may take part in social interactions.
func (s Student) Active() bool { return s.Status == "active" && !s.Deleted }

// Tenant is the projected tenant (ref.tenants) seen by the scope rules.
type Tenant struct {
	ID   uuid.UUID
	Type string
	// CrossSchool is settings_subset.cross_school_social_enabled (ADR-0054, only meaningful for a unit tenant).
	CrossSchool bool
	// CommonFriendsEnabled is chat.common_tenant_social_overrides.friends_enabled of a common tenant, true while System
	// Admin stored no override (common_tenant_social_defaults: friends on, owner 2026-11-28, CHILD_SAFETY §3.6).
	CommonFriendsEnabled bool
}

// FriendsEnabled reports whether friend requests and friend codes are allowed in the tenant (F095-AC-23): always outside a
// common tenant; in a common tenant unless the System Admin incident switch turned friends off.
func (t Tenant) FriendsEnabled() bool { return t.Type != TenantCommon || t.CommonFriendsEnabled }

// Scope violations; the API answers all of them with 403 SOCIAL_SCOPE_VIOLATION without telling which one (a block
// never reveals its direction, F095-AC-02).
var (
	ErrFriendsDisabled = errors.New("social: friends are disabled in this tenant")
	ErrOtherSchool     = errors.New("social: other school without cross-school social")
	ErrBlocked         = errors.New("social: blocked")
)

// ScopePolicy decides whether two students of one tenant may become friends (SOCIAL_SYSTEM §4.1, §7, ADR-0054):
// same school, or another school of a unit tenant with cross_school_social_enabled; never when either blocked the
// other; never in a common tenant whose friends System Admin switched off. Different tenants never reach it (404 before).
type ScopePolicy struct{}

// Check returns nil or one of the scope violations.
func (ScopePolicy) Check(t Tenant, from, to Student, blocked bool) error {
	switch {
	case !t.FriendsEnabled():
		return ErrFriendsDisabled
	case blocked:
		return ErrBlocked
	case from.SchoolID != to.SchoolID && (t.Type != TenantUnit || !t.CrossSchool):
		return ErrOtherSchool
	}
	return nil
}

// Limits is friend_limits (CONFIG_DEFAULTS, SOCIAL_SYSTEM §4.1): pending requests, requests per day, friends.
type Limits struct {
	MaxPending  int
	MaxPerDay   int
	MaxFriends  int
	DayLocation *time.Location // calendar day of the per-day limit (platform timezone)
}

// Counters are the facts the limits are checked against.
type Counters struct {
	Pending int // live pending requests sent by the student
	Today   int // requests sent by the student since the start of the day
	Friends int // live friendships of the student
}

// Limit violations: FRIEND_LIMIT_REACHED (409) for pending/friends, RATE_LIMITED (429) for the daily quota.
var (
	ErrLimitReached = errors.New("social: friend limit reached")
	ErrDailyLimit   = errors.New("social: daily friend request limit reached")
)

// CheckSend checks a new request against the limits (F095-AC-03).
func (l Limits) CheckSend(c Counters) error {
	switch {
	case c.Pending >= l.MaxPending, c.Friends >= l.MaxFriends:
		return ErrLimitReached
	case c.Today >= l.MaxPerDay:
		return ErrDailyLimit
	}
	return nil
}

// CheckFriends checks that a student may get one more friend.
func (l Limits) CheckFriends(friends int) error {
	if friends >= l.MaxFriends {
		return ErrLimitReached
	}
	return nil
}

func (l Limits) loc() *time.Location {
	if l.DayLocation == nil {
		return time.UTC
	}
	return l.DayLocation
}

// DayStart is the start of the calendar day of now (per-day limit window).
func (l Limits) DayStart(now time.Time) time.Time {
	t := now.In(l.loc())
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, l.loc())
}

// RetryAfter is the delay until the per-day window resets.
func (l Limits) RetryAfter(now time.Time) time.Duration {
	return l.DayStart(now).AddDate(0, 0, 1).Sub(now)
}

// codeRe is the friend code format of the contract (FriendCode.code).
var codeRe = regexp.MustCompile(`^[A-Z0-9]{6,12}$`)

// ValidCode reports a well-formed friend code (F095-AC-08: otherwise 422).
func ValidCode(code string) bool { return codeRe.MatchString(code) }

// CodeAlphabet and CodeLength are the friend codes generated by the server: 8 characters A–Z0–9 [Confirmed].
const (
	CodeAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	CodeLength   = 8
)

// Presence is the presence of a student as module chat last received it (gateway signal ew.rt.presence.*).
type Presence struct {
	Status   string
	Activity string
	AreaID   string
	Hidden   bool
	Ts       int64
}

// Presence statuses (MULTIPLAYER_GAMEPLAY §5).
const (
	PresenceOffline = "offline"
	PresenceOnline  = "online"
)

// Visible is the presence the friends of the student see (F095-AC-21): hidden → offline; during an exam the activity
// is the generic "learning" and no area is shown [Proposed F095].
func (p Presence) Visible() Presence {
	if p.Hidden || p.Status == "" || p.Status == PresenceOffline {
		return Presence{Status: PresenceOffline, Ts: p.Ts}
	}
	v := Presence{Status: p.Status, Activity: p.Activity, AreaID: p.AreaID, Ts: p.Ts}
	if v.Activity == "exam" {
		v.Activity, v.AreaID = "learning", ""
	}
	return v
}

// SameView reports that friends would see no change between p and q.
func (p Presence) SameView(q Presence) bool {
	return p.Status == q.Status && p.Activity == q.Activity && p.AreaID == q.AreaID
}
