package config

import (
	"errors"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// Defaults of CONFIG_DEFAULTS for the Safety Team operations (F124, owner MODERATION §6.1, §6.3).
const (
	DefaultBreakGlassMaxDurationHours = 4   // break_glass_max_duration_hours
	DefaultBreakGlassMaxWindowDays    = 30  // break_glass_max_window_days
	DefaultOncallShiftMaxHours        = 168 // oncall_shift_max_hours
)

// SafetyOps is the configuration of the on-call rotation and break-glass: shift ceiling, grant duration ceiling, ceiling of
// the message window of a grant.
type SafetyOps struct {
	ShiftMax           time.Duration
	BreakGlassDuration time.Duration
	BreakGlassWindow   time.Duration
}

// SafetyOpsFrom reads EW_ONCALL_SHIFT_MAX_HOURS, EW_BREAK_GLASS_MAX_DURATION_HOURS and EW_BREAK_GLASS_MAX_WINDOW_DAYS
// (defaults of CONFIG_DEFAULTS). Values may only be tightened: a duration above 4 hours or a window above 30 days is refused
// (the OpenAPI maximum of duration_minutes is 240).
func SafetyOpsFrom(c *config.Config) (SafetyOps, error) {
	shift := c.Int("EW_ONCALL_SHIFT_MAX_HOURS", DefaultOncallShiftMaxHours)
	dur := c.Int("EW_BREAK_GLASS_MAX_DURATION_HOURS", DefaultBreakGlassMaxDurationHours)
	win := c.Int("EW_BREAK_GLASS_MAX_WINDOW_DAYS", DefaultBreakGlassMaxWindowDays)
	if shift < 1 || dur < 1 || dur > DefaultBreakGlassMaxDurationHours || win < 1 || win > DefaultBreakGlassMaxWindowDays {
		return SafetyOps{}, errors.New("chat config: EW_ONCALL_SHIFT_MAX_HOURS ≥ 1, EW_BREAK_GLASS_MAX_DURATION_HOURS 1..4, EW_BREAK_GLASS_MAX_WINDOW_DAYS 1..30")
	}
	return SafetyOps{ShiftMax: time.Duration(shift) * time.Hour, BreakGlassDuration: time.Duration(dur) * time.Hour,
		BreakGlassWindow: time.Duration(win) * 24 * time.Hour}, nil
}
