package gameauth

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

func TestConfigFrom(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	t.Setenv("EW_GAME_MIN_CLIENT_VERSION", "1.2.0")
	t.Setenv("EW_GAME_LOGIN_TICKET_KEY", key)
	t.Setenv("EW_GAME_LOGIN_TICKET_TTL", "90s")
	c, err := ConfigFrom(config.Load())
	if err != nil {
		t.Fatal(err)
	}
	if c.UseCase.MinClientVersion != "1.2.0" || c.UseCase.TicketTTL != 90*time.Second || len(c.TicketKey) != 32 || c.Curfew != DefaultPlatformCurfew {
		t.Fatalf("config = %+v", c.UseCase)
	}
	t.Setenv("EW_GAME_LOGIN_TICKET_KEY", "short")
	if _, err := ConfigFrom(config.Load()); err == nil || strings.Contains(err.Error(), "short") {
		t.Fatalf("bad key: %v", err)
	}
	t.Setenv("EW_GAME_MIN_CLIENT_VERSION", "")
	if _, err := ConfigFrom(config.Load()); err == nil || !strings.Contains(err.Error(), "EW_GAME_MIN_CLIENT_VERSION") {
		t.Fatalf("missing: %v", err)
	}
}

func TestPlatformCurfewDefault(t *testing.T) {
	p, err := NewPlatformCurfew("")
	if err != nil || p.Curfew.Start != 21*60+30 || p.Curfew.End != 6*60+30 {
		t.Fatalf("curfew = %+v %v", p.Curfew, err)
	}
}
