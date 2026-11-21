// Package job holds the scheduled jobs of module competition (worker profile jobs).
package job

import (
	"context"
	"time"
)

// CloseIdleHubInstances is the job close-idle-hub-instances (F091 T018, AC-10): the room host closes its own empty
// instances after hub_instance_idle_close; this job closes the summary rows of instances whose room host vanished
// together with their Valkey state (ADR-0004).
const CloseIdleHubInstances = "close-idle-hub-instances"

// CloseIdleInterval is the period of the job.
const CloseIdleInterval = time.Minute

// OrphanCloser closes orphan hub rooms (command.Hub.CloseOrphanRooms).
type OrphanCloser interface {
	CloseOrphanRooms(ctx context.Context, batch int) (int, error)
}

// CloseIdleHubInstancesJob returns one run of the job.
func CloseIdleHubInstancesJob(h OrphanCloser) func(ctx context.Context) (int, error) {
	return func(ctx context.Context) (int, error) { return h.CloseOrphanRooms(ctx, 200) }
}

// PartyInvitationExpiry is the job party-invitation-expiry (F100-AC-14, plan: every 15 s): pending party invitations
// past expires_at become expired (reason ttl) with party.invitation_expired and party.invitation.updated.
const PartyInvitationExpiry = "party-invitation-expiry"

// InvitationExpirer expires the pending invitations past their expiry (command.Parties.ExpireInvitations bound to its
// platform repository).
type InvitationExpirer func(ctx context.Context, batch int) (int, error)

// PartyInvitationExpiryJob returns one run of the job.
func PartyInvitationExpiryJob(h InvitationExpirer) func(ctx context.Context) (int, error) {
	return func(ctx context.Context) (int, error) { return h(ctx, 500) }
}
