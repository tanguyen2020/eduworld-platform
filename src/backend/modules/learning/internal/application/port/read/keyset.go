package read

import "github.com/google/uuid"

// Seek is the keyset position of one list page (API_DESIGN §5, owner rule 2026-11-23 "pagination in the query, never
// in memory"): the read model returns at most Limit rows ordered by the list's sort columns then the id, strictly after
// After. The query layer asks limit+1 rows to know has_more. Limit ≤ 0 is only for the bounded internal reads (a
// version's whole tree for an import diff, the applied versions of a student): never for a list operation.
type Seek struct {
	After *Key
	Limit int
}

// Key is the position after which a page starts: the sort values of the last row of the previous page, in the order of
// the list's ORDER BY, and its id (the tie breaker).
type Key struct {
	Vals []any
	ID   uuid.UUID
}
