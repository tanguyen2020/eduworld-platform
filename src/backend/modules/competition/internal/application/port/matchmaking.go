package port

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	mm "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/matchmaking"
)

// Ticket is a matchmaking ticket (F104): Valkey only, never in PostgreSQL (ADR-0004, FR-MM-006). Everything is filled
// by the server from the projections; the request only chooses mode, type, subject and topic (F104-AC-01).
type Ticket struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	OwnerID      uuid.UUID // the student (solo) or the party leader who created it
	Queue        string    // mm.QueueKey.String()
	Mode         string
	Type         string
	ScopeKind    string
	ScopeID      uuid.UUID
	GradeKey     string
	SubjectID    uuid.UUID
	TopicID      *uuid.UUID
	PartyID      *uuid.UUID
	AllowFill    bool
	TeamSize     int
	Students     []uuid.UUID
	Schools      []uuid.UUID // school of each student (same order)
	Classes      []uuid.UUID // known classes of the students
	Grade        int         // common grade of the students (H1)
	Rating       mm.Rating
	EnqueuedAt   time.Time
	Status       string
	CancelReason string
	ProposalID   *uuid.UUID
	MatchID      *uuid.UUID
	Requeued     bool
	BotOffered   bool
	OfflineSince *time.Time
}

// Has reports a student of the ticket.
func (t Ticket) Has(student uuid.UUID) bool {
	for _, s := range t.Students {
		if s == student {
			return true
		}
	}
	return false
}

// Candidate is the pairing view of a queued ticket.
func (t Ticket) Candidate() mm.Candidate {
	return mm.Candidate{TicketID: t.ID, Students: t.Students, Classes: t.Classes, Grade: t.Grade, Rating: t.Rating, EnqueuedAt: t.EnqueuedAt,
		PartyID: t.PartyID, AllowFill: t.AllowFill}
}

// Proposal is a match proposal with what the match needs once every side accepted (Valkey, TTL of the ready check).
type Proposal struct {
	mm.Proposal
	TenantIDs        []uuid.UUID
	Mode             string
	Type             string
	ScopeKind        string
	SubjectID        uuid.UUID
	TopicID          *uuid.UUID
	Grade            int // lowest grade of the match
	Ranked           bool
	RulesetVersionID uuid.UUID
	TournamentNodeID *uuid.UUID
	Direct           bool                    // CreateDirectProposal (no queue, no ticket to requeue)
	Schools          map[uuid.UUID]uuid.UUID // student → school
	Tenants          map[uuid.UUID]uuid.UUID // student → tenant
	Parties          map[uuid.UUID]uuid.UUID // student → party (party tickets)
	Ratings          map[uuid.UUID]mm.Rating // student → rating at proposal time (internal)
	CreatedAt        time.Time
	AcceptedAt       *time.Time
}

// Errors of the matchmaking store.
var (
	// ErrTicketActive: the student (or a member, or the party) already holds an active ticket.
	ErrTicketActive = errors.New("matchmaking: ticket already active")
)

// MatchmakingStore is the ephemeral state of matchmaking in Valkey (VALKEY_DESIGN §3.1, ew:competition:mm:*): tickets,
// queues, proposals, per-queue locks, ticket rate and decline cooldown. Every transition is one atomic script.
type MatchmakingStore interface {
	// Create stores a queued ticket and its queue entry; every student key (and the party key) is taken NX. When one
	// is held, returns ErrTicketActive with the ticket holding it (uuid.Nil if unknown).
	Create(ctx context.Context, t Ticket) (uuid.UUID, error)
	// Ticket returns a ticket, nil when unknown (never existed, ended long ago or lost with Valkey).
	Ticket(ctx context.Context, id uuid.UUID) (*Ticket, error)
	// TicketOfParty returns the active ticket of a party, uuid.Nil when none.
	TicketOfParty(ctx context.Context, party uuid.UUID) (uuid.UUID, error)
	// End moves a ticket in one of the from statuses to status (cancelled/expired) with reason, removes it from its
	// queue and frees its students and party; false when the ticket was not in from.
	End(ctx context.Context, id uuid.UUID, status, reason string, from []string) (bool, error)
	// Flag sets the bot offer or the offline-since mark of a queued ticket.
	Flag(ctx context.Context, id uuid.UUID, botOffered *bool, offlineSince *time.Time, clearOffline bool) error
	// Queues returns the queue keys that may hold tickets.
	Queues(ctx context.Context) ([]string, error)
	// Queued returns up to limit queued tickets of a queue, oldest first; an empty queue is dropped from the set.
	Queued(ctx context.Context, queue string, limit int) ([]Ticket, error)
	// Lock takes the per-queue matchmaker lock for ttl; Unlock releases it when still owned.
	Lock(ctx context.Context, queue, owner string, ttl time.Duration) (bool, error)
	Unlock(ctx context.Context, queue, owner string) error
	// Propose creates the proposal and moves its tickets from the queue to proposed in one script; false when a
	// ticket is no longer queued (nothing changed).
	Propose(ctx context.Context, p Proposal) (bool, error)
	// Proposal returns a proposal, nil when unknown.
	Proposal(ctx context.Context, id uuid.UUID) (*Proposal, error)
	// Accept records the ready of a student of an open proposal before its deadline; the proposal becomes accepted
	// with the last one. mm.ErrProposalClosed when not open or past its deadline.
	Accept(ctx context.Context, id, student uuid.UUID, now time.Time) (*Proposal, bool, error)
	// Cancel cancels an open (or accepted, failed) proposal: requeues res.Requeue with their enqueued_at
	// (requeued = true) and ends res.Cancel with res.Reason; false when the proposal is not open anymore.
	Cancel(ctx context.Context, id uuid.UUID, res mm.Resolution, from []string) (bool, error)
	// Complete marks the proposal and its tickets matched with match and frees the students and parties.
	Complete(ctx context.Context, id, match uuid.UUID) error
	// Due returns the proposals past their deadline (open) or accepted for longer than stale.
	Due(ctx context.Context, now time.Time, stale time.Duration) ([]uuid.UUID, error)
	// TicketRate counts a new ticket of student in the sliding hour (mm_ticket_rate); returns the wait when the limit is
	// reached (nothing counted then).
	TicketRate(ctx context.Context, student uuid.UUID, now time.Time, perHour int) (time.Duration, error)
	// Cooldown returns the remaining decline cooldown of student (0: none).
	Cooldown(ctx context.Context, student uuid.UUID) (time.Duration, error)
	// Blame counts a decline/timeout of student; at limit in window, sets the cooldown.
	Blame(ctx context.Context, student uuid.UUID, now time.Time, limit int, window, cooldown time.Duration) error
	// Statuses returns the status of each ticket still in Valkey; a missing id was lost (F104-AC-11).
	Statuses(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
}

// RatingReader returns the Glicko-2 ratings of students for a mode and subject (F106 implements it; until then the
// default rating glicko2_params of the configuration).
type RatingReader interface {
	Ratings(ctx context.Context, tenant uuid.UUID, students []uuid.UUID, mode string, subject uuid.UUID, def mm.Rating) (map[uuid.UUID]mm.Rating, error)
}

// MatchmakingMetrics are the eduworld_mm_* metrics (MATCHMAKING §10): no school or student label.
type MatchmakingMetrics interface {
	Ticket(mode, result string)
	QueueSize(mode, gradeKey, subject string, n int)
	Wait(mode, result string, d time.Duration)
	Proposal(result string)
	RatingGap(gap float64)
	MatchCreated(mode string)
	Loop(d time.Duration)
}
