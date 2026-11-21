package config

import (
	"errors"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// Defaults of CONFIG_DEFAULTS for the world (owner WORLD_DESIGN §8, EVENT_DESIGN world.area_entered).
const (
	DefaultDiscoveryReportRate = 30          // discovery_report_rate: ≤ 30 per minute per student
	DiscoveryWindow            = time.Minute // window of discovery_report_rate
	AreaEnteredDedupe          = time.Minute // world.area_entered at most once per area per minute
	DefaultInteractRadius      = 3.0         // interact_radius_default (m) of an anchor without interact_radius_m [Proposed]
	DefaultInteractTolerance   = 1.5         // interact_distance_tolerance (m): hub position sampling [Proposed]
)

// WorldFrom reads the F071 configuration (T002): EW_GAME_DISCOVERY_REPORT_RATE (discovery_report_rate, 30 per minute,
// 1..120). The presence lives as long as the game session heartbeat (game_session_heartbeat_timeout, heartbeat).
func WorldFrom(c *config.Config, heartbeat time.Duration) (command.WorldConfig, error) {
	out := command.WorldConfig{PresenceTTL: heartbeat, DiscoveryRate: c.Int("EW_GAME_DISCOVERY_REPORT_RATE", DefaultDiscoveryReportRate),
		DiscoveryWindow: DiscoveryWindow, EnteredDedupe: AreaEnteredDedupe, InteractRadius: DefaultInteractRadius,
		InteractTolerance: DefaultInteractTolerance}
	switch {
	case out.DiscoveryRate < 1 || out.DiscoveryRate > 120:
		return out, errors.New("game config: EW_GAME_DISCOVERY_REPORT_RATE must be 1..120")
	case out.PresenceTTL < time.Second:
		return out, errors.New("game config: the presence TTL (game_session_heartbeat_timeout) must be ≥ 1s")
	}
	return out, nil
}
