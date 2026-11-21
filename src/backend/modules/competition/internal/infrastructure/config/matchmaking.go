package config

import (
	"fmt"
	"os"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// Deployment settings of matchmaking (F104 T002). The rules (windows, timeouts, rates) are the published matchmaking
// configuration (competition.game_config_versions, platform only), never environment variables.
const (
	DefaultMMTick      = time.Second      // EW_MM_TICK: matchmaker pass (MATCHMAKING §6: every 1 s; local 500 ms)
	DefaultMMLockTTL   = 5 * time.Second  // per-queue lock (VALKEY_DESIGN ew:mm:lock: 5 s)
	DefaultMMConfigTTL = 30 * time.Second // cache of the published configuration
)

// Matchmaking are the matchmaking settings of a binary.
type Matchmaking struct {
	Tick     time.Duration
	Settings command.MatchmakingSettings
}

// MatchmakingFrom reads EW_MM_TICK, EW_MM_LOCK_TTL and EW_MM_CONFIG_TTL; the lock owner is EW_INSTANCE_ID (default the
// host name).
func MatchmakingFrom(c *config.Config) (Matchmaking, error) {
	owner := c.String("EW_INSTANCE_ID")
	if owner == "" {
		owner, _ = os.Hostname()
	}
	m := Matchmaking{Tick: c.Duration("EW_MM_TICK", DefaultMMTick), Settings: command.MatchmakingSettings{Owner: owner,
		LockTTL: c.Duration("EW_MM_LOCK_TTL", DefaultMMLockTTL), ConfigTTL: c.Duration("EW_MM_CONFIG_TTL", DefaultMMConfigTTL)}}
	switch {
	case m.Tick < 100*time.Millisecond || m.Tick > 10*time.Second:
		return m, fmt.Errorf("competition matchmaking config: invalid EW_MM_TICK")
	case m.Settings.LockTTL < m.Tick:
		return m, fmt.Errorf("competition matchmaking config: EW_MM_LOCK_TTL below EW_MM_TICK")
	case m.Settings.ConfigTTL < 0:
		return m, fmt.Errorf("competition matchmaking config: invalid EW_MM_CONFIG_TTL")
	}
	if m.Settings.Owner == "" {
		m.Settings.Owner = "competition"
	}
	return m, nil
}
