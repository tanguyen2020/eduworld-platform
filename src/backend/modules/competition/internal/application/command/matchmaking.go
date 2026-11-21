package command

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/view"
	mm "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/matchmaking"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Realtime message types of matchmaking (AsyncAPI sendMatchmakingEvents).
const (
	rtTicketUpdated     = "matchmaking.ticket.updated"
	rtProposalCreated   = "matchmaking.proposal.created"
	rtProposalUpdated   = "matchmaking.proposal.updated"
	rtProposalCancelled = "matchmaking.proposal.cancelled"
	rtMatchAssigned     = "matchmaking.match.assigned"
)

// Audit actions of matchmaking (ids only — R14). Tickets are ephemeral and not audited (plan F104).
const (
	AuditMatchCreated = "match.created"
	AuditDuelSent     = "duel.sent"
	AuditDuelAccepted = "duel.accepted"
	AuditDuelDeclined = "duel.declined"
	AuditDuelExpired  = "duel.expired"
)

// MatchmakingSettings are the deployment settings of matchmaking (the rules come from the published configuration).
type MatchmakingSettings struct {
	Owner       string        // id of this instance in the per-queue locks
	LockTTL     time.Duration // per-queue lock of the matchmaker (MATCHMAKING §6: a dead worker is replaced within it)
	ConfigTTL   time.Duration // how long the published configuration is cached (0: read every time)
	StaleAccept time.Duration // an accepted proposal without its match is retried after this delay
}

// Matchmaking is the matchmaking of module competition (F104): tickets and proposals in Valkey (ADR-0004), matches and
// duels in PostgreSQL with their outbox events and audit (CQRS write side, ADR-0087).
type Matchmaking struct {
	UoW         write.UnitOfWork
	AfterCommit write.AfterCommit
	Repo        write.MatchRepository
	Directory   write.Directory       // tenant setting (cross-school), friendships, public cards
	Parties     write.PartyRepository // the party of a team ticket (F100)
	Events      write.EventPublisher
	Audit       write.Auditor
	Store       port.MatchmakingStore
	Realtime    port.PartyRealtime      // nil: no push
	Presence    port.PartyPresence      // nil: nobody online (team tickets refused, queued tickets kept)
	Ratings     port.RatingReader       // nil: the default rating of the configuration (until F106)
	Metrics     port.MatchmakingMetrics // nil: no metrics
	Settings    MatchmakingSettings
	Now         func() time.Time
	Configs     *ConfigCache   // nil: no cache
	Tracker     *TicketTracker // nil: no lost-ticket detection
}

func (h Matchmaking) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

// ConfigCache keeps the published matchmaking configuration for a short time (the matchmaker reads it every tick).
type ConfigCache struct {
	mu  sync.Mutex
	ttl time.Duration
	cfg mm.Config
	at  time.Time
	ok  bool
}

// NewConfigCache returns a cache valid for ttl.
func NewConfigCache(ttl time.Duration) *ConfigCache { return &ConfigCache{ttl: ttl} }

// config returns the configuration in effect at now; it must run inside a transaction of ctx.
func (h Matchmaking) config(ctx context.Context, now time.Time) (mm.Config, error) {
	c := h.Configs
	if c == nil || c.ttl <= 0 {
		return h.Repo.ActiveConfig(ctx, now)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ok && now.Sub(c.at) < c.ttl && now.After(c.at) {
		return c.cfg, nil
	}
	cfg, err := h.Repo.ActiveConfig(ctx, now)
	if err != nil {
		return mm.Config{}, err
	}
	c.cfg, c.at, c.ok = cfg, now, true
	return cfg, nil
}

// send pushes a message to students (best effort: the client resyncs with getMatchmakingTicket).
func (h Matchmaking) send(ctx context.Context, to []uuid.UUID, typ string, payload any) {
	if h.Realtime == nil || len(to) == 0 {
		return
	}
	if err := h.Realtime.Send(ctx, to, typ, payload); err != nil {
		slog.WarnContext(ctx, "competition: matchmaking push failed", "type", typ, "error", err)
	}
}

// later runs fn after the commit of the transaction of ctx.
func (h Matchmaking) later(ctx context.Context, fn func(ctx context.Context)) error {
	if h.AfterCommit == nil {
		return nil
	}
	return h.AfterCommit(ctx, fn)
}

func (h Matchmaking) ticketMetric(mode string, err error) {
	if h.Metrics == nil {
		return
	}
	result := "ok"
	if err != nil {
		var pe *problem.Error
		if errors.As(err, &pe) {
			result = string(pe.Code)
		} else {
			result = "error"
		}
	}
	h.Metrics.Ticket(mode, result)
}

// rateLimited is a 429 with Retry-After (mm_ticket_rate, decline cooldown — F104-AC-13).
func rateLimited(wait time.Duration) error {
	return RetryAfter{Err: problem.New(problem.RateLimited, "").Retry(), After: max(wait, time.Second)}
}

// mmError maps the errors of matchmaking to the problem codes of the contract.
func mmError(err error) error {
	var ve mm.ValidationError
	var pe mm.PolicyError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &ve):
		return problem.Invalid(ve.Field, problem.SubInvalid, ve.Message)
	case errors.As(err, &pe):
		return problem.New(problem.PolicyDenied, "")
	case errors.Is(err, port.ErrTicketActive):
		return problem.New(problem.MmTicketAlreadyActive, "")
	case errors.Is(err, mm.ErrNotParticipant), errors.Is(err, mm.ErrNotDuelOpponent):
		return problem.New(problem.NotFound, "")
	case errors.Is(err, mm.ErrProposalClosed):
		return problem.New(problem.Conflict, "proposal closed")
	case errors.Is(err, mm.ErrDuelGone):
		return problem.New(problem.DuelExpired, "")
	case errors.Is(err, write.ErrDuelPending):
		return problem.New(problem.Conflict, "a pending duel exists")
	case errors.Is(err, party.ErrNotLeader):
		return problem.New(problem.PartyNotLeader, "")
	case errors.Is(err, party.ErrNotMember), errors.Is(err, party.ErrDisbanded):
		return problem.New(problem.PartyNotFound, "")
	}
	return err
}

