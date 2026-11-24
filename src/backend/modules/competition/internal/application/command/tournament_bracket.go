package command

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/view"
	mm "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/matchmaking"
	trn "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/tournament"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/tournament/bracket"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// timersOf returns tournament_timers of the pinned template version of t (platform defaults for a missing value).
func (h Tournaments) timersOf(ctx context.Context, t *write.TournamentRow) (trn.Timers, error) {
	tpl, err := h.Repo.TemplateVersion(ctx, t.RuleVersionID)
	if err != nil {
		return h.Timers, err
	}
	if tpl == nil {
		return h.Timers, nil
	}
	return tpl.TimersOr(h.Timers), nil
}

// bracketState is a loaded bracket: the tournament, its entries by id, its groups and its graph.
type bracketState struct {
	t       *write.TournamentRow
	entries map[uuid.UUID]*write.EntryRow
	order   []uuid.UUID // entries in seed order
	groups  []write.GroupRow
	g       *trn.Graph
	added   map[uuid.UUID]bool // nodes created in this change set
	touched map[uuid.UUID]bool // entries changed in this change set
	timers  trn.Timers
}

func (h Tournaments) loadBracket(ctx context.Context, t *write.TournamentRow) (*bracketState, error) {
	entries, err := h.Repo.Entries(ctx, t.Tenant, t.ID)
	if err != nil {
		return nil, err
	}
	nodes, err := h.Repo.Nodes(ctx, t.Tenant, t.ID, true)
	if err != nil {
		return nil, err
	}
	groups, err := h.Repo.Groups(ctx, t.Tenant, t.ID)
	if err != nil {
		return nil, err
	}
	timers, err := h.timersOf(ctx, t)
	if err != nil {
		return nil, err
	}
	s := &bracketState{t: t, entries: map[uuid.UUID]*write.EntryRow{}, groups: groups, g: trn.NewGraph(nodes), added: map[uuid.UUID]bool{},
		touched: map[uuid.UUID]bool{}, timers: timers}
	for i := range entries {
		e := &entries[i]
		s.entries[e.ID] = e
		if e.Seed != nil {
			s.order = append(s.order, e.ID)
		}
	}
	sort.Slice(s.order, func(i, j int) bool { return *s.entries[s.order[i]].Seed < *s.entries[s.order[j]].Seed })
	return s, nil
}

// save writes the nodes created or changed, the entries changed and the group standings (one transaction).
func (h Tournaments) save(ctx context.Context, s *bracketState) error {
	for _, n := range s.g.Nodes {
		switch {
		case s.added[n.ID]:
			if err := h.Repo.InsertNode(ctx, s.t.Tenant, s.t.ID, n); err != nil {
				return err
			}
		case n.Changed:
			if err := h.Repo.UpdateNode(ctx, s.t.Tenant, n); err != nil {
				return err
			}
			n.Version++
		}
		n.Changed = false
	}
	s.added = map[uuid.UUID]bool{}
	for id := range s.touched {
		if err := h.Repo.UpdateEntry(ctx, *s.entries[id]); err != nil {
			return err
		}
		s.entries[id].Version++
	}
	s.touched = map[uuid.UUID]bool{}
	for i := range s.groups {
		gr := &s.groups[i]
		var members []uuid.UUID
		for _, e := range s.order {
			if g := s.entries[e].Group; g != nil && *g == gr.ID {
				members = append(members, e)
			}
		}
		var nodes []*trn.Node
		for _, n := range s.g.Nodes {
			if n.GroupID != nil && *n.GroupID == gr.ID {
				nodes = append(nodes, n)
			}
		}
		gr.Standings = trn.Standings(members, nodes)
		if err := h.Repo.UpdateGroupStandings(ctx, *gr); err != nil {
			return err
		}
	}
	return nil
}

func (s *bracketState) setEntry(id uuid.UUID, status string) {
	if e := s.entries[id]; e != nil && e.Status != status {
		e.Status = status
		s.touched[id] = true
	}
}

// eliminate marks the losers of the decided knockout nodes without a losers-bracket next node.
func (s *bracketState) eliminate(n *trn.Node) {
	if !n.Elimination() || n.NextLose != nil {
		return
	}
	switch {
	case n.Status == trn.NodeVoid:
		for _, e := range []*uuid.UUID{n.A, n.B} {
			if e != nil {
				s.setEntry(*e, trn.EntryEliminated)
			}
		}
	case n.Loser() != nil:
		s.setEntry(*n.Loser(), trn.EntryEliminated)
	}
}

