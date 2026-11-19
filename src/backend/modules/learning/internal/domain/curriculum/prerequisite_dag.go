package curriculum

import (
	"slices"
	"strings"

	"github.com/google/uuid"
)

// CycleError reports a cycle of the prerequisite graph (F021-AC-12): the LOs on the cycle in order, the first
// repeated at the end (A → B → C → A).
type CycleError struct {
	Path []uuid.UUID
}

func (e CycleError) Error() string {
	parts := make([]string, len(e.Path))
	for i, id := range e.Path {
		parts[i] = id.String()
	}
	return "curriculum: prerequisite cycle " + strings.Join(parts, " -> ")
}

// PrerequisiteDAG is the prerequisite graph of one curriculum version: objective → its prerequisites.
type PrerequisiteDAG map[uuid.UUID][]uuid.UUID

// With returns a copy of the graph where id has the prerequisites prereqs (the change being validated).
func (g PrerequisiteDAG) With(id uuid.UUID, prereqs []uuid.UUID) PrerequisiteDAG {
	out := make(PrerequisiteDAG, len(g)+1)
	for k, v := range g {
		out[k] = v
	}
	out[id] = slices.Clone(prereqs)
	return out
}

// DetectCycle finds a cycle with a depth-first search in three colours (white/grey/black) and returns it as a
// CycleError, or nil when the graph is acyclic. Nodes are visited in id order so the result is deterministic.
func (g PrerequisiteDAG) DetectCycle() error {
	const (
		white = iota
		grey
		black
	)
	colour := make(map[uuid.UUID]int, len(g))
	var stack []uuid.UUID
	var found []uuid.UUID
	var visit func(n uuid.UUID) bool
	visit = func(n uuid.UUID) bool {
		colour[n] = grey
		stack = append(stack, n)
		next := slices.Clone(g[n])
		slices.SortFunc(next, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
		for _, m := range next {
			switch colour[m] {
			case grey:
				i := slices.Index(stack, m)
				found = append(slices.Clone(stack[i:]), m)
				return true
			case white:
				if visit(m) {
					return true
				}
			}
		}
		stack = stack[:len(stack)-1]
		colour[n] = black
		return false
	}
	nodes := make([]uuid.UUID, 0, len(g))
	for n := range g {
		nodes = append(nodes, n)
	}
	slices.SortFunc(nodes, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	for _, n := range nodes {
		if colour[n] == white && visit(n) {
			return CycleError{Path: found}
		}
	}
	return nil
}

// Dependents returns the objectives that list id as a prerequisite (deletion check, F021 edge case).
func (g PrerequisiteDAG) Dependents(id uuid.UUID) []uuid.UUID {
	var out []uuid.UUID
	for n, ps := range g {
		if n != id && slices.Contains(ps, id) {
			out = append(out, n)
		}
	}
	slices.SortFunc(out, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	return out
}