// ticketID is the id of a new ticket: derived from the Idempotency-Key so a retry after a lost commit finds the same
// ticket (research R2 of F101), else a UUIDv7.
func ticketID(ctx context.Context, me uuid.UUID, salt string) uuid.UUID {
	if p, ok := idempotency.PendingFrom(ctx); ok {
		return uuid.NewSHA1(matchmakingNamespace, []byte(salt+"/"+me.String()+"/"+p.Key.Key.String()))
	}
	return newID()
}

// matchmakingNamespace is the UUIDv5 namespace of the ids derived by matchmaking.
var matchmakingNamespace = uuid.MustParse("0193a5f4-6c1e-7b2a-9d1f-00000000f104")

// CreateTicket implements createMatchmakingTicket (F104-AC-01…03, AC-07, AC-08, AC-13, AC-14, AC-17, AC-18, AC-26):
// grade, school, class, tenant and rating come from the server; the ticket lives in Valkey only.
func (h Matchmaking) CreateTicket(ctx context.Context, in mm.Request) (res view.Ticket, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.CreateMatchmakingTicket")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return res, err
	}
	defer func() { h.ticketMetric(in.Mode, err) }()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		now := h.now()
		cfg, err := h.config(ctx, now)
		if err != nil {
			return err
		}
		if err := cfg.CheckRequest(in); err != nil {
			return err
		}
		if wait, err := h.Store.Cooldown(ctx, me); err != nil {
			return err
		} else if wait > 0 {
			return rateLimited(wait)
		}
		ids, partyType, err := h.ticketStudents(ctx, tenant, me, in)
		if err != nil {
			return err
		}
		ps, err := h.players(ctx, tenant, me, ids, now)
		if err != nil {
			return err
		}
		ps[0].PartyType = partyType
		scope, err := h.scope(ctx, tenant, ps)
		if err != nil {
			return err
		}
		if err := cfg.CheckPlayers(in, ps); err != nil {
			return err
		}
		rating, err := h.rating(ctx, tenant, ids, in.Mode, in.SubjectID, cfg)
		if err != nil {
			return err
		}
		key := mm.QueueKey{Mode: in.Mode, Type: in.Type, Scope: scope, GradeKey: mm.GradeKey(in.Type, ps[0].Grade), Subject: in.SubjectID,
			Topic: in.TopicID, TeamSize: cfg.TeamSize(in, len(ids))}
		id := ticketID(ctx, me, "ticket")
		if cur, err := h.Store.Ticket(ctx, id); err != nil {
			return err
		} else if cur != nil && cur.Has(me) { // retry of a request whose commit was lost: same ticket
			res = view.NewTicket(*cur, cfg, now)
			return idempotency.SetResult(ctx, res.Body())
		}
		if wait, err := h.Store.TicketRate(ctx, me, now, cfg.TicketRate.PerHour); err != nil {
			return err
		} else if wait > 0 {
			return rateLimited(wait)
		}
		t := port.Ticket{ID: id, TenantID: tenant, OwnerID: me, Queue: key.String(), Mode: in.Mode, Type: in.Type, ScopeKind: scope.Kind,
			ScopeID: scope.ID, GradeKey: key.GradeKey, SubjectID: in.SubjectID, TopicID: in.TopicID, PartyID: in.PartyID, AllowFill: in.AllowFill,
			TeamSize: key.TeamSize, Grade: ps[0].Grade, Rating: rating, EnqueuedAt: now, Status: mm.StatusQueued}
		for _, p := range ps {
			t.Students = append(t.Students, p.ID)
			t.Schools = append(t.Schools, p.SchoolID)
			if p.ClassID != nil {
				t.Classes = append(t.Classes, *p.ClassID)
			}
		}
		if _, err := h.Store.Create(ctx, t); err != nil {
			return err
		}
		if h.Tracker != nil {
			h.Tracker.Track(t)
		}
		res = view.NewTicket(t, cfg, now)
		if err := idempotency.SetResult(ctx, res.Body()); err != nil {
			return err
		}
		return h.later(ctx, func(ctx context.Context) { h.send(ctx, t.Students, rtTicketUpdated, res.Updated()) })
	})
	return res, mmError(err)
}

