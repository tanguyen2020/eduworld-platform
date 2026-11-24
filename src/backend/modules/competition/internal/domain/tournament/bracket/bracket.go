// Package bracket holds the pure bracket algorithms of the tournaments (COMPETITION_SYSTEM §10.5, F107 US3/US5): single
// and double elimination, round robin (circle method), groups by snake seeding and the knockout after the groups. Every
// function is deterministic: the same seeded entries always give the same nodes, so a bracket generated twice is the
// same bracket (F107-AC-14). Nodes are addressed by Key (bracket, round, position); the caller derives the node ids.
package bracket

import (
	"errors"
	"fmt"
)

// Brackets of a node.
const (
	Winners    = "winners"
	Losers     = "losers"
	GrandFinal = "grand_final"
	Group      = "group"
)

// ErrTooFew: fewer entries than the algorithm needs.
var ErrTooFew = errors.New("bracket: too few entries")

// Key addresses a node in its tournament.
type Key struct {
	Bracket  string
	Round    int
	Position int
}

func (k Key) String() string { return fmt.Sprintf("%s/%d/%d", k.Bracket, k.Round, k.Position) }

// Node is one generated node: the seeds in its two slots (0 = none: a bye or a slot fed by another node), where its
// winner and its loser go (slot 0 = entry_a, 1 = entry_b) and its group.
type Node struct {
	Key          Key
	SeedA, SeedB int
	NextWin      *Key
	NextWinSlot  int
	NextLose     *Key
	NextLoseSlot int
	Group        string
	Wave         int // scheduling wave: 0 for the first matches, max(feeders)+1 after
}

// Feeds reports whether n sends its winner or its loser into slot of k.
func (n Node) Feeds(k Key, slot int) bool {
	return (n.NextWin != nil && *n.NextWin == k && n.NextWinSlot == slot) || (n.NextLose != nil && *n.NextLose == k && n.NextLoseSlot == slot)
}

// Size returns the bracket size of n entries: the smallest power of two ≥ n.
func Size(n int) int {
	s := 1
	for s < n {
		s *= 2
	}
	return s
}

// SeedOrder returns the standard seed positions of a bracket of size s (a power of two): position i of round 1 holds
// seed SeedOrder(s)[i]. Seeds 1 and 2 are in different halves, 1–4 in different quarters, and so on, so the best seeds
// only meet in the last rounds; seed k plays seed s+1−k in round 1 (byes go to the best seeds).
func SeedOrder(s int) []int {
	order := []int{1}
	for len(order) < s {
		n := len(order) * 2
		next := make([]int, 0, n)
		for _, x := range order {
			next = append(next, x, n+1-x)
		}
		order = next
	}
	return order
}

func log2(s int) int {
	r := 0
	for s > 1 {
		s /= 2
		r++
	}
	return r
}

func key(b string, round, pos int) *Key { return &Key{Bracket: b, Round: round, Position: pos} }

// SingleElim builds a single elimination bracket of n seeded entries (seeds 1..n, n ≥ 2): Size(n) slots, byes for the
// best seeds (a round-1 node with one seed), seeds 1 and 2 meet only in the final (F107-AC-11, AC-TRN-001).
func SingleElim(n int) ([]Node, error) {
	if n < 2 {
		return nil, ErrTooFew
	}
	s := Size(n)
	order := SeedOrder(s)
	rounds := log2(s)
	var out []Node
	for r := 1; r <= rounds; r++ {
		count := s >> r
		for p := 0; p < count; p++ {
			nd := Node{Key: Key{Bracket: Winners, Round: r, Position: p}, Wave: r - 1}
			if r == 1 {
				nd.SeedA, nd.SeedB = seat(order[2*p], n), seat(order[2*p+1], n)
			}
			if r < rounds {
				nd.NextWin, nd.NextWinSlot = key(Winners, r+1, p/2), p%2
			}
			out = append(out, nd)
		}
	}
	return out, nil
}

