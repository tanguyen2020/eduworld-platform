package command

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/view"
	mm "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/matchmaking"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
)

// Jobs of matchmaking (worker profile jobs).
const (
	MatchmakerJob   = "matchmaker"
	TicketExpiryJob = "matchmaking-ticket-expiry"
	DuelExpiryJob   = "duel-expiry"
)

// TicketTracker remembers the waiting tickets this worker saw, to tell their students when Valkey lost them
// (F104-AC-11, ADR-0004): a ticket that vanished without an end is cancelled with reason system.
type TicketTracker struct {
	mu sync.Mutex
	m  map[uuid.UUID][]uuid.UUID
}

// NewTicketTracker returns an empty tracker.
func NewTicketTracker() *TicketTracker { return &TicketTracker{m: map[uuid.UUID][]uuid.UUID{}} }

// Track remembers a waiting ticket.
func (k *TicketTracker) Track(t port.Ticket) {
	if t.Status != mm.StatusQueued && t.Status != mm.StatusProposed {
		return
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	k.m[t.ID] = append([]uuid.UUID(nil), t.Students...)
}

// Forget drops a ticket that ended normally.
func (k *TicketTracker) Forget(id uuid.UUID) {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.m, id)
}

func (k *TicketTracker) snapshot() map[uuid.UUID][]uuid.UUID {
	k.mu.Lock()
	defer k.mu.Unlock()
	out := make(map[uuid.UUID][]uuid.UUID, len(k.m))
	for id, s := range k.m {
		out[id] = s
	}
	return out
}

func keyOf(t port.Ticket) mm.QueueKey {
	return mm.QueueKey{Mode: t.Mode, Type: t.Type, Scope: mm.Scope{Kind: t.ScopeKind, ID: t.ScopeID}, GradeKey: t.GradeKey, Subject: t.SubjectID,
		Topic: t.TopicID, TeamSize: t.TeamSize}
}

// Tick is one run of the matchmaker (MATCHMAKING §6, EW_MM_TICK): every queue under its lock (one matchmaker per queue
// whatever the number of workers, F104-AC-09), greedy pairing, proposals; then the ready checks past their deadline
// and the accepted proposals whose match is still missing. Returns the proposals and matches made.
func (h Matchmaking) Tick(ctx context.Context) (int, error) {
	start := time.Now()
	now := h.now()
	cfg, err := h.platformConfig(ctx, now)
	if err != nil {
		return 0, err
	}
	queues, err := h.Store.Queues(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	var errs []error
	for _, q := range queues {
		ok, err := h.Store.Lock(ctx, q, h.Settings.Owner, h.lockTTL())
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !ok {
			continue
		}
		c, err := h.matchQueue(ctx, q, cfg, now)
		if uerr := h.Store.Unlock(ctx, q, h.Settings.Owner); uerr != nil {
			slog.WarnContext(ctx, "competition: matchmaker lock not released", "error", uerr)
		}
		n += c
		if err != nil {
			errs = append(errs, err)
		}
	}
	c, err := h.dueProposals(ctx, now)
	n += c
	if err != nil {
		errs = append(errs, err)
	}
	if h.Metrics != nil {
		h.Metrics.Loop(time.Since(start))
	}
	return n, errors.Join(errs...)
}

func (h Matchmaking) lockTTL() time.Duration {
	if h.Settings.LockTTL > 0 {
		return h.Settings.LockTTL
	}
	return 5 * time.Second
}

// queueData are the projections read for one queue in one tenant transaction.
type queueData struct {
	players map[uuid.UUID]mm.Player
	blocked map[write.Pair]bool
	recent  map[write.Pair]bool
	ratings map[uuid.UUID]mm.Rating
	cards   map[uuid.UUID]port.Card
}

// matchQueue pairs one queue: H5 is checked again (account, restriction, hours) and a student who changed grade
// leaves the queue (F104-AC-08 edge case); blocked pairs never meet (H6).
func (h Matchmaking) matchQueue(ctx context.Context, q string, cfg mm.Config, now time.Time) (int, error) {
	limit := max(cfg.Batch, 2*cfg.BucketThreshold)
	tickets, err := h.Store.Queued(ctx, q, limit)
	if err != nil || len(tickets) == 0 {
		return 0, err
	}
	first := tickets[0]
	if h.Metrics != nil {
		h.Metrics.QueueSize(first.Mode, first.GradeKey, first.SubjectID.String(), len(tickets))
	}
	var students []uuid.UUID
	for _, t := range tickets {
		if t.TenantID == first.TenantID {
			students = append(students, t.Students...)
		}
	}
	sc, err := authz.EventScope(first.TenantID)
	if err != nil {
		return 0, err
	}
	var d queueData
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		var err error
		if d.players, err = h.Repo.Players(ctx, first.TenantID, students, now); err != nil {
			return err
		}
		if d.blocked, err = h.Repo.BlockedPairs(ctx, first.TenantID, students); err != nil {
			return err
		}
		if d.recent, err = h.Repo.RecentPairs(ctx, first.TenantID, students, now.Add(-cfg.RecentWindow())); err != nil {
			return err
		}
		if d.ratings, err = h.ratings(ctx, first.TenantID, students, first.Mode, first.SubjectID, cfg); err != nil {
			return err
		}
		d.cards, err = h.Directory.Cards(ctx, first.TenantID, students)
		return err
	})
	if err != nil {
		return 0, err
	}
	byID := map[uuid.UUID]port.Ticket{}
	var cands []mm.Candidate
	for _, t := range tickets {
		if h.Tracker != nil {
			h.Tracker.Track(t)
		}
		if t.TenantID != first.TenantID || !h.stillEligible(t, d.players) {
			if ok, err := h.Store.End(ctx, t.ID, mm.StatusCancelled, mm.CancelSystem, []string{mm.StatusQueued}); err == nil && ok {
				h.ended(ctx, t, mm.StatusCancelled, mm.CancelSystem, now)
			}
			continue
		}
		byID[t.ID] = t
		cands = append(cands, t.Candidate())
	}
	env := mm.Env{Now: now,
		Blocked: func(a, b uuid.UUID) bool { return d.blocked[write.PairOf(a, b)] },
		Recent:  func(a, b uuid.UUID) bool { return d.recent[write.PairOf(a, b)] }}
	n := 0
	for _, g := range cfg.Pair(keyOf(first), cands, env) {
		ok, err := h.propose(ctx, q, g, byID, d, cfg, now)
		if err != nil {
			return n, err
		}
		if ok {
			n++
		}
	}
	return n, nil
}

