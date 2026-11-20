package hub

import "time"

// Movement validation (REALTIME_ARCHITECTURE §6, F091-AC-12): the distance travelled may not exceed
// max_speed_mps × Δt × 1.2 and every position must lie inside the area bounds. Δt is server time (R5), never the
// client's. To absorb network jitter the allowance is a budget that accrues at max_speed × 1.2 per second, capped at
// one second of travel, and each accepted move spends its distance [Proposed F091 A-3]: over any window the accepted
// distance stays ≤ max_speed × Δt × 1.2 + one second of slack.
const (
	SpeedTolerance = 1.2
	budgetWindow   = time.Second
	// ViolationLimit is the number of violations per minute above which an integrity signal is written
	// (REALTIME_ARCHITECTURE §6: > 20/min, never an automatic sanction — F091-AC-13).
	ViolationLimit  = 20
	violationWindow = time.Minute
)

// Correction reasons of hub.player.corrected.
const (
	ReasonSpeed       = "speed"
	ReasonOutOfBounds = "out_of_bounds"
)

// Mover is the server-side movement state of one member.
type Mover struct {
	Pos        Vec3
	budget     float64
	lastAt     time.Time
	violations []time.Time
	signalled  time.Time
}

// NewMover places a member at p.
func NewMover(p Vec3, now time.Time, maxSpeed float64) Mover {
	return Mover{Pos: p.Quantize(), budget: maxSpeed * SpeedTolerance * budgetWindow.Seconds(), lastAt: now}
}

// Move validates next against the last valid position. ok = false leaves Pos unchanged (the client is corrected to it).
func (m *Mover) Move(next Vec3, now time.Time, area Area) (ok bool, reason string) {
	rate := area.MaxSpeedMPS * SpeedTolerance
	if dt := now.Sub(m.lastAt).Seconds(); dt > 0 {
		m.budget += rate * dt
		if limit := rate * budgetWindow.Seconds(); m.budget > limit {
			m.budget = limit
		}
	}
	m.lastAt = now
	if !next.Finite() || !area.Contains(next) {
		return false, ReasonOutOfBounds
	}
	next = next.Quantize()
	d := m.Pos.Dist(next)
	if d > m.budget+1e-9 {
		return false, ReasonSpeed
	}
	m.budget -= d
	m.Pos = next
	return true, ""
}

// Violation records one violation at now and returns the number in the last minute and whether an integrity signal
// is due (above ViolationLimit, at most one per minute).
func (m *Mover) Violation(now time.Time) (count int, signal bool) {
	cut := now.Add(-violationWindow)
	keep := m.violations[:0]
	for _, t := range m.violations {
		if t.After(cut) {
			keep = append(keep, t)
		}
	}
	m.violations = append(keep, now)
	count = len(m.violations)
	if count > ViolationLimit && now.Sub(m.signalled) >= violationWindow {
		m.signalled = now
		return count, true
	}
	return count, false
}
