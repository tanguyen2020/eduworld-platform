package curriculum

import (
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"
)

func ids(n int) []uuid.UUID {
	out := make([]uuid.UUID, n)
	for i := range out {
		out[i] = uuid.New()
	}
	return out
}

// isCycle checks that path is a closed walk of g (first = last, each step a prerequisite edge).
func isCycle(g PrerequisiteDAG, path []uuid.UUID) bool {
	if len(path) < 2 || path[0] != path[len(path)-1] {
		return false
	}
	for i := 0; i+1 < len(path); i++ {
		if !slices.Contains(g[path[i]], path[i+1]) {
			return false
		}
	}
	return true
}

// F021-AC-12, SC-003: cycles are detected — short, long (≥ 5 LOs) and self — and returned as a path.
func TestDetectCycle(t *testing.T) {
	n := ids(6)
	a, b, c := n[0], n[1], n[2]
	g := PrerequisiteDAG{a: {b}, b: {c}, c: nil}
	if err := g.DetectCycle(); err != nil {
		t.Fatalf("A→B→C is acyclic: %v", err)
	}
	g2 := g.With(c, []uuid.UUID{a})
	var ce CycleError
	if err := g2.DetectCycle(); !errors.As(err, &ce) || !isCycle(g2, ce.Path) || len(ce.Path) != 4 {
		t.Fatalf("C→A: %v", err)
	}
	if g[c] != nil {
		t.Fatal("With must not modify the graph")
	}
	// Long cycle of 6 LOs.
	long := PrerequisiteDAG{}
	for i := range n {
		long[n[i]] = []uuid.UUID{n[(i+1)%len(n)]}
	}
	if err := long.DetectCycle(); !errors.As(err, &ce) || len(ce.Path) != 7 || !isCycle(long, ce.Path) {
		t.Fatalf("6-cycle: %v", err)
	}
	// Self loop.
	if err := (PrerequisiteDAG{a: {a}}).DetectCycle(); !errors.As(err, &ce) || len(ce.Path) != 2 {
		t.Fatalf("self loop: %v", err)
	}
	// Diamond is acyclic.
	d := ids(4)
	diamond := PrerequisiteDAG{d[0]: {d[1], d[2]}, d[1]: {d[3]}, d[2]: {d[3]}}
	if err := diamond.DetectCycle(); err != nil {
		t.Fatalf("diamond: %v", err)
	}
	if got := diamond.Dependents(d[3]); len(got) != 2 {
		t.Fatalf("dependents of the sink: %v", got)
	}
}
