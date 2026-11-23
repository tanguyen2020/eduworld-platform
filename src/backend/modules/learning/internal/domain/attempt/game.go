package attempt

import (
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
)

// GameScored reports an attempt of the minigame plugin (its game score starts at 0, F073).
func (a *Attempt) GameScored() bool { return a.GameScore != nil }

// ServedInterval is the server interval of a mini-game answer (MINIGAME_ENGINE §6, F073-AC-08): from the moment the item
// was served — the previous answer of the attempt, else its start — to now, on the server clock. Pauses, resumes and
// hints do not move it (a paused student still sees the item), and the client elapsed time never bounds it.
func ServedInterval(startedAt time.Time, lastAnswered *time.Time, now time.Time) int64 {
	served := startedAt
	if lastAnswered != nil && lastAnswered.After(served) {
		served = *lastAnswered
	}
	return max(now.Sub(served).Milliseconds(), 0)
}

// ApplyGame records the game result of one answer (score and current correct streak).
func (a *Attempt) ApplyGame(p activity.GamePoints) {
	s := max(p.Score, 0)
	a.GameScore, a.GameStreak = &s, max(p.Streak, 0)
}

// DeadlinePassed reports an attempt closed by its deadline: expired by the worker, or past deadline_at + deadline_grace
// (a late answer of a timed mini-game is ATTEMPT_DEADLINE_PASSED, F073-AC-11).
func (a *Attempt) DeadlinePassed(now time.Time, p Policy) bool {
	if a.DeadlineAt == nil {
		return false
	}
	return a.Status == StatusExpired || (a.Status == StatusInProgress && past(*a.DeadlineAt, p.DeadlineGrace, now))
}
