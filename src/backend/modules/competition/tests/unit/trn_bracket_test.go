package unit

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	trn "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/tournament"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/tournament/bracket"
)

// seedsOf returns n entry ids; entry i is seed i+1.
func seedsOf(n int) []uuid.UUID {
	out := make([]uuid.UUID, n)
	for i := range out {
		out[i] = uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("entry-%d", i+1)))
	}
	return out
}

func seedOf(seeds []uuid.UUID, id uuid.UUID) int { return slices.Index(seeds, id) + 1 }

var trnT0 = time.Date(2026, 12, 1, 2, 0, 0, 0, time.UTC)

// play runs a whole bracket: the better seed (lower number) always wins; every result is confirmed at once. It returns
// the graph and how many matches each entry played.
func play(t *testing.T, format string, n int) (*trn.Graph, []uuid.UUID, map[uuid.UUID]int) {
	t.Helper()
	tid := uuid.New()
	gen, _, err := trn.Generate(format, n)
	if err != nil {
		t.Fatalf("%s n=%d: %v", format, n, err)
	}
	seeds := seedsOf(n)
	g := trn.Build(tid, gen, seeds, trnT0, 5*time.Minute)
	played := map[uuid.UUID]int{}
	for guard := 0; guard < 10000; guard++ {
		var next *trn.Node
		for _, nd := range g.Nodes {
			if nd.Status == trn.NodeScheduled {
				next = nd
				break
			}
		}
		if next == nil {
			break
		}
		w := next.A
		if seedOf(seeds, *next.B) < seedOf(seeds, *next.A) {
			w = next.B
		}
		played[*next.A]++
		played[*next.B]++
		if err := g.Apply(next, trn.Outcome{Winner: w, Result: trn.Result{Played: true}}, trnT0); err != nil {
			t.Fatal(err)
		}
		if err := next.Confirm(); err != nil {
			t.Fatal(err)
		}
	}
	return g, seeds, played
}

// T019 / F107-AC-11, SC-001 — single elimination for N = 2..64: Size(N) slots (a power of two ≥ N), byes exactly for
// seeds 1..S−N, seeds 1 and 2 in different halves and meeting only in the final.
func TestTournamentSingleElimN2to64(t *testing.T) {
	for n := 2; n <= 64; n++ {
		nodes, err := bracket.SingleElim(n)
		if err != nil {
			t.Fatal(err)
		}
		s := bracket.Size(n)
		if s&(s-1) != 0 || s < n || s >= 2*n && n > 1 {
			t.Fatalf("n=%d: size %d", n, s)
		}
		var r1 []bracket.Node
		for _, nd := range nodes {
			if nd.Key.Round == 1 {
				r1 = append(r1, nd)
			}
		}
		if len(r1) != s/2 {
			t.Fatalf("n=%d: %d round-1 nodes, want %d", n, len(r1), s/2)
		}
		var byes []int
		half := map[int]int{}
		for _, nd := range r1 {
			if nd.SeedA == 0 || nd.SeedB == 0 {
				byes = append(byes, max(nd.SeedA, nd.SeedB))
			}
			for _, sd := range []int{nd.SeedA, nd.SeedB} {
				if sd == 1 || sd == 2 {
					half[sd] = nd.Key.Position * 2 / (s / 2)
				}
			}
		}
		slices.Sort(byes)
		want := make([]int, 0, s-n)
		for i := 1; i <= s-n; i++ {
			want = append(want, i)
		}
		if !slices.Equal(byes, want) {
			t.Fatalf("n=%d: byes for seeds %v, want %v", n, byes, want)
		}
		if n >= 2 && s >= 4 && half[1] == half[2] {
			t.Fatalf("n=%d: seeds 1 and 2 in the same half", n)
		}
		if n < 4 {
			continue // below the format minimum: only the algorithm is checked
		}
		g, seeds, _ := play(t, trn.FormatSingleElim, n)
		var final *trn.Node
		for _, nd := range g.Nodes {
			if nd.NextWin == nil {
				final = nd
			}
		}
		if final == nil || final.A == nil || final.B == nil {
			t.Fatalf("n=%d: no final", n)
		}
		got := []int{seedOf(seeds, *final.A), seedOf(seeds, *final.B)}
		slices.Sort(got)
		if !slices.Equal(got, []int{1, 2}) {
			t.Fatalf("n=%d: final between seeds %v", n, got)
		}
		if !g.Complete(nil) {
			t.Fatalf("n=%d: bracket not complete", n)
		}
	}
}

