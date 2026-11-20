// Package retrywrong is the review list of wrong answers (LEARNING_EXPERIENCE §6.1, F032 US4): a question answered
// wrong at its first scored try enters the list of the student with a due date by retry_wrong_schedule; two correct
// answers on two different days take it out. No I/O.
package retrywrong

import "time"

// Statuses of an entry ("leaving the list" is cleared, never a deletion, AGENTS R21).
const (
	StatusActive  = "active"
	StatusCleared = "cleared"
)

// Schedule is retry_wrong_schedule (CONFIG_DEFAULTS, owner LEARNING_EXPERIENCE §6.1): the review intervals in days.
type Schedule []int

// DefaultSchedule is 1–3–7 days [Confirmed].
func DefaultSchedule() Schedule { return Schedule{1, 3, 7} }

// interval returns the delay of a stage (the last one repeats).
func (s Schedule) interval(stage int) time.Duration {
	if len(s) == 0 {
		return 24 * time.Hour
	}
	stage = min(max(stage, 0), len(s)-1)
	return time.Duration(s[stage]) * 24 * time.Hour
}

// CorrectToClear is the number of correct answers on different days that clear an entry.
const CorrectToClear = 2

// Entry is the review state of one question of a student.
type Entry struct {
	Stage         int
	NextDueAt     time.Time
	CorrectStreak int
	LastCorrectOn string // calendar day (YYYY-MM-DD) of the last correct answer
	WrongCount    int
	LastWrongAt   time.Time
	Status        string
}

// New is the entry of a first-try wrong answer at now: stage 0, due after the first interval.
func New(now time.Time, s Schedule) Entry {
	return Entry{NextDueAt: now.Add(s.interval(0)), WrongCount: 1, LastWrongAt: now, Status: StatusActive}
}

// Wrong records another wrong answer: the schedule restarts and the streak is lost.
func (e *Entry) Wrong(now time.Time, s Schedule) {
	if e.Status != StatusActive {
		return
	}
	e.Stage, e.CorrectStreak, e.LastCorrectOn = 0, 0, ""
	e.WrongCount++
	e.LastWrongAt = now
	e.NextDueAt = now.Add(s.interval(0))
}

// Correct records a correct answer on day (the calendar day of now): a second correct day in a row clears the entry,
// otherwise the next review moves to the next interval. Several correct answers on the same day count once. It
// reports whether the entry was cleared.
func (e *Entry) Correct(now time.Time, day string, s Schedule) bool {
	if e.Status != StatusActive || day == e.LastCorrectOn {
		return false
	}
	e.CorrectStreak++
	e.LastCorrectOn = day
	if e.CorrectStreak >= CorrectToClear {
		e.Status = StatusCleared
		return true
	}
	e.Stage = min(e.Stage+1, max(len(s)-1, 0))
	e.NextDueAt = now.Add(s.interval(e.Stage))
	return false
}

// Clear takes the entry out after a re-evaluation turned the first answer correct (F032 edge case).
func (e *Entry) Clear() bool {
	if e.Status != StatusActive {
		return false
	}
	e.Status = StatusCleared
	return true
}

// Due reports whether the entry is due for review at now.
func (e Entry) Due(now time.Time) bool { return e.Status == StatusActive && !now.Before(e.NextDueAt) }
