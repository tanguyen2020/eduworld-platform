// Package progression is the level side of the game progression (F074, PROGRESSION_SYSTEM §4): the versioned XP curve,
// the level table it generates, the titles by level range and the level-up rule with floor protection. Pure domain: no
// I/O, no clock.
package progression

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
)

// ErrInvalid is a curve that cannot be published (F074-AC-23); FieldError says which field.
var ErrInvalid = errors.New("invalid progression curve")

// FieldError is a validation failure of one field (JSON pointer style path, no value).
type FieldError struct {
	Field  string
	Reason string
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Reason }
func (e *FieldError) Unwrap() error { return ErrInvalid }

func invalid(field, format string, args ...any) error {
	return &FieldError{Field: field, Reason: fmt.Sprintf(format, args...)}
}

// Formula is xp_to_next(L) = round(base × L^exponent + linear × L).
type Formula struct {
	Kind     string  `json:"kind"`
	Base     float64 `json:"base"`
	Exponent float64 `json:"exponent"`
	Linear   float64 `json:"linear"`
}

// XPToNext is the XP needed to go from level l to l+1 by the formula.
func (f Formula) XPToNext(l int) int64 {
	return int64(math.Round(f.Base*math.Pow(float64(l), f.Exponent) + f.Linear*float64(l)))
}

// Override replaces the formula for one level.
type Override struct {
	Level    int   `json:"level"`
	XPToNext int64 `json:"xp_to_next"`
}

// Title is a level title from FromLevel up to the next title (PROGRESSION_SYSTEM §4.3); Name holds vi and en.
type Title struct {
	Code      string            `json:"code"`
	FromLevel int               `json:"from_level"`
	Name      map[string]string `json:"name"`
}

// Reward is one level reward (progression-curve.schema.json $defs.reward), applied by a registered grant applier.
type Reward map[string]any

// Type is the reward type (currency, item, badge, title, collection_item).
func (r Reward) Type() string {
	s, _ := r["type"].(string)
	return s
}

// LevelRewards are the rewards of one level.
type LevelRewards struct {
	Level   int      `json:"level"`
	Rewards []Reward `json:"rewards"`
}

// Curve is one version of the platform XP curve (specs/schemas/progression-curve.schema.json).
type Curve struct {
	CurveID         string         `json:"curve_id"`
	Code            string         `json:"code"`
	MaxLevel        int            `json:"max_level"`
	Formula         Formula        `json:"formula"`
	Overrides       []Override     `json:"level_overrides"`
	Titles          []Title        `json:"titles"`
	LevelRewards    []LevelRewards `json:"level_rewards"`
	FloorProtection *bool          `json:"level_floor_protection"`
}

// ParseCurve reads an authored or stored curve payload.
func ParseCurve(raw []byte) (Curve, error) {
	var c Curve
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, invalid("payload", "not a progression curve object")
	}
	return c, nil
}

// Protected reports level_floor_protection (default true).
func (c Curve) Protected() bool { return c.FloorProtection == nil || *c.FloorProtection }

// Validate checks what the JSON Schema cannot (F074-AC-23): overrides and rewards inside max_level and unique, XP to
// the next level non-decreasing ("thresholds increase monotonically"), a title from level 1 with unique start levels,
// and every reward type served by a registered grant applier (hasApplier).
func (c Curve) Validate(hasApplier func(rewardType string) bool) error {
	if c.MaxLevel < 2 || c.MaxLevel > 200 {
		return invalid("max_level", "must be 2..200")
	}
	if c.Formula.Kind != "power_linear" || c.Formula.Base < 1 || c.Formula.Exponent < 0.5 || c.Formula.Exponent > 3 || c.Formula.Linear < 0 {
		return invalid("formula", "power_linear with base ≥ 1, exponent 0.5..3, linear ≥ 0")
	}
	seen := map[int]bool{}
	for i, o := range c.Overrides {
		switch {
		case o.Level < 1 || o.Level >= c.MaxLevel:
			return invalid(fmt.Sprintf("level_overrides/%d/level", i), "must be 1..max_level-1")
		case seen[o.Level]:
			return invalid(fmt.Sprintf("level_overrides/%d/level", i), "duplicate level")
		case o.XPToNext < 1:
			return invalid(fmt.Sprintf("level_overrides/%d/xp_to_next", i), "must be ≥ 1")
		}
		seen[o.Level] = true
	}
	var prev int64
	for l := 1; l < c.MaxLevel; l++ {
		x := c.xpToNext(l)
		if x < 1 || x < prev {
			return invalid("level_overrides", "XP to the next level must not decrease (level %d)", l)
		}
		prev = x
	}
	if len(c.Titles) == 0 {
		return invalid("titles", "at least one title")
	}
	starts := map[int]bool{}
	codes := map[string]bool{}
	for i, t := range c.Titles {
		switch {
		case t.FromLevel < 1 || t.FromLevel > c.MaxLevel:
			return invalid(fmt.Sprintf("titles/%d/from_level", i), "must be 1..max_level")
		case starts[t.FromLevel]:
			return invalid(fmt.Sprintf("titles/%d/from_level", i), "duplicate start level")
		case codes[t.Code] || t.Code == "":
			return invalid(fmt.Sprintf("titles/%d/code", i), "empty or duplicate title code")
		case t.Name["vi"] == "" || t.Name["en"] == "":
			return invalid(fmt.Sprintf("titles/%d/name", i), "vi and en are required")
		}
		starts[t.FromLevel], codes[t.Code] = true, true
	}
	if !starts[1] {
		return invalid("titles", "one title must start at level 1")
	}
	rewarded := map[int]bool{}
	for i, lr := range c.LevelRewards {
		switch {
		case lr.Level < 2 || lr.Level > c.MaxLevel:
			return invalid(fmt.Sprintf("level_rewards/%d/level", i), "must be 2..max_level")
		case rewarded[lr.Level]:
			return invalid(fmt.Sprintf("level_rewards/%d/level", i), "duplicate level")
		}
		rewarded[lr.Level] = true
		for j, r := range lr.Rewards {
			if hasApplier == nil || !hasApplier(r.Type()) {
				return invalid(fmt.Sprintf("level_rewards/%d/rewards/%d/type", i, j), "no grant applier for reward type %q", r.Type())
			}
		}
	}
	return nil
}

