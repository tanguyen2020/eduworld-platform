package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/config"
	pkgconfig "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// T001: defaults are the CONFIG_DEFAULTS values; variables override them; malformed values are named, never echoed.
func TestFrom(t *testing.T) {
	c := config.Defaults("gw-1")
	if c.Heartbeat != 15*time.Second || c.Timeout != 45*time.Second || c.Ping != 20*time.Second || c.MaxFrame != 16384 ||
		c.ResumeWindow != 120*time.Second || c.ResumeMax != 500 || c.CmdTimeout != 2*time.Second || c.MaxConnections != 4000 ||
		c.AcceptRate != 200 || c.FrameRate != 30 || c.FrameBurst != 60 || c.MoveRate != 12 || c.Inflight != 8 || c.SendQueue != 256 {
		t.Fatalf("defaults %+v", c)
	}
	env := map[string]string{"EW_NODE_ID": "gw-2", "EW_WS_HEARTBEAT": "10s", "EW_CORS_ALLOWED_ORIGINS": "https://a.example, https://b.example"}
	c, err := config.From(pkgconfig.FromGetenv(func(k string) string { return env[k] }))
	if err != nil || c.NodeID != "gw-2" || c.Heartbeat != 10*time.Second || len(c.AllowedOrigins) != 2 {
		t.Fatalf("%+v %v", c, err)
	}
	env = map[string]string{"EW_NODE_ID": "gw.bad", "EW_WS_DRAIN_WINDOW": "45s", "EW_WS_SEND_QUEUE": "0"}
	_, err = config.From(pkgconfig.FromGetenv(func(k string) string { return env[k] }))
	if err == nil || !strings.Contains(err.Error(), "EW_NODE_ID") || !strings.Contains(err.Error(), "EW_WS_DRAIN_WINDOW") ||
		!strings.Contains(err.Error(), "EW_WS_SEND_QUEUE") || strings.Contains(err.Error(), "gw.bad") {
		t.Fatalf("invalid config: %v", err)
	}
}
