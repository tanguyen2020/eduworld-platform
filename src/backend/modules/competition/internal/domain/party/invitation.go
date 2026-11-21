package party

import (
	"time"

	"github.com/google/uuid"
)

// Invitation statuses (PARTY_SYSTEM §13.2).
const (
	InvitationPending   = "pending"
	InvitationAccepted  = "accepted"
	InvitationRejected  = "rejected"
	InvitationExpired   = "expired"
	InvitationCancelled = "cancelled"
)

// Reasons an invitation ends without an answer (party_invitations.end_reason, event payload reason).
const (
	EndTTL              = "ttl"
	EndPartyDisbanded   = "party_disbanded"
	EndJoinedOtherParty = "joined_other_party"
	EndPendingOverflow  = "pending_overflow"
	EndCancelled        = "cancelled"
	EndInviterKicked    = "inviter_kicked"
	EndInviterLeft      = "inviter_left"
	EndBlocked          = "blocked"
	EndPolicy           = "policy"
)

// Invitation is a party invitation with a server-side expiry (party_invite_ttl).
type Invitation struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	SchoolID    uuid.UUID
	PartyID     uuid.UUID
	InviterID   uuid.UUID
	InviteeID   uuid.UUID
	Status      string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	RespondedAt *time.Time
	EndReason   string
}

// Live reports a pending invitation not yet expired at now.
func (i *Invitation) Live(now time.Time) bool {
	return i.Status == InvitationPending && now.Before(i.ExpiresAt)
}

func (i *Invitation) end(status, reason string, now time.Time) {
	i.Status = status
	i.EndReason = reason
	at := now
	i.RespondedAt = &at
}

// Respond checks that caller may accept or reject (F100-AC-12): the invitee only, a pending invitation, not expired.
func (i *Invitation) Respond(caller uuid.UUID, now time.Time) error {
	if i.InviteeID != caller {
		return ErrNotInvitee
	}
	if i.Status != InvitationPending {
		return ErrNotPending
	}
	if !now.Before(i.ExpiresAt) {
		return ErrExpired
	}
	return nil
}

// Accept marks the invitation accepted.
func (i *Invitation) Accept(now time.Time) { i.end(InvitationAccepted, "", now) }

// Reject marks the invitation rejected (F100-AC-13).
func (i *Invitation) Reject(now time.Time) { i.end(InvitationRejected, "", now) }

// Expire marks the invitation expired with reason (F100-AC-14).
func (i *Invitation) Expire(reason string, now time.Time) { i.end(InvitationExpired, reason, now) }

// Cancel marks the invitation cancelled with reason.
func (i *Invitation) Cancel(reason string, now time.Time) { i.end(InvitationCancelled, reason, now) }

// CanCancel reports whether caller may cancel (F100-AC-13): the inviter or the current leader of the party.
func (i *Invitation) CanCancel(caller, leader uuid.UUID) error {
	if caller != i.InviterID && caller != leader {
		return ErrCannotCancel
	}
	if i.Status != InvitationPending {
		return ErrNotPending
	}
	return nil
}