func (c Curve) xpToNext(l int) int64 {
	for _, o := range c.Overrides {
		if o.Level == l {
			return o.XPToNext
		}
	}
	return c.Formula.XPToNext(l)
}

// Level is one row of the level table (game.progression_levels).
type Level struct {
	Level      int
	XPRequired *int64 // XP from this level to the next; nil at max_level
	Cumulative int64  // XP to reach this level
	TitleCode  string
	Rewards    []Reward
}

// Table is the level table of a curve version.
type Table struct {
	CurveID         string
	Version         int
	MaxLevel        int
	FloorProtection bool
	Levels          []Level // index = level-1
	Titles          []Title // sorted by FromLevel
}

// Table generates the level table (cumulative thresholds, titles, rewards) of the curve published as version.
func (c Curve) Table(version int) Table {
	titles := slices.Clone(c.Titles)
	sort.Slice(titles, func(i, j int) bool { return titles[i].FromLevel < titles[j].FromLevel })
	t := Table{CurveID: c.CurveID, Version: version, MaxLevel: c.MaxLevel, FloorProtection: c.Protected(), Titles: titles}
	rewards := map[int][]Reward{}
	for _, lr := range c.LevelRewards {
		rewards[lr.Level] = lr.Rewards
	}
	var cum int64
	for l := 1; l <= c.MaxLevel; l++ {
		lv := Level{Level: l, Cumulative: cum, TitleCode: t.titleAt(l).Code, Rewards: rewards[l]}
		if l < c.MaxLevel {
			x := c.xpToNext(l)
			lv.XPRequired = &x
			cum += x
		}
		t.Levels = append(t.Levels, lv)
	}
	return t
}

func (t Table) titleAt(level int) Title {
	var out Title
	for _, ti := range t.Titles {
		if ti.FromLevel <= level {
			out = ti
		}
	}
	return out
}

// TitleAt is the title of a level: the title with the greatest start level ≤ level (F074-AC-15).
func (t Table) TitleAt(level int) Title { return t.titleAt(level) }

// Level returns the row of level l (clamped to 1..max_level).
func (t Table) Level(l int) Level {
	if len(t.Levels) == 0 {
		return Level{Level: 1}
	}
	l = max(1, min(l, len(t.Levels)))
	return t.Levels[l-1]
}

// LevelFor is the greatest level whose cumulative XP is ≤ xp.
func (t Table) LevelFor(xp int64) int {
	i := sort.Search(len(t.Levels), func(i int) bool { return t.Levels[i].Cumulative > xp })
	return max(1, i)
}

// Apply is the level reached with xpTotal from the current level (F074-AC-13, AC-16, AC-21): never below the current
// level when floor protection is on, capped at max_level; reached lists every level gained in order (one level.reached
// each).
func (t Table) Apply(xpTotal int64, current int) (level int, reached []int) {
	level = t.LevelFor(xpTotal)
	if t.FloorProtection && level < current {
		level = current
	}
	level = min(level, t.MaxLevel)
	for l := current + 1; l <= level; l++ {
		reached = append(reached, l)
	}
	return level, reached
}

// Status is the progress bar of a level (cumulative semantics, F074-AC-17): XP earned in the level and XP still missing
// to the next one (nil at max_level). With floor protection a raised curve can leave xp_total under the level's
// threshold: xp_in_level is then 0 and xp_to_next counts from xp_total to the next threshold of the new curve.
func (t Table) Status(level int, xpTotal int64) (xpInLevel int64, xpToNext *int64) {
	lv := t.Level(level)
	xpInLevel = max(0, xpTotal-lv.Cumulative)
	if lv.XPRequired == nil || level >= t.MaxLevel {
		return xpInLevel, nil
	}
	next := max(0, lv.Cumulative+*lv.XPRequired-xpTotal)
	if lv.XPRequired != nil {
		xpInLevel = min(xpInLevel, *lv.XPRequired)
	}
	return xpInLevel, &next
}
