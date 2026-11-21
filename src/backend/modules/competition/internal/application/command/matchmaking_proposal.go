package command

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/view"
	mm "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/matchmaking"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// proposalOf returns a proposal the caller takes part in, within its tenant; anything else is 404 (F104-AC-24).
func (h Matchmaking) proposalOf(ctx context.Context, tenant, me, id uuid.UUID) (*port.Proposal, error) {
	p, err := h.Store.Proposal(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil || !p.Has(me) || p.Tenants[me] != tenant {
		return nil, problem.New(problem.NotFound, "")
	}
	return p, nil
}

// AcceptProposal implements acceptMatchProposal and WS matchmaking.proposal.accept (F104-AC-05): the last accept
// creates the match (match_id = proposal_id) then pushes matchmaking.match.assigned.
func (h Matchmaking) AcceptProposal(ctx context.Context, id uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.AcceptMatchProposal")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return err
	}
	if _, err := h.proposalOf(ctx, tenant, me, id); err != nil {
		return err
	}
	now := h.now()
	p, complete, err := h.Store.Accept(ctx, id, me, now)
	if err != nil {
		return mmError(err)
	}
	h.send(ctx, p.Students(), rtProposalUpdated, view.ProposalUpdated(*p))
	if complete {
		return mmError(h.finalize(ctx, *p, now))
	}
	return nil
}

// DeclineProposal implements declineMatchProposal and WS matchmaking.proposal.decline (F104-AC-06).
func (h Matchmaking) DeclineProposal(ctx context.Context, id uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.DeclineMatchProposal")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return err
	}
	if _, err := h.proposalOf(ctx, tenant, me, id); err != nil {
		return err
	}
	return mmError(h.decline(ctx, id, me, mm.CancelDeclined, h.now()))
}

// decline cancels a proposal declined by student: its ticket is cancelled (reason), the others go back to their queue
// with their wait kept; the decline counts toward the cooldown (mm_ticket_rate).
func (h Matchmaking) decline(ctx context.Context, id, student uuid.UUID, reason string, now time.Time) error {
	p, err := h.Store.Proposal(ctx, id)
	if err != nil {
		return err
	}
	if p == nil {
		return mm.ErrProposalClosed
	}
	res := p.Decline(student)
	res.Reason = reason
	return h.cancelProposal(ctx, *p, res, mm.ProposalDeclined, now)
}

// cancelProposal applies a resolution to an open proposal: pushes matchmaking.proposal.cancelled with requeued per
// recipient and the new status of every ticket; blamed students count toward the cooldown.
func (h Matchmaking) cancelProposal(ctx context.Context, p port.Proposal, res mm.Resolution, why string, now time.Time) error {
	from := []string{mm.ProposalOpen}
	if why == mm.ProposalUnavailable {
		from = append(from, mm.ProposalAccepted)
	}
	if p.Direct {
		res.Requeue, res.Cancel = nil, nil // no queue behind a direct proposal: the tournament decides (F107)
	}
	ok, err := h.Store.Cancel(ctx, p.ID, res, from)
	if err != nil {
		return err
	}
	if !ok {
		return mm.ErrProposalClosed
	}
	cfg := h.cachedOrDefault(ctx, now)
	if why != mm.ProposalUnavailable {
		for _, s := range res.Blamed {
			if err := h.Store.Blame(ctx, s, now, cfg.TicketRate.DeclineLimit, cfg.DeclineWindow(), cfg.Cooldown()); err != nil {
				slog.WarnContext(ctx, "competition: decline not counted", "error", err)
			}
		}
	}
	if h.Metrics != nil {
		h.Metrics.Proposal(why)
	}
	requeued := map[uuid.UUID]bool{}
	for _, id := range res.Requeue {
		requeued[id] = true
	}
	for _, t := range p.Tickets {
		h.send(ctx, t.Students, rtProposalCancelled, view.ProposalCancelled(p.ID, why, requeued[t.TicketID]))
	}
	for _, id := range append(append([]uuid.UUID(nil), res.Requeue...), res.Cancel...) {
		t, err := h.Store.Ticket(ctx, id)
		if err != nil || t == nil {
			continue
		}
		if t.Status == mm.StatusQueued {
			h.send(ctx, t.Students, rtTicketUpdated, view.NewTicket(*t, cfg, now).Updated())
		} else {
			h.ended(ctx, *t, t.Status, t.CancelReason, now)
		}
	}
	return nil
}

