package matchmaking

import (
	"math"
	"slices"
	"sort"
	"time"

	"github.com/google/uuid"
)

// Candidate is a queued ticket as the matchmaker sees it (MATCHMAKING §6).
type Candidate struct {
	TicketID   uuid.UUID
	Students   []uuid.UUID
	Grade      int
	Rating     Rating // μ_team / RD_team for a party ticket
	EnqueuedAt time.Time
	PartyID    *uuid.UUID
	AllowFill  bool
}

// Size is the number of players of the ticket.
func (c Candidate) Size() int { return len(c.Students) }

// Env carries what the pure pairing needs from outside: the time and the pairwise checks read by the application
// (H6 blocks, recent opponents within recent_opponent_window).
type Env struct {
	Now     time.Time
	Blocked func(a, b uuid.UUID) bool
	Recent  func(a, b uuid.UUID) bool
}

func (e Env) blocked(a, b Candidate) bool {
	if e.Blocked == nil {
		return false
	}
	for _, x := range a.Students {
		for _, y := range b.Students {
			if e.Blocked(x, y) {
				return true
			}
		}
	}
	return false
}

func (e Env) recent(a, b Candidate) bool {
	if e.Recent == nil {
		return false
	}
	for _, x := range a.Students {
		for _, y := range b.Students {
			if e.Recent(x, y) {
				return true
			}
		}
	}
	return false
}

// Group is one pairing result: the tickets of a proposal and the two teams (student ids, team 0 and team 1).
type Group struct {
	Tickets []Candidate
	Teams   [2][]uuid.UUID
}

// TicketIDs returns the tickets of the group.
func (g Group) TicketIDs() []uuid.UUID {
	out := make([]uuid.UUID, len(g.Tickets))
	for i, t := range g.Tickets {
		out[i] = t.TicketID
	}
	return out
}

// Cost is cost(a, b) of MATCHMAKING §2.2 (smaller is better) for the window w of the pair.
func (c Config) Cost(k QueueKey, a, b Candidate, w float64, env Env) float64 {
	cw := c.CostWeights
	cost := 0.0
	if !math.IsInf(w, 1) && w > 0 {
		cost += cw.Rating * math.Abs(a.Rating.Mu-b.Rating.Mu) / w
	}
	cost += cw.RD * math.Max(0, a.Rating.RD+b.Rating.RD-300) / 300
	if env.recent(a, b) {
		cost += cw.RecentOpponent
	}
	return cost
}

// compatible are the hard checks of a pair inside one queue: H1 (grade window), H6 (blocks) and the rating window of
// the anchor (expanding with its wait; random matchmaking ignores rating).
func (c Config) compatible(k QueueKey, anchor, b Candidate, env Env) (bool, float64) {
	wa, wb := env.Now.Sub(anchor.EnqueuedAt), env.Now.Sub(b.EnqueuedAt)
	if !c.GradeCompatible(k.Type, anchor.Grade, b.Grade, wa, wb) || env.blocked(anchor, b) {
		return false, 0
	}
	if k.Type == TypeRandom {
		return true, math.Inf(1)
	}
	w := c.Window(max(wa, wb))
	if math.Abs(anchor.Rating.Mu-b.Rating.Mu) > w {
		return false, 0
	}
	return true, w
}

func byWait(cs []Candidate) []Candidate {
	out := slices.Clone(cs)
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].EnqueuedAt.Equal(out[j].EnqueuedAt) {
			return out[i].EnqueuedAt.Before(out[j].EnqueuedAt)
		}
		return out[i].TicketID.String() < out[j].TicketID.String()
	})
	return out
}

// Pair runs one greedy pass over the candidates of a queue (MATCHMAKING §6): oldest ticket first, partner of smallest
// cost within the window (random: the oldest compatible partner, FIFO). Hard constraints are never relaxed. Above
// bucket_threshold candidates are bucketed by rating: only the rating neighbours inside the window are scanned.
func (c Config) Pair(k QueueKey, cands []Candidate, env Env) []Group {
	if k.Mode == ModeTeamBattle {
		return c.pairTeams(k, cands, env)
	}
	order := byWait(cands)
	used := make(map[uuid.UUID]bool, len(order))
	var byMu []int // indexes of order sorted by rating, for the bucketed search
	if len(order) > c.BucketThreshold && k.Type != TypeRandom {
		byMu = make([]int, len(order))
		for i := range order {
			byMu[i] = i
		}
		sort.SliceStable(byMu, func(i, j int) bool { return order[byMu[i]].Rating.Mu < order[byMu[j]].Rating.Mu })
	}
	var out []Group
	for i, a := range order {
		if used[a.TicketID] {
			continue
		}
		best, bestCost := -1, math.Inf(1)
		consider := func(j int) {
			b := order[j]
			if j == i || used[b.TicketID] {
				return
			}
			ok, w := c.compatible(k, a, b, env)
			if !ok {
				return
			}
			if k.Type == TypeRandom {
				if best < 0 || j < best {
					best, bestCost = j, 0
				}
				return
			}
			if cost := c.Cost(k, a, b, w, env); cost < bestCost || (cost == bestCost && j < best) {
				best, bestCost = j, cost
			}
		}
		if byMu != nil {
			w := c.Window(env.Now.Sub(a.EnqueuedAt))
			lo := sort.Search(len(byMu), func(x int) bool { return order[byMu[x]].Rating.Mu >= a.Rating.Mu-w })
			for x := lo; x < len(byMu) && order[byMu[x]].Rating.Mu <= a.Rating.Mu+w; x++ {
				consider(byMu[x])
			}
		} else {
			for j := range order {
				consider(j)
			}
		}
		if best < 0 {
			continue
		}
		b := order[best]
		used[a.TicketID], used[b.TicketID] = true, true
		out = append(out, Group{Tickets: []Candidate{a, b}, Teams: [2][]uuid.UUID{slices.Clone(a.Students), slices.Clone(b.Students)}})
	}
	return out
}
