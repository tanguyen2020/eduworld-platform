package assignment

import (
	"strings"
	"testing"
	"time"
)

func valid() Fields {
	t0 := time.Date(2026, 11, 23, 1, 0, 0, 0, time.UTC)
	f := Fields{Title: "Ôn tập chương 3", Mode: ModePractice, Targets: []Target{{Type: TargetClass, HasRef: true}}, OpenAt: t0,
		DueAt: t0.Add(72 * time.Hour)}
	f.Defaults(FeedbackAfterClose, ReleaseAfterClose)
	return f
}

func fields(vs []Violation) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v.Field
	}
	return out
}

// F041-AC-06: graded defaults to graded_assignment_release_default; graded + follow_latest and feedback earlier than
// the release are refused.
func TestReleasePolicy(t *testing.T) {
	g := Fields{Mode: ModeGraded}
	g.Defaults(FeedbackAfterClose, ReleaseAfterClose)
	if g.FeedbackTiming != FeedbackAfterClose || g.ResultRelease != ReleaseAfterClose || g.MaxAttempts != 1 || g.ScorePolicy != ScoreBest {
		t.Fatalf("graded defaults %+v", g)
	}
	p := valid()
	if p.FeedbackTiming != FeedbackImmediate || p.ResultRelease != ReleaseImmediate {
		t.Fatalf("practice defaults %+v", p)
	}
	if vs := p.Validate(168); len(vs) != 0 {
		t.Fatalf("valid: %+v", vs)
	}
	f := valid()
	f.Mode, f.FollowLatest = ModeGraded, true
	if got := fields(f.Validate(168)); len(got) != 1 || got[0] != "/follow_latest" {
		t.Fatalf("graded follow_latest: %v", got)
	}
	for _, c := range []struct{ fb, rr string }{{FeedbackImmediate, ReleaseAfterClose}, {FeedbackAfterSubmit, ReleaseManual}} {
		f := valid()
		f.FeedbackTiming, f.ResultRelease = c.fb, c.rr
		if got := fields(f.Validate(168)); len(got) != 1 || got[0] != "/feedback_timing" {
			t.Fatalf("%s before %s: %v", c.fb, c.rr, got)
		}
	}
	for _, c := range []struct{ fb, rr string }{{FeedbackAfterClose, ReleaseManual}, {FeedbackNever, ReleaseAfterClose}, {FeedbackImmediate, ReleaseImmediate}} {
		if !FeedbackAllowed(c.fb, c.rr) {
			t.Fatalf("%s with %s refused", c.fb, c.rr)
		}
	}
}

// F041-AC-07: lengths, ranges and the window are checked field by field.
func TestValidateLimits(t *testing.T) {
	f := valid()
	long := strings.Repeat("a", 2001)
	f.Title, f.Instructions = strings.Repeat("a", 121), &long
	f.Targets = make([]Target, 51)
	f.LateHours, f.MaxAttempts = 169, 11
	f.DueAt = f.OpenAt
	got := fields(f.Validate(168))
	for _, want := range []string{"/title", "/instructions", "/targets", "/due_at", "/late_policy/late_window_hours", "/max_attempts"} {
		found := false
		for _, g := range got {
			found = found || g == want
		}
		if !found {
			t.Fatalf("missing %s in %v", want, got)
		}
	}
	f = valid()
	f.LateHours = 48
	if got := fields(f.Validate(24)); len(got) != 1 {
		t.Fatalf("tenant maximum of the late window: %v", got)
	}
	f = valid()
	f.Targets = []Target{{Type: TargetClass}, {Type: TargetStudents}, {Type: TargetStudents, StudentIDs: 201}, {Type: "school", HasRef: true}}
	if got := fields(f.Validate(168)); len(got) != 4 || got[0] != "/targets/0/target_id" || got[3] != "/targets/3/target_type" {
		t.Fatalf("targets: %v", got)
	}
}

// F041-AC-10, AC-11, AC-20: editable fields, publish, cancel, archive and the status after close.
func TestLifeCycle(t *testing.T) {
	if !CanEdit(StatusDraft, []string{"targets", "mode"}) || CanEdit(StatusOpen, []string{"title"}) ||
		!CanEdit(StatusOpen, []string{"due_at", "instructions", "late_policy"}) || CanEdit(StatusClosed, []string{"due_at"}) ||
		CanEdit(StatusCancelled, []string{"instructions"}) || CanEdit(StatusScheduled, []string{"targets"}) {
		t.Fatal("editable fields")
	}
	now := time.Now()
	if PublishStatus(now.Add(time.Hour), now) != StatusScheduled || PublishStatus(now, now) != StatusOpen {
		t.Fatal("publish status")
	}
	if !CanCancel(StatusScheduled, true) || !CanCancel(StatusOpen, false) || CanCancel(StatusOpen, true) || CanCancel(StatusClosed, false) {
		t.Fatal("cancel")
	}
	if AfterClose(ReleaseAfterClose) != StatusResultsReleased || AfterClose(ReleaseImmediate) != StatusArchived ||
		AfterClose(ReleaseManual) != StatusClosed {
		t.Fatal("after close")
	}
	if !CanArchive(StatusResultsReleased, ReleaseManual) || CanArchive(StatusClosed, ReleaseManual) || !CanArchive(StatusClosed, ReleaseImmediate) {
		t.Fatal("archive")
	}
	if ResultsVisible(ReleaseManual, StatusClosed) || !ResultsVisible(ReleaseManual, StatusResultsReleased) ||
		ResultsVisible(ReleaseAfterClose, StatusOpen) || !ResultsVisible(ReleaseImmediate, StatusOpen) {
		t.Fatal("results visible")
	}
}

// F041-AC-13, AC-15: window with a late window, and the score policies.
func TestWindowAndScore(t *testing.T) {
	t0 := time.Now()
	w := Window{OpenAt: t0, DueAt: t0.Add(time.Hour), Late: 2 * time.Hour}
	if w.Open(t0.Add(-time.Second)) || !w.Open(t0.Add(150*time.Minute)) || w.Open(t0.Add(181*time.Minute)) || !w.IsLate(t0.Add(61*time.Minute)) {
		t.Fatal("window")
	}
	xs := []Scored{{AttemptID: [16]byte{1}, Score: 6, SubmittedAt: t0}, {AttemptID: [16]byte{2}, Score: 9, SubmittedAt: t0.Add(time.Minute)},
		{AttemptID: [16]byte{3}, Score: 9, SubmittedAt: t0.Add(2 * time.Minute)}}
	for policy, want := range map[string]byte{ScoreBest: 2, ScoreLast: 3, ScoreFirst: 1} {
		if got, _ := Pick(policy, xs); got.AttemptID[0] != want {
			t.Fatalf("%s picked %d", policy, got.AttemptID[0])
		}
	}
	if _, ok := Pick(ScoreBest, nil); ok {
		t.Fatal("empty")
	}
}
