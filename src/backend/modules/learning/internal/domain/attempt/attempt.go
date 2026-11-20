// Package attempt is the Attempt aggregate of the Activity & Attempt Engine (F030, ACTIVITY_ENGINE §6–§8, §11): one
// student working one pinned version, its delivered items, the lifecycle (respond, pause, submit, expire, abandon),
// the FeedbackTiming policy and the scoring of §7.2. Times are server times and scores are computed here only
// (ADR-0016); the aggregate never knows a concrete activity type (the plugin evaluates). No I/O.
package attempt

import (
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
)

// Mode is AttemptMode.
type Mode string

// Attempt modes (ACTIVITY_ENGINE §6.1).
const (
	ModePractice    Mode = "practice"
	ModeAssignment  Mode = "assignment"
	ModeQuest       Mode = "quest"
	ModeMinigame    Mode = "minigame"
	ModeChallenge   Mode = "challenge"
	ModeExam        Mode = "exam"
	ModeCompetition Mode = "competition"
	ModeRetryWrong  Mode = "retry_wrong"
	ModeDiagnostic  Mode = "diagnostic"
)

// Status is AttemptStatus (pause is the paused_at flag, not a status).
type Status string

// Attempt statuses (ACTIVITY_ENGINE §6.2).
const (
	StatusInProgress Status = "in_progress"
	StatusSubmitted  Status = "submitted"
	StatusGraded     Status = "graded"
	StatusExpired    Status = "expired"
	StatusAbandoned  Status = "abandoned"
)

// FeedbackTiming is when the student sees correctness, explanation and option feedback of each item (§8.1).
type FeedbackTiming string

// Feedback timings.
const (
	FeedbackImmediate   FeedbackTiming = "immediate"
	FeedbackAfterSubmit FeedbackTiming = "after_submit"
	FeedbackAfterClose  FeedbackTiming = "after_close"
	FeedbackNever       FeedbackTiming = "never"
)

// Submit reasons.
const (
	ReasonManual       = "manual"
	ReasonTimeout      = "timeout"
	ReasonAutoComplete = "auto_complete"
)

// Errors of the lifecycle (mapped to problem codes by the application).
var (
	ErrNotInProgress    = errors.New("attempt: not in progress")          // 409 ATTEMPT_NOT_IN_PROGRESS
	ErrExpired          = errors.New("attempt: expired or abandoned")     // 410 ATTEMPT_EXPIRED
	ErrDeadlinePassed   = errors.New("attempt: deadline passed")          // 422 ATTEMPT_DEADLINE_PASSED
	ErrAlreadySubmitted = errors.New("attempt: already submitted")        // 409 ATTEMPT_ALREADY_SUBMITTED
	ErrPositionInvalid  = errors.New("attempt: no item at this position") // 422 VALIDATION_FAILED
	ErrAnswerLimit      = errors.New("attempt: answer limit reached")     // 409 ANSWER_LIMIT_REACHED
)

// Policy holds the parameters of CONFIG_DEFAULTS used by the lifecycle (owner ACTIVITY_ENGINE, LEARNING_EXPERIENCE §4.2).
type Policy struct {
	IdleTimeout   time.Duration // attempt_idle_timeout
	ResumeWindow  time.Duration // attempt_resume_window
	DeadlineGrace time.Duration // deadline_grace
	// AttemptsPerQuestionG13 / AttemptsPerQuestion are attempts_per_question for grades 1–3 and the other grades.
	AttemptsPerQuestionG13 int
	AttemptsPerQuestion    int
}

// DefaultPolicy is CONFIG_DEFAULTS [Proposed]: idle 5 min, resume window 24 h, grace 2 s, 3 tries (g1_3) or 2.
func DefaultPolicy() Policy {
	return Policy{IdleTimeout: 5 * time.Minute, ResumeWindow: 24 * time.Hour, DeadlineGrace: 2 * time.Second, AttemptsPerQuestionG13: 3,
		AttemptsPerQuestion: 2}
}

