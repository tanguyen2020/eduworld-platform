package config

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/challenge"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/roomhost"
)

// Defaults of CONFIG_DEFAULTS for the Challenge Engine (owner CHALLENGE_BOSS_SYSTEM §2.2, §2.4; ACTIVITY_ENGINE for
// attempt_resume_window and deadline_grace). difficulty_levels and g1_3_challenge_rules are platform constants of the
// owner table: a challenge version overrides them per profile (seconds_per_item, lives), never the client.
const (
	DefaultChallengeAttemptsPerDay = 5                // challenge_attempts_per_day
	DefaultChallengeFailCooldown   = 60 * time.Second // challenge_fail_cooldown (from Hard)
	DefaultAttemptResumeWindow     = 24 * time.Hour   // attempt_resume_window
	ChallengeDeadlineGrace         = 2 * time.Second  // deadline_grace (code)
	ChallengeG13TimeFactor         = 1.5              // g1_3_challenge_rules: time × 1.5
	ChallengeRegenStreak           = 3                // challenge_survival_regen_streak
	ChallengeDeadlineInterval      = 15 * time.Second // challenge-deadline scan (fallback of the room timer) [Proposed]
	ChallengeJobBatch              = 500
	ChallengeRoomType              = "challenge_solo"
	ChallengeRoomIndex             = "ew:rt:rooms:game" // orphan index of the rooms of module game (VALKEY_DESIGN §3.1)
)

// difficultyLevels is the table of difficulty_levels (CHALLENGE_BOSS_SYSTEM §2.2): survival lives and seconds per item.
var (
	difficultyLives   = map[string]int{challenge.Easy: 5, challenge.Normal: 4, challenge.Hard: 3, challenge.Expert: 3, challenge.Legendary: 2}
	difficultySeconds = map[string]int{challenge.Easy: 40, challenge.Normal: 30, challenge.Hard: 25, challenge.Expert: 20, challenge.Legendary: 20}
)

// ChallengeFrom reads the F076 configuration (T003): EW_GAME_CHALLENGE_ATTEMPTS_PER_DAY, EW_GAME_CHALLENGE_FAIL_COOLDOWN,
// EW_GAME_ATTEMPT_RESUME_WINDOW and EW_GAME_QUEST_RESET_TIME (day and week of the attempt counters) with the
// CONFIG_DEFAULTS values.
func ChallengeFrom(c *config.Config) (command.ChallengeConfig, error) {
	reset, err := ParseReset(c.StringOr("EW_GAME_QUEST_RESET_TIME", DefaultQuestResetTime))
	if err != nil {
		return command.ChallengeConfig{}, err
	}
	out := command.ChallengeConfig{Reset: reset, LearningRetry: LearningRetryAfter, JobBatch: ChallengeJobBatch,
		Rules: challenge.Rules{DefaultLives: difficultyLives, DefaultSeconds: difficultySeconds,
			AttemptsPerDay: c.Int("EW_GAME_CHALLENGE_ATTEMPTS_PER_DAY", DefaultChallengeAttemptsPerDay),
			FailCooldown:   c.Duration("EW_GAME_CHALLENGE_FAIL_COOLDOWN", DefaultChallengeFailCooldown), CooldownFrom: challenge.Hard,
			G13TimeFactor: ChallengeG13TimeFactor, G13MaxLevel: challenge.Hard,
			ResumeWindow: c.Duration("EW_GAME_ATTEMPT_RESUME_WINDOW", DefaultAttemptResumeWindow), DeadlineGrace: ChallengeDeadlineGrace,
			RegenStreak: ChallengeRegenStreak}}
	switch r := out.Rules; {
	case r.AttemptsPerDay < 1 || r.AttemptsPerDay > 100:
		return out, errors.New("game config: EW_GAME_CHALLENGE_ATTEMPTS_PER_DAY must be 1..100")
	case r.FailCooldown < 0 || r.FailCooldown > 24*time.Hour:
		return out, errors.New("game config: EW_GAME_CHALLENGE_FAIL_COOLDOWN must be 0..24h")
	case r.ResumeWindow < time.Hour || r.ResumeWindow > 7*24*time.Hour:
		return out, errors.New("game config: EW_GAME_ATTEMPT_RESUME_WINDOW must be 1h..168h")
	}
	return out, nil
}

// ChallengeRoomFrom is the room host of the solo challenges (pkg/roomhost): owner EW_INSTANCE_ID (default the host name),
// lease 10 s renewed every 3 s, orphan scan every 2 s, timer resolution 250 ms (VALKEY_DESIGN §3.1, SC-004 ≤ 15 s).
func ChallengeRoomFrom(c *config.Config) (roomhost.Config, error) {
	id := c.String("EW_INSTANCE_ID")
	if id == "" {
		id, _ = os.Hostname()
	}
	if id == "" || len(id) > 64 || strings.ContainsAny(id, " \t") {
		return roomhost.Config{}, errors.New("game config: EW_INSTANCE_ID must be 1..64 characters without spaces")
	}
	return roomhost.Config{Owner: id, LeaseTTL: 10 * time.Second, RenewEvery: 3 * time.Second, WatchEvery: 2 * time.Second,
		TickEvery: 250 * time.Millisecond, Batch: 50, Grace: roomhost.Grace{ChallengeRoomType: ChallengeDeadlineGrace}}, nil
}