func randomSeed() int64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UnixNano()
	}
	return int64(binary.BigEndian.Uint64(b[:]) >> 1)
}

// contenders reads the ratings of the entries for a rating seeding (F107-AC-13): μ of the active season (the average of
// the members of a team), provisional entries after the rated ones (RD above the provisional limit of the Ranking Rules).
func (h Tournaments) contenders(ctx context.Context, t *write.TournamentRow, entries []*write.EntryRow) ([]bracket.Contender, error) {
	out := make([]bracket.Contender, 0, len(entries))
	if t.SeedingPolicy != bracket.SeedRating || h.Ratings == nil {
		for _, e := range entries {
			out = append(out, bracket.Contender{ID: e.ID})
		}
		return out, nil
	}
	def := mm.Rating{Mu: 1500, RD: 350}
	maxRD := 110.0
	if h.Rules != nil {
		rules, _, err := h.Rules.Rules(ctx, h.now())
		if err != nil {
			return nil, err
		}
		def, maxRD = mm.Rating{Mu: rules.Glicko2.R0, RD: rules.Glicko2.RD0}, rules.Provisional.MaxRD
	}
	var students []uuid.UUID
	for _, e := range entries {
		students = append(students, e.Members...)
	}
	mode := t.Mode
	ratings, err := h.Ratings.Ratings(ctx, t.Tenant, students, mode, t.SubjectID, def)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		c := bracket.Contender{ID: e.ID}
		for _, m := range e.Members {
			r := ratings[m]
			c.Mu += r.Mu / float64(len(e.Members))
			if r.RD > maxRD {
				c.Provisional = true
			}
		}
		out = append(out, c)
	}
	return out, nil
}