// seat is seed when it is a real entry, 0 for a bye.
func seat(seed, n int) int {
	if seed > n {
		return 0
	}
	return seed
}

// DoubleElim builds a double elimination bracket of n seeded entries (n ≥ 4, F107-AC-22, AC-TRN-002): the winners
// bracket of SingleElim; the loser of winners round 1 node j drops to losers round 1 node j/2 (slot j%2); the loser of
// winners round k+1 drops into losers round 2k (slot 1) in reverse order on odd k (standard mapping that delays
// rematches); losers rounds alternate drop-in rounds (2k) and reduction rounds (2k+1); the winners final and the losers
// final meet in a single grand final (bracket reset off by default, COMPETITION_SYSTEM §10.5).
func DoubleElim(n int) ([]Node, error) {
	if n < 4 {
		return nil, ErrTooFew
	}
	wb, err := SingleElim(n)
	if err != nil {
		return nil, err
	}
	s := Size(n)
	rounds := log2(s)            // winners rounds
	lbRounds := 2 * (rounds - 1) // losers rounds
	gf := key(GrandFinal, 1, 0)
	lbCount := func(lr int) int { // nodes of losers round lr
		return s >> (2 + (lr-1)/2)
	}
	for i := range wb {
		nd := &wb[i]
		r, p := nd.Key.Round, nd.Key.Position
		switch r {
		case rounds:
			nd.NextWin, nd.NextWinSlot = gf, 0
			nd.NextLose, nd.NextLoseSlot = key(Losers, lbRounds, 0), 1
		case 1:
			nd.NextLose, nd.NextLoseSlot = key(Losers, 1, p/2), p%2
		default:
			k := r - 1 // the loser of winners round k+1 drops into losers round 2k
			m := lbCount(2 * k)
			pos := p
			if k%2 == 1 {
				pos = m - 1 - p
			}
			nd.NextLose, nd.NextLoseSlot = key(Losers, 2*k, pos), 1
		}
	}
	out := wb
	for lr := 1; lr <= lbRounds; lr++ {
		m := lbCount(lr)
		for p := 0; p < m; p++ {
			nd := Node{Key: Key{Bracket: Losers, Round: lr, Position: p}}
			switch {
			case lr == lbRounds:
				nd.NextWin, nd.NextWinSlot = gf, 1
			case lr%2 == 1: // reduction or first round → next drop-in round, same position, slot 0
				nd.NextWin, nd.NextWinSlot = key(Losers, lr+1, p), 0
			default: // drop-in round → next reduction round
				nd.NextWin, nd.NextWinSlot = key(Losers, lr+1, p/2), p%2
			}
			out = append(out, nd)
		}
	}
	out = append(out, Node{Key: *gf})
	return Waves(out), nil
}

// Waves sets the scheduling wave of every node: 0 when nothing feeds it, else one more than its latest feeder.
func Waves(nodes []Node) []Node {
	idx := map[Key]int{}
	for i, n := range nodes {
		idx[n.Key] = i
		nodes[i].Wave = 0
	}
	for changed := true; changed; {
		changed = false
		for _, n := range nodes {
			for _, next := range []*Key{n.NextWin, n.NextLose} {
				if next == nil {
					continue
				}
				j, ok := idx[*next]
				if ok && nodes[j].Wave < n.Wave+1 {
					nodes[j].Wave = n.Wave + 1
					changed = true
				}
			}
		}
	}
	return nodes
}