// stillEligible is the H5/H1 recheck of a queued ticket.
func (h Matchmaking) stillEligible(t port.Ticket, players map[uuid.UUID]mm.Player) bool {
	for _, s := range t.Students {
		p, ok := players[s]
		if !ok || mm.Eligible(p) != nil || p.Grade != t.Grade {
			return false
		}
	}
	return true
}

// propose turns a group into a proposal with the ready check deadline (mm_ready_check) and pushes it with the public
// cards only (F104-AC-05).
func (h Matchmaking) propose(ctx context.Context, q string, g mm.Group, byID map[uuid.UUID]port.Ticket, d queueData, cfg mm.Config,
	now time.Time) (bool, error) {
	first := byID[g.Tickets[0].TicketID]
	p := port.Proposal{Proposal: mm.Proposal{ID: newID(), Queue: q, Teams: g.Teams, Accepted: map[uuid.UUID]bool{},
		Deadline: now.Add(cfg.ReadyCheck()), Status: mm.ProposalOpen}, TenantIDs: []uuid.UUID{first.TenantID}, Mode: first.Mode,
		Type: first.Type, ScopeKind: first.ScopeKind, SubjectID: first.SubjectID, TopicID: first.TopicID, Grade: 9,
		Ranked: cfg.Ranked(first.Mode, first.Type), RulesetVersionID: cfg.VersionID, Schools: map[uuid.UUID]uuid.UUID{},
		Tenants: map[uuid.UUID]uuid.UUID{}, Parties: map[uuid.UUID]uuid.UUID{}, Ratings: map[uuid.UUID]mm.Rating{}, CreatedAt: now}
	for _, c := range g.Tickets {
		t := byID[c.TicketID]
		p.Tickets = append(p.Tickets, mm.ProposalTicket{TicketID: t.ID, Students: t.Students})
		p.Grade = min(p.Grade, t.Grade)
		for i, s := range t.Students {
			p.Tenants[s] = t.TenantID
			if i < len(t.Schools) {
				p.Schools[s] = t.Schools[i]
			}
			if t.PartyID != nil {
				p.Parties[s] = *t.PartyID
			}
			p.Ratings[s] = d.ratings[s]
		}
	}
	ok, err := h.Store.Propose(ctx, p)
	if err != nil || !ok {
		return false, err
	}
	for _, pt := range p.Tickets {
		h.send(ctx, pt.Students, rtProposalCreated, view.ProposalCreated(p, pt.TicketID, d.cards))
		t := byID[pt.TicketID]
		t.Status, t.ProposalID = mm.StatusProposed, &p.ID
		h.send(ctx, pt.Students, rtTicketUpdated, view.NewTicket(t, cfg, now).Updated())
	}
	return true, nil
}

// dueProposals cancels the ready checks past their deadline (F104-AC-06: timeout) and retries the match of the
// accepted proposals whose creation failed (same id: never a second match — edge case).
func (h Matchmaking) dueProposals(ctx context.Context, now time.Time) (int, error) {
	stale := h.Settings.StaleAccept
	if stale <= 0 {
		stale = 5 * time.Second
	}
	ids, err := h.Store.Due(ctx, now, stale)
	if err != nil {
		return 0, err
	}
	n := 0
	var errs []error
	for _, id := range ids {
		p, err := h.Store.Proposal(ctx, id)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if p == nil {
			continue
		}
		switch {
		case p.Status == mm.ProposalOpen && !now.Before(p.Deadline):
			if err := h.cancelProposal(ctx, *p, p.Timeout(), mm.ProposalTimeout, now); err != nil && !errors.Is(err, mm.ErrProposalClosed) {
				errs = append(errs, err)
			}
		case p.Status == mm.ProposalAccepted:
			if err := h.finalize(ctx, *p, now); err != nil && !errors.Is(err, mm.ErrProposalClosed) {
				errs = append(errs, err)
				continue
			}
			n++
		}
	}
	return n, errors.Join(errs...)
}