// GenerateTournamentBracket implements generateTournamentBracket (F107-AC-11…AC-14, AC-22, AC-23): from seeding, the
// confirmed entries seeded by the policy (rating, random with a stored seed, manual order), the bracket of the format
// with its groups and schedule (rounds round_gap apart, inside the access hours of the school) written in one
// transaction; the tournament becomes scheduled. Below the minimum → 422 VALIDATION_FAILED; already scheduled → 422
// INVALID_STATE_TRANSITION; idempotent (Idempotency-Key); audit tournament.bracket_generated.
func (h Tournaments) GenerateTournamentBracket(ctx context.Context, id uuid.UUID, order []uuid.UUID) (out map[string]any, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.GenerateBracket")
	defer func() { done(err) }()
	err = h.organizerTournament(ctx, id, func(ctx context.Context, p authz.Principal, t *write.TournamentRow) error {
		if err := trn.Move(t.Status, trn.StatusScheduled); err != nil {
			return err
		}
		all, err := h.Repo.Entries(ctx, t.Tenant, t.ID)
		if err != nil {
			return err
		}
		var confirmed []*write.EntryRow
		for i := range all {
			if all[i].Status == trn.EntryConfirmed || all[i].Status == trn.EntryCheckedIn {
				confirmed = append(confirmed, &all[i])
			}
		}
		if len(confirmed) < max(t.MinEntries, trn.MinEntries(t.Format)) {
			return bracket.ErrTooFew
		}
		cs, err := h.contenders(ctx, t, confirmed)
		if err != nil {
			return err
		}
		if t.SeedingPolicy == bracket.SeedRandom && t.SeedValue == nil {
			v := randomSeed()
			t.SeedValue = &v
		}
		if t.SeedingPolicy == bracket.SeedManual && len(order) == 0 {
			return problem.Invalid("entry_order", problem.SubRequired, "is required by a manual seeding")
		}
		var seedValue int64
		if t.SeedValue != nil {
			seedValue = *t.SeedValue
		}
		seeds, err := bracket.Order(t.SeedingPolicy, cs, seedValue, order)
		if err != nil {
			return err
		}
		gen, groups, err := trn.Generate(t.Format, len(seeds))
		if err != nil {
			return err
		}
		g := trn.Build(t.ID, gen, seeds, t.StartsAt, t.RoundGap)
		if t.School != nil {
			var at []time.Time
			for _, n := range g.Nodes {
				if n.ScheduledAt != nil {
					at = append(at, *n.ScheduledAt)
				}
			}
			open, err := h.Repo.SchoolOpen(ctx, t.Tenant, *t.School, at)
			if err != nil {
				return err
			}
			if !open {
				return problem.Invalid("starts_at", problem.SubInvalid, "a round falls outside the access hours of the school")
			}
		}
		byID := map[uuid.UUID]*write.EntryRow{}
		for _, e := range confirmed {
			byID[e.ID] = e
		}
		codes := make([]string, 0, len(groups))
		for code := range groups {
			codes = append(codes, code)
		}
		sort.Strings(codes)
		for _, code := range codes {
			gr := write.GroupRow{ID: trn.GroupID(t.ID, code), Tournament: t.ID, Tenant: t.Tenant, Code: code}
			if err := h.Repo.InsertGroup(ctx, gr); err != nil {
				return err
			}
			for _, sd := range groups[code] {
				gid := gr.ID
				byID[seeds[sd-1]].Group = &gid
			}
		}
		for i, eid := range seeds {
			e := byID[eid]
			sd := i + 1
			e.Seed = &sd
			if err := h.Repo.UpdateEntry(ctx, *e); err != nil {
				return err
			}
			e.Version++
		}
		for _, n := range g.Nodes {
			if err := h.Repo.InsertNode(ctx, t.Tenant, t.ID, n); err != nil {
				return err
			}
			n.Changed = false
		}
		t.Status = trn.StatusScheduled
		if err := h.Repo.UpdateTournament(ctx, *t); err != nil {
			return err
		}
		t.Version++
		if err := h.Audit.Record(ctx, audit.Event{Action: AuditBracketGenerated, TargetType: "tournament", TargetID: t.ID.String(),
			Result: audit.ResultSuccess, SchoolID: schoolOf(t), Metadata: map[string]any{"format": t.Format, "entries": len(seeds),
				"nodes": len(g.Nodes), "seeding_policy": t.SeedingPolicy, "seed_value": t.SeedValue}}); err != nil {
			return err
		}
		s, err := h.loadBracket(ctx, t)
		if err != nil {
			return err
		}
		out = bracketView(s, nil, false)
		return idempotency.SetResult(ctx, out)
	})
	return out, trnError(err)
}

// bracketView renders a loaded bracket (Bracket of the contract).
func bracketView(s *bracketState, cards map[uuid.UUID]port.Card, student bool) map[string]any {
	codes := map[uuid.UUID]string{}
	for _, gr := range s.groups {
		codes[gr.ID] = gr.Code
	}
	entries := make([]map[string]any, 0, len(s.entries))
	ids := make([]uuid.UUID, 0, len(s.entries))
	for id := range s.entries {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := s.entries[ids[i]], s.entries[ids[j]]
		if (a.Seed == nil) != (b.Seed == nil) {
			return a.Seed != nil
		}
		if a.Seed != nil && *a.Seed != *b.Seed {
			return *a.Seed < *b.Seed
		}
		return ids[i].String() < ids[j].String()
	})
	for _, id := range ids {
		e := s.entries[id]
		code := ""
		if e.Group != nil {
			code = codes[*e.Group]
		}
		entries = append(entries, view.Entry(*e, cards, code, nil))
	}
	nodes := make([]map[string]any, 0, len(s.g.Nodes))
	sorted := append([]*trn.Node(nil), s.g.Nodes...)
	sort.Slice(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if a.Wave != b.Wave {
			return a.Wave < b.Wave
		}
		if a.Bracket != b.Bracket {
			return a.Bracket < b.Bracket
		}
		if a.Round != b.Round {
			return a.Round < b.Round
		}
		return a.Position < b.Position
	})
	for _, n := range sorted {
		code := ""
		if n.GroupID != nil {
			code = codes[*n.GroupID]
		}
		nodes = append(nodes, view.Node(n, code))
	}
	groups := make([]map[string]any, 0, len(s.groups))
	for _, gr := range s.groups {
		groups = append(groups, view.Group(gr.Code, gr.Standings))
	}
	out := map[string]any{"tournament_id": s.t.ID, "format": s.t.Format, "status": s.t.Status, "entries": entries, "nodes": nodes, "groups": groups}
	if !student {
		out["seed_value"] = s.t.SeedValue
	}
	return out
}