// ticketStudents returns the students of a new ticket: the caller, or the members of its competition party when the
// caller is the leader and every member is online (F104-AC-14).
func (h Matchmaking) ticketStudents(ctx context.Context, tenant, me uuid.UUID, in mm.Request) ([]uuid.UUID, string, error) {
	if in.PartyID == nil {
		return []uuid.UUID{me}, "", nil
	}
	p, err := h.Parties.Load(ctx, tenant, *in.PartyID, false)
	if err != nil {
		return nil, "", err
	}
	if p == nil || !p.Active() {
		return nil, "", problem.New(problem.PartyNotFound, "")
	}
	if err := p.RequireMember(me); err != nil {
		return nil, "", err
	}
	if err := p.RequireLeader(me); err != nil {
		return nil, "", err
	}
	ids := p.MemberIDs()
	sort.Slice(ids, func(i, j int) bool { return ids[i] == me || (ids[j] != me && ids[i].String() < ids[j].String()) })
	online := map[uuid.UUID]bool{}
	if h.Presence != nil {
		if online, err = h.Presence.Online(ctx, ids); err != nil {
			return nil, "", problem.New(problem.Unavailable, "presence").Retry()
		}
	}
	for _, id := range ids {
		if !online[id] {
			return nil, "", problem.New(problem.PartyMemberNotOnline, "")
		}
	}
	return ids, p.Type, nil
}

// players returns the matchmaking view of ids (the caller first): the caller not projected yet → 503 retryable; a
// member unknown in the tenant is inactive.
func (h Matchmaking) players(ctx context.Context, tenant, me uuid.UUID, ids []uuid.UUID, now time.Time) ([]mm.Player, error) {
	m, err := h.Repo.Players(ctx, tenant, ids, now)
	if err != nil {
		return nil, err
	}
	if _, ok := m[me]; !ok {
		return nil, problem.New(problem.Unavailable, "student not projected yet").Retry()
	}
	out := make([]mm.Player, 0, len(ids))
	for _, id := range ids {
		p, ok := m[id]
		if !ok {
			p = mm.Player{ID: id}
		}
		out = append(out, p)
	}
	return out, nil
}

// scope is H3 (F104-AC-03): the school of the students by default; the whole tenant when a unit tenant enabled
// cross-school play (cross_school_social_enabled, ADR-0054). Never two tenants in a queue.
func (h Matchmaking) scope(ctx context.Context, tenant uuid.UUID, ps []mm.Player) (mm.Scope, error) {
	t, err := h.Directory.Tenant(ctx, tenant)
	if err != nil {
		return mm.Scope{}, err
	}
	if t == nil {
		return mm.Scope{}, problem.New(problem.Unavailable, "tenant not projected yet").Retry()
	}
	if t.Type == "unit" && t.CrossSchool {
		return mm.Scope{Kind: mm.ScopeTenant, ID: tenant}, nil
	}
	for _, p := range ps[1:] {
		if p.SchoolID != ps[0].SchoolID {
			return mm.Scope{}, mm.PolicyError{Rule: mm.RuleCrossSchool}
		}
	}
	return mm.Scope{Kind: mm.ScopeSchool, ID: ps[0].SchoolID}, nil
}

// rating is the rating of a ticket: the student's, or μ_team/RD_team of a party (F104-AC-14).
func (h Matchmaking) rating(ctx context.Context, tenant uuid.UUID, ids []uuid.UUID, mode string, subject uuid.UUID, cfg mm.Config) (mm.Rating, error) {
	rs, err := h.ratings(ctx, tenant, ids, mode, subject, cfg)
	if err != nil {
		return mm.Rating{}, err
	}
	list := make([]mm.Rating, 0, len(ids))
	for _, id := range ids {
		list = append(list, rs[id])
	}
	return mm.TeamRating(list), nil
}