// AnswerLimit is attempts_per_question for the grade of the student (nil = not known: the stricter value).
func (p Policy) AnswerLimit(grade *int) int {
	if grade != nil && *grade >= 1 && *grade <= 3 {
		return p.AttemptsPerQuestionG13
	}
	return p.AttemptsPerQuestion
}

// DefaultFeedback is the FeedbackTiming of a mode (§8.1).
func DefaultFeedback(m Mode) FeedbackTiming {
	if m == ModeDiagnostic || m == ModeExam {
		return FeedbackAfterSubmit
	}
	return FeedbackImmediate
}

// ResolveFeedback applies the feedback timing of the activity over the default of the mode; exam and diagnostic never
// show feedback immediately.
func ResolveFeedback(m Mode, activityTiming string) FeedbackTiming {
	t := FeedbackTiming(activityTiming)
	switch t {
	case FeedbackImmediate, FeedbackAfterSubmit, FeedbackAfterClose, FeedbackNever:
	default:
		return DefaultFeedback(m)
	}
	if t == FeedbackImmediate && (m == ModeExam || m == ModeDiagnostic) {
		return DefaultFeedback(m)
	}
	return t
}

// Item is one delivered question (assessment.attempt_items).
type Item struct {
	ID                uuid.UUID
	Position          int
	QuestionVersionID uuid.UUID
	QuestionType      string
	ObjectiveIDs      []uuid.UUID
	Presentation      json.RawMessage
	PayloadHash       string
	MaxScore          float64
	AnswerCount       int
	FinalIsCorrect    *bool
	FinalScore        *float64
	HintsRevealed     int
	RowVersion        int64
}

// Attempt is the aggregate (assessment.attempts + its items).
type Attempt struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	SchoolID         uuid.UUID
	StudentID        uuid.UUID
	Mode             Mode
	ContextType      string
	ContextID        uuid.UUID
	ActivityID       *uuid.UUID
	ActivityType     string
	SourceKind       string
	SourceVersionID  uuid.UUID
	Status           Status
	Seed             int64
	StartedAt        time.Time
	DeadlineAt       *time.Time
	SubmittedAt      *time.Time
	GradedAt         *time.Time
	PausedAt         *time.Time
	PausedMS         int64
	LastActivityAt   time.Time
	Feedback         FeedbackTiming
	DisplayLanguage  string
	SubmitReason     *string
	Score            *float64
	MaxScore         *float64
	DurationMS       *int64
	ClientElapsedMS  *int64
	ObjectiveIDs     []uuid.UUID
	RecommendationID *uuid.UUID
	IdempotencyKey   string
	RowVersion       int64
	Items            []Item
}

// Item returns the item at position.
func (a *Attempt) Item(position int) (*Item, error) {
	for i := range a.Items {
		if a.Items[i].Position == position {
			return &a.Items[i], nil
		}
	}
	return nil, ErrPositionInvalid
}

// past reports t + d < now.
func past(t time.Time, d time.Duration, now time.Time) bool { return now.After(t.Add(d)) }

// CheckActive refuses a command on an attempt that no longer takes responses: abandoned or expired → ErrExpired (410),
// submitted/graded → ErrNotInProgress (409); past deadline_at + deadline_grace → ErrExpired.
func (a *Attempt) CheckActive(now time.Time, p Policy) error {
	switch a.Status {
	case StatusInProgress:
	case StatusAbandoned, StatusExpired:
		return ErrExpired
	default:
		return ErrNotInProgress
	}
	if a.DeadlineAt != nil && past(*a.DeadlineAt, p.DeadlineGrace, now) {
		return ErrExpired
	}
	return nil
}

