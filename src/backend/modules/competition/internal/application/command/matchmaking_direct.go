package command

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/view"
	mm "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/matchmaking"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// StartBotMatch implements startBotMatch (F104-AC-12, MATCHMAKING §9): the owner of a queued solo 1v1 ticket that
// waited mm_bot_after plays the practice bot "Robot Học Giỏi": the ticket is cancelled (user) and an unranked bot match
// of the same mode, subject and grade is created (match id derived from the ticket: a retry creates nothing more).
func (h Matchmaking) StartBotMatch(ctx context.Context, ticketID uuid.UUID) (res map[string]any, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.StartBotMatch")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return nil, err
	}
	t, err := h.ticketOf(ctx, tenant, me, ticketID)
	if err != nil {
		return nil, err
	}
	now := h.now()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		cfg, err := h.config(ctx, now)
		if err != nil {
			return err
		}
		if t.Status != mm.StatusQueued || t.Mode != mm.ModeBattle1v1 || t.PartyID != nil || t.OwnerID != me ||
			(!t.BotOffered && now.Sub(t.EnqueuedAt) < cfg.BotAfter()) {
			return problem.New(problem.Conflict, "no bot offer for this ticket")
		}
		matchID := uuid.NewSHA1(matchmakingNamespace, []byte("bot/"+t.ID.String()))
		school := uuid.Nil
		if len(t.Schools) > 0 {
			school = t.Schools[0]
		}
		r := t.Rating
		m := write.NewMatch{ID: matchID, Mode: t.Mode, Type: mm.TypeBot, SubjectID: t.SubjectID, TopicID: t.TopicID, Grade: t.Grade,
			Ranked: false, IsBot: true, ScopeKind: mm.ScopeSchool, TenantIDs: []uuid.UUID{tenant}, RulesetVersionID: cfg.VersionID, At: now,
			Participants: []write.MatchParticipant{{StudentID: me, TenantID: tenant, SchoolID: school, TeamNo: 0, Rating: &r}}}
		if err := h.createMatch(ctx, m); err != nil {
			return err
		}
		ended, err := h.Store.End(ctx, t.ID, mm.StatusCancelled, mm.CancelUser, []string{mm.StatusQueued})
		if err != nil {
			return err
		}
		if !ended { // proposed or ended meanwhile: no bot match (the transaction rolls back)
			return problem.New(problem.Conflict, "ticket is no longer waiting")
		}
		cards, err := h.Directory.Cards(ctx, tenant, []uuid.UUID{me})
		if err != nil {
			return err
		}
		res = view.Match(write.Match{ID: matchID, Mode: t.Mode, Type: mm.TypeBot, SubjectID: t.SubjectID, Grade: t.Grade, Status: "lobby",
			IsBot: true, Teams: map[uuid.UUID]int{me: 0}}, cards)
		if err := idempotency.SetResult(ctx, res); err != nil {
			return err
		}
		return h.later(ctx, func(ctx context.Context) {
			h.ended(ctx, *t, mm.StatusCancelled, mm.CancelUser, now)
			h.send(ctx, []uuid.UUID{me}, rtMatchAssigned, view.MatchAssigned(matchID, t.Mode))
			if h.Metrics != nil {
				h.Metrics.MatchCreated(t.Mode)
			}
		})
	})
	return res, mmError(err)
}

// DirectEntry is one side of a direct proposal: its students with their tenant and school.
type DirectEntry struct {
	Students []uuid.UUID
	Tenants  []uuid.UUID
	Schools  []uuid.UUID
	PartyID  *uuid.UUID
}

// DirectInput is a match of a tournament bracket (F107, F104-AC-21).
type DirectInput struct {
	TournamentNodeID uuid.UUID
	TournamentID     *uuid.UUID
	// Attempt numbers the ready checks of a node (reschedule, team re-proposal): attempt 0 keeps the F104 id formula.
	Attempt int
	// ReadyCheck overrides the ready-check deadline of the configuration (F107: the no-show limit of tournament_timers).
	ReadyCheck time.Duration
	Mode       string
	SubjectID  uuid.UUID
	TopicID    *uuid.UUID
	Grade      int    // lowest grade of the match
	ScopeKind  string // school, tenant or platform (platform events only, ADR-0054)
	Entries    [2]DirectEntry
}

