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

// PartyPresenceSweeper is the job party-presence-sweeper (F101 research R3, PARTY_SYSTEM §9): member statuses, leader
// grace and automatic transfer, dormant parties, session memberships past their grace; one replica per cycle under the
// lock ew:lock:job:party-presence-sweeper, independent of the gateway node holding the connections.
const PartyPresenceSweeper = "party-presence-sweeper"

// SweepInterval is the cycle of the sweeper (PARTY_SYSTEM §9: 5 s, code constant — spec F101 Assumptions).
const SweepInterval = 5 * time.Second

// PresenceSweeper runs one cycle (command.Sweeper.Run).
type PresenceSweeper interface {
	Run(ctx context.Context) (int, error)
}

// PartyPresenceSweeperJob returns one run of the job.
func PartyPresenceSweeperJob(s PresenceSweeper) func(ctx context.Context) (int, error) {
	return s.Run
}

// Matchmaker is the job matchmaker (F104 T022, MATCHMAKING §6): one pass over every queue each EW_MM_TICK, one
// matchmaker per queue across replicas (per-queue lock), then the ready checks past their deadline.
const Matchmaker = "matchmaker"

// MatchmakingTicketExpiry is the job matchmaking-ticket-expiry (F104 T026): mm_ticket_ttl, mm_bot_after, disconnected
// students and tickets lost with Valkey.
const MatchmakingTicketExpiry = "matchmaking-ticket-expiry"

// DuelExpiry is the job duel-expiry (F104 T035): pending friend duels past 60 s.
const DuelExpiry = "duel-expiry"

// DuelExpiryInterval is the period of duel-expiry (code constant: a 60 s duel expires within 5 s).
const DuelExpiryInterval = 5 * time.Second

// MatchmakingRunner runs one pass of a matchmaking job (command.Matchmaking.Tick / ExpireTickets).
type MatchmakingRunner func(ctx context.Context) (int, error)

// MatchmakingJob returns one run of a matchmaking job.
func MatchmakingJob(run MatchmakingRunner) func(ctx context.Context) (int, error) { return run }
