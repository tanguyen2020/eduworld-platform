package progression

import (
	"errors"
	"testing"
)

func defaultCurve() Curve {
	return Curve{CurveID: "c", Code: "progression.default", MaxLevel: 60, Formula: Formula{Kind: "power_linear", Base: 100, Exponent: 1.35, Linear: 20},
		Titles: []Title{
			{Code: "explorer", FromLevel: 10, Name: map[string]string{"vi": "Nhà thám hiểm", "en": "Explorer"}},
			{Code: "beginner", FromLevel: 1, Name: map[string]string{"vi": "Tân binh", "en": "Beginner"}},
			{Code: "adventurer", FromLevel: 20, Name: map[string]string{"vi": "Người phiêu lưu", "en": "Adventurer"}},
			{Code: "champion", FromLevel: 35, Name: map[string]string{"vi": "Nhà vô địch", "en": "Champion"}},
			{Code: "master", FromLevel: 50, Name: map[string]string{"vi": "Bậc thầy", "en": "Master"}},
		},
		LevelRewards: []LevelRewards{{Level: 10, Rewards: []Reward{{"type": "title", "title_code": "explorer"}}}}}
}

// PROGRESSION_SYSTEM §4.1 table: XP to the next level and cumulative XP of the default curve.
func TestDefaultCurveTable(t *testing.T) {
	tb := defaultCurve().Table(1)
	for _, c := range []struct {
		level     int
		next, cum int64
	}{{1, 120, 0}, {5, 978, 1646}, {6, 1243, 2624}, {10, 2439, 9328}, {11, 2766, 11767}} {
		lv := tb.Level(c.level)
		if lv.XPRequired == nil || *lv.XPRequired != c.next || lv.Cumulative != c.cum {
			t.Fatalf("level %d: next %v cum %d, want %d %d", c.level, lv.XPRequired, lv.Cumulative, c.next, c.cum)
		}
	}
	if tb.Level(60).XPRequired != nil || len(tb.Levels) != 60 {
		t.Fatalf("max level row: %+v, %d rows", tb.Level(60), len(tb.Levels))
	}
}

// F074-AC-13: crossing two thresholds at once reaches each level in order.
func TestApplyMultipleLevels(t *testing.T) {
	tb := defaultCurve().Table(1)
	level, reached := tb.Apply(2624, 4) // exactly level 6
	if level != 6 || len(reached) != 2 || reached[0] != 5 || reached[1] != 6 {
		t.Fatalf("got %d %v", level, reached)
	}
	if level, reached = tb.Apply(2623, 5); level != 5 || len(reached) != 0 {
		t.Fatalf("below threshold: %d %v", level, reached)
	}
}

// F074-AC-15: titles by level range (unsorted input), bilingual names.
func TestTitles(t *testing.T) {
	tb := defaultCurve().Table(1)
	for level, code := range map[int]string{1: "beginner", 9: "beginner", 10: "explorer", 19: "explorer", 20: "adventurer", 34: "adventurer",
		35: "champion", 50: "master", 60: "master"} {
		if got := tb.TitleAt(level); got.Code != code || got.Name["vi"] == "" || got.Name["en"] == "" {
			t.Fatalf("level %d: %+v", level, got)
		}
		if tb.Level(level).TitleCode != code {
			t.Fatalf("level row %d title %s", level, tb.Level(level).TitleCode)
		}
	}
}

// F074-AC-16: at max_level XP keeps counting, xp_to_next is nil, no level is reached.
func TestMaxLevel(t *testing.T) {
	c := defaultCurve()
	c.MaxLevel = 3
	c.Titles = c.Titles[1:2]
	c.LevelRewards = nil
	tb := c.Table(1)
	level, reached := tb.Apply(1_000_000, 3)
	if level != 3 || len(reached) != 0 {
		t.Fatalf("max: %d %v", level, reached)
	}
	if in, next := tb.Status(3, 1_000_000); next != nil || in == 0 {
		t.Fatalf("status at max: %d %v", in, next)
	}
}

// F074-AC-21: a raised curve keeps the level (floor protection) and counts xp_to_next from the new thresholds; without
// protection the level follows the XP.
func TestFloorProtection(t *testing.T) {
	v1 := defaultCurve().Table(1)
	xp := v1.Level(12).Cumulative + 10
	raised := defaultCurve()
	raised.Formula.Base = 200
	v2 := raised.Table(2)
	level, reached := v2.Apply(xp, 12)
	if level != 12 || len(reached) != 0 {
		t.Fatalf("protected: %d %v", level, reached)
	}
	in, next := v2.Status(12, xp)
	if in != 0 || next == nil || *next != v2.Level(13).Cumulative-xp {
		t.Fatalf("status v2: %d %v", in, next)
	}
	off := false
	raised.FloorProtection = &off
	if level, _ := raised.Table(2).Apply(xp, 12); level >= 12 {
		t.Fatalf("unprotected level %d", level)
	}
}

// F074-AC-23: what the JSON Schema cannot check is refused with the failing field.
func TestValidate(t *testing.T) {
	titleOnly := func(typ string) bool { return typ == "title" || typ == "xp" }
	if err := defaultCurve().Validate(titleOnly); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(c *Curve){
		"level_overrides":         func(c *Curve) { c.Overrides = []Override{{Level: 5, XPToNext: 10}} },
		"level_overrides/0/level": func(c *Curve) { c.Overrides = []Override{{Level: 60, XPToNext: 99999}} },
		"titles":                  func(c *Curve) { c.Titles = c.Titles[:1] },
		"titles/1/from_level":     func(c *Curve) { c.Titles[1].FromLevel = 10 },
		"level_rewards/0/rewards/0/type": func(c *Curve) {
			c.LevelRewards[0].Rewards[0] = Reward{"type": "currency", "currency_code": "xu", "amount": 5}
		},
		"level_rewards/0/level": func(c *Curve) { c.LevelRewards[0].Level = 61 },
		"max_level":             func(c *Curve) { c.MaxLevel = 1 },
	}
	for field, edit := range cases {
		c := defaultCurve()
		edit(&c)
		err := c.Validate(titleOnly)
		var fe *FieldError
		if !errors.As(err, &fe) || fe.Field != field || !errors.Is(err, ErrInvalid) {
			t.Fatalf("%s: %v", field, err)
		}
	}
}