// CreateDirectProposal creates a proposal without queue for a tournament match (F104-AC-21): same ready check, match
// created idempotently by proposal_id with tournament_node_id. Called in process by module competition (F107).
// The proposal id is derived from the bracket node so asking twice yields the same proposal.
func (h Matchmaking) CreateDirectProposal(ctx context.Context, in DirectInput) (id uuid.UUID, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.CreateDirectProposal")
	defer func() { done(err) }()
	if in.TournamentNodeID == uuid.Nil || in.SubjectID == uuid.Nil || in.Grade < 1 || in.Grade > 9 ||
		len(in.Entries[0].Students) == 0 || len(in.Entries[1].Students) == 0 {
		return uuid.Nil, mm.ValidationError{Field: "entries", Message: "two entries, a subject and a grade are required"}
	}
	if in.ScopeKind != mm.ScopeSchool && in.ScopeKind != mm.ScopeTenant && in.ScopeKind != mm.ScopePlatform {
		return uuid.Nil, mm.ValidationError{Field: "scope_kind", Message: "unknown scope"}
	}
	now := h.now()
	cfg, err := h.platformConfig(ctx, now)
	if err != nil {
		return uuid.Nil, err
	}
	id = DirectProposalID(in.TournamentNodeID, in.Attempt)
	node := in.TournamentNodeID
	ready := cfg.ReadyCheck()
	if in.ReadyCheck > 0 {
		ready = in.ReadyCheck
	}
	p := port.Proposal{Proposal: mm.Proposal{ID: id, Accepted: map[uuid.UUID]bool{}, Deadline: now.Add(ready), Status: mm.ProposalOpen},
		Mode: in.Mode, Type: mm.TypeTournament, ScopeKind: in.ScopeKind, SubjectID: in.SubjectID, TopicID: in.TopicID, Grade: in.Grade,
		Ranked: false, RulesetVersionID: cfg.VersionID, TournamentNodeID: &node, TournamentID: in.TournamentID, Direct: true, Schools: map[uuid.UUID]uuid.UUID{},
		Tenants: map[uuid.UUID]uuid.UUID{}, Parties: map[uuid.UUID]uuid.UUID{}, Ratings: map[uuid.UUID]mm.Rating{}, CreatedAt: now}
	seen := map[uuid.UUID]bool{}
	for side, e := range in.Entries {
		if len(e.Tenants) != len(e.Students) || len(e.Schools) != len(e.Students) {
			return uuid.Nil, mm.ValidationError{Field: "entries", Message: "tenant and school of every student"}
		}
		entry := uuid.NewSHA1(matchmakingNamespace, []byte("entry/"+id.String()+"/"+string(rune('0'+side))))
		p.Tickets = append(p.Tickets, mm.ProposalTicket{TicketID: entry, Students: e.Students})
		p.Teams[side] = append(p.Teams[side], e.Students...)
		for i, s := range e.Students {
			p.Tenants[s], p.Schools[s], p.Ratings[s] = e.Tenants[i], e.Schools[i], cfg.DefaultRating
			if e.PartyID != nil {
				p.Parties[s] = *e.PartyID
			}
			if !seen[e.Tenants[i]] {
				seen[e.Tenants[i]] = true
				p.TenantIDs = append(p.TenantIDs, e.Tenants[i])
			}
		}
	}
	if in.ScopeKind != mm.ScopePlatform && len(p.TenantIDs) != 1 {
		return uuid.Nil, mm.ValidationError{Field: "scope_kind", Message: "a tenant or school match has one tenant"}
	}
	ok, err := h.Store.Propose(ctx, p)
	if err != nil {
		return uuid.Nil, err
	}
	if !ok {
		return id, nil // already proposed
	}
	var cards map[uuid.UUID]port.Card
	if len(p.TenantIDs) == 1 {
		sc, err := scopeOf(p)
		if err != nil {
			return uuid.Nil, err
		}
		if err := h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
			var err error
			cards, err = h.Directory.Cards(ctx, p.TenantIDs[0], p.Students())
			return err
		}); err != nil {
			return uuid.Nil, err
		}
	}
	for _, t := range p.Tickets {
		h.send(ctx, t.Students, rtProposalCreated, view.ProposalCreated(p, t.TicketID, cards))
	}
	return id, nil
}