// withdrawFromBracket takes withdrawn entries out of the nodes not started (bye for their opponents, F107-AC-10).
func (h Tournaments) withdrawFromBracket(ctx context.Context, t *write.TournamentRow, gone map[uuid.UUID]bool) error {
	s, err := h.loadBracket(ctx, t)
	if err != nil {
		return err
	}
	s.g.Withdraw(gone)
	return h.save(ctx, s)
}

// MatchOutcome is what the bracket needs of match.completed / match.aborted.
type MatchOutcome struct {
	MatchID uuid.UUID
	Status  string // completed, cancelled
	Lines   []MatchLine
	EndedAt time.Time
}

// outcomeOf decides the node outcome of a completed tournament match (F107-AC-17): team 0 is entry A, team 1 entry B; a
// side with a win wins; a draw in a knockout goes to the side with more correct answers, then to the better seed; nobody
// winning or drawing (everyone left, forfeits on both sides) is a double forfeit.
func (s *bracketState) outcomeOf(n *trn.Node, m MatchOutcome) trn.Outcome {
	var side [2]trn.Side
	won, drew := [2]bool{}, false
	forfeit := [2]bool{true, true}
	for _, l := range m.Lines {
		if l.Team < 0 || l.Team > 1 {
			continue
		}
		side[l.Team].Correct += l.Correct
		switch l.Outcome {
		case "win":
			won[l.Team] = true
		case "draw":
			drew = true
		}
		if l.Outcome != "forfeit" && l.Outcome != "abandoned" {
			forfeit[l.Team] = false
		}
	}
	res := trn.Result{A: side[0], B: side[1], Played: true}
	for i, f := range forfeit {
		if f {
			res.Forfeit = map[int]string{0: "a", 1: "b"}[i]
		}
	}
	switch {
	case m.Status != "completed" || (!won[0] && !won[1] && !drew):
		res.DoubleForfeit, res.Played = true, false
		return trn.Outcome{DoubleForfeit: true, Result: res}
	case won[0] && !won[1]:
		res.A.Outcome, res.B.Outcome = "win", "loss"
		return trn.Outcome{Winner: n.A, Result: res}
	case won[1] && !won[0]:
		res.A.Outcome, res.B.Outcome = "loss", "win"
		return trn.Outcome{Winner: n.B, Result: res}
	}
	res.A.Outcome, res.B.Outcome = "draw", "draw"
	if !n.Elimination() {
		return trn.Outcome{Draw: true, Result: res}
	}
	w := n.A
	switch {
	case side[1].Correct > side[0].Correct:
		w = n.B
	case side[1].Correct == side[0].Correct && s.seedOf(n.B) < s.seedOf(n.A):
		w = n.B
	}
	return trn.Outcome{Winner: w, Result: res}
}

func (s *bracketState) seedOf(e *uuid.UUID) int {
	if e == nil || s.entries[*e] == nil || s.entries[*e].Seed == nil {
		return 1 << 20
	}
	return *s.entries[*e].Seed
}

// AdvanceOnMatch is the consumer side of match.completed for tournaments (F107-AC-17): the result of the match of a node
// moves the winner (and the loser of a double elimination) on at once, idempotently by match id (a replay or the result
// of an older attempt changes nothing); the complaint window starts; the tournament completes after its last node.
func (h Tournaments) AdvanceOnMatch(ctx context.Context, tenant uuid.UUID, m MatchOutcome) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.AdvanceOnMatch")
	defer func() { done(err) }()
	tid, nid, err := h.Repo.NodeOfMatch(ctx, tenant, m.MatchID)
	if err != nil || tid == nil {
		return 0, err
	}
	t, err := h.Repo.Tournament(ctx, tenant, *tid, true)
	if err != nil || t == nil || t.Status != trn.StatusInProgress {
		return 0, err
	}
	s, err := h.loadBracket(ctx, t)
	if err != nil {
		return 0, err
	}
	node := s.g.Node(*nid)
	if node == nil || node.MatchID == nil || *node.MatchID != m.MatchID || node.Status != trn.NodeInMatch {
		return 0, nil // replayed, or an older attempt of the node
	}
	o := s.outcomeOf(node, m)
	if err := s.g.Apply(node, o, h.now().Add(s.timers.Complaint)); err != nil {
		return 0, err
	}
	s.eliminate(node)
	if err := h.settle(ctx, s); err != nil {
		return 0, err
	}
	return 1, h.later(ctx, func(ctx context.Context) { h.pushNode(ctx, s, node) })
}

