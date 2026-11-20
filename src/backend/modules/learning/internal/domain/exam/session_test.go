package exam

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func valid() Settings {
	t0 := time.Date(2026, 11, 21, 1, 0, 0, 0, time.UTC)
	d := DefaultsOf(TypeChapter)
	return Settings{Title: "Kiểm tra chương 2", Type: TypeChapter, IsOfficial: d.IsOfficial, WindowStart: t0, WindowEnd: t0.Add(time.Hour),
		DurationSec: 900, MaxAttempts: d.MaxAttempts, FeedbackTiming: d.FeedbackTiming, ResultRelease: d.ResultRelease,
		LateStartPolicy: LateUntilWindowEnd, Audience: AudienceAssigned, Strategy: StrategyFixed, VariantCount: 1, ScorePolicy: "last"}
}

func fields(vs []Violation) map[string]bool {
	out := map[string]bool{}
	for _, v := range vs {
		out[v.Field] = true
	}
	return out
}

// F042-AC-01: the defaults of the six exam types (EXAM_ENGINE §2).
func TestDefaultsOf(t *testing.T) {
	for _, c := range []struct {
		typ      Type
		official bool
		attempts int
		release  string
		feedback string
	}{
		{TypePracticeTest, false, 5, ReleaseImmediate, FeedbackAfterSubmit},
		{TypeMock, false, 3, ReleaseImmediate, FeedbackAfterSubmit},
		{TypeChapter, true, 1, ReleaseAfterClose, FeedbackAfterClose},
		{TypeSemester, true, 1, ReleaseAfterClose, FeedbackAfterClose},
		{TypeSubject, true, 1, ReleaseAfterClose, FeedbackAfterClose},
		{TypeCustom, false, 1, ReleaseAfterClose, FeedbackAfterClose},
	} {
		d := DefaultsOf(c.typ)
		if d.IsOfficial != c.official || d.MaxAttempts != c.attempts || d.ResultRelease != c.release || d.FeedbackTiming != c.feedback {
			t.Errorf("%s: %+v", c.typ, d)
		}
	}
	if Type("timed").Known() || !TypeMock.Practice() || TypeChapter.Practice() {
		t.Fatal("types")
	}
}

// F042-AC-03, AC-08, AC-27 (T009): every invalid field is named.
func TestSettingsValidate(t *testing.T) {
	if vs := valid().Validate(); len(vs) != 0 {
		t.Fatalf("valid: %v", vs)
	}
	s := valid()
	s.Title = string(make([]rune, 256))
	s.WindowEnd = s.WindowStart
	s.MaxAttempts = 6
	s.DurationSec = 30
	s.FeedbackTiming = FeedbackImmediate
	s.LateStartPolicy = "x"
	s.Accommodations = []Accommodation{{StudentID: uuid.Nil, ExtraTimePercent: 0}, {StudentID: uuid.Nil, ExtraTimePercent: 101}}
	got := fields(s.Validate())
	for _, f := range []string{"/title", "/window_end", "/max_attempts", "/duration_seconds", "/feedback_timing", "/late_start_policy",
		"/accommodations/0/extra_time_percent", "/accommodations/1/extra_time_percent", "/accommodations/1/student_id"} {
		if !got[f] {
			t.Errorf("missing %s in %v", f, got)
		}
	}
	s = valid()
	s.FeedbackTiming = FeedbackAfterSubmit // after_close results with feedback at submit
	if !fields(s.Validate())["/feedback_timing"] {
		t.Fatal("after_close + after_submit")
	}
	s = valid()
	s.Audience = AudienceLibrary
	if !fields(s.Validate())["/audience"] {
		t.Fatal("library of an official chapter test")
	}
	s = valid()
	s.Type, s.IsOfficial, s.Strategy = TypePracticeTest, false, StrategyPerStudent
	if !fields(s.Validate())["/result_release"] {
		t.Fatal("per_student needs immediate results")
	}
	s.ResultRelease, s.FeedbackTiming, s.Audience = ReleaseImmediate, FeedbackAfterSubmit, AudienceLibrary
	if vs := s.Validate(); len(vs) != 0 {
		t.Fatalf("library practice: %v", vs)
	}
}

// F042-AC-05, AC-09, AC-15: effective duration with extra time, deadline capped by the window (until_window_end), the
// offline sync grace.
func TestDeadline(t *testing.T) {
	t0 := time.Date(2026, 11, 21, 1, 0, 0, 0, time.UTC)
	if EffectiveDuration(900, 25) != 1125 || EffectiveDuration(900, 0) != 900 || EffectiveDuration(901, 1) != 911 {
		t.Fatal("effective duration")
	}
	end := t0.Add(20 * time.Minute)
	if d := Deadline(t0.Add(10*time.Minute), end, 900, LateUntilWindowEnd); !d.Equal(end) {
		t.Fatalf("late start: %v", d)
	}
	if d := Deadline(t0.Add(10*time.Minute), end, 900, LateFullDuration); !d.Equal(t0.Add(25 * time.Minute)) {
		t.Fatalf("full duration: %v", d)
	}
	dl, grace := t0.Add(15*time.Minute), 30*time.Second
	before := dl.Add(-time.Second)
	if !AcceptsAnswer(dl, dl.Add(-time.Second), nil, grace) || AcceptsAnswer(dl, dl.Add(10*time.Second), nil, grace) ||
		!AcceptsAnswer(dl, dl.Add(10*time.Second), &before, grace) || AcceptsAnswer(dl, dl.Add(31*time.Second), &before, grace) ||
		AcceptsAnswer(dl, dl.Add(10*time.Second), &dl, grace) {
		t.Fatal("accepts answer")
	}
}

// F042-AC-09, AC-27: the variant of a student is stable and spread; the per_student seed changes with the attempt.
func TestVariantAndSeed(t *testing.T) {
	session := uuid.New()
	counts := map[int]int{}
	for range 400 {
		s := uuid.New()
		v := Variant(s, session, 4)
		if v < 1 || v > 4 || v != Variant(s, session, 4) {
			t.Fatal("variant")
		}
		counts[v]++
	}
	if len(counts) != 4 || Variant(uuid.New(), session, 1) != 1 {
		t.Fatalf("spread %v", counts)
	}
	st := uuid.New()
	if first := PerStudentSeed(session, st, 1); first == PerStudentSeed(session, st, 2) || first != PerStudentSeed(session, uuid.MustParse(st.String()), 1) {
		t.Fatal("per_student seed")
	}
}

// F042-AC-20, AC-21: when students see results, when manual release applies; the scaled score.
func TestResultsVisible(t *testing.T) {
	for _, c := range []struct {
		release string
		status  Status
		want    bool
	}{
		{ReleaseImmediate, StatusOpen, true}, {ReleaseAfterClose, StatusOpen, false}, {ReleaseAfterClose, StatusClosed, true},
		{ReleaseManual, StatusClosed, false}, {ReleaseManual, StatusReleased, true}, {ReleaseImmediate, StatusCancelled, false},
	} {
		if ResultsVisible(c.release, c.status) != c.want {
			t.Errorf("%s/%s", c.release, c.status)
		}
	}
	if !CanRelease(ReleaseManual, StatusClosed) || CanRelease(ReleaseManual, StatusOpen) || CanRelease(ReleaseAfterClose, StatusClosed) {
		t.Fatal("can release")
	}
	if ScaledScore(7, 8, 10) != 8.8 || ScaledScore(1, 0, 10) != 0 {
		t.Fatal("scaled")
	}
}
