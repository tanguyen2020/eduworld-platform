package read

import (
	"entgo.io/ent/dialect/sql"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
)

// seekAfter is the keyset predicate of a list page (API_DESIGN §5): (cols..., idCol) > (vals..., id) for a list in
// ascending order, < for a list in descending order. The ent query keeps its interceptors (tenant scope, deleted_at IS
// NULL — R8, R21, R27); nil when the page is the first one.
func seekAfter(after *read.Key, desc bool, idCol string, cols ...string) func(*sql.Selector) {
	if after == nil {
		return nil
	}
	return func(s *sql.Selector) {
		columns := s.Columns(append(append([]string{}, cols...), idCol)...)
		args := append(append([]any{}, after.Vals...), after.ID)
		if desc {
			s.Where(sql.CompositeLT(columns, args...))
			return
		}
		s.Where(sql.CompositeGT(columns, args...))
	}
}

// limitOf is the LIMIT of a keyset read (0 = the bounded internal reads of read.Seek).
func limitOf(seek read.Seek) int {
	if seek.Limit <= 0 {
		return 0
	}
	return seek.Limit
}

// seekArgs are the arguments of a keyset condition written in SQL: the n sort values and the id of the position, all
// nil on the first page (the SQL tests the first one for NULL), then the LIMIT (nil = no limit, the bounded reads).
func seekArgs(seek read.Seek, n int) []any {
	out := make([]any, n+2)
	if seek.After != nil && len(seek.After.Vals) == n {
		copy(out, seek.After.Vals)
		out[n] = seek.After.ID
	}
	if seek.Limit > 0 {
		out[n+1] = seek.Limit
	}
	return out
}
