package matchmaking

import (
	"slices"
	"sort"

	"github.com/google/uuid"
)

// pairTeams builds team matches of k.TeamSize players a side (MATCHMAKING §7–§8): oldest ticket first, then the
// compatible tickets in wait order (grade window, blocks and rating window against the anchor; every ticket against
// every other for H1/H6) until two full teams can be assigned. A party stays on one team; at most one party per team;
// a party without allow_fill is its whole team; solo tickets are spread by snake draft on rating.
func (c Config) pairTeams(k QueueKey, cands []Candidate, env Env) []Group {
	n := k.TeamSize
	order := byWait(cands)
	used := map[uuid.UUID]bool{}
	var out []Group
	for i, a := range order {
		if used[a.TicketID] || !fits(a, n) {
			continue
		}
		group := []Candidate{a}
		players := a.Size()
		for j := i + 1; j < len(order) && players < 2*n; j++ {
			b := order[j]
			if used[b.TicketID] || !fits(b, n) || players+b.Size() > 2*n {
				continue
			}
			if ok, _ := c.compatible(k, a, b, env); !ok {
				continue
			}
			if !c.fitsGroup(k, group, b, env) {
				continue
			}
			next := append(slices.Clone(group), b)
			if !assignable(next, n) {
				continue
			}
			group = next
			players += b.Size()
		}
		if players != 2*n {
			continue
		}
		teams, ok := Draft(group, n)
		if !ok {
			continue
		}
		for _, t := range group {
			used[t.TicketID] = true
		}
		out = append(out, Group{Tickets: group, Teams: teams})
	}
	return out
}

// fits reports a ticket usable for teams of n: a party at most n, exactly n without allow_fill.
func fits(t Candidate, n int) bool {
	if t.PartyID == nil {
		return t.Size() == 1
	}
	return t.Size() <= n && (t.AllowFill || t.Size() == n)
}

// fitsGroup checks H1 and H6 of b against every ticket already in the group.
func (c Config) fitsGroup(k QueueKey, group []Candidate, b Candidate, env Env) bool {
	for _, g := range group {
		if !c.GradeCompatible(k.Type, g.Grade, b.Grade, env.Now.Sub(g.EnqueuedAt), env.Now.Sub(b.EnqueuedAt)) || env.blocked(g, b) {
			return false
		}
	}
	return true
}

// assignable reports whether the tickets can still fill two teams of n: at most two parties, room for the solos.
func assignable(group []Candidate, n int) bool {
	parties, solos, room := 0, 0, 2*n
	for _, t := range group {
		if t.PartyID != nil {
			parties++
			room -= t.Size()
		} else {
			solos++
		}
	}
	return parties <= 2 && solos <= room
}

// Draft assigns the tickets of a full group to two teams of n (MATCHMAKING §7 Team Assignment): each party keeps its
// team, the solo tickets are drafted by rating, best first, in snake order (A1, B1, B2, A2…) starting with the weaker
// team and skipping full teams.
func Draft(group []Candidate, n int) ([2][]uuid.UUID, bool) {
	var teams [2][]uuid.UUID
	var sum [2]float64
	next := 0
	var solos []Candidate
	for _, t := range group {
		if t.PartyID == nil {
			solos = append(solos, t)
			continue
		}
		if next > 1 {
			return teams, false
		}
		teams[next] = append(teams[next], t.Students...)
		sum[next] += t.Rating.Mu * float64(t.Size())
		next++
	}
	sort.SliceStable(solos, func(i, j int) bool { return solos[i].Rating.Mu > solos[j].Rating.Mu })
	turn := 0
	if sum[1] < sum[0] {
		turn = 1
	}
	picks := 0
	for _, s := range solos {
		if len(teams[turn]) >= n {
			turn = 1 - turn
		}
		if len(teams[turn]) >= n {
			return teams, false
		}
		teams[turn] = append(teams[turn], s.Students...)
		picks++
		// snake: A, B, B, A, A, B… → switch after the first pick, then every two picks
		if picks%2 == 1 {
			turn = 1 - turn
		}
	}
	return teams, len(teams[0]) == n && len(teams[1]) == n
}
