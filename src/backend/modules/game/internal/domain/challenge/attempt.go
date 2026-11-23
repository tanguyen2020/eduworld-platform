package challenge

import (
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
)

// Attempt statuses (game.challenge_attempts.status).
const (
	StatusInProgress = "in_progress"
	StatusSucceeded  = "succeeded"
	StatusFailed     = "failed"
	StatusAbandoned  = "abandoned"
)

// End reasons (challenge.completed end_reason).
const (
	EndGoalReached         = "goal_reached"
	EndLivesDepleted       = "lives_depleted"
	EndAccuracyMissed      = "accuracy_missed"
	EndDeadlinePassed      = "deadline_passed"
	EndActivitiesExhausted = "activities_exhausted"
	EndPlayer              = "player"
	EndResumeExpired       = "resume_expired"
	EndStudentUnavailable  = "student_unavailable"
)

// MaxActivities is the most learning attempts one challenge attempt opens (CHECK cardinality ≤ 200).
const MaxActivities = 200

// ErrInvalidState is a transition the status does not allow (422 INVALID_STATE_TRANSITION).
var ErrInvalidState = errors.New("invalid state transition")

// Attempt is a ChallengeAttempt (game.challenge_attempts): one play of a student on a pinned version (ADR-0043). Score,
// lives and goal progress change only from graded results of its own learning attempts (ADR-0016, R5).
type Attempt struct {
	ID                  uuid.UUID
	TenantID            uuid.UUID
	StudentID           uuid.UUID
	SchoolID            uuid.UUID
	ChallengeID         uuid.UUID
	VersionID           uuid.UUID
	Type                string
	Difficulty          string
	Status              string
	EndReason           string
	Score               int // correct answers graded by module learning
	ItemsAnswered       int
	Streak              int // consecutive fully correct activities (survival life regeneration)
	ActivitiesCompleted int
	LivesLeft           *int
	GoalProgress        float64
	PeriodKey           string
	FirstClear          bool
	StartedAt           time.Time
	DeadlineAt          *time.Time
	EndedAt             *time.Time
	LearningAttemptIDs  []uuid.UUID
	Processed           []uuid.UUID
	Version             int64 // optimistic concurrency
}

// Start is what a new attempt needs.
type Start struct {
	ID, TenantID, StudentID, SchoolID, ChallengeID, VersionID uuid.UUID
	Type, Difficulty, PeriodKey                               string
	Lives                                                     *int
	DeadlineAt                                                *time.Time
	At                                                        time.Time
}

// NewAttempt starts an attempt in_progress.
func NewAttempt(s Start) *Attempt {
	return &Attempt{ID: s.ID, TenantID: s.TenantID, StudentID: s.StudentID, SchoolID: s.SchoolID, ChallengeID: s.ChallengeID,
		VersionID: s.VersionID, Type: s.Type, Difficulty: s.Difficulty, Status: StatusInProgress, LivesLeft: s.Lives, PeriodKey: s.PeriodKey,
		StartedAt: s.At, DeadlineAt: s.DeadlineAt, LearningAttemptIDs: []uuid.UUID{}, Processed: []uuid.UUID{}}
}

// Active reports an attempt in progress.
func (a *Attempt) Active() bool { return a.Status == StatusInProgress }

// Due is when the room timer ends the attempt (server clock, before the grace): deadline_at, or the end of
// attempt_resume_window for a challenge without a time limit.
func (a *Attempt) Due(r Rules) time.Time {
	if a.DeadlineAt != nil {
		return *a.DeadlineAt
	}
	return a.StartedAt.Add(r.ResumeWindow)
}

// CanNext checks that a next activity may open at now: in progress and before its due time (F076-AC-13).
func (a *Attempt) CanNext(now time.Time, r Rules) error {
	if !a.Active() || now.After(a.Due(r)) {
		return ErrInvalidState
	}
	return nil
}

// NextRef is the activity the next learning attempt opens: the activities in order, cycling except for the finite
// types (perfect, knowledge), whose set ends after one pass.
func (a *Attempt) NextRef(v Version) (ActivityRef, bool) {
	refs := v.Source.ActivityRefs
	n := len(a.LearningAttemptIDs)
	if len(refs) == 0 || (v.Finite() && n >= len(refs)) {
		return ActivityRef{}, false
	}
	return refs[n%len(refs)], true
}

// Opened links a learning attempt to the attempt (a retried open with the same learning attempt adds nothing).
func (a *Attempt) Opened(attemptID uuid.UUID) error {
	if slices.Contains(a.LearningAttemptIDs, attemptID) {
		return nil
	}
	if len(a.LearningAttemptIDs) >= MaxActivities {
		return ErrInvalidState
	}
	a.LearningAttemptIDs = append(a.LearningAttemptIDs, attemptID)
	return nil
}