// cachedOrDefault returns the cached configuration, else reads it in a short platform transaction.
func (h Matchmaking) cachedOrDefault(ctx context.Context, now time.Time) mm.Config {
	if c := h.Configs; c != nil {
		c.mu.Lock()
		cfg, ok := c.cfg, c.ok
		c.mu.Unlock()
		if ok {
			return cfg
		}
	}
	cfg, err := h.platformConfig(ctx, now)
	if err != nil {
		slog.WarnContext(ctx, "competition: matchmaking configuration unavailable", "error", err)
	}
	return cfg
}

// platformConfig reads the configuration in a platform transaction (jobs).
func (h Matchmaking) platformConfig(ctx context.Context, now time.Time) (mm.Config, error) {
	ps, err := authz.SystemPlatformScope(MatchmakerJob)
	if err != nil {
		return mm.Config{}, err
	}
	var cfg mm.Config
	err = h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
		var err error
		cfg, err = h.config(ctx, now)
		return err
	})
	return cfg, err
}

// scopeOf is the transaction scope of a proposal: its tenant, or the platform for a platform event (ADR-0054).
func scopeOf(p port.Proposal) (tenancy.Scope, error) {
	if p.ScopeKind == mm.ScopePlatform || len(p.TenantIDs) != 1 {
		return authz.SystemPlatformScope(MatchmakerJob)
	}
	return authz.EventScope(p.TenantIDs[0])
}

// finalize creates the match of a fully accepted proposal (match_id = proposal_id, idempotent), then marks the
// tickets matched and pushes matchmaking.match.assigned. A ticket lost with Valkey cancels the proposal: no match is
// ever created from a lost ticket (F104-AC-11).
func (h Matchmaking) finalize(ctx context.Context, p port.Proposal, now time.Time) error {
	if !p.Direct {
		ids := make([]uuid.UUID, 0, len(p.Tickets))
		for _, t := range p.Tickets {
			ids = append(ids, t.TicketID)
		}
		alive, err := h.Store.Statuses(ctx, ids)
		if err != nil {
			return err
		}
		gone := map[uuid.UUID]bool{}
		for _, id := range ids {
			if _, ok := alive[id]; !ok {
				gone[id] = true
			}
		}
		if len(gone) > 0 {
			err := h.cancelProposal(ctx, p, p.Unavailable(gone, mm.CancelSystem), mm.ProposalUnavailable, now)
			if err != nil && !errors.Is(err, mm.ErrProposalClosed) {
				return err
			}
			return mm.ErrProposalClosed
		}
	}
	sc, err := scopeOf(p)
	if err != nil {
		return err
	}
	m := write.NewMatch{ID: p.ID, Mode: p.Mode, Type: p.Type, SubjectID: p.SubjectID, TopicID: p.TopicID, Grade: p.Grade, Ranked: p.Ranked,
		ScopeKind: p.ScopeKind, TenantIDs: p.TenantIDs, TournamentNodeID: p.TournamentNodeID, RulesetVersionID: p.RulesetVersionID, At: now}
	for _, s := range p.Students() {
		r := p.Ratings[s]
		part := write.MatchParticipant{StudentID: s, TenantID: p.Tenants[s], SchoolID: p.Schools[s], TeamNo: p.TeamOf(s), Rating: &r}
		if pid, ok := p.Parties[s]; ok {
			pid := pid
			part.PartyID = &pid
		}
		m.Participants = append(m.Participants, part)
	}
	if err := h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error { return h.createMatch(ctx, m) }); err != nil {
		return err // the proposal stays accepted: the matchmaker retries with the same id
	}
	if err := h.Store.Complete(ctx, p.ID, p.ID); err != nil {
		return err
	}
	h.assigned(ctx, p, now)
	return nil
}