func (h Matchmaking) ratings(ctx context.Context, tenant uuid.UUID, ids []uuid.UUID, mode string, subject uuid.UUID, cfg mm.Config) (map[uuid.UUID]mm.Rating, error) {
	out := map[uuid.UUID]mm.Rating{}
	if h.Ratings != nil {
		rs, err := h.Ratings.Ratings(ctx, tenant, ids, mode, subject, cfg.DefaultRating)
		if err != nil {
			return nil, err
		}
		out = rs
	}
	for _, id := range ids {
		if _, ok := out[id]; !ok {
			out[id] = cfg.DefaultRating
		}
	}
	return out, nil
}

// ticketOf returns a ticket of the caller's tenant in which the caller takes part; anything else is 404 (F104-AC-24).
func (h Matchmaking) ticketOf(ctx context.Context, tenant, me, id uuid.UUID) (*port.Ticket, error) {
	t, err := h.Store.Ticket(ctx, id)
	if err != nil {
		return nil, err
	}
	if t == nil || t.TenantID != tenant || !t.Has(me) {
		return nil, problem.New(problem.NotFound, "")
	}
	return t, nil
}

// CancelTicket implements cancelMatchmakingTicket and WS matchmaking.ticket.cancel (F104-AC-10): the owner of a solo
// ticket or the leader of a team ticket; a proposed ticket declines its proposal. Cancelling an ended ticket is a
// no-op; a matched one is 409.
func (h Matchmaking) CancelTicket(ctx context.Context, id uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.CancelMatchmakingTicket")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return err
	}
	t, err := h.ticketOf(ctx, tenant, me, id)
	if err != nil {
		return err
	}
	if t.OwnerID != me {
		return problem.New(problem.PartyNotLeader, "")
	}
	now := h.now()
	switch t.Status {
	case mm.StatusCancelled, mm.StatusExpired:
		return nil
	case mm.StatusMatched:
		return problem.New(problem.Conflict, "ticket matched")
	case mm.StatusProposed:
		if t.ProposalID != nil {
			if err := h.decline(ctx, *t.ProposalID, me, mm.CancelUser, now); err == nil || !errors.Is(err, mm.ErrProposalClosed) {
				return mmError(err)
			}
		}
	}
	ok, err := h.Store.End(ctx, id, mm.StatusCancelled, mm.CancelUser, []string{mm.StatusQueued})
	if err != nil {
		return err
	}
	if !ok {
		cur, err := h.Store.Ticket(ctx, id)
		if err != nil {
			return err
		}
		if cur != nil && (cur.Status == mm.StatusMatched || cur.Status == mm.StatusProposed) { // match being created
			return problem.New(problem.Conflict, "ticket matched")
		}
		return nil
	}
	h.ended(ctx, *t, mm.StatusCancelled, mm.CancelUser, now)
	return nil
}

// ended pushes the end of a ticket to its students and records its wait.
func (h Matchmaking) ended(ctx context.Context, t port.Ticket, status, reason string, now time.Time) {
	t.Status, t.CancelReason = status, reason
	if h.Tracker != nil {
		h.Tracker.Forget(t.ID)
	}
	h.send(ctx, t.Students, rtTicketUpdated, view.Ticket{T: t, Now: now}.Updated())
	if h.Metrics != nil {
		h.Metrics.Wait(t.Mode, status, now.Sub(t.EnqueuedAt))
	}
}

// PartyChanged cancels the waiting team ticket of a party whose members changed (F104-AC-15): queued → cancelled
// (party_changed); proposed → its proposal is cancelled, the party ticket ends (party_changed) and the other tickets go
// back to their queue. Every student of the ticket is told. Returns whether a ticket was cancelled.
func (h Matchmaking) PartyChanged(ctx context.Context, tenant, partyID uuid.UUID) (bool, error) {
	id, err := h.Store.TicketOfParty(ctx, partyID)
	if err != nil || id == uuid.Nil {
		return false, err
	}
	t, err := h.Store.Ticket(ctx, id)
	if err != nil || t == nil || t.TenantID != tenant {
		return false, err
	}
	now := h.now()
	if t.Status == mm.StatusProposed && t.ProposalID != nil {
		p, err := h.Store.Proposal(ctx, *t.ProposalID)
		if err != nil {
			return false, err
		}
		if p != nil {
			res := p.Unavailable(map[uuid.UUID]bool{t.ID: true}, mm.CancelPartyChanged)
			if err := h.cancelProposal(ctx, *p, res, mm.ProposalUnavailable, now); err == nil {
				return true, nil
			} else if !errors.Is(err, mm.ErrProposalClosed) {
				return false, err
			}
		}
	}
	ok, err := h.Store.End(ctx, t.ID, mm.StatusCancelled, mm.CancelPartyChanged, []string{mm.StatusQueued, mm.StatusProposed})
	if err != nil || !ok {
		return false, err
	}
	h.ended(ctx, *t, mm.StatusCancelled, mm.CancelPartyChanged, now)
	return true, nil
}
