package view

import (
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	mm "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/matchmaking"
)

// Ticket is the MatchmakingTicket of the contract (REST) and the payload of matchmaking.ticket.updated (realtime):
// never the rating, school, class or grade of anyone (R16).
type Ticket struct {
	T           port.Ticket
	Now         time.Time
	BotOffer    bool
	Widened     bool
	CreatedAt   time.Time
	WaitSeconds int
}

// NewTicket builds the view of t at now with the configuration of its queue.
func NewTicket(t port.Ticket, c mm.Config, now time.Time) Ticket {
	wait := now.Sub(t.EnqueuedAt)
	if wait < 0 {
		wait = 0
	}
	queued := t.Status == mm.StatusQueued
	bot := queued && t.Mode == mm.ModeBattle1v1 && t.PartyID == nil && (t.BotOffered || wait >= c.BotAfter())
	widened := queued && t.Type != mm.TypeRandom && c.Widened(wait)
	return Ticket{T: t, Now: now, BotOffer: bot, Widened: widened, CreatedAt: t.EnqueuedAt, WaitSeconds: int(wait / time.Second)}
}

// Body is the MatchmakingTicket of the contract.
func (v Ticket) Body() map[string]any {
	t := v.T
	m := map[string]any{"ticket_id": t.ID, "status": t.Status, "mode": t.Mode, "matchmaking_type": t.Type, "subject_id": t.SubjectID,
		"created_at": v.CreatedAt.UTC().Format(time.RFC3339Nano), "wait_seconds": v.WaitSeconds, "search_widened": v.Widened,
		"bot_offer_available": v.BotOffer, "requeued": t.Requeued}
	if t.TopicID != nil {
		m["topic_id"] = *t.TopicID
	}
	if t.PartyID != nil {
		m["party_id"] = *t.PartyID
	}
	if t.CancelReason != "" && t.Status == mm.StatusCancelled {
		m["cancel_reason"] = t.CancelReason
	}
	if t.ProposalID != nil {
		m["proposal_id"] = *t.ProposalID
	}
	if t.MatchID != nil {
		m["match_id"] = *t.MatchID
	}
	return m
}

// Updated is the payload of matchmaking.ticket.updated.
func (v Ticket) Updated() map[string]any {
	t := v.T
	m := map[string]any{"ticket_id": t.ID, "status": t.Status, "mode": t.Mode, "matchmaking_type": t.Type, "subject_id": t.SubjectID,
		"wait_ms": max(v.Now.Sub(t.EnqueuedAt).Milliseconds(), 0), "search_widened": v.Widened, "bot_offer_available": v.BotOffer,
		"requeued": t.Requeued}
	if t.CancelReason != "" && t.Status == mm.StatusCancelled {
		m["cancel_reason"] = t.CancelReason
	}
	return m
}

// TicketCancelled is the payload of matchmaking.ticket.updated for a ticket that no longer exists (lost with Valkey).
func TicketCancelled(id uuid.UUID, reason string) map[string]any {
	return map[string]any{"ticket_id": id, "status": mm.StatusCancelled, "cancel_reason": reason}
}

// ProposalCreated is the payload of matchmaking.proposal.created for one participant: nickname, avatar and level of
// everyone (no real name, school or class — R16, F104-AC-05), the teams by user id.
func ProposalCreated(p port.Proposal, ticket uuid.UUID, cards map[uuid.UUID]port.Card) map[string]any {
	parts := make([]map[string]any, 0, len(p.Students()))
	for _, s := range p.Students() {
		parts = append(parts, CardBody(card(cards, s)))
	}
	return map[string]any{"proposal_id": p.ID, "ticket_id": ticket, "deadline_at": p.Deadline.UnixMilli(), "mode": p.Mode,
		"teams": [][]uuid.UUID{nonNil(p.Teams[0]), nonNil(p.Teams[1])}, "participants": parts}
}

func nonNil(ids []uuid.UUID) []uuid.UUID {
	if ids == nil {
		return []uuid.UUID{}
	}
	return ids
}

// ProposalUpdated is the payload of matchmaking.proposal.updated.
func ProposalUpdated(p port.Proposal) map[string]any {
	n := 0
	for _, s := range p.Students() {
		if p.Accepted[s] {
			n++
		}
	}
	return map[string]any{"proposal_id": p.ID, "accepted_count": n, "total": len(p.Students())}
}

// ProposalCancelled is the payload of matchmaking.proposal.cancelled.
func ProposalCancelled(id uuid.UUID, reason string, requeued bool) map[string]any {
	return map[string]any{"proposal_id": id, "reason": reason, "requeued": requeued}
}

// MatchAssigned is the payload of matchmaking.match.assigned: the room of the match is opened by the battle engine
// (F105) with room_id = match_id.
func MatchAssigned(id uuid.UUID, mode string) map[string]any {
	return map[string]any{"match_id": id, "room_id": id.String(), "mode": mode}
}

// Match is the Match of the contract for a participant (no rating, no school — R16).
func Match(m write.Match, cards map[uuid.UUID]port.Card) map[string]any {
	parts := make([]map[string]any, 0, len(m.Teams))
	for id, team := range m.Teams {
		parts = append(parts, map[string]any{"player": CardBody(card(cards, id)), "team_no": team})
	}
	return map[string]any{"match_id": m.ID, "mode": m.Mode, "matchmaking_type": m.Type, "subject_id": m.SubjectID, "status": m.Status,
		"ranked": m.Ranked, "is_bot": m.IsBot, "room_id": m.ID, "participants": parts}
}

// Duel is the Duel of the contract.
func Duel(d mm.Duel) map[string]any {
	m := map[string]any{"duel_id": d.ID, "challenger_id": d.ChallengerID, "opponent_id": d.OpponentID, "subject_id": d.SubjectID,
		"status": d.Status, "expires_at": d.ExpiresAt.UTC().Format(time.RFC3339Nano)}
	if d.TopicID != nil {
		m["topic_id"] = *d.TopicID
	}
	if d.MatchID != nil {
		m["match_id"] = *d.MatchID
	}
	return m
}

// CompetitionMode is one item of listCompetitionModes.
func CompetitionMode(m mm.ModeConfig, ranked bool, subjects []uuid.UUID) map[string]any {
	if subjects == nil {
		subjects = []uuid.UUID{}
	}
	rt := m.RankedTypes
	if rt == nil {
		rt = []string{}
	}
	return map[string]any{"mode": m.Mode, "matchmaking_types": m.Types, "ranked": ranked, "ranked_matchmaking_types": rt,
		"min_grade": m.MinGrade, "subjects": subjects}
}
