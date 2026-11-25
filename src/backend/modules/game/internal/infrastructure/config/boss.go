package config

import (
	"errors"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// F103 parameters (CONFIG_DEFAULTS §4, §5; owners CHALLENGE_BOSS_SYSTEM, QUEST_SYSTEM).
const (
	// DefaultBossReconnectGrace is boss_reconnect_grace (120 s): ready deadline of a new encounter and reconnect grace of
	// a boss version without reconnect_grace_seconds.
	DefaultBossReconnectGrace = 120 * time.Second
	// DefaultCoopQuestReconnectGrace is coop_quest_reconnect_grace (120 s).
	DefaultCoopQuestReconnectGrace = 120 * time.Second
	// BossRoomType and PartyQuestRoomType are the room types of the encounters and shared quest sessions on the room host
	// of module game (same orphan index as the challenge rooms).
	BossRoomType       = "boss"
	PartyQuestRoomType = "party_quest"
	// BossRoomCheckEvery is the period of the room checks (presence of the members, deadlines) of a live encounter or
	// session: a member offline is noticed within it (the grace starts then).
	BossRoomCheckEvery = 5 * time.Second
	// BossPlanRetries are the retries of the question plans of a round before the encounter is abandoned (edge case of
	// the spec: 2, without counting an attempt of the students).
	BossPlanRetries = 2
	// BossQuestionCount caps the questions of one requirement plan (the activity gives its natural count, ≤ 30).
	BossQuestionCount = 10
)

// BossFrom reads the F103 boss configuration: EW_GAME_BOSS_RECONNECT_GRACE (boss_reconnect_grace), the counters of
// challenge_attempts_per_day and quest_reset_time shared with the solo challenges (F076), the heartbeat timeout of the
// game sessions (presence).
func BossFrom(c *config.Config, ch command.ChallengeConfig, heartbeat time.Duration) (command.BossConfig, error) {
	out := command.BossConfig{Grace: c.Duration("EW_GAME_BOSS_RECONNECT_GRACE", DefaultBossReconnectGrace), CheckEvery: BossRoomCheckEvery,
		PlanRetries: BossPlanRetries, AttemptsPerDay: ch.Rules.AttemptsPerDay, HeartbeatTimeout: heartbeat, Reset: ch.Reset,
		QuestionCount: BossQuestionCount}
	if out.Grace < 10*time.Second || out.Grace > 10*time.Minute {
		return out, errors.New("game config: EW_GAME_BOSS_RECONNECT_GRACE must be 10s..10m")
	}
	return out, nil
}

// PartyQuestFrom reads the F103 co-op quest configuration: EW_GAME_COOP_QUEST_RECONNECT_GRACE
// (coop_quest_reconnect_grace) and the heartbeat timeout of the game sessions (presence).
func PartyQuestFrom(c *config.Config, heartbeat time.Duration) (command.PartyQuestConfig, error) {
	out := command.PartyQuestConfig{Grace: c.Duration("EW_GAME_COOP_QUEST_RECONNECT_GRACE", DefaultCoopQuestReconnectGrace),
		CheckEvery: BossRoomCheckEvery, HeartbeatTimeout: heartbeat}
	if out.Grace < 10*time.Second || out.Grace > 10*time.Minute {
		return out, errors.New("game config: EW_GAME_COOP_QUEST_RECONNECT_GRACE must be 10s..10m")
	}
	return out, nil
}