// touch ends a pause (its length is excluded from the duration) and records activity.
func (a *Attempt) touch(now time.Time) {
	if a.PausedAt != nil {
		if d := now.Sub(*a.PausedAt); d > 0 {
			a.PausedMS += d.Milliseconds()
		}
		a.PausedAt = nil
	}
	a.LastActivityAt = now
}

// Answer is one recorded response decision (before grading): its attempt number, whether it is scored and the
// response time measured by the server.
type Answer struct {
	Position   int
	AttemptNo  int
	IsScored   bool
	ResponseMS int64
	// HintsUsed is the number of hint levels open before this answer (ACTIVITY_ENGINE §8.3, F032-AC-14).
	HintsUsed int
}

// Respond admits a response to the item at position (§8.1, LEARNING_EXPERIENCE §4.2): with immediate feedback the
// first answer is scored and the retries are not (when allow_unscored_retry, otherwise no retry); with deferred feedback
// every answer is scored and the last one counts. The number of answers per item is bounded by attempts_per_question.
func (a *Attempt) Respond(position int, now time.Time, p Policy, limit int, allowRetry bool, clientElapsedMS *int64) (Answer, error) {
	if err := a.CheckActive(now, p); err != nil {
		return Answer{}, err
	}
	it, err := a.Item(position)
	if err != nil {
		return Answer{}, err
	}
	if limit < 1 {
		limit = 1
	}
	if a.Feedback == FeedbackImmediate && it.AnswerCount >= 1 && !allowRetry {
		limit = 1
	}
	if it.AnswerCount >= limit {
		return Answer{}, ErrAnswerLimit
	}
	// server_interval_ms (ACTIVITY_ENGINE §11.1): from the last activity (start, previous answer or resume) to now; the
	// tail of a pause still open is excluded (paused_at − last_activity_at, F032).
	end := now
	if a.PausedAt != nil && a.PausedAt.Before(now) {
		end = *a.PausedAt
	}
	server := end.Sub(a.LastActivityAt).Milliseconds()
	if server < 0 {
		server = 0
	}
	if clientElapsedMS != nil && *clientElapsedMS >= 0 && *clientElapsedMS < server {
		server = *clientElapsedMS
	}
	a.touch(now)
	ans := Answer{Position: position, AttemptNo: it.AnswerCount + 1, ResponseMS: server, HintsUsed: it.HintsRevealed}
	ans.IsScored = a.Feedback != FeedbackImmediate || it.AnswerCount == 0
	return ans, nil
}

// Record applies a graded answer to its item (answer count, final result of the scored answers).
func (a *Attempt) Record(ans Answer, isCorrect bool, score float64) {
	it, err := a.Item(ans.Position)
	if err != nil {
		return
	}
	it.AnswerCount++
	if ans.IsScored {
		c, s := isCorrect, activity.Round2(score)
		it.FinalIsCorrect, it.FinalScore = &c, &s
	}
}

// Pause sets paused_at (client :pause or idle detection); a paused attempt still resumes on the next command.
func (a *Attempt) Pause(now time.Time, p Policy) error {
	if err := a.CheckActive(now, p); err != nil {
		return err
	}
	if a.PausedAt == nil {
		t := now
		a.PausedAt = &t
	}
	return nil
}

// Resume ends a pause when the student reads the attempt again (getActivityAttempt of an in-progress attempt).
func (a *Attempt) Resume(now time.Time, p Policy) error {
	if err := a.CheckActive(now, p); err != nil {
		return err
	}
	a.touch(now)
	return nil
}

// ItemResults are the final item results (§7.2: unanswered = 0).
func (a *Attempt) ItemResults() []activity.ItemResult {
	out := make([]activity.ItemResult, len(a.Items))
	for i, it := range a.Items {
		r := activity.ItemResult{Position: it.Position, MaxScore: it.MaxScore}
		if it.FinalScore != nil {
			r.Answered, r.Score = true, *it.FinalScore
			r.IsCorrect = it.FinalIsCorrect != nil && *it.FinalIsCorrect
		}
		out[i] = r
	}
	return out
}

