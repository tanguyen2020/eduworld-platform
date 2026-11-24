package config

import (
	"fmt"
	"time"

	trn "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/tournament"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// DefaultTournamentTick is the period of the job tournament-scheduler (EW_TOURNAMENT_TICK, F107 plan: 15 s).
const DefaultTournamentTick = 15 * time.Second

// DefaultTournamentTimers are tournament_timers of CONFIG_DEFAULTS (COMPETITION_SYSTEM §10.4, §10.7): check-in 15 min,
// no-show 3 min, complaint 10 min, reschedule after 2 min at most twice. The published template carries them (timers);
// these values only complete a template that omits one. They are rules, never environment variables.
var DefaultTournamentTimers = trn.Timers{CheckIn: 15 * time.Minute, NoShow: 3 * time.Minute, Complaint: 10 * time.Minute,
	Reschedule: 2 * time.Minute, RescheduleMax: 2}

// Tournament are the tournament settings of a binary.
type Tournament struct {
	Tick   time.Duration
	Timers trn.Timers
}

// TournamentFrom reads EW_TOURNAMENT_TICK (5 s – 5 min).
func TournamentFrom(c *config.Config) (Tournament, error) {
	t := Tournament{Tick: c.Duration("EW_TOURNAMENT_TICK", DefaultTournamentTick), Timers: DefaultTournamentTimers}
	if t.Tick < 5*time.Second || t.Tick > 5*time.Minute {
		return t, fmt.Errorf("competition tournament config: invalid EW_TOURNAMENT_TICK")
	}
	return t, nil
}
