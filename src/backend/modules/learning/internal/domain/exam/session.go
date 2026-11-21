// Package exam is the domain of exam sessions (F042, EXAM_ENGINE §2, §5–§8): the exam types and their defaults, the
// rules of a session (window, release policy, accommodations, audience), the server deadline of an attempt, the
// variant of a student and when results become visible. Papers are package exam/paper, regrades exam/regrade. Pure Go.
package exam

import (
	"hash/fnv"
	"slices"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Type is ExamType (EXAM_ENGINE §2).
type Type string

// Exam types.
const (
	TypePracticeTest Type = "practice_test"
	TypeMock         Type = "mock"
	TypeChapter      Type = "chapter"
	TypeSemester     Type = "semester"
	TypeSubject      Type = "subject"
	TypeCustom       Type = "custom"
)

var types = []Type{TypePracticeTest, TypeMock, TypeChapter, TypeSemester, TypeSubject, TypeCustom}

// Known reports an exam type.
func (t Type) Known() bool { return slices.Contains(types, t) }

// Practice reports the practice types (practice_test, mock).
func (t Type) Practice() bool { return t == TypePracticeTest || t == TypeMock }

// Status is the status of a session.
type Status string

// Session statuses (data-model §2).
const (
	StatusScheduled Status = "scheduled"
	StatusOpen      Status = "open"
	StatusClosed    Status = "closed"
	StatusReleased  Status = "results_released"
	StatusCancelled Status = "cancelled"
)

// Release policies (ResultRelease) and feedback timings of an exam.
const (
	ReleaseImmediate  = "immediate"
	ReleaseAfterClose = "after_close"
	ReleaseManual     = "manual"

	FeedbackAfterSubmit = "after_submit"
	FeedbackAfterClose  = "after_close"
	FeedbackNever       = "never"
	FeedbackImmediate   = "immediate"
)

// Navigation modes of an exam (assessment blueprint `navigation`, EXAM_ENGINE §3): free moves between items, sequential_no_return
// only forward. The blueprint default is free.
const (
	NavigationFree               = "free"
	NavigationSequentialNoReturn = "sequential_no_return"
)

// NavigationOf is the navigation of a session from its blueprint value (F064 T004): unknown or absent → free.
func NavigationOf(v string) string {
	if v == NavigationSequentialNoReturn {
		return v
	}
	return NavigationFree
}

// Audiences, late start policies, variant strategies, participation statuses and submit reasons.
const (
	AudienceAssigned = "assigned"
	AudienceLibrary  = "library"

	LateUntilWindowEnd = "until_window_end"
	LateFullDuration   = "full_duration"

	StrategyFixed           = "fixed"
	StrategyIndependentDraw = "independent_draw"
	StrategyPerStudent      = "per_student"

	ParticipationNotStarted = "not_started"
	ParticipationInProgress = "in_progress"
	ParticipationSubmitted  = "submitted"
	ParticipationGraded     = "graded"
	ParticipationAbsent     = "absent"

	ReasonManual       = "manual"
	ReasonAutoTimeout  = "auto_timeout"
	ReasonWindowClosed = "window_closed"
)

// Limits of a session (contract ExamSessionCreate, data-model §3).
const (
	MaxTitle          = 255
	MaxAttemptsLimit  = 5
	MaxAccommodations = 200
	MaxExtraPercent   = 100
	MinDurationSec    = 60
	MaxDurationSec    = 14400
	MaxVariants       = 20
)

// Defaults are the defaults of an exam type (EXAM_ENGINE §2, §8): official or not, attempts, release and feedback.
type Defaults struct {
	IsOfficial     bool
	MaxAttempts    int
	ResultRelease  string
	FeedbackTiming string
}

// DefaultsOf returns the defaults of an exam type: practice_test unlimited (the contract maximum), mock 3, the official
// types one attempt released after the close; practice types show results and feedback right after the submit.
func DefaultsOf(t Type) Defaults {
	switch t {
	case TypePracticeTest:
		return Defaults{MaxAttempts: MaxAttemptsLimit, ResultRelease: ReleaseImmediate, FeedbackTiming: FeedbackAfterSubmit}
	case TypeMock:
		return Defaults{MaxAttempts: 3, ResultRelease: ReleaseImmediate, FeedbackTiming: FeedbackAfterSubmit}
	case TypeCustom:
		return Defaults{MaxAttempts: 1, ResultRelease: ReleaseAfterClose, FeedbackTiming: FeedbackAfterClose}
	}
	return Defaults{IsOfficial: true, MaxAttempts: 1, ResultRelease: ReleaseAfterClose, FeedbackTiming: FeedbackAfterClose}
}

// Accommodation is a personal extra time (FR-EXM-009).
type Accommodation struct {
	StudentID        uuid.UUID `json:"student_id"`
	ExtraTimePercent int       `json:"extra_time_percent"`
}

// Settings are the resolved settings of a new session.
type Settings struct {
	Title              string
	Type               Type
	IsOfficial         bool
	WindowStart        time.Time
	WindowEnd          time.Time
	DurationSec        int
	MaxAttempts        int
	FeedbackTiming     string
	ResultRelease      string
	LateStartPolicy    string
	Audience           string
	Strategy           string
	VariantCount       int
	ScorePolicy        string
	ShowCorrectAnswers bool
	ShowExplanations   bool
	Accommodations     []Accommodation
}

// Violation is one invalid field (JSON pointer of the request).
type Violation struct {
	Field  string
	Reason string
}

// Validate applies F042-AC-03, AC-08, AC-27 and data-model §3: title 1–255, window_end after window_start, attempts and
// duration in range, an exam never shows feedback immediately and after_close results never come with feedback before
// the close, the library holds only unofficial practice types, per_student is practice released immediately, extra
// time 1–100 % once per student.
func (s Settings) Validate() []Violation {
	var vs []Violation
	add := func(f, r string) { vs = append(vs, Violation{Field: f, Reason: r}) }
	if n := utf8.RuneCountInString(s.Title); n < 1 || n > MaxTitle {
		add("/title", "must be 1–255 characters")
	}
	if !s.Type.Known() {
		add("/type", "is not an exam type")
	}
	if !s.WindowEnd.After(s.WindowStart) {
		add("/window_end", "must be after window_start")
	}
	if s.MaxAttempts < 1 || s.MaxAttempts > MaxAttemptsLimit {
		add("/max_attempts", "must be 1–5")
	}
	if s.DurationSec < MinDurationSec || s.DurationSec > MaxDurationSec {
		add("/duration_seconds", "must be 60–14400 seconds")
	}
	switch s.FeedbackTiming {
	case FeedbackAfterSubmit, FeedbackAfterClose, FeedbackNever:
	case FeedbackImmediate:
		add("/feedback_timing", "an exam cannot show feedback immediately (EXAM_ENGINE §3)")
	default:
		add("/feedback_timing", "is not a feedback timing")
	}
	switch s.ResultRelease {
	case ReleaseImmediate, ReleaseManual:
	case ReleaseAfterClose:
		if s.FeedbackTiming == FeedbackAfterSubmit {
			add("/feedback_timing", "after_close results need feedback after_close or never")
		}
	default:
		add("/result_release", "is not a result release")
	}
	if s.LateStartPolicy != LateUntilWindowEnd && s.LateStartPolicy != LateFullDuration {
		add("/late_start_policy", "is not a late start policy")
	}
	if s.Audience == AudienceLibrary && (!s.Type.Practice() || s.IsOfficial) {
		add("/audience", "the library holds unofficial practice_test and mock sessions only")
	}
	if s.Strategy == StrategyPerStudent && (s.ResultRelease != ReleaseImmediate || s.FeedbackTiming != FeedbackAfterSubmit) {
		add("/result_release", "per_student papers release results immediately with feedback after the submit")
	}
	if s.VariantCount < 1 || s.VariantCount > MaxVariants {
		add("/variants", "must be 1–20 variants")
	}
	return append(vs, ValidateAccommodations(s.Accommodations)...)
}

// ValidateAccommodations checks a list of personal extra times (createExamSession, putExamAccommodations — FR-EXM-009):
// at most MaxAccommodations, percent 1–MaxExtraPercent, one entry per student.
func ValidateAccommodations(acc []Accommodation) []Violation {
	var vs []Violation
	add := func(f, r string) { vs = append(vs, Violation{Field: f, Reason: r}) }
	if len(acc) > MaxAccommodations {
		add("/accommodations", "at most 200")
	}
	seen := map[uuid.UUID]bool{}
	for i, a := range acc {
		p := "/accommodations/" + strconv.Itoa(i)
		if a.ExtraTimePercent < 1 || a.ExtraTimePercent > MaxExtraPercent {
			add(p+"/extra_time_percent", "must be 1–100")
		}
		if seen[a.StudentID] {
			add(p+"/student_id", "is listed twice")
		}
		seen[a.StudentID] = true
	}
	return vs
}

// CanChangeAccommodations: personal extra times change only before the session closes (scheduled, open).
func CanChangeAccommodations(st Status) bool {
	return st == StatusScheduled || st == StatusOpen
}

// InitialStatus is open when the window has started, scheduled otherwise.
func InitialStatus(windowStart, now time.Time) Status {
	if !windowStart.After(now) {
		return StatusOpen
	}
	return StatusScheduled
}

// EffectiveDuration adds the extra time of an accommodation (rounded up to the second).
func EffectiveDuration(durationSec, extraPercent int) int {
	if extraPercent <= 0 {
		return durationSec
	}
	return durationSec + (durationSec*extraPercent+99)/100
}

// Deadline is deadline_at of an attempt (F042-AC-05, AC-09, EXAM_ENGINE §5): start + effective duration, capped by
// window_end with until_window_end.
func Deadline(startedAt, windowEnd time.Time, effectiveSec int, latePolicy string) time.Time {
	d := startedAt.Add(time.Duration(effectiveSec) * time.Second)
	if latePolicy != LateFullDuration && windowEnd.Before(d) {
		return windowEnd
	}
	return d
}

// AcceptsAnswer reports whether an answer is still taken (F042-AC-15, EXAM_ENGINE §6): received before deadline_at plus
// the offline sync grace, and answered (client clock, when sent) before deadline_at.
func AcceptsAnswer(deadline, receivedAt time.Time, answeredAtClient *time.Time, grace time.Duration) bool {
	if receivedAt.After(deadline.Add(grace)) {
		return false
	}
	if receivedAt.After(deadline) && (answeredAtClient == nil || !answeredAtClient.Before(deadline)) {
		return false
	}
	return true
}

// Variant is the variant of a student (EXAM_ENGINE §4): hash(student_id, exam_session_id) mod n, 1-based.
func Variant(studentID, sessionID uuid.UUID, n int) int {
	if n <= 1 {
		return 1
	}
	h := fnv.New64a()
	_, _ = h.Write(studentID[:])
	_, _ = h.Write(sessionID[:])
	return int(h.Sum64()%uint64(n)) + 1
}

// PerStudentSeed is the seed of a per_student paper (F042-AC-27): from the session, the student and the attempt number.
func PerStudentSeed(sessionID, studentID uuid.UUID, attemptNo int) int64 {
	h := fnv.New64a()
	_, _ = h.Write(sessionID[:])
	_, _ = h.Write(studentID[:])
	_, _ = h.Write([]byte(strconv.Itoa(attemptNo)))
	return int64(h.Sum64() & (1<<62 - 1))
}

// ResultsVisible reports whether students see the results of a graded attempt (F042-AC-20, EXAM_ENGINE §8): immediate
// at grading, after_close once the session closed, manual once released; a cancelled session never.
func ResultsVisible(release string, status Status) bool {
	switch status {
	case StatusReleased:
		return true
	case StatusCancelled:
		return false
	}
	switch release {
	case ReleaseImmediate:
		return true
	case ReleaseAfterClose:
		return status == StatusClosed
	}
	return false
}

// CanRelease reports whether releaseExamResults applies (F042-AC-21): a manual session whose window is closed.
func CanRelease(release string, status Status) bool {
	return release == ReleaseManual && status == StatusClosed
}

// ScaledScore is the score on score_scale (CONFIG_DEFAULTS: scale 10, rounded to one decimal).
func ScaledScore(score, maxScore, scale float64) float64 {
	if maxScore <= 0 {
		return 0
	}
	v := score / maxScore * scale
	return float64(int64(v*10+0.5)) / 10
}