// MatchAborted is the consumer side of match.aborted for tournaments (F107-AC-19): the node is played again
// tournament_timers.reschedule later (a new ready check), at most reschedule_max times; then it waits for the organizer.
func (h Tournaments) MatchAborted(ctx context.Context, tenant, matchID uuid.UUID) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.TournamentMatchAborted")
	defer func() { done(err) }()
	tid, nid, err := h.Repo.NodeOfMatch(ctx, tenant, matchID)
	if err != nil || tid == nil {
		return 0, err
	}
	t, err := h.Repo.Tournament(ctx, tenant, *tid, true)
	if err != nil || t == nil || t.Status != trn.StatusInProgress {
		return 0, err
	}
	s, err := h.loadBracket(ctx, t)
	if err != nil {
		return 0, err
	}
	node := s.g.Node(*nid)
	if node == nil || node.MatchID == nil || *node.MatchID != matchID || node.Status != trn.NodeInMatch {
		return 0, nil
	}
	h.reschedule(s, node, "server_failure")
	return 1, h.save(ctx, s)
}

// reschedule plays a node again after a technical failure, or asks the organizer after the last reschedule.
func (h Tournaments) reschedule(s *bracketState, node *trn.Node, reason string) {
	if node.Reschedules < s.timers.RescheduleMax {
		at := h.now().Add(s.timers.Reschedule)
		node.Reschedules++
		node.Attempts++
		node.Status, node.ScheduledAt, node.MatchID, node.StartedAt, node.Notices, node.Changed = trn.NodeScheduled, &at, nil, nil, 3, true
		return
	}
	node.RequireDecision()
	if h.Metrics != nil {
		h.Metrics.Forfeit("decision_" + reason)
	}
}

// settle runs after a node changed: the knockout of a finished group stage is created, the tournament completes after
// its last node (results, rewards), and everything is saved.
func (h Tournaments) settle(ctx context.Context, s *bracketState) error {
	if s.t.Format == trn.FormatGroups && !s.hasKnockout() && s.g.Complete(func(n *trn.Node) bool { return n.Bracket == trn.BracketGroup }) {
		if err := h.knockout(ctx, s); err != nil {
			return err
		}
	}
	if err := h.save(ctx, s); err != nil {
		return err
	}
	if (s.t.Format != trn.FormatGroups || s.hasKnockout()) && s.g.Complete(nil) {
		return h.complete(ctx, s)
	}
	return nil
}

func (s *bracketState) hasKnockout() bool {
	for _, n := range s.g.Nodes {
		if n.Bracket != trn.BracketGroup {
			return true
		}
	}
	return false
}

// knockout creates the knockout of the top 2 of every group (F107-AC-23), the next rounds after the group stage.
func (h Tournaments) knockout(ctx context.Context, s *bracketState) error {
	sort.Slice(s.groups, func(i, j int) bool { return s.groups[i].Code < s.groups[j].Code })
	var winners, runners []uuid.UUID
	lastWave := 0
	for _, n := range s.g.Nodes {
		lastWave = max(lastWave, n.Wave)
	}
	for _, gr := range s.groups {
		var members []uuid.UUID
		for _, e := range s.order {
			if g := s.entries[e].Group; g != nil && *g == gr.ID && s.entries[e].Status != trn.EntryWithdrawn {
				members = append(members, e)
			}
		}
		var nodes []*trn.Node
		for _, n := range s.g.Nodes {
			if n.GroupID != nil && *n.GroupID == gr.ID {
				nodes = append(nodes, n)
			}
		}
		rows := trn.Standings(members, nodes)
		if len(rows) >= 1 {
			winners = append(winners, rows[0].Entry)
		}
		if len(rows) >= 2 {
			runners = append(runners, rows[1].Entry)
		}
		for _, r := range rows[min(2, len(rows)):] {
			s.setEntry(r.Entry, trn.EntryEliminated)
		}
	}
	seeds := bracket.KnockoutSeeds(winners, runners)
	gen, err := bracket.Knockout(len(seeds), lastWave+1)
	if err != nil {
		return err
	}
	start := s.t.StartsAt
	if earliest := h.now().Add(s.t.RoundGap).Add(-time.Duration(lastWave+1) * s.t.RoundGap); earliest.After(start) {
		start = earliest
	}
	ko := trn.Build(s.t.ID, gen, seeds, start, s.t.RoundGap)
	for _, n := range ko.Nodes {
		s.g.Add(n)
		s.added[n.ID] = true
	}
	s.g.Resolve()
	return nil
}

