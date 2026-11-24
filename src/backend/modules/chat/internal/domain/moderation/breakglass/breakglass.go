// Package breakglass holds the rules of the break-glass access of the System Admin to chat outside a moderation case
// (MODERATION §6.1, CHILD_SAFETY §7, F124 US4): request limits, two-person decision, a time-boxed grant readable only
// by its requester within its scope, revocation and expiry. Standard library and uuid only.
package breakglass

import (
	"errors"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Statuses of a grant.
const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusRejected = "rejected"
	StatusExpired  = "expired"
	StatusRevoked  = "revoked"
)

// Scopes of a grant: the messages of one channel, or the messages sent by one user, of the target tenant.
const (
	ScopeChannel = "channel"
	ScopeUser    = "user"
)

// Statuses lists the statuses (filter of listBreakGlassGrants).
var Statuses = []string{StatusPending, StatusApproved, StatusRejected, StatusExpired, StatusRevoked}

// Bounds of the texts (ADR-0041; reason ≥ 20 characters, MODERATION §6.1).
const (
	MinReason = 20
	MaxText   = 1000
)

var (
	// ErrSelfDecision: the requester decides its own request (two-person rule → 403 PERMISSION_DENIED).
	ErrSelfDecision = errors.New("breakglass: the requester cannot decide its own request")
	// ErrNotPending: decision of a request that is no longer pending, or revocation of an ended grant (409).
	ErrNotPending = errors.New("breakglass: the request is not pending")
)

// FieldError is a request outside the limits (422 VALIDATION_FAILED on Field).
type FieldError struct{ Field, Reason string }

func (e FieldError) Error() string { return "breakglass: " + e.Field + ": " + e.Reason }

// Limits are break_glass_max_duration_hours and break_glass_max_window_days.
type Limits struct {
	MaxDuration time.Duration
	MaxWindow   time.Duration
}

// Request is createBreakGlassRequest.
type Request struct {
	ScopeType       string
	ScopeID         uuid.UUID
	WindowFrom      time.Time
	WindowTo        time.Time
	DurationMinutes int
	Reason          string
}

// Validate checks the request against the limits (F124-AC-21).
func (r Request) Validate(l Limits) error {
	n := utf8.RuneCountInString(r.Reason)
	switch {
	case !slices.Contains([]string{ScopeChannel, ScopeUser}, r.ScopeType):
		return FieldError{"scope_type", "channel or user"}
	case !r.WindowTo.After(r.WindowFrom):
		return FieldError{"window_to", "must be after window_from"}
	case l.MaxWindow > 0 && r.WindowTo.Sub(r.WindowFrom) > l.MaxWindow:
		return FieldError{"window", "longer than break_glass_max_window_days"}
	case r.DurationMinutes < 1 || (l.MaxDuration > 0 && time.Duration(r.DurationMinutes)*time.Minute > l.MaxDuration):
		return FieldError{"duration_minutes", "1 to break_glass_max_duration_hours × 60"}
	case n < MinReason || n > MaxText:
		return FieldError{"reason", "20 to 1000 characters"}
	}
	return nil
}

// Grant is the state of a break-glass request/grant.
type Grant struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	ScopeType       string
	ScopeID         uuid.UUID
	WindowFrom      time.Time
	WindowTo        time.Time
	DurationMinutes int
	Status          string
	RequestedBy     uuid.UUID
	DecidedBy       *uuid.UUID
	DecidedAt       *time.Time
	ExpiresAt       *time.Time
	RevokedBy       *uuid.UUID
	RevokedAt       *time.Time
}

// Decide approves (expires_at = now + duration) or rejects a pending request by a decider other than the requester.
func (g Grant) Decide(by uuid.UUID, approve bool, now time.Time) (Grant, error) {
	if by == g.RequestedBy {
		return g, ErrSelfDecision
	}
	if g.Status != StatusPending {
		return g, ErrNotPending
	}
	t, d := now, by
	g.DecidedBy, g.DecidedAt = &d, &t
	if !approve {
		g.Status = StatusRejected
		return g, nil
	}
	exp := now.Add(time.Duration(g.DurationMinutes) * time.Minute)
	g.Status, g.ExpiresAt = StatusApproved, &exp
	return g, nil
}

// Revoke ends a pending or approved grant at once.
func (g Grant) Revoke(by uuid.UUID, now time.Time) (Grant, error) {
	if g.Status != StatusPending && g.Status != StatusApproved {
		return g, ErrNotPending
	}
	t, b := now, by
	g.Status, g.RevokedBy, g.RevokedAt = StatusRevoked, &b, &t
	return g, nil
}

// ExpiredAt reports an approved grant past expires_at (the expirer marks it expired).
func (g Grant) ExpiredAt(now time.Time) bool {
	return g.Status == StatusApproved && g.ExpiresAt != nil && !now.Before(*g.ExpiresAt)
}

// ReadableBy reports whether user may read the messages of the grant now: its requester, approved, not expired
// (F124-AC-19; anything else → 403 BREAK_GLASS_NOT_ACTIVE).
func (g Grant) ReadableBy(user uuid.UUID, now time.Time) bool {
	return g.Status == StatusApproved && user == g.RequestedBy && g.ExpiresAt != nil && now.Before(*g.ExpiresAt)
}