// ExpireTickets is one run of the job matchmaking-ticket-expiry (F104-AC-10, AC-11, AC-12): tickets waiting longer
// than mm_ticket_ttl expire; past mm_bot_after a solo 1v1 ticket is offered the practice bot; a student disconnected
// longer than disconnect_cancel loses its ticket (system); tickets lost with Valkey are reported as cancelled (system).
func (h Matchmaking) ExpireTickets(ctx context.Context) (int, error) {
	now := h.now()
	cfg, err := h.platformConfig(ctx, now)
	if err != nil {
		return 0, err
	}
	queues, err := h.Store.Queues(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, q := range queues {
		tickets, err := h.Store.Queued(ctx, q, 10*cfg.BucketThreshold)
		if err != nil {
			return n, err
		}
		online := h.onlineOf(ctx, tickets)
		for _, t := range tickets {
			if h.Tracker != nil {
				h.Tracker.Track(t)
			}
			c, err := h.sweepTicket(ctx, t, online, cfg, now)
			if err != nil {
				return n, err
			}
			n += c
		}
	}
	lost, err := h.reportLost(ctx)
	return n + lost, err
}

func (h Matchmaking) onlineOf(ctx context.Context, tickets []port.Ticket) map[uuid.UUID]bool {
	if h.Presence == nil {
		return nil
	}
	var ids []uuid.UUID
	for _, t := range tickets {
		ids = append(ids, t.Students...)
	}
	m, err := h.Presence.Online(ctx, ids)
	if err != nil {
		slog.WarnContext(ctx, "competition: presence unavailable for matchmaking", "error", err)
		return nil // presence unknown: nobody is cancelled for it
	}
	return m
}

func (h Matchmaking) sweepTicket(ctx context.Context, t port.Ticket, online map[uuid.UUID]bool, cfg mm.Config, now time.Time) (int, error) {
	wait := now.Sub(t.EnqueuedAt)
	if wait >= cfg.TicketTTL() {
		ok, err := h.Store.End(ctx, t.ID, mm.StatusExpired, "", []string{mm.StatusQueued})
		if err != nil || !ok {
			return 0, err
		}
		h.ended(ctx, t, mm.StatusExpired, "", now)
		return 1, nil
	}
	if online != nil {
		all := true
		for _, s := range t.Students {
			all = all && online[s]
		}
		switch {
		case all && t.OfflineSince != nil:
			if err := h.Store.Flag(ctx, t.ID, nil, nil, true); err != nil {
				return 0, err
			}
		case !all && t.OfflineSince == nil:
			if err := h.Store.Flag(ctx, t.ID, nil, &now, false); err != nil {
				return 0, err
			}
		case !all && now.Sub(*t.OfflineSince) >= cfg.DisconnectCancel():
			ok, err := h.Store.End(ctx, t.ID, mm.StatusCancelled, mm.CancelSystem, []string{mm.StatusQueued})
			if err != nil || !ok {
				return 0, err
			}
			h.ended(ctx, t, mm.StatusCancelled, mm.CancelSystem, now)
			return 1, nil
		}
	}
	if t.Mode == mm.ModeBattle1v1 && t.PartyID == nil && !t.BotOffered && wait >= cfg.BotAfter() {
		yes := true
		if err := h.Store.Flag(ctx, t.ID, &yes, nil, false); err != nil {
			return 0, err
		}
		t.BotOffered = true
		h.send(ctx, t.Students, rtTicketUpdated, view.NewTicket(t, cfg, now).Updated())
		return 1, nil
	}
	return 0, nil
}

// reportLost tells the students of tracked tickets that vanished from Valkey (FLUSHALL, failover without
// persistence): matchmaking.ticket.updated cancelled with reason system, so the client creates a new ticket.
func (h Matchmaking) reportLost(ctx context.Context) (int, error) {
	if h.Tracker == nil {
		return 0, nil
	}
	known := h.Tracker.snapshot()
	if len(known) == 0 {
		return 0, nil
	}
	ids := make([]uuid.UUID, 0, len(known))
	for id := range known {
		ids = append(ids, id)
	}
	statuses, err := h.Store.Statuses(ctx, ids)
	if err != nil {
		return 0, err
	}
	n := 0
	for id, students := range known {
		st, ok := statuses[id]
		switch {
		case !ok:
			h.send(ctx, students, rtTicketUpdated, view.TicketCancelled(id, mm.CancelSystem))
			h.Tracker.Forget(id)
			n++
		case st != mm.StatusQueued && st != mm.StatusProposed:
			h.Tracker.Forget(id)
		}
	}
	return n, nil
}
