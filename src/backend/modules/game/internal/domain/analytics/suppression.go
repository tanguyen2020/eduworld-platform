package analytics

import "sort"

// Cell population of a suppression group: Key identifies the cell (a dims value), Players its distinct players.
type Population struct {
	Key     string
	Players int64
}

// Suppress returns the keys of the hidden cells of one group (all values of one dimension set in one bucket) under
// analytics_min_cell_size k (GAME_ANALYTICS A-5, DASHBOARD_DATA §8 rule 7, F080-AC-16): a cell with fewer than k players
// is hidden; when exactly one cell of a group of several is hidden, the smallest visible cell is hidden too (complementary
// suppression) so that the visible total of the scope minus the visible cells never reveals a hidden cell. The group must
// always be the WHOLE dimension, never only the filtered values, so that two queries with different filters cannot be
// differenced either.
func Suppress(cells []Population, k int) map[string]bool {
	hidden := map[string]bool{}
	for _, c := range cells {
		if c.Players < int64(k) {
			hidden[c.Key] = true
		}
	}
	if len(hidden) == 1 && len(cells) > 1 {
		vis := make([]Population, 0, len(cells))
		for _, c := range cells {
			if !hidden[c.Key] {
				vis = append(vis, c)
			}
		}
		sort.Slice(vis, func(i, j int) bool {
			if vis[i].Players != vis[j].Players {
				return vis[i].Players < vis[j].Players
			}
			return vis[i].Key < vis[j].Key
		})
		if len(vis) > 0 {
			hidden[vis[0].Key] = true
		}
	}
	return hidden
}

// Small reports whether a single cell (a scope total, a cohort, a funnel step) is below k.
func Small(players int64, k int) bool { return players < int64(k) }