// complete writes the final ranks (tournament_results, F107-AC-17, AC-21), marks the champion, publishes
// tournament.completed, then grants the rewards (F107-AC-20).
func (h Tournaments) complete(ctx context.Context, s *bracketState) error {
	t := s.t
	now := h.now()
	groups := map[uuid.UUID][]uuid.UUID{}
	for _, e := range s.order {
		if g := s.entries[e].Group; g != nil {
			groups[*g] = append(groups[*g], e)
		}
	}
	placements := trn.Ranks(t.Format, s.order, s.g.Nodes, groups)
	results := make([]map[string]any, 0, len(placements))
	for _, p := range placements {
		e := s.entries[p.Entry]
		if _, err := h.Repo.InsertResult(ctx, write.ResultRow{Tournament: t.ID, Tenant: t.Tenant, Entry: p.Entry, School: e.School,
			Placement: p}); err != nil {
			return err
		}
		if p.Rank == 1 {
			s.setEntry(p.Entry, trn.EntryChampion)
		}
		results = append(results, map[string]any{"entry_id": p.Entry, "final_rank": p.Rank, "student_ids": e.Members})
	}
	if err := h.save(ctx, s); err != nil {
		return err
	}
	if err := trn.Move(t.Status, trn.StatusCompleted); err != nil {
		return err
	}
	t.Status, t.CompletedAt = trn.StatusCompleted, &now
	if err := h.Repo.UpdateTournament(ctx, *t); err != nil {
		return err
	}
	t.Version++
	tenant := t.Tenant
	if err := h.Events.Publish(ctx, write.Event{Type: EventTournamentCompleted, SubjectType: "tournament", SubjectID: t.ID.String(),
		PartitionKey: t.ID.String(), TenantID: &tenant, OccurredAt: now, Payload: map[string]any{"tournament_id": t.ID,
			"challenge_id": t.ChallengeID, "school_id": t.School, "format": t.Format, "completed_at": now, "results": results}}); err != nil {
		return err
	}
	if err := h.Audit.Record(ctx, audit.Event{Action: AuditTournamentCompleted, TargetType: "tournament", TargetID: t.ID.String(),
		Result: audit.ResultSuccess, SchoolID: schoolOf(t), Metadata: map[string]any{"entries": len(placements)}}); err != nil {
		return err
	}
	return h.awardRewards(ctx, t, placements, s.entries)
}

