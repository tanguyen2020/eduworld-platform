package progression

import (
	"errors"
	"testing"
)

// F079-AC-10: XP of a play day = min(minutes × xp_simulation_minute_rate, xp_daily_cap).
func TestDailyXP(t *testing.T) {
	for _, c := range []struct {
		minutes, rate, limit int
		want                 int64
	}{{25, 14, 1000, 350}, {120, 14, 1000, 1000}, {1, 14, 1000, 14}, {240, 1000, 100000, 100000}} {
		if got := DailyXP(c.minutes, c.rate, c.limit); got != c.want {
			t.Errorf("DailyXP(%d, %d, %d) = %d, want %d", c.minutes, c.rate, c.limit, got, c.want)
		}
	}
}

// F079-AC-10: the simulation table is the level table of the curve (formula + level_overrides); a draft without titles
// or rewards simulates; a broken level part names its field; the level is capped at max_level.
func TestSimulationTable(t *testing.T) {
	c := Curve{MaxLevel: 4, Formula: Formula{Kind: "power_linear", Base: 50, Exponent: 1, Linear: 0}, Overrides: []Override{{Level: 2, XPToNext: 120}}}
	tb, err := c.SimulationTable()
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		next *int64
		cum  int64
	}{{p(50), 0}, {p(120), 50}, {p(150), 170}, {nil, 320}}
	for i, w := range want {
		lv := tb.Level(i + 1)
		if lv.Cumulative != w.cum || (w.next == nil) != (lv.XPRequired == nil) || (w.next != nil && *lv.XPRequired != *w.next) {
			t.Fatalf("level %d: %+v", i+1, lv)
		}
	}
	for xp, lvl := range map[int64]int{0: 1, 49: 1, 50: 2, 169: 2, 170: 3, 320: 4, 1_000_000: 4} {
		if got := tb.LevelFor(xp); got != lvl {
			t.Errorf("LevelFor(%d) = %d, want %d", xp, got, lvl)
		}
	}
	for field, bad := range map[string]Curve{
		"max_level":                    {MaxLevel: 1, Formula: c.Formula},
		"formula":                      {MaxLevel: 4},
		"level_overrides/0/level":      {MaxLevel: 4, Formula: c.Formula, Overrides: []Override{{Level: 4, XPToNext: 10}}},
		"level_overrides":              {MaxLevel: 4, Formula: c.Formula, Overrides: []Override{{Level: 2, XPToNext: 10}}},
		"level_overrides/0/xp_to_next": {MaxLevel: 4, Formula: c.Formula, Overrides: []Override{{Level: 1, XPToNext: 0}}},
	} {
		_, err := bad.SimulationTable()
		var fe *FieldError
		if !errors.As(err, &fe) || fe.Field != field {
			t.Errorf("%s: %v", field, err)
		}
	}
}

func p(v int64) *int64 { return &v }
