package season

import (
	"errors"
	"slices"
	"testing"
)

// F079-AC-15: the season track simulation — a tier every per_tier points, at most the last tier, reported from tier 1;
// thresholds are the cumulative points of each tier; a track without points or tiers names the field.
func TestTrackSimulation(t *testing.T) {
	tr := track() // 30 tiers × 100 points, 100 points/day
	tr.Tiers = tr.Tiers[:3]
	if err := tr.Simulable(); err != nil {
		t.Fatal(err)
	}
	for pts, tier := range map[int64]int{0: 1, 99: 1, 100: 1, 199: 1, 200: 2, 300: 3, 10_000: 3} {
		if got := tr.LevelAt(pts); got != tier {
			t.Errorf("LevelAt(%d) = %d, want %d", pts, got, tier)
		}
	}
	want := []Threshold{{Tier: 1, ToNext: 100, Total: 100}, {Tier: 2, ToNext: 100, Total: 200}, {Tier: 3, ToNext: 0, Total: 300}}
	if got := tr.Thresholds(); !slices.Equal(got, want) {
		t.Fatalf("thresholds %+v, want %+v", got, want)
	}
	for field, edit := range map[string]func(*Track){
		"points.per_tier":  func(x *Track) { x.Points.PerTier = 0 },
		"points.daily_cap": func(x *Track) { x.Points.DailyCap = 0 },
		"tiers":            func(x *Track) { x.Tiers = nil },
	} {
		bad := tr
		edit(&bad)
		var fe *FieldError
		if err := bad.Simulable(); !errors.As(err, &fe) || fe.Field != field {
			t.Errorf("%s: %v", field, err)
		}
	}
}
