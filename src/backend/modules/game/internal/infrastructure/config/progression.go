package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/reward"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// Defaults of CONFIG_DEFAULTS for the progression (owner PROGRESSION_SYSTEM §3.2). The XP of each source lives in the
// published reward rules (xp_objective_mastered = the seed rule mastery.objective_mastered); the curve (xp_curve,
// max_level, level_titles_at, level_floor_protection) is the published progression curve.
const (
	DefaultXPDailyCap          = 1000           // xp_daily_cap (XP/day, ICT calendar day)
	DefaultXPRepeatMultipliers = "1,0.5,0.25,0" // xp_repeat_multipliers (last one for every later completion)
	DefaultXPRepeatWindowDays  = 7              // xp_repeat_multipliers window (days, per content version)
	DefaultXPBelowGrade        = 0.25           // xp_below_grade_multiplier
	DefaultXPBelowGradeGap     = 2              // xp_below_grade_multiplier: content ≥ 2 grades below the student
	// DefaultXPSimulationPerMinute is xp_simulation_minute_rate (XP per active minute of the progression simulation of the
	// console, F079-AC-10): PROGRESSION_SYSTEM §4.1 targets 20–30 minutes/day ≈ 300–400 XP/day.
	DefaultXPSimulationPerMinute = 14
	ProgressionJobBatch          = 500
	CurveRolloutInterval         = time.Minute // curve-rollout scan [Proposed]
	XPReconcileInterval          = 24 * time.Hour
)

// ProgressionFrom reads the F074 policy (T002): EW_GAME_XP_DAILY_CAP, EW_GAME_XP_REPEAT_MULTIPLIERS (comma list),
// EW_GAME_XP_REPEAT_WINDOW_DAYS, EW_GAME_XP_BELOW_GRADE_MULTIPLIER with the CONFIG_DEFAULTS values [Proposed: the
// platform settings pipeline of module identity carries no progression key yet — same pattern as the quest config of
// F072].
func ProgressionFrom(c *config.Config) (reward.Policy, error) {
	p := reward.Policy{DailyCap: c.Int("EW_GAME_XP_DAILY_CAP", DefaultXPDailyCap), BelowGradeGap: DefaultXPBelowGradeGap,
		Repeat: reward.Diminishing{WindowDays: c.Int("EW_GAME_XP_REPEAT_WINDOW_DAYS", DefaultXPRepeatWindowDays)}}
	ms, err := ParseMultipliers(c.StringOr("EW_GAME_XP_REPEAT_MULTIPLIERS", DefaultXPRepeatMultipliers))
	if err != nil {
		return p, err
	}
	p.Repeat.Multipliers = ms
	below, err := strconv.ParseFloat(c.StringOr("EW_GAME_XP_BELOW_GRADE_MULTIPLIER", strconv.FormatFloat(DefaultXPBelowGrade, 'f', -1, 64)), 64)
	if err != nil || below < 0 || below > 1 {
		return p, errors.New("game config: EW_GAME_XP_BELOW_GRADE_MULTIPLIER must be 0..1")
	}
	p.BelowGradeMultiplier = below
	switch {
	case p.DailyCap < 1 || p.DailyCap > 100000:
		return p, errors.New("game config: EW_GAME_XP_DAILY_CAP must be 1..100000")
	case p.Repeat.WindowDays < 1 || p.Repeat.WindowDays > 90:
		return p, errors.New("game config: EW_GAME_XP_REPEAT_WINDOW_DAYS must be 1..90")
	}
	return p, nil
}

// ParseMultipliers reads a comma list of multipliers in 0..1 (1 to 10 values).
func ParseMultipliers(s string) ([]float64, error) {
	parts := strings.Split(s, ",")
	if len(parts) == 0 || len(parts) > 10 {
		return nil, errors.New("game config: EW_GAME_XP_REPEAT_MULTIPLIERS needs 1..10 values")
	}
	out := make([]float64, 0, len(parts))
	for _, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil || f < 0 || f > 1 {
			return nil, fmt.Errorf("game config: EW_GAME_XP_REPEAT_MULTIPLIERS value %q must be 0..1", p)
		}
		out = append(out, f)
	}
	return out, nil
}

// XPSimulationRateFrom reads EW_GAME_XP_SIMULATION_PER_MINUTE (xp_simulation_minute_rate, 1..1000, CONFIG_DEFAULTS value
// when unset) — the XP per active minute simulateGameConfig assumes for a progression curve (F079-AC-10).
func XPSimulationRateFrom(c *config.Config) (int, error) {
	rate := c.Int("EW_GAME_XP_SIMULATION_PER_MINUTE", DefaultXPSimulationPerMinute)
	if rate < 1 || rate > 1000 {
		return rate, errors.New("game config: EW_GAME_XP_SIMULATION_PER_MINUTE must be 1..1000")
	}
	return rate, nil
}
