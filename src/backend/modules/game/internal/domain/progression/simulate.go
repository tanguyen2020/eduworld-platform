package progression

// DailyXP is the XP of one play day in the progression simulation of the console (simulateGameConfig, F079-AC-10):
// min(minutes × minuteRate, dailyCap) — minuteRate is xp_simulation_minute_rate, dailyCap is xp_daily_cap.
func DailyXP(minutes, minuteRate, dailyCap int) int64 {
	return min(int64(minutes)*int64(minuteRate), int64(dailyCap))
}

// SimulationTable checks the level part of a curve version (max_level, formula, level_overrides — not the titles or
// rewards a draft may still lack) and returns its level table: the thresholds of the simulation and LevelFor (capped at
// max_level). A FieldError names what is missing.
func (c Curve) SimulationTable() (Table, error) {
	if err := c.validateLevels(); err != nil {
		return Table{}, err
	}
	return c.Table(0), nil
}