// Answered counts the items with a scored answer.
func (a *Attempt) Answered() int {
	n := 0
	for _, it := range a.Items {
		if it.FinalScore != nil {
			n++
		}
	}
	return n
}

// Submit moves an in-progress attempt to graded with the result of the plugin (§6.2: submitted → graded in the same
// transaction): duration = submitted_at − started_at − paused_ms on the server clock. A manual submit after
// deadline_at + deadline_grace is ErrDeadlinePassed; the timeout submit of the worker is allowed.
func (a *Attempt) Submit(now time.Time, p Policy, reason string, r activity.Result) error {
	switch a.Status {
	case StatusInProgress:
	case StatusExpired:
		if reason != ReasonTimeout {
			return ErrExpired
		}
	case StatusAbandoned:
		return ErrExpired
	default:
		return ErrAlreadySubmitted
	}
	if reason != ReasonTimeout && a.DeadlineAt != nil && past(*a.DeadlineAt, p.DeadlineGrace, now) {
		return ErrDeadlinePassed
	}
	end := now
	if reason == ReasonTimeout && a.DeadlineAt != nil && a.DeadlineAt.Before(now) {
		end = *a.DeadlineAt
	}
	if a.PausedAt != nil {
		if d := end.Sub(*a.PausedAt); d > 0 {
			a.PausedMS += d.Milliseconds()
		}
		a.PausedAt = nil
	}
	d := end.Sub(a.StartedAt).Milliseconds() - a.PausedMS
	if d < 0 {
		d = 0
	}
	score, maxScore := r.Score, r.MaxScore
	a.SubmittedAt, a.GradedAt, a.DurationMS, a.Score, a.MaxScore = &end, &now, &d, &score, &maxScore
	a.SubmitReason = &reason
	a.Status = StatusGraded
	a.LastActivityAt = now
	return nil
}

// Expire marks an attempt past deadline_at + deadline_grace (§6.2): the worker then submits it with reason timeout when
// it has at least one answer; without answers it stays expired and is not graded.
func (a *Attempt) Expire(now time.Time, p Policy) bool {
	if a.Status != StatusInProgress || a.DeadlineAt == nil || !past(*a.DeadlineAt, p.DeadlineGrace, now) {
		return false
	}
	a.Status = StatusExpired
	return true
}

// resumable are the modes with a resume window (§6.3); assignment (due_at, F041) and exam (F042) are not abandoned here.
var resumable = []Mode{ModePractice, ModeRetryWrong, ModeDiagnostic, ModeQuest, ModeChallenge, ModeMinigame}

// Abandon marks an attempt inactive longer than attempt_resume_window as abandoned (§6.2).
func (a *Attempt) Abandon(now time.Time, p Policy) bool {
	if a.Status != StatusInProgress || !slices.Contains(resumable, a.Mode) || !past(a.LastActivityAt, p.ResumeWindow, now) {
		return false
	}
	a.Status = StatusAbandoned
	return true
}

// IdlePause sets paused_at once the attempt has been inactive for attempt_idle_timeout (§6.2): the pause starts when the
// timeout elapsed, so the idle tail is excluded from the duration.
func (a *Attempt) IdlePause(now time.Time, p Policy) bool {
	if a.Status != StatusInProgress || a.PausedAt != nil || !past(a.LastActivityAt, p.IdleTimeout, now) {
		return false
	}
	t := a.LastActivityAt.Add(p.IdleTimeout)
	a.PausedAt = &t
	return true
}

// ShowsItemFeedback reports whether item feedback (correctness, explanation, option feedback) may be shown now.
func (a *Attempt) ShowsItemFeedback() bool {
	switch a.Feedback {
	case FeedbackImmediate:
		return true
	case FeedbackAfterSubmit:
		return a.Status == StatusGraded
	}
	return false
}
