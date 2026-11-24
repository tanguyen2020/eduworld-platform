package tournament

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/tournament/bracket"
)

// NodeID derives the id of a node from its tournament and key: generating a bracket again yields the same ids
// (idempotent bracket, F107-AC-14).
func NodeID(tournamentID uuid.UUID, k bracket.Key) uuid.UUID {
	return uuid.NewSHA1(tournamentID, []byte("node/"+k.String()))
}

// GroupID derives the id of a group from its tournament and code.
func GroupID(tournamentID uuid.UUID, code string) uuid.UUID {
	return uuid.NewSHA1(tournamentID, []byte("group/"+code))
}

// Build turns generated nodes into graph nodes: seeds become entries (seeds[i] = entry of seed i+1), keys become ids,
// group codes become group ids, scheduled_at = starts + wave × gap. The graph is then resolved (round-1 byes move on).
func Build(tournamentID uuid.UUID, gen []bracket.Node, seeds []uuid.UUID, starts time.Time, gap time.Duration) *Graph {
	entry := func(seed int) *uuid.UUID {
		if seed < 1 || seed > len(seeds) {
			return nil
		}
		e := seeds[seed-1]
		return &e
	}
	nodes := make([]*Node, 0, len(gen))
	for _, g := range gen {
		at := starts.Add(time.Duration(g.Wave) * gap)
		n := &Node{ID: NodeID(tournamentID, g.Key), Bracket: g.Key.Bracket, Round: g.Key.Round, Position: g.Key.Position,
			A: entry(g.SeedA), B: entry(g.SeedB), Wave: g.Wave, ScheduledAt: &at, Status: NodePending, Changed: true}
		if g.Group != "" {
			id := GroupID(tournamentID, g.Group)
			n.GroupID = &id
		}
		if g.NextWin != nil {
			id := NodeID(tournamentID, *g.NextWin)
			n.NextWin, n.NextWinSlot = &id, g.NextWinSlot
		}
		if g.NextLose != nil {
			id := NodeID(tournamentID, *g.NextLose)
			n.NextLose, n.NextLoseSlot = &id, g.NextLoseSlot
		}
		nodes = append(nodes, n)
	}
	gr := NewGraph(nodes)
	gr.Resolve()
	return gr
}

// ErrUnknownFormat: a format outside Formats.
var ErrUnknownFormat = errors.New("tournament: unknown format")

// Generate builds the bracket of a format for seeded entries (F107-AC-11, AC-12, AC-22, AC-23): the nodes and, for a
// round robin or a group stage, the groups (code → seeds). It refuses fewer entries than the format needs.
func Generate(format string, n int) ([]bracket.Node, map[string][]int, error) {
	if n < MinEntries(format) {
		return nil, nil, bracket.ErrTooFew
	}
	switch format {
	case FormatSingleElim:
		nodes, err := bracket.SingleElim(n)
		return nodes, nil, err
	case FormatDoubleElim:
		nodes, err := bracket.DoubleElim(n)
		return nodes, nil, err
	case FormatRoundRobin:
		nodes, err := bracket.RoundRobin(n, bracket.GroupCode(0), 0, 0)
		seeds := make([]int, n)
		for i := range seeds {
			seeds[i] = i + 1
		}
		return nodes, map[string][]int{bracket.GroupCode(0): seeds}, err
	case FormatGroups:
		nodes, groups, err := bracket.GroupStage(n)
		out := map[string][]int{}
		for g, s := range groups {
			out[bracket.GroupCode(g)] = s
		}
		return nodes, out, err
	}
	return nil, nil, ErrUnknownFormat
}
