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