// RoundRobin builds the round robin of n seeded entries (n ≥ 2) in group (circle method, F107-AC-12, AC-TRN-003): an odd
// n gets a phantom entry (its pairs are byes and produce no node), every entry plays n−1 matches (n even) or n−1 with
// one bye round (n odd), no pair twice. Round r is wave offset+r−1; positions start at posBase (unique per group).
func RoundRobin(n int, group string, offset, posBase int) ([]Node, error) {
	if n < 2 {
		return nil, ErrTooFew
	}
	m := n
	if m%2 == 1 {
		m++
	}
	ring := make([]int, m) // seeds; 0 is the phantom
	for i := range ring {
		if i < n {
			ring[i] = i + 1
		}
	}
	var out []Node
	for r := 1; r < m; r++ {
		pos := posBase
		for i := 0; i < m/2; i++ {
			a, b := ring[i], ring[m-1-i]
			if a == 0 || b == 0 {
				continue
			}
			out = append(out, Node{Key: Key{Bracket: Group, Round: r, Position: pos}, SeedA: a, SeedB: b, Group: group, Wave: offset + r - 1})
			pos++
		}
		// rotate every seat but the first one clockwise
		last := ring[m-1]
		copy(ring[2:], ring[1:m-1])
		ring[1] = last
	}
	return out, nil
}

// GroupCount is the number of groups of the group stage for n entries (n ≥ 8): n/4, so every group has 4 or 5 entries
// (COMPETITION_SYSTEM §10.5: groups of 4–6).
func GroupCount(n int) int { return n / 4 }

// SnakeGroups splits seeds 1..n into GroupCount(n) groups by snake order (row k left to right when k is even, right to
// left when odd — F107-AC-23): group g holds the seeds listed in out[g].
func SnakeGroups(n int) ([][]int, error) {
	if n < 8 {
		return nil, ErrTooFew
	}
	g := GroupCount(n)
	out := make([][]int, g)
	for i := 0; i < n; i++ {
		row, col := i/g, i%g
		if row%2 == 1 {
			col = g - 1 - col
		}
		out[col] = append(out[col], i+1)
	}
	return out, nil
}

// GroupCode is the code of group g (A, B, C…).
func GroupCode(g int) string { return string(rune('A' + g)) }

// GroupStage builds the round robin of every snake group: positions are unique per round across groups (group g uses
// positions from g*8), the seeds stay the overall seeds.
func GroupStage(n int) ([]Node, [][]int, error) {
	groups, err := SnakeGroups(n)
	if err != nil {
		return nil, nil, err
	}
	var out []Node
	for g, seeds := range groups {
		nodes, err := RoundRobin(len(seeds), GroupCode(g), 0, g*8)
		if err != nil {
			return nil, nil, err
		}
		for _, nd := range nodes {
			nd.SeedA, nd.SeedB = seeds[nd.SeedA-1], seeds[nd.SeedB-1]
			out = append(out, nd)
		}
	}
	return out, groups, nil
}

// KnockoutSeeds orders the qualifiers of the groups for the knockout (top 2 per group, F107-AC-23): group winners first
// in group order (seeds 1..G), then the runners-up (seeds G+1..2G) rotated by the smallest shift for which no winner meets
// the runner-up of its own group in round 1. winners[g] and runners[g] are the entries of group g.
func KnockoutSeeds[T any](winners, runners []T) []T {
	g := len(winners)
	order := SeedOrder(Size(2 * g))
	groupOf := func(seed, shift int) int {
		if seed <= g {
			return seed - 1
		}
		return (seed - g - 1 + shift) % g
	}
	best := 0
	for shift := 0; shift < g; shift++ {
		clash := false
		for i := 0; i+1 < len(order); i += 2 {
			a, b := order[i], order[i+1]
			if a <= 2*g && b <= 2*g && groupOf(a, shift) == groupOf(b, shift) {
				clash = true
				break
			}
		}
		if !clash {
			best = shift
			break
		}
	}
	out := make([]T, 0, 2*g)
	out = append(out, winners...)
	for i := 0; i < len(runners); i++ {
		out = append(out, runners[(i+best)%len(runners)])
	}
	return out
}

// Knockout builds the single elimination after the groups for q qualifiers starting at round 1 of the winners bracket,
// waves after the group stage (offset).
func Knockout(q, offset int) ([]Node, error) {
	nodes, err := SingleElim(q)
	if err != nil {
		return nil, err
	}
	for i := range nodes {
		nodes[i].Wave += offset
	}
	return nodes, nil
}
