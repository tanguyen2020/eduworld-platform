package tournament

import (
	"sort"

	"github.com/google/uuid"
)

// Points of a group match (COMPETITION_SYSTEM §10.5): win 3, draw 1, loss 0.
const (
	PointsWin  = 3
	PointsDraw = 1
)

// Row is one entry of a group table.
type Row struct {
	Entry          uuid.UUID `json:"entry_id"`
	Rank           int       `json:"rank"`
	Points         int       `json:"points"`
	Wins           int       `json:"wins"`
	Draws          int       `json:"draws"`
	Losses         int       `json:"losses"`
	CorrectFor     int       `json:"correct_for"`
	CorrectAgainst int       `json:"correct_against"`
	CorrectDiff    int       `json:"correct_diff"`
	TimeMS         int64     `json:"time_ms"`
}

// counted reports a group node whose result counts in the table (decided, not void).
func counted(n *Node) bool {
	switch n.Status {
	case NodeAwaitingConfirm, NodeConfirmed, NodeOverridden:
		return n.A != nil && n.B != nil && !n.DecisionRequired
	}
	return false
}

// Standings computes the table of the entries from their group nodes (F107-AC-12): points (win 3, draw 1, loss 0, both
// lose on a double forfeit) → difference of correct answers → head to head among the tied entries → total time of the
// correct answers (lower first) → entry id. Void matches (withdrawn entry) do not count.
func Standings(entries []uuid.UUID, nodes []*Node) []Row {
	rows := map[uuid.UUID]*Row{}
	for _, e := range entries {
		rows[e] = &Row{Entry: e}
	}
	for _, n := range nodes {
		if !counted(n) {
			continue
		}
		a, b := rows[*n.A], rows[*n.B]
		if a == nil || b == nil {
			continue
		}
		var ra, rb Side
		if n.Result != nil {
			ra, rb = n.Result.A, n.Result.B
		}
		a.CorrectFor, a.CorrectAgainst, a.TimeMS = a.CorrectFor+ra.Correct, a.CorrectAgainst+rb.Correct, a.TimeMS+ra.TimeMS
		b.CorrectFor, b.CorrectAgainst, b.TimeMS = b.CorrectFor+rb.Correct, b.CorrectAgainst+ra.Correct, b.TimeMS+rb.TimeMS
		switch {
		case n.Result != nil && n.Result.DoubleForfeit:
			a.Losses++
			b.Losses++
		case n.Draw:
			a.Draws++
			b.Draws++
			a.Points += PointsDraw
			b.Points += PointsDraw
		case n.Winner != nil && *n.Winner == *n.A:
			a.Wins++
			b.Losses++
			a.Points += PointsWin
		case n.Winner != nil:
			b.Wins++
			a.Losses++
			b.Points += PointsWin
		}
	}
	out := make([]Row, 0, len(rows))
	for _, e := range entries {
		r := rows[e]
		r.CorrectDiff = r.CorrectFor - r.CorrectAgainst
		out = append(out, *r)
	}
	h2h := headToHead(nodes)
	sort.SliceStable(out, func(i, j int) bool {
		x, y := out[i], out[j]
		if x.Points != y.Points {
			return x.Points > y.Points
		}
		if x.CorrectDiff != y.CorrectDiff {
			return x.CorrectDiff > y.CorrectDiff
		}
		if hx, hy := h2h(x.Entry, y.Entry), h2h(y.Entry, x.Entry); hx != hy {
			return hx > hy
		}
		if x.TimeMS != y.TimeMS {
			return x.TimeMS < y.TimeMS
		}
		return x.Entry.String() < y.Entry.String()
	})
	for i := range out {
		out[i].Rank = i + 1
	}
	return out
}

// headToHead returns the points x took from its matches against y.
func headToHead(nodes []*Node) func(x, y uuid.UUID) int {
	return func(x, y uuid.UUID) int {
		pts := 0
		for _, n := range nodes {
			if !counted(n) || !n.Has(x) || !n.Has(y) {
				continue
			}
			switch {
			case n.Draw:
				pts += PointsDraw
			case n.Winner != nil && *n.Winner == x:
				pts += PointsWin
			}
		}
		return pts
	}
}

// Placement is the final rank of an entry with its record.
type Placement struct {
	Entry         uuid.UUID
	Rank          int
	Wins, Losses  int
	Draws, Points int
	Played        int // matches the entry completed (no-shows and walkovers excluded)
}