// assigned pushes the match to every participant and the matched tickets.
func (h Matchmaking) assigned(ctx context.Context, p port.Proposal, now time.Time) {
	h.send(ctx, p.Students(), rtMatchAssigned, view.MatchAssigned(p.ID, p.Mode))
	for _, t := range p.Tickets {
		if p.Direct {
			continue
		}
		cur, err := h.Store.Ticket(ctx, t.TicketID)
		if err != nil || cur == nil {
			continue
		}
		if h.Tracker != nil {
			h.Tracker.Forget(cur.ID)
		}
		h.send(ctx, cur.Students, rtTicketUpdated, view.Ticket{T: *cur, Now: now}.Updated())
		if h.Metrics != nil {
			h.Metrics.Wait(cur.Mode, mm.StatusMatched, now.Sub(cur.EnqueuedAt))
		}
	}
	if h.Metrics != nil {
		h.Metrics.Proposal("accepted")
		h.Metrics.MatchCreated(p.Mode)
		if len(p.Teams[0]) > 0 && len(p.Teams[1]) > 0 {
			h.Metrics.RatingGap(teamGap(p))
		}
	}
}

func teamGap(p port.Proposal) float64 {
	var sum [2]float64
	for i, team := range p.Teams {
		for _, s := range team {
			sum[i] += p.Ratings[s].Mu
		}
		sum[i] /= float64(max(len(team), 1))
	}
	if sum[0] > sum[1] {
		return sum[0] - sum[1]
	}
	return sum[1] - sum[0]
}

// createMatch inserts the match with match.created and its audit in the transaction of ctx; a match that exists
// already (retry) writes nothing.
func (h Matchmaking) createMatch(ctx context.Context, m write.NewMatch) error {
	created, err := h.Repo.CreateMatch(ctx, m)
	if err != nil || !created {
		return err
	}
	parts := make([]map[string]any, 0, len(m.Participants))
	for _, p := range m.Participants {
		row := map[string]any{"student_id": p.StudentID, "tenant_id": p.TenantID, "school_id": p.SchoolID, "team_no": p.TeamNo, "party_id": nil}
		if p.PartyID != nil {
			row["party_id"] = *p.PartyID
		}
		parts = append(parts, row)
	}
	payload := map[string]any{"match_id": m.ID, "mode": m.Mode, "matchmaking_type": m.Type, "ranked": m.Ranked, "is_bot": m.IsBot,
		"subject_id": m.SubjectID, "topic_id": m.TopicID, "grade_level": m.Grade, "scope_kind": m.ScopeKind,
		"ruleset_version_id": m.RulesetVersionID, "duel_id": m.DuelID, "tournament_node_id": m.TournamentNodeID, "room_id": m.ID,
		"participants": parts}
	var tenant *uuid.UUID
	if len(m.TenantIDs) == 1 {
		t := m.TenantIDs[0]
		tenant = &t
	}
	if err := h.Events.Publish(ctx, write.Event{Type: "match.created", SubjectType: "match", SubjectID: m.ID.String(),
		PartitionKey: m.ID.String(), TenantID: tenant, OccurredAt: m.At, Payload: payload}); err != nil {
		return err
	}
	school := ""
	if len(m.Participants) > 0 && m.ScopeKind == mm.ScopeSchool {
		school = m.Participants[0].SchoolID.String()
	}
	return h.Audit.Record(ctx, audit.Event{Action: AuditMatchCreated, TargetType: "match", TargetID: m.ID.String(), Result: audit.ResultSuccess,
		SchoolID: school, Metadata: map[string]any{"mode": m.Mode, "matchmaking_type": m.Type, "ranked": m.Ranked, "is_bot": m.IsBot,
			"participants": len(m.Participants)}})
}
