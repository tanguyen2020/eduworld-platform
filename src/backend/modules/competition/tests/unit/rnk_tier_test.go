package unit

import (
	"errors"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/ranking"
)

// F106-AC-08: tiers come from the Ranking Rules (thresholds and names, not hardcoded): promotion at the threshold,
// demotion only below the lower threshold minus the hysteresis (50), no flip-flop around a threshold.
func TestTierThresholdsAndHysteresis(t *testing.T) {
	tiers := seedRules(t).Tiers
	for _, c := range []struct {
		rating  float64
		current string
		want    string
	}{
		{1000, "", "T1"}, {1299.99, "", "T1"}, {1300, "", "T2"}, {1450, "", "T3"}, {1599, "", "T3"}, {1600, "", "T4"},
		{1750, "", "T5"}, {1900, "", "T6"}, {2500, "", "T6"},
		// hysteresis: in T4 since 1600, stays T4 down to 1550 inclusive
		{1580, "T4", "T4"}, {1550, "T4", "T4"}, {1549.9, "T4", "T3"},
		// promotion is immediate
		{1601, "T3", "T4"},
		// a fall of two bands demotes straight to the raw tier
		{1400, "T4", "T2"},
	} {
		if got := tiers.TierOf(c.rating, c.current); got != c.want {
			t.Fatalf("TierOf(%v, %q) = %s, want %s", c.rating, c.current, got, c.want)
		}
	}
	// oscillating around 1600 never flips back and forth once promoted
	cur := tiers.TierOf(1605, "T3")
	for _, r := range []float64{1595, 1603, 1580, 1610, 1560} {
		cur = tiers.TierOf(r, cur)
		if cur != "T4" {
			t.Fatalf("flip-flop at %v: %s", r, cur)
		}
	}
	if tiers.Name("T3", "vi-VN") != "Chồi Non" || tiers.Name("T3", "en-US") != "Young Bud" || tiers.Name("T3", "fr-FR") != "Chồi Non" {
		t.Fatal("tier names per locale")
	}
	if !tiers.Higher("T5", "T2") || tiers.Higher("T2", "T2") || tiers.Peak("T4", "T3") != "T4" || tiers.Peak("T3", "T5") != "T5" {
		t.Fatal("tier order/peak")
	}
}

// F106-AC-08: rules with tiers out of order, or a personal rank for grades 1–3, are refused (never published).
func TestRulesValidation(t *testing.T) {
	r := seedRules(t)
	bad := r
	bad.Tiers.Levels = append([]ranking.TierLevel(nil), r.Tiers.Levels...)
	x := 1200.0
	bad.Tiers.Levels[3].MinRating = &x
	if err := bad.Validate(); !errors.Is(err, ranking.ErrRules) {
		t.Fatalf("tiers out of order accepted: %v", err)
	}
	g := r
	g.Visibility.G13.PersonalRank = true
	if err := g.Validate(); !errors.Is(err, ranking.ErrRules) {
		t.Fatal("a personal rank for grades 1–3 accepted")
	}
}