// awardRewards grants the reward rules of the pinned template (F107-AC-20): one tournament_rewards row per (tournament,
// entry, rule) — granted at most once, tournament.rewarded for module game —, the Competition Points of the entry
// (source_type tournament, one ledger row per student and tournament), then the tournament is rewarded. Rewards are
// cosmetic and never touch learning scores (R19).
func (h Tournaments) awardRewards(ctx context.Context, t *write.TournamentRow, placements []trn.Placement, entries map[uuid.UUID]*write.EntryRow) error {
	now := h.now()
	tenant := t.Tenant
	var season *write.SeasonRow
	if h.Points != nil {
		var err error
		if season, err = h.Points.SeasonAt(ctx, now); err != nil {
			return err
		}
	}
	var students []uuid.UUID
	for _, e := range entries {
		students = append(students, e.Members...)
	}
	info, err := h.Repo.Students(ctx, tenant, students, now)
	if err != nil {
		return err
	}
	for _, p := range placements {
		e := entries[p.Entry]
		total := 0
		for _, rule := range t.RewardRules {
			if !rule.Applies(p.Rank, p.Played) {
				continue
			}
			total += rule.Points
			ref := rule.RewardRef
			if ref == nil {
				ref = map[string]string{}
			}
			ok, err := h.Repo.InsertReward(ctx, write.RewardRow{Tournament: t.ID, Tenant: tenant, Entry: p.Entry, RuleID: rule.RuleID,
				Students: e.Members, RewardRef: ref, Points: rule.Points, At: now})
			if err != nil {
				return err
			}
			if !ok {
				continue // granted before (F107-AC-20: never twice)
			}
			if err := h.Events.Publish(ctx, write.Event{Type: EventTournamentRewarded, SubjectType: "tournament", SubjectID: t.ID.String(),
				PartitionKey: t.ID.String(), TenantID: &tenant, OccurredAt: now, Payload: map[string]any{"tournament_id": t.ID,
					"entry_id": p.Entry, "rule_id": rule.RuleID, "final_rank": p.Rank, "student_ids": e.Members, "reward_ref": ref,
					"points": rule.Points}}); err != nil {
				return err
			}
		}
		if total > 0 && season != nil {
			if err := h.tournamentPoints(ctx, t, season, e, info, min(total, 1000), now); err != nil {
				return err
			}
		}
	}
	if err := trn.Move(t.Status, trn.StatusRewarded); err != nil {
		return err
	}
	t.Status, t.RewardedAt = trn.StatusRewarded, &now
	if err := h.Repo.UpdateTournament(ctx, *t); err != nil {
		return err
	}
	t.Version++
	return h.Audit.Record(ctx, audit.Event{Action: AuditTournamentRewarded, TargetType: "tournament", TargetID: t.ID.String(),
		Result: audit.ResultSuccess, SchoolID: schoolOf(t), Metadata: map[string]any{"entries": len(placements)}})
}

func (h Tournaments) tournamentPoints(ctx context.Context, t *write.TournamentRow, season *write.SeasonRow, e *write.EntryRow,
	info map[uuid.UUID]write.StudentInfo, amount int, now time.Time) error {
	tenant := t.Tenant
	subject := t.SubjectID
	mode := t.Mode
	for _, m := range e.Members {
		s, ok := info[m]
		if !ok || s.Grade < 1 {
			continue
		}
		pe := write.PointEntry{Tenant: tenant, School: s.School, Student: m, Grade: s.Grade, Season: season.ID, Subject: &subject, Mode: &mode,
			SourceType: "tournament", SourceID: t.ID, RuleVersion: t.RuleVersionID, Day: now, At: now}
		pe.Award.Amount, pe.Award.Reason, pe.Award.Breakdown = amount, "tournament", map[string]int{"tournament": amount}
		ok, err := h.Points.InsertPoints(ctx, pe)
		if err != nil || !ok {
			return err
		}
		balance, err := h.Points.SeasonPoints(ctx, tenant, m, season.ID)
		if err != nil {
			return err
		}
		if err := h.Events.Publish(ctx, write.Event{Type: EventPointsAwarded, SubjectType: "student", SubjectID: m.String(),
			PartitionKey: m.String(), TenantID: &tenant, OccurredAt: now, Payload: map[string]any{"student_id": m, "school_id": s.School,
				"season_id": season.ID, "source_type": "tournament", "source_id": t.ID, "mode": mode, "subject_id": subject, "points": amount,
				"balance_after": balance, "capped": false, "day_key": now.Format("2006-01-02"), "rule_version_id": t.RuleVersionID}}); err != nil {
			return err
		}
	}
	return nil
}

// AwardRewards grants the rewards of a completed tournament again (job tournament-scheduler; a second run grants
// nothing more, F107-AC-20).
func (h Tournaments) AwardRewards(ctx context.Context, tenant, id uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.AwardRewards")
	defer func() { done(err) }()
	t, err := h.Repo.Tournament(ctx, tenant, id, true)
	if err != nil || t == nil || t.Status != trn.StatusCompleted {
		return err
	}
	s, err := h.loadBracket(ctx, t)
	if err != nil {
		return err
	}
	groups := map[uuid.UUID][]uuid.UUID{}
	for _, e := range s.order {
		if g := s.entries[e].Group; g != nil {
			groups[*g] = append(groups[*g], e)
		}
	}
	return h.awardRewards(ctx, t, trn.Ranks(t.Format, s.order, s.g.Nodes, groups), s.entries)
}

