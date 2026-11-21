package analytics

import "sort"

// Cell is one cell of an aggregated report: its group (the row whose total is shown, e.g. a school or a tenant) and its
// number of students.
type Cell struct {
	Group    string
	Students int
}

// Suppress returns, per cell, whether it is hidden by k-anonymity (F043-AC-25, analytics_min_cell_size): a cell with fewer
// than k students is hidden; when exactly one cell of a group of several is hidden, the smallest other cell of that group is
// hidden too (complementary suppression), so that a total cannot reveal the hidden cell. Totals must be computed from the
// visible cells only.
func Suppress(cells []Cell, k int) []bool {
	hidden := make([]bool, len(cells))
	groups := map[string][]int{}
	for i, c := range cells {
		if c.Students < k {
			hidden[i] = true
		}
		groups[c.Group] = append(groups[c.Group], i)
	}
	keys := make([]string, 0, len(groups))
	for g := range groups {
		keys = append(keys, g)
	}
	sort.Strings(keys)
	for _, g := range keys {
		idx := groups[g]
		n := 0
		for _, i := range idx {
			if hidden[i] {
				n++
			}
		}
		if n != 1 || len(idx) < 2 {
			continue
		}
		best := -1
		for _, i := range idx {
			if !hidden[i] && (best < 0 || cells[i].Students < cells[best].Students) {
				best = i
			}
		}
		if best >= 0 {
			hidden[best] = true
		}
	}
	return hidden
}

// GradeMask is the k-anonymity of the grades of one school (ADR-0095, FR-ANL-016): a grade not listed has no student and
// is hidden.
type GradeMask struct {
	hidden map[int]bool
}

// Hidden reports whether the cells of grade g are hidden (fewer than k students, complementary suppression, unknown grade).
func (m GradeMask) Hidden(g int) bool {
	h, ok := m.hidden[g]
	return !ok || h
}

// Visible lists the visible grades in ascending order (never nil: an empty list filters every grade out).
func (m GradeMask) Visible() []int {
	out := []int{}
	for g, h := range m.hidden {
		if !h {
			out = append(out, g)
		}
	}
	sort.Ints(out)
	return out
}

// SuppressGrades applies Suppress to the grades of one school (one group, so that the school total of the visible grades
// cannot reveal a hidden one): sizes maps a grade to its number of students; the unknown grade 0 is always hidden.
func SuppressGrades(sizes map[int]int, k int) GradeMask {
	gs := make([]int, 0, len(sizes))
	for g := range sizes {
		gs = append(gs, g)
	}
	sort.Ints(gs)
	cells := make([]Cell, len(gs))
	for i, g := range gs {
		n := sizes[g]
		if g == 0 {
			n = 0
		}
		cells[i] = Cell{Group: "school", Students: n}
	}
	hidden := Suppress(cells, k)
	m := GradeMask{hidden: make(map[int]bool, len(gs))}
	for i, g := range gs {
		m.hidden[g] = hidden[i]
	}
	return m
}
