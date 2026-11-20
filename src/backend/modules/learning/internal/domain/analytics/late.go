package analytics

import "time"

// LateAction is what the rollup consumer does with an event of a day (DASHBOARD_DATA §5, F043-AC-04).
type LateAction int

// Actions of Late.
const (
	Apply        LateAction = iota // day not closed yet: add to the day
	ApplyReclose                   // day closed, event within the late window: add and mark the day for reclosing
	Drop                           // day closed, event later than the window: rollups unchanged, metric only
)

// Late decides the action for an event that occurred at occurredAt, processed at now, on a day whose closing state is
// dayFinal.
func Late(occurredAt, now time.Time, window time.Duration, dayFinal bool) LateAction {
	if !dayFinal {
		return Apply
	}
	if now.Sub(occurredAt) <= window {
		return ApplyReclose
	}
	return Drop
}
