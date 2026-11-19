package config

import (
	"strings"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

func TestSessionFrom(t *testing.T) {
	t.Setenv("EW_GAME_MIN_CLIENT_VERSION", "1.2.0")
	c, err := SessionFrom(config.Load())
	if err != nil {
		t.Fatal(err)
	}
	u := c.UseCase
	if u.MinClientVersion != "1.2.0" || u.HeartbeatTimeout != DefaultHeartbeatTimeout || u.Maintenance.Active || u.PlatformCurfew.Zero() ||
		c.ClientConfig.LatestClientVersion != "1.2.0" || !c.ClientConfig.RememberUsernameDefault ||
		strings.Join(c.ClientConfig.SupportedLocales, ",") != DefaultLocales || c.IPHashKey != nil || c.TimeoutScan != DefaultTimeoutScan {
		t.Fatalf("defaults: %+v", c)
	}
	t.Setenv("EW_GAME_LATEST_CLIENT_VERSION", "1.3.0")
	t.Setenv("EW_GAME_SESSION_HEARTBEAT_TIMEOUT", "30s")
	t.Setenv("EW_GAME_MAINTENANCE", "true")
	t.Setenv("EW_GAME_MAINTENANCE_MESSAGE", "Bảo trì")
	t.Setenv("EW_GAME_MAINTENANCE_UNTIL", "2026-11-19T10:00:00Z")
	t.Setenv("EW_GAME_IP_HASH_KEY", strings.Repeat("k", 32))
	t.Setenv("EW_PLATFORM_CURFEW", "22:00-06:00")
	c, err = SessionFrom(config.Load())
	if err != nil {
		t.Fatal(err)
	}
	if c.UseCase.HeartbeatTimeout != 30*time.Second || !c.UseCase.Maintenance.Active || c.UseCase.Maintenance.Until == nil ||
		!c.ClientConfig.Maintenance().Active || c.ClientConfig.LatestClientVersion != "1.3.0" || len(c.IPHashKey) != 32 ||
		c.UseCase.PlatformCurfew.Start != 22*60 {
		t.Fatalf("overrides: %+v", c)
	}
	for k, v := range map[string]string{"EW_GAME_IP_HASH_KEY": "short-secret-value", "EW_PLATFORM_CURFEW": "late",
		"EW_GAME_LATEST_CLIENT_VERSION": "1.0.0", "EW_GAME_MAINTENANCE_UNTIL": "tomorrow"} {
		old := config.Load().String(k)
		t.Setenv(k, v)
		if _, err := SessionFrom(config.Load()); err == nil || strings.Contains(err.Error(), v) {
			t.Fatalf("%s=%s: %v", k, v, err)
		}
		t.Setenv(k, old)
	}
	t.Setenv("EW_GAME_MIN_CLIENT_VERSION", "")
	if _, err := SessionFrom(config.Load()); err == nil || !strings.Contains(err.Error(), "EW_GAME_MIN_CLIENT_VERSION") {
		t.Fatalf("missing: %v", err)
	}
}
