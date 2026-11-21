package config

import (
	"fmt"
	"os"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// Deployment settings of the battle room host (F105 T002). The rules of the modes (questions, points, graces) are the
// published configuration pinned by the match, never environment variables.
const (
	DefaultBattleTickHz        = 10
	DefaultBattleCheckpoint    = 5 * time.Second // EW_BATTLE_ROOM_CHECKPOINT: checkpoint refresh besides every transition
	DefaultBattlePlanDeadline  = 3 * time.Second // EW_BATTLE_PLAN_DEADLINE: createQuestionPlan (selection + render)
	DefaultBattleScoreDeadline = 2 * time.Second // EW_BATTLE_SCORE_DEADLINE: scorePlanAnswer (module_call_deadline write)
)

// Battle are the battle settings of a binary.
type Battle struct {
	InstanceID    string
	TickHz        int
	Checkpoint    time.Duration
	PlanDeadline  time.Duration
	ScoreDeadline time.Duration
}

// BattleFrom reads EW_BATTLE_TICK_HZ, EW_BATTLE_ROOM_CHECKPOINT, EW_BATTLE_PLAN_DEADLINE and EW_BATTLE_SCORE_DEADLINE; the
// owner of the room leases is EW_INSTANCE_ID (default the host name).
func BattleFrom(c *config.Config) (Battle, error) {
	id := c.String("EW_INSTANCE_ID")
	if id == "" {
		id, _ = os.Hostname()
	}
	if id == "" {
		id = "competition"
	}
	b := Battle{InstanceID: id, TickHz: c.Int("EW_BATTLE_TICK_HZ", DefaultBattleTickHz),
		Checkpoint:    c.Duration("EW_BATTLE_ROOM_CHECKPOINT", DefaultBattleCheckpoint),
		PlanDeadline:  c.Duration("EW_BATTLE_PLAN_DEADLINE", DefaultBattlePlanDeadline),
		ScoreDeadline: c.Duration("EW_BATTLE_SCORE_DEADLINE", DefaultBattleScoreDeadline)}
	switch {
	case b.TickHz < 1 || b.TickHz > 30:
		return b, fmt.Errorf("competition battle config: invalid EW_BATTLE_TICK_HZ")
	case b.Checkpoint < time.Second || b.Checkpoint > time.Minute:
		return b, fmt.Errorf("competition battle config: invalid EW_BATTLE_ROOM_CHECKPOINT")
	case b.PlanDeadline < 100*time.Millisecond || b.ScoreDeadline < 100*time.Millisecond:
		return b, fmt.Errorf("competition battle config: invalid EW_BATTLE_PLAN_DEADLINE/EW_BATTLE_SCORE_DEADLINE")
	}
	return b, nil
}
