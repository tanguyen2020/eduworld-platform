package hub

import "math"

// CellSize is the side of an interest grid cell in metres (REALTIME_ARCHITECTURE §6 [Confirmed]).
const CellSize = 16.0

// Cell is an interest grid cell on the ground plane (x, z).
type Cell struct{ X, Z int }

// CellOf returns the cell of p.
func CellOf(p Vec3) Cell {
	return Cell{X: int(math.Floor(p[0] / CellSize)), Z: int(math.Floor(p[2] / CellSize))}
}

// Near reports that b is the cell a or one of its 8 neighbours.
func (a Cell) Near(b Cell) bool {
	dx, dz := a.X-b.X, a.Z-b.Z
	return dx >= -1 && dx <= 1 && dz >= -1 && dz <= 1
}

// Entity is the state of one avatar in a snapshot (AsyncAPI HubEntity): eid, position, yaw, animation only.
type Entity struct {
	EID int     `json:"eid"`
	P   Vec3    `json:"p"`
	R   float64 `json:"r"`
	A   string  `json:"a,omitempty"`
}

// Delta computes the snapshot of one viewer (F091-AC-14): the entities in the viewer's cell and its 8 neighbours that
// are new or changed since last, and the eids that left the area of interest (or the instance). It returns the new
// view state of the viewer. The viewer itself is never in the list.
func Delta(viewer Entity, all []Entity, last map[int]Entity) (changed []Entity, removed []int, next map[int]Entity) {
	vc := CellOf(viewer.P)
	next = make(map[int]Entity, len(last))
	for _, e := range all {
		if e.EID == viewer.EID || !vc.Near(CellOf(e.P)) {
			continue
		}
		next[e.EID] = e
		if prev, ok := last[e.EID]; !ok || prev != e {
			changed = append(changed, e)
		}
	}
	for eid := range last {
		if _, ok := next[eid]; !ok {
			removed = append(removed, eid)
		}
	}
	return changed, removed, next
}
