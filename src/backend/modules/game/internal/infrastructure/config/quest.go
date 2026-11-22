package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// Defaults of CONFIG_DEFAULTS for the quest engine (owner QUEST_SYSTEM §2.2, §2.4, §3.1; LEARNING_EXPERIENCE §2.1).
const (
	DefaultMaxActiveQuests   = 10      // max_active_quests
	DefaultDailyQuests       = 3       // daily_quests_per_day
	DefaultWeeklyQuests      = 3       // weekly_quests_per_week
	DefaultQuestResetTime    = "04:00" // quest_reset_time (ICT; weekly: Monday at the same time)
	DefaultPassRatio         = 0.7     // quest_pass_rule: correct ratio
	DefaultPassMinQuestions  = 8       // quest_pass_rule: minimum questions
	DefaultPassAttemptsDaily = 5       // quest_pass_rule: attempts per day
	DefaultBlockG13          = 3       // learning_block_max_items g1_3
	DefaultBlockG45          = 5       // learning_block_max_items g4_5
	DefaultBlockG69          = 8       // learning_block_max_items g6_9
	LearningRetryAfter       = 5 * time.Second
	QuestJobBatch            = 500
	QuestJobInterval         = time.Minute // quest-period-expiry and quest-failure-timer scan [Proposed]
)

// ict is the time zone of quest_reset_time (Asia/Ho_Chi_Minh, UTC+7, no DST).
var ict = time.FixedZone("ICT", 7*3600)

// QuestFrom reads the F072 configuration (T003): EW_GAME_MAX_ACTIVE_QUESTS, EW_GAME_DAILY_QUESTS_PER_DAY,
// EW_GAME_WEEKLY_QUESTS_PER_WEEK, EW_GAME_QUEST_RESET_TIME (HH:MM ICT) with the CONFIG_DEFAULTS values; the pass rule
// and the learning blocks are the platform defaults of CONFIG_DEFAULTS (a quest overrides its pass rule per step).
func QuestFrom(c *config.Config) (command.QuestConfig, error) {
	out := command.QuestConfig{MaxActive: c.Int("EW_GAME_MAX_ACTIVE_QUESTS", DefaultMaxActiveQuests),
		DailyPerDay: c.Int("EW_GAME_DAILY_QUESTS_PER_DAY", DefaultDailyQuests), WeeklyPerWeek: c.Int("EW_GAME_WEEKLY_QUESTS_PER_WEEK", DefaultWeeklyQuests),
		PassRule:      quest.DefaultPassRule{MinCorrectRatio: DefaultPassRatio, MinQuestions: DefaultPassMinQuestions, MaxAttemptsPerDay: DefaultPassAttemptsDaily},
		BlockMax:      map[string]int{"g1_3": DefaultBlockG13, "g4_5": DefaultBlockG45, "g6_9": DefaultBlockG69},
		LearningRetry: LearningRetryAfter, JobBatch: QuestJobBatch, JobInterval: QuestJobInterval}
	reset, err := ParseReset(c.StringOr("EW_GAME_QUEST_RESET_TIME", DefaultQuestResetTime))
	if err != nil {
		return out, err
	}
	out.Reset = reset
	switch {
	case out.MaxActive < 1 || out.MaxActive > 50:
		return out, errors.New("game config: EW_GAME_MAX_ACTIVE_QUESTS must be 1..50")
	case out.DailyPerDay < 0 || out.DailyPerDay > 10:
		return out, errors.New("game config: EW_GAME_DAILY_QUESTS_PER_DAY must be 0..10")
	case out.WeeklyPerWeek < 0 || out.WeeklyPerWeek > 10:
		return out, errors.New("game config: EW_GAME_WEEKLY_QUESTS_PER_WEEK must be 0..10")
	}
	return out, nil
}

// ParseReset reads a quest_reset_time HH:MM (ICT).
func ParseReset(s string) (quest.Reset, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return quest.Reset{}, fmt.Errorf("game config: EW_GAME_QUEST_RESET_TIME %q must be HH:MM: %w", s, err)
	}
	return quest.Reset{Hour: t.Hour(), Minute: t.Minute(), Location: ict}, nil
}
