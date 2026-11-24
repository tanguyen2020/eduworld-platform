package unit

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/challenge"
	trn "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/tournament"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/tournament/bracket"
)

// T014 / F107-AC-07 — the lifecycle of COMPETITION_SYSTEM §10.3: forward steps only, cancel from draft, published,
// registration_open and scheduled; everything else is ErrInvalidTransition.
func TestTournamentLifecycle(t *testing.T) {
	path := []string{trn.StatusDraft, trn.StatusPublished, trn.StatusRegistrationOpen, trn.StatusRegistrationClosed, trn.StatusSeeding,
		trn.StatusScheduled, trn.StatusInProgress, trn.StatusCompleted, trn.StatusRewarded, trn.StatusArchived}
	for i := 0; i+1 < len(path); i++ {
		if err := trn.Move(path[i], path[i+1]); err != nil {
			t.Fatalf("%s → %s: %v", path[i], path[i+1], err)
		}
	}
	for _, s := range trn.Statuses {
		want := slices.Contains([]string{trn.StatusDraft, trn.StatusPublished, trn.StatusRegistrationOpen, trn.StatusScheduled}, s)
		if trn.Cancellable(s) != want {
			t.Errorf("cancellable(%s) = %v", s, !want)
		}
	}
	for _, bad := range [][2]string{{trn.StatusDraft, trn.StatusRegistrationOpen}, {trn.StatusRegistrationOpen, trn.StatusScheduled},
		{trn.StatusCompleted, trn.StatusInProgress}, {trn.StatusCancelled, trn.StatusDraft}, {trn.StatusInProgress, trn.StatusCancelled},
		{trn.StatusSeeding, trn.StatusCancelled}} {
		if !errors.Is(trn.Move(bad[0], bad[1]), trn.ErrInvalidTransition) {
			t.Errorf("%s → %s accepted", bad[0], bad[1])
		}
	}
	if trn.MinEntries(trn.FormatSingleElim) != 4 || trn.MinEntries(trn.FormatDoubleElim) != 4 || trn.MinEntries(trn.FormatRoundRobin) != 3 ||
		trn.MinEntries(trn.FormatGroups) != 8 {
		t.Fatal("minimum entries of COMPETITION_SYSTEM §10.4")
	}
	if _, _, err := trn.Generate(trn.FormatSingleElim, 3); !errors.Is(err, bracket.ErrTooFew) {
		t.Fatal("single elimination needs 4 entries (F107-AC-14: 422 below the minimum)")
	}
}

