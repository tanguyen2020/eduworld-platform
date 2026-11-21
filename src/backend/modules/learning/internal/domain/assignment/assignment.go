// Package assignment is the school assignment of the LMS (F041, SCHOOL_LMS §4): its life cycle, the fields editable in
// each status, the release policy (FeedbackTiming ≤ ResultRelease), the window of a student (personal due date and late
// window) and the score policy of the submission. It does no I/O.
package assignment

import (
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// Status of an assignment (AssignmentStatus).
type Status string

// Statuses (SCHOOL_LMS §4).
const (
	StatusDraft           Status = "draft"
	StatusScheduled       Status = "scheduled"
	StatusOpen            Status = "open"
	StatusClosed          Status = "closed"
	StatusResultsReleased Status = "results_released"
	StatusArchived        Status = "archived"
	StatusCancelled       Status = "cancelled"
)

// Modes, release policies, feedback timings, score policies, content types and target types of the contract.
const (
	ModePractice = "practice"
	ModeGraded   = "graded"

	ReleaseImmediate  = "immediate"
	ReleaseAfterClose = "after_close"
	ReleaseManual     = "manual"

	FeedbackImmediate   = "immediate"
	FeedbackAfterSubmit = "after_submit"
	FeedbackAfterClose  = "after_close"
	FeedbackNever       = "never"

	ScoreBest  = "best"
	ScoreLast  = "last"
	ScoreFirst = "first"

	ContentActivity    = "activity"
	ContentQuestionSet = "question_set"
	ContentBlueprint   = "assessment_blueprint"
	ContentLesson      = "lesson"
	ContentQuest       = "quest"
	ContentMinigame    = "minigame_config"

	// TargetGrade is (school, grade_level) ± students; TargetStudents a list of students of the school (ADR-0095 mục 3).
	TargetGrade    = "grade"
	TargetStudents = "students"
)

// Limits (ADR-0041, AssignmentCreate).
const (
	MaxTitle          = 120
	MaxInstructions   = 2000
	MaxTargets        = 9
	MaxTargetStudents = 200
	MaxLateHours      = 168
	MaxAttempts       = 10
	MaxReason         = 500
)

// SupportedActivityTypes are the activity types an assignment accepts in this release (SCHOOL_LMS §6, ADR-0072):
// written_response, speaking and project are deferred.
var SupportedActivityTypes = []string{"quiz", "interactive_exercise", "minigame", "video_lesson", "audio_lesson", "simulation"}

// Violation is one invalid field (JSON pointer of the request).
type Violation struct{ Field, Reason string }

// Target is one AssignmentTarget: Grade is 0 when absent; Include, Exclude and StudentIDs are list lengths.
type Target struct {
	Type       string
	Grade      int
	Include    int
	Exclude    int
	StudentIDs int
}

// Fields are the editable fields of an assignment (AssignmentCreate, AssignmentUpdate).
type Fields struct {
	Title          string
	Instructions   *string
	Mode           string
	FollowLatest   bool
	Targets        []Target
	OpenAt, DueAt  time.Time
	AllowLate      bool
	LateHours      int
	MaxAttempts    int
	ScorePolicy    string
	FeedbackTiming string
	ResultRelease  string
}

// Defaults fills what the request left out (F041-AC-06): graded → graded_assignment_release_default (after_close/after_close),
// practice → immediate/immediate; max_attempts 1, score_policy best.
func (f *Fields) Defaults(gradedFeedback, gradedRelease string) {
	if f.MaxAttempts == 0 {
		f.MaxAttempts = 1
	}
	if f.ScorePolicy == "" {
		f.ScorePolicy = ScoreBest
	}
	if f.Mode == ModeGraded {
		if f.FeedbackTiming == "" {
			f.FeedbackTiming = gradedFeedback
		}
		if f.ResultRelease == "" {
			f.ResultRelease = gradedRelease
		}
		return
	}
	if f.FeedbackTiming == "" {
		f.FeedbackTiming = FeedbackImmediate
	}
	if f.ResultRelease == "" {
		f.ResultRelease = ReleaseImmediate
	}
}

var feedbacks = []string{FeedbackImmediate, FeedbackAfterSubmit, FeedbackAfterClose, FeedbackNever}
var releases = []string{ReleaseImmediate, ReleaseAfterClose, ReleaseManual}

// FeedbackAllowed is the constraint of ACTIVITY_ENGINE §8.1: per-question feedback is never shown before the results
// are released — after_close and manual releases only allow after_close or never.
func FeedbackAllowed(feedback, release string) bool {
	if release == ReleaseImmediate {
		return true
	}
	return feedback == FeedbackAfterClose || feedback == FeedbackNever
}

// Validate checks the fields of a create or a draft update (F041-AC-06, AC-07); maxLateHours is the tenant maximum of
// late_window_hours.
func (f Fields) Validate(maxLateHours int) []Violation {
	var vs []Violation
	add := func(field, reason string) { vs = append(vs, Violation{field, reason}) }
	if n := utf8.RuneCountInString(strings.TrimSpace(f.Title)); n == 0 || utf8.RuneCountInString(f.Title) > MaxTitle {
		add("/title", "must have 1 to 120 characters")
	}
	if f.Instructions != nil && utf8.RuneCountInString(*f.Instructions) > MaxInstructions {
		add("/instructions", "is longer than 2000 characters")
	}
	if f.Mode != ModePractice && f.Mode != ModeGraded {
		add("/mode", "is not practice or graded")
	}
	if f.Mode == ModeGraded && f.FollowLatest {
		add("/follow_latest", "is not allowed for a graded assignment")
	}
	vs = append(vs, ValidateTargets(f.Targets)...)
	if !f.DueAt.After(f.OpenAt) {
		add("/due_at", "must be after open_at")
	}
	if maxLateHours <= 0 || maxLateHours > MaxLateHours {
		maxLateHours = MaxLateHours
	}
	if f.LateHours < 0 || f.LateHours > maxLateHours {
		add("/late_policy/late_window_hours", "is outside 0 to the maximum late window")
	}
	if f.MaxAttempts < 1 || f.MaxAttempts > MaxAttempts {
		add("/max_attempts", "must be 1 to 10")
	}
	if !slices.Contains([]string{ScoreBest, ScoreLast, ScoreFirst}, f.ScorePolicy) {
		add("/score_policy", "is not best, last or first")
	}
	switch {
	case !slices.Contains(feedbacks, f.FeedbackTiming):
		add("/feedback_timing", "is not a feedback timing")
	case !slices.Contains(releases, f.ResultRelease):
		add("/result_release", "is not a result release")
	case !FeedbackAllowed(f.FeedbackTiming, f.ResultRelease):
		add("/feedback_timing", "is earlier than the release of the results")
	}
	return vs
}

// ValidateTargets checks the targets of an assignment or an exam session (ADR-0095 mục 3): 1 to 9 targets; a grade
// target has grade_level 1–9 (each grade once), at most 200 included and 200 excluded students and no student_ids; at
// most one students target with 1 to 200 students and no grade, include nor exclude.
func ValidateTargets(ts []Target) []Violation {
	var vs []Violation
	add := func(field, reason string) { vs = append(vs, Violation{field, reason}) }
	if len(ts) == 0 || len(ts) > MaxTargets {
		add("/targets", "must have 1 to 9 targets")
	}
	grades := map[int]bool{}
	lists := 0
	for i, t := range ts {
		at := "/targets/" + itoa(i)
		switch t.Type {
		case TargetGrade:
			switch {
			case t.Grade < 1 || t.Grade > 9:
				add(at+"/grade_level", "must be 1 to 9 for a grade target")
			case grades[t.Grade]:
				add(at+"/grade_level", "a grade is targeted once")
			}
			grades[t.Grade] = true
			if t.StudentIDs > 0 {
				add(at+"/student_ids", "is only for target_type students")
			}
			if t.Include > MaxTargetStudents {
				add(at+"/include_student_ids", "must have at most 200 students")
			}
			if t.Exclude > MaxTargetStudents {
				add(at+"/exclude_student_ids", "must have at most 200 students")
			}
		case TargetStudents:
			lists++
			if lists > 1 {
				add(at, "only one students target is allowed")
			}
			if t.StudentIDs == 0 || t.StudentIDs > MaxTargetStudents {
				add(at+"/student_ids", "must have 1 to 200 students")
			}
			if t.Grade != 0 {
				add(at+"/grade_level", "is only for target_type grade")
			}
			if t.Include > 0 || t.Exclude > 0 {
				add(at, "include_student_ids and exclude_student_ids are only for target_type grade")
			}
		default:
			add(at+"/target_type", "is not grade or students")
		}
	}
	return vs
}

// Audience composes the students an assignment is given to (ADR-0095 mục 3) from the active students of each targeted
// grade of the school, the included students and the listed students: an excluded student is never targeted (exclude
// wins over include), a student held by several targets once, in the order of the targets.
func Audience[ID comparable](grades, includes, lists [][]ID, excludes []ID) []ID {
	out := []ID{}
	seen := map[ID]bool{}
	for _, x := range excludes {
		seen[x] = true
	}
	for _, group := range [][][]ID{grades, includes, lists} {
		for _, xs := range group {
			for _, x := range xs {
				if !seen[x] {
					seen[x] = true
					out = append(out, x)
				}
			}
		}
	}
	return out
}

// LateWindow is the late window of the assignment (0 when late submissions are not allowed).
func (f Fields) LateWindow() time.Duration {
	if !f.AllowLate {
		return 0
	}
	return time.Duration(f.LateHours) * time.Hour
}

// Editable lists the fields a status lets change (F041-AC-10): every field in draft, due_at/instructions/late_policy
// when open, nothing otherwise. A scheduled assignment is edited like a draft except its targets (snapshot taken).
func Editable(s Status) []string {
	switch s {
	case StatusDraft:
		return nil // everything
	case StatusScheduled:
		return []string{"title", "instructions", "open_at", "due_at", "late_policy", "max_attempts", "score_policy",
			"feedback_timing", "result_release"}
	case StatusOpen:
		return []string{"due_at", "instructions", "late_policy"}
	}
	return []string{}
}

// CanEdit reports whether every field of changed may change in status s.
func CanEdit(s Status, changed []string) bool {
	allowed := Editable(s)
	if allowed == nil {
		return true
	}
	for _, c := range changed {
		if !slices.Contains(allowed, c) {
			return false
		}
	}
	return true
}

// PublishStatus is the status after publish (F041-AC-08): scheduled while open_at is ahead, otherwise open.
func PublishStatus(openAt, now time.Time) Status {
	if openAt.After(now) {
		return StatusScheduled
	}
	return StatusOpen
}

// CanCancel reports whether an assignment may be cancelled (F041-AC-11): scheduled, or open without a submission.
func CanCancel(s Status, hasSubmission bool) bool {
	return s == StatusScheduled || (s == StatusOpen && !hasSubmission)
}

// CanArchive reports whether an assignment may be archived: results released, or closed when its results were visible
// from the start (immediate); a manual release must be released first.
func CanArchive(s Status, release string) bool {
	return s == StatusResultsReleased || (s == StatusClosed && release == ReleaseImmediate)
}

// AfterClose is the status of an assignment its scheduler closes (F041-AC-20): after_close releases the results,
// immediate archives, manual stays closed until the school releases.
func AfterClose(release string) Status {
	switch release {
	case ReleaseAfterClose:
		return StatusResultsReleased
	case ReleaseImmediate:
		return StatusArchived
	}
	return StatusClosed
}

// ResultsVisible reports whether the student may see the results of the assignment now (F041-AC-19).
func ResultsVisible(release string, s Status) bool {
	if release == ReleaseImmediate {
		return true
	}
	return s == StatusResultsReleased || s == StatusArchived
}

// Window is the time a student may start or submit an attempt (F041-AC-13): open_at to the personal (or common) due
// date plus the late window.
type Window struct {
	OpenAt, DueAt time.Time
	Late          time.Duration
}

// Open reports whether now is inside the window.
func (w Window) Open(now time.Time) bool {
	return !now.Before(w.OpenAt) && !now.After(w.DueAt.Add(w.Late))
}

// IsLate reports whether a submission at t is late (after the due date, inside the late window).
func (w Window) IsLate(t time.Time) bool { return t.After(w.DueAt) }

// ClosesAt is the end of the window.
func (w Window) ClosesAt() time.Time { return w.DueAt.Add(w.Late) }

// Scored is one graded attempt of a submission.
type Scored struct {
	AttemptID   [16]byte
	Score, Max  float64
	SubmittedAt time.Time
}

// Pick returns the attempt the score policy keeps (best: highest score, earliest on ties; last: latest; first:
// earliest); ok is false without attempt.
func Pick(policy string, xs []Scored) (Scored, bool) {
	if len(xs) == 0 {
		return Scored{}, false
	}
	out := xs[0]
	for _, x := range xs[1:] {
		switch policy {
		case ScoreLast:
			if x.SubmittedAt.After(out.SubmittedAt) {
				out = x
			}
		case ScoreFirst:
			if x.SubmittedAt.Before(out.SubmittedAt) {
				out = x
			}
		default:
			if x.Score > out.Score || (x.Score == out.Score && x.SubmittedAt.Before(out.SubmittedAt)) {
				out = x
			}
		}
	}
	return out, true
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
