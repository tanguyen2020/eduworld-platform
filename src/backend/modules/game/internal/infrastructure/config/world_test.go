package config

import (
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// T002: the world parameters come from the configuration with the CONFIG_DEFAULTS values.
func TestWorldFrom(t *testing.T) {
	c, err := WorldFrom(config.Load(), 45*time.Second)
	if err != nil || c.DiscoveryRate != 30 || c.PresenceTTL != 45*time.Second || c.DiscoveryWindow != time.Minute || c.InteractRadius != 3 {
		t.Fatalf("defaults: %+v %v", c, err)
	}
	t.Setenv("EW_GAME_DISCOVERY_REPORT_RATE", "0")
	if _, err := WorldFrom(config.Load(), 45*time.Second); err == nil {
		t.Fatal("rate 0 must be refused")
	}
}