// organizerNode loads a node of a tournament of the organizer for confirm/override.
func (h Tournaments) organizerNode(ctx context.Context, id, nodeID uuid.UUID, fn func(ctx context.Context, p authz.Principal, s *bracketState, n *trn.Node) error) error {
	return h.organizerTournament(ctx, id, func(ctx context.Context, p authz.Principal, t *write.TournamentRow) error {
		s, err := h.loadBracket(ctx, t)
		if err != nil {
			return err
		}
		n := s.g.Node(nodeID)
		if n == nil {
			return problem.New(problem.NotFound, "")
		}
		if t.Status != trn.StatusInProgress {
			return trn.ErrInvalidTransition
		}
		return fn(ctx, p, s, n)
	})
}

// ConfirmBracketNode implements confirmBracketNode (F107-AC-17): the organizer closes the complaint window of a decided
// node early; audit tournament.node_confirmed.
func (h Tournaments) ConfirmBracketNode(ctx context.Context, id, nodeID uuid.UUID) (out map[string]any, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.ConfirmBracketNode")
	defer func() { done(err) }()
	err = h.organizerNode(ctx, id, nodeID, func(ctx context.Context, p authz.Principal, s *bracketState, n *trn.Node) error {
		if err := n.Confirm(); err != nil {
			return err
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: AuditNodeConfirmed, TargetType: "bracket_node", TargetID: n.ID.String(),
			Result: audit.ResultSuccess, SchoolID: schoolOf(s.t), Metadata: map[string]any{"tournament_id": s.t.ID}}); err != nil {
			return err
		}
		if err := h.settle(ctx, s); err != nil {
			return err
		}
		out = view.Node(n, "")
		return nil
	})
	return out, trnError(err)
}

// OverrideBracketNode implements overrideBracketNode (F107-AC-18, AC-19): the organizer changes the winner of a node in
// its complaint window (or decides a node waiting after the last reschedule) with a mandatory reason; a next node already
// started → 422 INVALID_STATE_TRANSITION; the entries already moved on are swapped in the same transaction; audit
// tournament.node_overridden with the actor, the reason, the winner before and after.
func (h Tournaments) OverrideBracketNode(ctx context.Context, id, nodeID, winner uuid.UUID, reason string) (out map[string]any, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.OverrideBracketNode")
	defer func() { done(err) }()
	if err := reasonOK(reason); err != nil {
		return nil, err
	}
	err = h.organizerNode(ctx, id, nodeID, func(ctx context.Context, p authz.Principal, s *bracketState, n *trn.Node) error {
		before := n.Winner
		oldLoser := n.Loser()
		if err := s.g.Override(n, winner, reason, p.UserID, h.now()); err != nil {
			return err
		}
		if oldLoser != nil && *oldLoser == winner {
			s.setEntry(winner, trn.EntryCheckedIn) // back in the tournament
		}
		s.eliminate(n)
		if err := h.Audit.Record(ctx, audit.Event{Action: AuditNodeOverridden, TargetType: "bracket_node", TargetID: n.ID.String(),
			Result: audit.ResultSuccess, SchoolID: schoolOf(s.t), Reason: reason, Metadata: map[string]any{"tournament_id": s.t.ID,
				"winner_before": before, "winner_after": winner}}); err != nil {
			return err
		}
		if h.Metrics != nil {
			h.Metrics.Override()
		}
		if err := h.settle(ctx, s); err != nil {
			return err
		}
		out = view.Node(n, "")
		if err := idempotency.SetResult(ctx, out); err != nil {
			return err
		}
		return h.later(ctx, func(ctx context.Context) { h.pushNode(ctx, s, n) })
	})
	return out, trnError(err)
}

// pushNode sends tournament.bracket.updated to the students of a node.
func (h Tournaments) pushNode(ctx context.Context, s *bracketState, n *trn.Node) {
	var to []uuid.UUID
	for _, e := range []*uuid.UUID{n.A, n.B} {
		if e != nil && s.entries[*e] != nil {
			to = append(to, s.entries[*e].Members...)
		}
	}
	h.send(ctx, to, rtTournamentBracketUpdated, map[string]any{"tournament_id": s.t.ID, "node_id": n.ID, "status": n.Status,
		"winner_entry_id": n.Winner})
}
