package attempt

import (
	"errors"
	"time"
)

// Errors of the hint policy (ACTIVITY_ENGINE §8.3, F032-AC-11, AC-12).
var (
	ErrHintsNotAllowed = errors.New("attempt: hints are not allowed in this attempt") // 403 HINTS_NOT_ALLOWED
	ErrHintsExhausted  = errors.New("attempt: every hint level is open")              // 409 HINTS_EXHAUSTED
)

// MaxHintLevels is the number of hint levels a question version may have (question.schema.json hints maxItems).
const MaxHintLevels = 5

// noHintModes are the modes without hints (ACTIVITY_ENGINE §8.3).
var noHintModes = []Mode{ModeExam, ModeDiagnostic, ModeCompetition}

// HintsAllowed reports whether a student may open hints in the attempt: immediate feedback and a mode outside
// exam, diagnostic and competition.
func (a *Attempt) HintsAllowed() bool {
	if a.Feedback != FeedbackImmediate {
		return false
	}
	for _, m := range noHintModes {
		if a.Mode == m {
			return false
		}
	}
	return true
}

// RevealHint opens the next hint level of the item at position (F032-AC-10…AC-13): levels open in order, never
// skipped; total is the number of hints of the pinned question version. It returns the level now open (1-based).
// Opening a hint ends a pause but does not restart the response interval of the item (the student is still reading
// the same question, ACTIVITY_ENGINE §11.1).
func (a *Attempt) RevealHint(position, total int, now time.Time, p Policy) (int, error) {
	if err := a.CheckActive(now, p); err != nil {
		return 0, err
	}
	it, err := a.Item(position)
	if err != nil {
		return 0, err
	}
	if !a.HintsAllowed() {
		return 0, ErrHintsNotAllowed
	}
	if total > MaxHintLevels {
		total = MaxHintLevels
	}
	if it.HintsRevealed >= total {
		return 0, ErrHintsExhausted
	}
	a.endPauseKeepInterval(now)
	it.HintsRevealed++
	return it.HintsRevealed, nil
}

// endPauseKeepInterval ends a pause without restarting the interval of the current item: the paused time is added to
// paused_ms and last_activity_at moves forward by the same length, so the pause is excluded from the next response time.
func (a *Attempt) endPauseKeepInterval(now time.Time) {
	if a.PausedAt == nil {
		return
	}
	d := now.Sub(*a.PausedAt)
	if d > 0 {
		a.PausedMS += d.Milliseconds()
		a.LastActivityAt = a.LastActivityAt.Add(d)
	}
	if a.LastActivityAt.After(now) {
		a.LastActivityAt = now
	}
	a.PausedAt = nil
}
