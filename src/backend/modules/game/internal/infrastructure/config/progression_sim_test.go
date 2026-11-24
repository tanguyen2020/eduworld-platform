package config

import (
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// F079-AC-10: xp_simulation_minute_rate comes from EW_GAME_XP_SIMULATION_PER_MINUTE with the CONFIG_DEFAULTS value (14).
func TestXPSimulationRateFrom(t *testing.T) {
	if r, err := XPSimulationRateFrom(config.Load()); err != nil || r != 14 {
		t.Fatalf("default: %d %v", r, err)
	}
	t.Setenv("EW_GAME_XP_SIMULATION_PER_MINUTE", "20")
	if r, err := XPSimulationRateFrom(config.Load()); err != nil || r != 20 {
		t.Fatalf("override: %d %v", r, err)
	}
	for _, v := range []string{"0", "1001"} {
		t.Setenv("EW_GAME_XP_SIMULATION_PER_MINUTE", v)
		if _, err := XPSimulationRateFrom(config.Load()); err == nil {
			t.Fatalf("%s accepted", v)
		}
	}
}