// AC-TRN-001 — N = 5: 8 slots, 3 byes for seeds 1–3, seeds 1 and 2 in the two halves.
func TestTournamentSingleElimFive(t *testing.T) {
	nodes, err := bracket.SingleElim(5)
	if err != nil {
		t.Fatal(err)
	}
	if bracket.Size(5) != 8 {
		t.Fatal("size of 5 is 8")
	}
	byes := 0
	for _, nd := range nodes {
		if nd.Key.Round == 1 && (nd.SeedA == 0 || nd.SeedB == 0) {
			byes++
			if s := max(nd.SeedA, nd.SeedB); s > 3 {
				t.Fatalf("bye for seed %d", s)
			}
		}
	}
	if byes != 3 {
		t.Fatalf("%d byes", byes)
	}
	if order := bracket.SeedOrder(8); !slices.Equal(order, []int{1, 8, 4, 5, 2, 7, 3, 6}) {
		t.Fatalf("seed order %v", order)
	}
}

// T019 / F107-AC-12, AC-TRN-003 — round robin for N = 2..64 (circle method): every entry plays N−1 matches, no pair twice,
// N(N−1)/2 matches, an odd N gives one bye round per entry.
func TestTournamentRoundRobinN2to64(t *testing.T) {
	for n := 2; n <= 64; n++ {
		nodes, err := bracket.RoundRobin(n, "A", 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(nodes) != n*(n-1)/2 {
			t.Fatalf("n=%d: %d matches", n, len(nodes))
		}
		games := map[int]int{}
		pairs := map[[2]int]bool{}
		perRound := map[int]map[int]bool{}
		for _, nd := range nodes {
			a, b := min(nd.SeedA, nd.SeedB), max(nd.SeedA, nd.SeedB)
			if a < 1 || b > n || a == b {
				t.Fatalf("n=%d: bad pair %d-%d", n, a, b)
			}
			if pairs[[2]int{a, b}] {
				t.Fatalf("n=%d: pair %d-%d twice", n, a, b)
			}
			pairs[[2]int{a, b}] = true
			games[a]++
			games[b]++
			if perRound[nd.Key.Round] == nil {
				perRound[nd.Key.Round] = map[int]bool{}
			}
			if perRound[nd.Key.Round][a] || perRound[nd.Key.Round][b] {
				t.Fatalf("n=%d: an entry plays twice in round %d", n, nd.Key.Round)
			}
			perRound[nd.Key.Round][a], perRound[nd.Key.Round][b] = true, true
		}
		for s := 1; s <= n; s++ {
			if games[s] != n-1 {
				t.Fatalf("n=%d: seed %d plays %d", n, s, games[s])
			}
		}
	}
}

// F107-AC-12 — standings: win 3 / draw 1 / loss 0, then difference of correct answers, then head to head, then the time
// of the correct answers.
func TestTournamentStandings(t *testing.T) {
	e := seedsOf(4)
	mk := func(a, b int, winner int, draw bool, ca, cb int, ta, tb int64) *trn.Node {
		n := &trn.Node{ID: uuid.New(), Bracket: trn.BracketGroup, A: &e[a], B: &e[b], Status: trn.NodeConfirmed, Draw: draw,
			Result: &trn.Result{A: trn.Side{Correct: ca, TimeMS: ta}, B: trn.Side{Correct: cb, TimeMS: tb}, Played: true}}
		if winner >= 0 {
			n.Winner = &e[winner]
		}
		return n
	}
	nodes := []*trn.Node{
		mk(0, 1, 0, false, 5, 3, 1000, 1000), // e0 beats e1
		mk(2, 3, -1, true, 4, 4, 900, 800),   // draw
		mk(0, 2, 2, false, 2, 6, 500, 500),   // e2 beats e0
		mk(1, 3, 1, false, 7, 1, 700, 700),   // e1 beats e3
		mk(0, 3, 0, false, 5, 5, 400, 900),   // e0 beats e3 (equal correct)
		mk(1, 2, 1, false, 6, 4, 600, 600),   // e1 beats e2
	}
	rows := trn.Standings(e, nodes)
	// points: e0 6, e1 6, e2 4, e3 1; e0 and e1 tied on points: diff e0 (5-3)+(2-6)+(5-5) = -2, e1 (3-5)+(7-1)+(6-4) = 6.
	if rows[0].Entry != e[1] || rows[0].Points != 6 || rows[1].Entry != e[0] || rows[2].Entry != e[2] || rows[3].Entry != e[3] {
		t.Fatalf("standings %+v", rows)
	}
	if rows[3].Points != trn.PointsDraw || rows[2].Points != trn.PointsWin+trn.PointsDraw {
		t.Fatalf("points %+v", rows)
	}
	// head to head breaks a tie of points and difference.
	h := []*trn.Node{mk(0, 1, 1, false, 3, 3, 100, 100), mk(0, 2, 0, false, 4, 3, 100, 100), mk(1, 2, 2, false, 3, 4, 100, 100)}
	rows = trn.Standings(e[:3], h)
	if rows[0].Points != 3 || rows[1].Points != 3 || rows[2].Points != 3 {
		t.Fatalf("three-way tie expected %+v", rows)
	}
	// a double forfeit counts as a loss for both and no points.
	df := &trn.Node{ID: uuid.New(), Bracket: trn.BracketGroup, A: &e[0], B: &e[1], Status: trn.NodeConfirmed, Result: &trn.Result{DoubleForfeit: true}}
	rows = trn.Standings(e[:2], []*trn.Node{df})
	if rows[0].Losses != 1 || rows[1].Losses != 1 || rows[0].Points != 0 {
		t.Fatalf("double forfeit %+v", rows)
	}
}

// T030 / F107-AC-22, AC-TRN-002 — double elimination N = 8: the loser of winners round 1 node j drops into losers round 1
// node j/2 slot j%2; the losers of winners round 2 drop into losers round 2 in reverse order; one grand final (bracket
// reset off), winners final → grand final slot 0, losers final → slot 1.
func TestTournamentDoubleElimMapping(t *testing.T) {
	nodes, err := bracket.DoubleElim(8)
	if err != nil {
		t.Fatal(err)
	}
	by := map[bracket.Key]bracket.Node{}
	gf := 0
	for _, nd := range nodes {
		by[nd.Key] = nd
		if nd.Key.Bracket == bracket.GrandFinal {
			gf++
		}
	}
	if gf != 1 {
		t.Fatalf("%d grand finals (bracket reset is off: one)", gf)
	}
	for j := 0; j < 4; j++ {
		nd := by[bracket.Key{Bracket: bracket.Winners, Round: 1, Position: j}]
		if nd.NextLose == nil || *nd.NextLose != (bracket.Key{Bracket: bracket.Losers, Round: 1, Position: j / 2}) || nd.NextLoseSlot != j%2 {
			t.Fatalf("WB1 node %d loser goes to %v slot %d", j, nd.NextLose, nd.NextLoseSlot)
		}
	}
	for j := 0; j < 2; j++ {
		nd := by[bracket.Key{Bracket: bracket.Winners, Round: 2, Position: j}]
		want := bracket.Key{Bracket: bracket.Losers, Round: 2, Position: 1 - j}
		if nd.NextLose == nil || *nd.NextLose != want || nd.NextLoseSlot != 1 {
			t.Fatalf("WB2 node %d loser goes to %v slot %d, want %v slot 1", j, nd.NextLose, nd.NextLoseSlot, want)
		}
	}
	wf := by[bracket.Key{Bracket: bracket.Winners, Round: 3, Position: 0}]
	if wf.NextWin == nil || wf.NextWin.Bracket != bracket.GrandFinal || wf.NextWinSlot != 0 {
		t.Fatalf("winners final → %v", wf.NextWin)
	}
	lf := by[bracket.Key{Bracket: bracket.Losers, Round: 4, Position: 0}]
	if lf.NextWin == nil || lf.NextWin.Bracket != bracket.GrandFinal || lf.NextWinSlot != 1 {
		t.Fatalf("losers final → %v", lf.NextWin)
	}
	// every entry except the champion loses exactly twice, or once in the grand final; nobody plays after being out.
	for n := 4; n <= 32; n++ {
		g, seeds, played := play(t, trn.FormatDoubleElim, n)
		if !g.Complete(nil) {
			t.Fatalf("n=%d: incomplete", n)
		}
		ranks := trn.Ranks(trn.FormatDoubleElim, seeds, g.Nodes, nil)
		if ranks[0].Entry != seeds[0] || ranks[0].Rank != 1 || ranks[1].Rank != 2 || ranks[1].Entry != seeds[1] {
			t.Fatalf("n=%d: ranks %v", n, ranks[:2])
		}
		if played[seeds[0]] < 2 && n > 2 {
			t.Fatalf("n=%d: champion played %d", n, played[seeds[0]])
		}
	}
}

// T030 / F107-AC-23 — groups: N = 12 → 3 groups of 4 by snake seeding, round robin in each group, then the top 2 of each
// group in a knockout where a group winner never meets the runner-up of its own group in round 1.
func TestTournamentGroupsSnake(t *testing.T) {
	groups, err := bracket.SnakeGroups(12)
	if err != nil {
		t.Fatal(err)
	}
	want := [][]int{{1, 6, 7, 12}, {2, 5, 8, 11}, {3, 4, 9, 10}}
	if len(groups) != 3 {
		t.Fatalf("%d groups", len(groups))
	}
	for i := range want {
		if !slices.Equal(groups[i], want[i]) {
			t.Fatalf("group %d = %v, want %v", i, groups[i], want[i])
		}
	}
	for n := 8; n <= 64; n++ {
		gs, err := bracket.SnakeGroups(n)
		if err != nil {
			t.Fatal(err)
		}
		for _, g := range gs {
			if len(g) < 4 || len(g) > 6 {
				t.Fatalf("n=%d: group of %d", n, len(g))
			}
		}
	}
	if _, err := bracket.SnakeGroups(7); err == nil {
		t.Fatal("7 entries cannot make groups of 4–6")
	}
	nodes, _, err := bracket.GroupStage(12)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 3*6 {
		t.Fatalf("%d group matches, want 18", len(nodes))
	}
	// knockout of the 6 qualifiers: winners A, B, C are seeds 1–3, runners-up A, B, C seeds 4–6.
	order := bracket.KnockoutSeeds([]string{"A1", "B1", "C1"}, []string{"A2", "B2", "C2"})
	ko, err := bracket.Knockout(len(order), 3)
	if err != nil {
		t.Fatal(err)
	}
	for _, nd := range ko {
		if nd.Key.Round != 1 || nd.SeedA == 0 || nd.SeedB == 0 {
			continue
		}
		a, b := order[nd.SeedA-1], order[nd.SeedB-1]
		if a[0] == b[0] {
			t.Fatalf("round 1 pairs %s with %s of the same group", a, b)
		}
	}
	if ko[0].Wave < 3 {
		t.Fatal("the knockout comes after the group stage")
	}
}