// record counts wins, draws, losses and completed matches of every entry over the decided nodes.
func record(nodes []*Node) map[uuid.UUID]*Placement {
	out := map[uuid.UUID]*Placement{}
	get := func(e uuid.UUID) *Placement {
		p := out[e]
		if p == nil {
			p = &Placement{Entry: e}
			out[e] = p
		}
		return p
	}
	for _, n := range nodes {
		if n.A == nil || n.B == nil || n.Status == NodeBye || n.Status == NodeVoid && (n.Result == nil || !n.Result.DoubleForfeit) {
			continue
		}
		if !n.Done() {
			continue
		}
		a, b := get(*n.A), get(*n.B)
		if n.Result != nil && n.Result.Played {
			if n.Result.Forfeit != "a" {
				a.Played++
			}
			if n.Result.Forfeit != "b" {
				b.Played++
			}
		}
		switch {
		case n.Result != nil && n.Result.DoubleForfeit:
			a.Losses++
			b.Losses++
		case n.Draw:
			a.Draws++
			b.Draws++
			a.Points += PointsDraw
			b.Points += PointsDraw
		case n.Winner != nil && *n.Winner == *n.A:
			a.Wins++
			b.Losses++
			a.Points += PointsWin
		case n.Winner != nil:
			b.Wins++
			a.Losses++
			b.Points += PointsWin
		}
	}
	return out
}

// eliminationRanks ranks the entries of a knockout (single, double, the knockout after the groups): the champion first,
// then by the wave of the node that eliminated them, later first; entries eliminated in the same wave share the rank
// (single elimination N = 8: 1, 2, 3, 3, 5, 5, 5, 5).
func eliminationRanks(entries []uuid.UUID, nodes []*Node) map[uuid.UUID]int {
	out := map[uuid.UUID]int{}
	out1 := map[uuid.UUID]int{} // elimination wave + 1 (0 = champion or never eliminated)
	champion := uuid.Nil
	for _, n := range nodes {
		if !n.Elimination() || !n.Done() {
			continue
		}
		if n.NextWin == nil && n.Winner != nil {
			champion = *n.Winner
		}
		var out2 []uuid.UUID
		switch {
		case n.Status == NodeVoid:
			for _, e := range []*uuid.UUID{n.A, n.B} {
				if e != nil {
					out2 = append(out2, *e)
				}
			}
		case n.NextLose == nil && n.Loser() != nil:
			out2 = append(out2, *n.Loser())
		}
		for _, e := range out2 {
			out1[e] = n.Wave + 1
		}
	}
	type pair struct {
		e uuid.UUID
		w int
	}
	var ps []pair
	for _, e := range entries {
		if e == champion {
			continue
		}
		if w, ok := out1[e]; ok {
			ps = append(ps, pair{e, w})
		}
	}
	sort.Slice(ps, func(i, j int) bool { return ps[i].w > ps[j].w })
	better := 0
	if champion != uuid.Nil {
		out[champion] = 1
		better = 1
	}
	for i := 0; i < len(ps); {
		j := i
		for j < len(ps) && ps[j].w == ps[i].w {
			j++
		}
		for k := i; k < j; k++ {
			out[ps[k].e] = better + 1
		}
		better += j - i
		i = j
	}
	return out
}

// Ranks computes the final placements of a completed tournament (F107-AC-17, AC-20): elimination ranks for the
// knockout formats; the table for a round robin; for groups then knockout the knockout ranks first, then the entries
// left in the group stage by their group position, points and difference of correct answers.
func Ranks(format string, entries []uuid.UUID, nodes []*Node, groups map[uuid.UUID][]uuid.UUID) []Placement {
	rec := record(nodes)
	rank := map[uuid.UUID]int{}
	switch format {
	case FormatRoundRobin:
		for _, r := range Standings(entries, nodes) {
			rank[r.Entry] = r.Rank
		}
	case FormatGroups:
		var ko []*Node
		var gn []*Node
		for _, n := range nodes {
			if n.Bracket == BracketGroup {
				gn = append(gn, n)
			} else {
				ko = append(ko, n)
			}
		}
		koRanks := eliminationRanks(entries, ko)
		for e, r := range koRanks {
			rank[e] = r
		}
		type left struct {
			e        uuid.UUID
			pos, pts int
			diff     int
		}
		var rest []left
		for g, members := range groups {
			var nodesOf []*Node
			for _, n := range gn {
				if n.GroupID != nil && *n.GroupID == g {
					nodesOf = append(nodesOf, n)
				}
			}
			for _, r := range Standings(members, nodesOf) {
				if _, ok := koRanks[r.Entry]; !ok {
					rest = append(rest, left{r.Entry, r.Rank, r.Points, r.CorrectDiff})
				}
			}
		}
		sort.Slice(rest, func(i, j int) bool {
			a, b := rest[i], rest[j]
			if a.pos != b.pos {
				return a.pos < b.pos
			}
			if a.pts != b.pts {
				return a.pts > b.pts
			}
			if a.diff != b.diff {
				return a.diff > b.diff
			}
			return a.e.String() < b.e.String()
		})
		next := len(koRanks) + 1
		for i, l := range rest {
			rank[l.e] = next + i
		}
	default:
		rank = eliminationRanks(entries, nodes)
	}
	out := make([]Placement, 0, len(entries))
	last := len(entries)
	for _, e := range entries {
		p := Placement{Entry: e}
		if r := rec[e]; r != nil {
			p = *r
		}
		p.Rank = rank[e]
		if p.Rank == 0 {
			p.Rank = last // withdrawn before playing: last place
		}
		out = append(out, p)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Rank < out[j].Rank })
	return out
}
