package ranking

// IsProvisional is rating_provisional (§7.4): fewer games than MinGames or RD above MaxRD. A provisional rating has no
// tier and is in no rating leaderboard; matchmaking still uses μ/RD (F106-AC-05).
func (r Rules) IsProvisional(games int, rd float64) bool {
	return games < r.Provisional.MinGames || rd > r.Provisional.MaxRD
}

// index returns the position of the tier code (-1 when unknown).
func (t Tiers) index(code string) int {
	for i, l := range t.Levels {
		if l.Code == code {
			return i
		}
	}
	return -1
}

// raw is the tier of a rating without hysteresis.
func (t Tiers) raw(rating float64) int {
	k := 0
	for i, l := range t.Levels {
		if l.MinRating == nil || rating >= *l.MinRating {
			k = i
		}
	}
	return k
}

// TierOf is the tier of a rating given the current tier (F106-AC-08): promotion at the threshold, demotion only when the
// rating falls below the lower threshold of the current tier by more than the hysteresis. current "" = no tier yet.
func (t Tiers) TierOf(rating float64, current string) string {
	if len(t.Levels) == 0 {
		return ""
	}
	k := t.raw(rating)
	c := t.index(current)
	if c > k && t.Levels[c].MinRating != nil && rating >= *t.Levels[c].MinRating-t.Hysteresis {
		k = c
	}
	return t.Levels[k].Code
}

// Rank is the order of a tier (0 = lowest, -1 unknown): "beat a higher tier" (Competition Points upset).
func (t Tiers) Rank(code string) int { return t.index(code) }

// TierFor is the plain tier of a rating (no hysteresis): the tier of an opponent or a virtual opponent.
func (t Tiers) TierFor(rating float64) string {
	if len(t.Levels) == 0 {
		return ""
	}
	return t.Levels[t.raw(rating)].Code
}

// Name is the display name of a tier for a locale (fallback: vi-VN, then any name, then the code).
func (t Tiers) Name(code, locale string) string {
	i := t.index(code)
	if i < 0 {
		return code
	}
	n := t.Levels[i].Names
	if s, ok := n[locale]; ok {
		return s
	}
	if s, ok := n["vi-VN"]; ok {
		return s
	}
	for _, s := range n {
		return s
	}
	return code
}

// Higher reports whether tier a is above tier b (unknown tiers never are).
func (t Tiers) Higher(a, b string) bool {
	ia, ib := t.index(a), t.index(b)
	return ia >= 0 && ib >= 0 && ia > ib
}

// Peak is the higher of two tiers (peak_tier_code).
func (t Tiers) Peak(peak, now string) string {
	if peak == "" || t.Higher(now, peak) {
		return now
	}
	return peak
}