// T020 / F107-AC-13 — seeding: rating (rated by μ descending, provisional after), random (the stored seed reproduces the
// order), manual (the exact order of the organizer); separate_class is refused (ADR-0095).
func TestTournamentSeeding(t *testing.T) {
	ids := seedsOf(6)
	cs := []bracket.Contender{{ID: ids[0], Mu: 1500, Provisional: true}, {ID: ids[1], Mu: 1700}, {ID: ids[2], Mu: 1600},
		{ID: ids[3], Mu: 1900, Provisional: true}, {ID: ids[4], Mu: 1650}, {ID: ids[5], Mu: 1400}}
	got, err := bracket.Order(bracket.SeedRating, cs, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []uuid.UUID{ids[1], ids[4], ids[2], ids[5], ids[3], ids[0]}
	if !slices.Equal(got, want) {
		t.Fatalf("rating order %v", got)
	}
	a, _ := bracket.Order(bracket.SeedRandom, cs, 42, nil)
	b, _ := bracket.Order(bracket.SeedRandom, cs, 42, nil)
	c, _ := bracket.Order(bracket.SeedRandom, cs, 43, nil)
	if !slices.Equal(a, b) {
		t.Fatal("random seeding is reproducible from its stored seed")
	}
	if slices.Equal(a, c) {
		t.Fatal("another seed shuffles differently")
	}
	manual := []uuid.UUID{ids[5], ids[4], ids[3], ids[2], ids[1], ids[0]}
	if got, err := bracket.Order(bracket.SeedManual, cs, 0, manual); err != nil || !slices.Equal(got, manual) {
		t.Fatalf("manual order %v %v", got, err)
	}
	if _, err := bracket.Order(bracket.SeedManual, cs, 0, manual[:5]); !errors.Is(err, bracket.ErrManualOrder) {
		t.Fatal("a manual order missing an entry is refused")
	}
	if _, err := bracket.Order("separate_class", cs, 0, nil); !errors.Is(err, bracket.ErrUnknownPolicy) || bracket.ValidPolicy("separate_class") {
		t.Fatal("separate_class is gone (ADR-0095)")
	}
	tpl := template(t, "grade_math_single_elim")
	var ve challenge.ValidationError
	if _, err := challenge.FilterParams(tpl, map[string]any{"seeding_policy": "separate_class"}); !errors.As(err, &ve) || ve.Field != "params.seeding_policy" {
		t.Fatalf("separate_class parameter: %v", err)
	}
}

// F107-AC-10, AC-16 — withdrawals and no-shows: a withdrawn round-1 entry gives its opponent a bye; when both left the
// node is void and the next node gets a bye; a knockout double forfeit voids the node; in a group both lose.
func TestTournamentByesAndForfeits(t *testing.T) {
	tid := uuid.New()
	gen, _, _ := trn.Generate(trn.FormatSingleElim, 8)
	seeds := seedsOf(8)
	g := trn.Build(tid, gen, seeds, trnT0, 5*time.Minute)
	g.Withdraw(map[uuid.UUID]bool{seeds[7]: true}) // seed 8 plays seed 1 in round 1
	var n1 *trn.Node
	for _, n := range g.Nodes {
		if n.Round == 1 && n.Position == 0 {
			n1 = n
		}
	}
	if n1.Status != trn.NodeBye || n1.Winner == nil || *n1.Winner != seeds[0] {
		t.Fatalf("seed 1 gets a bye: %+v", n1)
	}
	next := g.Node(*n1.NextWin)
	if next.A == nil || *next.A != seeds[0] {
		t.Fatal("seed 1 moved to round 2 slot A")
	}
	// both entries of round-1 node 1 (seeds 4 and 5) withdraw: void, then the round-2 opponent of seed 1 is a bye too.
	g.Withdraw(map[uuid.UUID]bool{seeds[3]: true, seeds[4]: true})
	var n2 *trn.Node
	for _, n := range g.Nodes {
		if n.Round == 1 && n.Position == 1 {
			n2 = n
		}
	}
	if n2.Status != trn.NodeVoid {
		t.Fatalf("both left: void, got %s", n2.Status)
	}
	if next.Status != trn.NodeBye || next.Winner == nil || *next.Winner != seeds[0] {
		t.Fatalf("round 2 of seed 1 is a bye: %s", next.Status)
	}
	// a double forfeit in round 1 node 2 (seeds 2 and 7) voids it; its round-2 node gives seeds 3/6's winner a bye later.
	var n3 *trn.Node
	for _, n := range g.Nodes {
		if n.Round == 1 && n.Position == 2 {
			n3 = n
		}
	}
	if err := g.Apply(n3, trn.Outcome{DoubleForfeit: true, Result: trn.Result{DoubleForfeit: true}}, trnT0); err != nil {
		t.Fatal(err)
	}
	if n3.Status != trn.NodeVoid {
		t.Fatalf("double forfeit in a knockout: void, got %s", n3.Status)
	}
	// replaying a result on a decided node changes nothing (F107-AC-17: idempotent per match).
	if err := g.Apply(n3, trn.Outcome{Winner: n3.A}, trnT0); !errors.Is(err, trn.ErrNodeState) {
		t.Fatal("a decided node refuses a second result")
	}
}

// F107-AC-18 — override: only in the complaint window or for a decision, never after the next node started; the
// entries that moved on are swapped.
func TestTournamentOverride(t *testing.T) {
	tid := uuid.New()
	gen, _, _ := trn.Generate(trn.FormatSingleElim, 4)
	seeds := seedsOf(4)
	g := trn.Build(tid, gen, seeds, trnT0, 5*time.Minute)
	var semi *trn.Node
	for _, n := range g.Nodes {
		if n.Round == 1 && n.Position == 0 {
			semi = n
		}
	}
	window := trnT0.Add(10 * time.Minute)
	if err := g.Apply(semi, trn.Outcome{Winner: semi.A, Result: trn.Result{Played: true}}, window); err != nil {
		t.Fatal(err)
	}
	final := g.Node(*semi.NextWin)
	if final.A == nil || *final.A != *semi.A {
		t.Fatal("the winner moved on at once")
	}
	by := uuid.New()
	if err := g.Override(semi, *semi.B, "phát hiện gian lận", by, window.Add(time.Second)); !errors.Is(err, trn.ErrNodeState) {
		t.Fatal("no override after the complaint window")
	}
	if err := g.Override(semi, uuid.New(), "x", by, trnT0); !errors.Is(err, trn.ErrNotInNode) {
		t.Fatal("the new winner is one of the two entries")
	}
	if err := g.Override(semi, *semi.B, "phát hiện gian lận", by, trnT0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if semi.Status != trn.NodeOverridden || *semi.Winner != *semi.B || final.A == nil || *final.A != *semi.B || *semi.OverrideReason == "" {
		t.Fatalf("override swaps the entry of the final: %+v", final)
	}
	// once the final started, the other semifinal can no longer be overridden.
	var semi2 *trn.Node
	for _, n := range g.Nodes {
		if n.Round == 1 && n.Position == 1 {
			semi2 = n
		}
	}
	if err := g.Apply(semi2, trn.Outcome{Winner: semi2.A, Result: trn.Result{Played: true}}, window); err != nil {
		t.Fatal(err)
	}
	now := trnT0.Add(2 * time.Minute)
	final.StartedAt, final.Status = &now, trn.NodeInMatch
	if err := g.Override(semi2, *semi2.B, "lý do", by, now); !errors.Is(err, trn.ErrNextStarted) {
		t.Fatalf("override after the next node started: %v", err)
	}
	// a node waiting for a decision (after the last reschedule, F107-AC-19) can be decided whatever the time.
	semi2.RequireDecision()
	final.StartedAt, final.Status = nil, trn.NodeScheduled
	if err := g.Override(semi2, *semi2.B, "người tổ chức quyết", by, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
}

// F107-AC-17, AC-20 — final ranks of a single elimination of 8: 1, 2, 3, 3, 5, 5, 5, 5; matches played count completed
// matches only (a bye is no match).
func TestTournamentRanks(t *testing.T) {
	g, seeds, played := play(t, trn.FormatSingleElim, 8)
	ranks := trn.Ranks(trn.FormatSingleElim, seeds, g.Nodes, nil)
	got := map[int][]int{}
	for _, p := range ranks {
		got[p.Rank] = append(got[p.Rank], seedOf(seeds, p.Entry))
		if p.Played != played[p.Entry] {
			t.Fatalf("seed %d played %d, record %d", seedOf(seeds, p.Entry), played[p.Entry], p.Played)
		}
	}
	if len(got[1]) != 1 || got[1][0] != 1 || len(got[2]) != 1 || got[2][0] != 2 || len(got[3]) != 2 || len(got[5]) != 4 {
		t.Fatalf("ranks %v", got)
	}
	g5, seeds5, _ := play(t, trn.FormatSingleElim, 5)
	for _, p := range trn.Ranks(trn.FormatSingleElim, seeds5, g5.Nodes, nil) {
		if p.Rank == 1 && p.Entry != seeds5[0] {
			t.Fatal("seed 1 wins")
		}
	}
	rr, rseeds, _ := play(t, trn.FormatRoundRobin, 5)
	rranks := trn.Ranks(trn.FormatRoundRobin, rseeds, rr.Nodes, nil)
	if rranks[0].Entry != rseeds[0] || rranks[0].Wins != 4 || rranks[4].Losses != 4 {
		t.Fatalf("round robin ranks %+v", rranks)
	}
}