// Graded is the graded result of one learning attempt (activity.graded of module learning).
type Graded struct {
	AttemptID uuid.UUID
	Correct   int
	Items     int
	Passed    *bool
	At        time.Time // occurred_at of the grading (server)
}

// Outcome of applying a result.
type Outcome struct {
	Changed bool
	Ended   bool
}

// Apply applies one graded result (F076-AC-10, AC-11): only a learning attempt of this challenge attempt, once (a
// replayed event finds it processed), while in progress and graded before deadline_at + deadline_grace. Score = correct
// answers; survival loses one life per wrong answer and restores one after RegenStreak fully correct rounds (up to
// maxLives); the goal follows its metric. Ends: no life left (failed), accuracy below a 100 % target (failed), goal
// reached (succeeded), finite set done (succeeded on an accuracy goal reached, failed otherwise).
func (a *Attempt) Apply(v Version, maxLives int, r Rules, g Graded) Outcome {
	if !a.Active() || !slices.Contains(a.LearningAttemptIDs, g.AttemptID) || slices.Contains(a.Processed, g.AttemptID) {
		return Outcome{}
	}
	if a.DeadlineAt != nil && g.At.After(a.DeadlineAt.Add(r.DeadlineGrace)) {
		return Outcome{} // graded too late: the timer ends the challenge (edge case), the learning record stays
	}
	a.Processed = append(a.Processed, g.AttemptID)
	correct, items := max(0, g.Correct), max(0, g.Items)
	correct = min(correct, items)
	wrong := items - correct
	full := items > 0 && wrong == 0
	solved := full
	if g.Passed != nil {
		solved = *g.Passed
	}
	a.Score += correct
	a.ItemsAnswered += items
	a.ActivitiesCompleted++
	if full {
		a.Streak++
	} else {
		a.Streak = 0
	}
	if a.LivesLeft != nil {
		lives := *a.LivesLeft - wrong
		if full && r.RegenStreak > 0 && a.Streak%r.RegenStreak == 0 && lives < maxLives {
			lives++
		}
		lives = max(lives, 0)
		a.LivesLeft = &lives
	}
	switch v.Goal.Metric {
	case MetricCorrect:
		a.GoalProgress = float64(a.Score)
	case MetricActivities, MetricRounds:
		a.GoalProgress = float64(a.ActivitiesCompleted)
	case MetricPuzzle:
		if solved {
			a.GoalProgress++
		}
	case MetricAccuracy:
		if a.ItemsAnswered > 0 {
			a.GoalProgress = float64(a.Score) / float64(a.ItemsAnswered)
		}
	}
	out := Outcome{Changed: true}
	end := func(status, reason string) {
		a.end(status, reason, g.At)
		out.Ended = true
	}
	switch {
	case a.LivesLeft != nil && *a.LivesLeft == 0:
		end(StatusFailed, EndLivesDepleted)
	case v.Goal.Metric == MetricAccuracy && v.Goal.Target >= 1 && a.Score < a.ItemsAnswered:
		end(StatusFailed, EndAccuracyMissed)
	case v.Goal.Metric != MetricAccuracy && a.GoalProgress >= v.Goal.Target:
		end(StatusSucceeded, EndGoalReached)
	case v.Finite() && len(a.Processed) >= len(v.Source.ActivityRefs):
		switch {
		case v.Goal.Metric != MetricAccuracy:
			end(StatusFailed, EndActivitiesExhausted)
		case a.GoalProgress >= v.Goal.Target:
			end(StatusSucceeded, EndGoalReached)
		default:
			end(StatusFailed, EndAccuracyMissed)
		}
	}
	return out
}

func (a *Attempt) end(status, reason string, at time.Time) {
	a.Status, a.EndReason = status, reason
	t := at.UTC()
	a.EndedAt = &t
}

// Abandon ends an attempt in progress as abandoned (player, student unavailable); an ended one is ErrInvalidState.
func (a *Attempt) Abandon(at time.Time, reason string) (bool, error) {
	if !a.Active() {
		return false, ErrInvalidState
	}
	a.end(StatusAbandoned, reason, at)
	return true, nil
}

// Expire ends an attempt past its due time + deadline_grace on the server clock (F076-AC-13, AC-15): failed
// (deadline_passed) with a time limit, abandoned (resume_expired) without one; false when not due or not in progress.
func (a *Attempt) Expire(now time.Time, r Rules) bool {
	if !a.Active() || now.Before(a.Due(r).Add(r.DeadlineGrace)) {
		return false
	}
	if a.DeadlineAt != nil {
		a.end(StatusFailed, EndDeadlinePassed, now)
	} else {
		a.end(StatusAbandoned, EndResumeExpired, now)
	}
	return true
}
