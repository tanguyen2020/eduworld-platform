package season

// Threshold is one tier of the track simulation (simulateGameConfig thresholds[], F079-AC-15): the cumulative season
// points that reach the tier and the points from it to the next tier (0 at the last tier).
type Threshold struct {
	Tier   int
	ToNext int
	Total  int
}

// Simulable checks what the simulation of a track version needs: points.per_tier and points.daily_cap at least 1 and
// at least one tier (a FieldError names the field).
func (t Track) Simulable() error {
	switch {
	case t.Points.PerTier < 1:
		return invalid("points.per_tier", "at least 1")
	case t.Points.DailyCap < 1:
		return invalid("points.daily_cap", "at least 1")
	case len(t.Tiers) == 0:
		return invalid("tiers", "at least one tier")
	}
	return nil
}

// LevelAt is the tier reported by the simulation for points: TierFor (a tier every per_tier points, at most the last
// tier), at least 1 — the level field of the simulation starts at 1 like a curve level.
func (t Track) LevelAt(points int64) int {
	return max(1, t.TierFor(int(points)))
}

// Thresholds are the tiers 1..n with their cumulative points (tier × per_tier).
func (t Track) Thresholds() []Threshold {
	out := make([]Threshold, 0, len(t.Tiers))
	for tier := 1; tier <= t.MaxTier(); tier++ {
		next := t.Points.PerTier
		if tier == t.MaxTier() {
			next = 0
		}
		out = append(out, Threshold{Tier: tier, ToNext: next, Total: tier * t.Points.PerTier})
	}
	return out
}
