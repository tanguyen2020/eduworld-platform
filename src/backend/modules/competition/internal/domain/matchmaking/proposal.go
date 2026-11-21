package matchmaking

import (
	"time"

	"github.com/google/uuid"
)

// Proposal states (Valkey, ephemeral).
const (
	ProposalOpen      = "open"
	ProposalAccepted  = "accepted" // every side accepted; the match is being created
	ProposalMatched   = "matched"
	ProposalCancelled = "cancelled"
)

// Proposal cancel reasons (AsyncAPI matchmaking.proposal.cancelled.reason).
const (
	ProposalDeclined    = "declined"
	ProposalTimeout     = "timeout"
	ProposalUnavailable = "participant_unavailable"
)

// ProposalTicket is one ticket of a proposal with its students and team.
type ProposalTicket struct {
	TicketID uuid.UUID
	Students []uuid.UUID
}

// Proposal is a match proposal and its ready check (MATCHMAKING §7). Its id becomes the match id.
type Proposal struct {
	ID       uuid.UUID
	Queue    string // queue key; empty for a direct proposal
	Tickets  []ProposalTicket
	Teams    [2][]uuid.UUID
	Accepted map[uuid.UUID]bool
	Deadline time.Time
	Status   string
}

// Students returns every student of the proposal.
func (p Proposal) Students() []uuid.UUID {
	var out []uuid.UUID
	for _, t := range p.Tickets {
		out = append(out, t.Students...)
	}
	return out
}

// Has reports a participant.
func (p Proposal) Has(student uuid.UUID) bool {
	for _, t := range p.Tickets {
		for _, s := range t.Students {
			if s == student {
				return true
			}
		}
	}
	return false
}

// TicketOf returns the ticket of a participant.
func (p Proposal) TicketOf(student uuid.UUID) uuid.UUID {
	for _, t := range p.Tickets {
		for _, s := range t.Students {
			if s == student {
				return t.TicketID
			}
		}
	}
	return uuid.Nil
}

// TeamOf returns the team (0 or 1) of a participant.
func (p Proposal) TeamOf(student uuid.UUID) int {
	for i, team := range p.Teams {
		for _, s := range team {
			if s == student {
				return i
			}
		}
	}
	return 0
}

// AllAccepted reports a complete ready check.
func (p Proposal) AllAccepted() bool {
	for _, s := range p.Students() {
		if !p.Accepted[s] {
			return false
		}
	}
	return len(p.Students()) > 0
}

// Resolution is what happens to the tickets of a cancelled proposal (MATCHMAKING §4.2, F104-AC-06): Requeue keeps
// their enqueued_at (requeued = true); Cancel ends them with Reason.
type Resolution struct {
	Requeue []uuid.UUID
	Cancel  []uuid.UUID
	Reason  string // ticket cancel reason of Cancel
	Blamed  []uuid.UUID
}

// Decline resolves a decline by student: its ticket is cancelled (declined_proposal), every other ticket goes back to
// its queue with its wait kept.
func (p Proposal) Decline(student uuid.UUID) Resolution {
	r := Resolution{Reason: CancelDeclined, Blamed: []uuid.UUID{student}}
	mine := p.TicketOf(student)
	for _, t := range p.Tickets {
		if t.TicketID == mine {
			r.Cancel = append(r.Cancel, t.TicketID)
		} else {
			r.Requeue = append(r.Requeue, t.TicketID)
		}
	}
	return r
}

// Timeout resolves an expired ready check: tickets whose every student accepted go back to the queue; the others are
// cancelled (timeout) and their silent students are blamed (cooldown count).
func (p Proposal) Timeout() Resolution {
	r := Resolution{Reason: CancelTimeout}
	for _, t := range p.Tickets {
		all := true
		for _, s := range t.Students {
			if !p.Accepted[s] {
				all = false
				r.Blamed = append(r.Blamed, s)
			}
		}
		if all {
			r.Requeue = append(r.Requeue, t.TicketID)
		} else {
			r.Cancel = append(r.Cancel, t.TicketID)
		}
	}
	return r
}

// Unavailable resolves a proposal with tickets that may no longer play (H5 recheck, party change): those tickets are
// cancelled with reason, the others go back to the queue.
func (p Proposal) Unavailable(gone map[uuid.UUID]bool, reason string) Resolution {
	r := Resolution{Reason: reason}
	for _, t := range p.Tickets {
		if gone[t.TicketID] {
			r.Cancel = append(r.Cancel, t.TicketID)
		} else {
			r.Requeue = append(r.Requeue, t.TicketID)
		}
	}
	return r
}