// DirectProposalID is the proposal id (= match id) of attempt of a tournament node: deterministic, so asking twice for
// the same attempt yields the same proposal and the same match (F104-AC-21, F107 edge case "hai worker").
func DirectProposalID(node uuid.UUID, attempt int) uuid.UUID {
	if attempt <= 0 {
		return uuid.NewSHA1(matchmakingNamespace, []byte("tournament/"+node.String()))
	}
	return uuid.NewSHA1(matchmakingNamespace, []byte("tournament/"+node.String()+"/"+strconv.Itoa(attempt)))
}

// DirectAcceptance returns who accepted a direct proposal (found false once its hash expired from Valkey: one minute
// after its deadline) and whether it is still open.
func (h Matchmaking) DirectAcceptance(ctx context.Context, id uuid.UUID) (accepted map[uuid.UUID]bool, open, found bool, err error) {
	p, err := h.Store.Proposal(ctx, id)
	if err != nil || p == nil {
		return nil, false, false, err
	}
	return p.Accepted, p.Status == mm.ProposalOpen || p.Status == mm.ProposalAccepted, true, nil
}

// ChallengeMatchInput is one Timed Quiz room of a grade challenge (F107-AC-27): at most 40 students of one school.
type ChallengeMatchInput struct {
	MatchID   uuid.UUID
	Tenant    uuid.UUID
	School    uuid.UUID
	SubjectID uuid.UUID
	Grade     int
	Students  []uuid.UUID
}

// CreateChallengeMatch creates the timed_quiz match of a grade challenge room in the transaction of ctx (unranked, scope
// school, match id given by the challenge so a retry creates nothing more); the battle engine opens its room on
// match.created and the students enter it with battle.state.request (F105 T034). The participants get
// matchmaking.match.assigned after the commit.
func (h Matchmaking) CreateChallengeMatch(ctx context.Context, in ChallengeMatchInput) (err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.CreateChallengeMatch")
	defer func() { done(err) }()
	if len(in.Students) == 0 || len(in.Students) > 40 || in.Grade < 1 || in.Grade > 9 {
		return mm.ValidationError{Field: "students", Message: "1 to 40 students of one grade band"}
	}
	now := h.now()
	cfg, err := h.config(ctx, now)
	if err != nil {
		return err
	}
	m := write.NewMatch{ID: in.MatchID, Mode: ModeTimedQuiz, Type: mm.TypeGrade, SubjectID: in.SubjectID, Grade: in.Grade, Ranked: false,
		ScopeKind: mm.ScopeSchool, TenantIDs: []uuid.UUID{in.Tenant}, RulesetVersionID: cfg.VersionID, At: now}
	for _, s := range in.Students {
		m.Participants = append(m.Participants, write.MatchParticipant{StudentID: s, TenantID: in.Tenant, SchoolID: in.School, TeamNo: 0})
	}
	if err := h.createMatch(ctx, m); err != nil {
		return err
	}
	students := append([]uuid.UUID(nil), in.Students...)
	return h.later(ctx, func(ctx context.Context) {
		h.send(ctx, students, rtMatchAssigned, view.MatchAssigned(in.MatchID, ModeTimedQuiz))
	})
}

// ModeTimedQuiz is the mode of the rooms of a grade challenge.
const ModeTimedQuiz = "timed_quiz"
