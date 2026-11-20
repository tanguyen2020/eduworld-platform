package analytics

import (
	"errors"
	"testing"
	"time"
)

func day(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t
}

// F043-AC-16, F043-AC-17 (TC-DASH-002, TC-DASH-003, AC-DASH-002/003): range limits and the grain chosen by the server.
func TestNewRangeGrainAndLimits(t *testing.T) {
	r := DefaultConfig().Range
	cases := []struct {
		from, to string
		hint     Grain
		want     Grain
		err      error
	}{
		{"2026-11-01", "2026-11-01", "", Day, nil},
		{"2026-10-01", "2026-10-31", "", Day, nil},             // 31 days
		{"2026-10-01", "2026-11-01", "", Week, nil},            // 32 days
		{"2026-05-20", "2026-11-17", "", Week, nil},            // ~6 months
		{"2025-11-17", "2026-11-17", "", Week, nil},            // 366 days
		{"2025-11-16", "2026-11-17", "", Month, nil},           // 367 days
		{"2024-11-18", "2026-11-17", "", Month, nil},           // 730 days
		{"2024-11-17", "2026-11-17", "", "", ErrRangeTooLarge}, // 731 days
		{"2026-11-17", "2026-11-01", "", "", ErrRangeInverted},
		{"2026-10-01", "2026-10-31", Month, Month, nil}, // coarser hint accepted
		{"2025-11-16", "2026-11-17", Week, "", ErrGrainTooFine},
		{"2026-10-01", "2026-11-01", Day, "", ErrGrainTooFine},
		{"2026-10-01", "2026-11-01", "hour", "", ErrGrainTooFine},
	}
	for _, c := range cases {
		got, err := NewRange(day(c.from), day(c.to), c.hint, r)
		if !errors.Is(err, c.err) || (err == nil && got.Grain != c.want) {
			t.Errorf("%s..%s hint %q: got %v %v, want %v %v", c.from, c.to, c.hint, got.Grain, err, c.want, c.err)
		}
	}
	if _, err := NewRange(time.Time{}, day("2026-11-01"), "", r); !errors.Is(err, ErrRangeMissing) {
		t.Errorf("missing from: %v", err)
	}
}

// DASHBOARD_DATA §8 rule 4: every series of a valid range has at most 120 points; weeks start on Monday.
func TestBucketsBounded(t *testing.T) {
	r := DefaultConfig().Range
	for _, c := range []struct {
		from, to string
		n        int
	}{{"2026-10-18", "2026-11-17", 31}, {"2025-11-17", "2026-11-17", 53}, {"2024-11-18", "2026-11-17", 25}} {
		rg, err := NewRange(day(c.from), day(c.to), "", r)
		if err != nil {
			t.Fatal(err)
		}
		b := rg.Buckets()
		if len(b) != c.n || len(b) > MaxPoints {
			t.Errorf("%s..%s: %d buckets, want %d", c.from, c.to, len(b), c.n)
		}
	}
	if got := BucketStart(day("2026-11-22"), Week); !got.Equal(day("2026-11-16")) {
		t.Errorf("ISO week of Sunday 2026-11-22 starts %v", got)
	}
	if got := BucketStart(day("2026-11-22"), Month); !got.Equal(day("2026-11-01")) {
		t.Errorf("month start %v", got)
	}
}

// F043-AC-16: page_size above dashboard_page_size_max is limited.
func TestPageSizeClamped(t *testing.T) {
	c := DefaultConfig()
	if c.PageSize(500) != 100 || c.PageSize(0) != 50 || c.PageSize(20) != 20 {
		t.Fatalf("page sizes %d %d %d", c.PageSize(500), c.PageSize(0), c.PageSize(20))
	}
}

// F043-AC-05: accuracy excludes rapid guesses; ratios are num/den, nil without denominator.
func TestAccuracyAndRatio(t *testing.T) {
	num, den := Accuracy(10, 7, 3, 2)
	if num != 5 || den != 7 {
		t.Fatalf("accuracy %d/%d", num, den)
	}
	if Ratio(1, 0) != nil || *Ratio(1, 3) != 0.333 {
		t.Fatal("ratio")
	}
	if v := BelowProficient([]int{4, 2, 1, 3, 1}); v == nil || *v != 0.429 {
		t.Fatalf("below proficient %v", v)
	}
}

// F043-AC-08 (TC-LMS-003): the "needs attention" rule of dashboard_attention_thresholds.
func TestAttentionThresholds(t *testing.T) {
	th := DefaultThresholds()
	seven, six := 7, 6
	cases := []struct {
		s    Signals
		want []Reason
	}{
		{Signals{}, nil},
		{Signals{Missed: 2}, []Reason{ReasonMissedAssignments}},
		{Signals{Missed: 1, Beginning: 2}, nil},
		{Signals{Beginning: 3}, []Reason{ReasonBeginningObjectives}},
		{Signals{DaysSinceLogin: &seven}, []Reason{ReasonInactiveGame}},
		{Signals{DaysSinceLogin: &six}, nil},
		{Signals{RapidNum: 3, RapidDen: 10}, []Reason{ReasonRapidGuess}},
		{Signals{RapidNum: 2, RapidDen: 10}, nil},
		{Signals{Missed: 3, Beginning: 4, RapidNum: 5, RapidDen: 5, ScheduledCleanup: true},
			[]Reason{ReasonMissedAssignments, ReasonBeginningObjectives, ReasonRapidGuess, ReasonScheduledCleanup}},
	}
	for i, c := range cases {
		got := th.Evaluate(c.s)
		if len(got) != len(c.want) {
			t.Errorf("case %d: %v, want %v", i, got, c.want)
			continue
		}
		for j := range got {
			if got[j] != c.want[j] {
				t.Errorf("case %d: %v, want %v", i, got, c.want)
			}
		}
	}
}

// F043-AC-25 (AC-DASH-011): cells < 5 students are hidden, and a lone hidden cell of a group hides the smallest other one
// so that the group total cannot reveal it.
func TestSuppressKAnonymity(t *testing.T) {
	cells := []Cell{{"s1", 30}, {"s1", 3}, {"s1", 12}, {"s2", 2}, {"s2", 1}, {"s2", 40}, {"s3", 4}, {"s4", 50}, {"s4", 60}}
	got := Suppress(cells, 5)
	want := []bool{false, true, true, true, true, false, true, false, false}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("suppressed %v, want %v", got, want)
		}
	}
	// No visible total of s1 can be combined with the visible cells to recover a hidden one: two cells are hidden.
	hidden := 0
	for i, c := range cells {
		if c.Group == "s1" && got[i] {
			hidden++
		}
	}
	if hidden < 2 {
		t.Fatal("complementary suppression missing")
	}
}

// F043-AC-04 (TC-DASH-005, AC-DASH-007): a late event within the window recloses the day, a later one is dropped.
func TestLateEvents(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	w := 48 * time.Hour
	if Late(now.Add(-60*time.Hour), now, w, false) != Apply {
		t.Error("open day")
	}
	if Late(now.Add(-30*time.Hour), now, w, true) != ApplyReclose {
		t.Error("30 h late")
	}
	if Late(now.Add(-50*time.Hour), now, w, true) != Drop {
		t.Error("50 h late")
	}
}

// dashboard_day_close_time 02:30 ICT: yesterday is closable after 02:30 only.
func TestLastClosable(t *testing.T) {
	c := DefaultConfig()
	before := time.Date(2026, 11, 20, 2, 0, 0, 0, Zone)
	after := time.Date(2026, 11, 20, 2, 31, 0, 0, Zone)
	if !c.LastClosable(before).Equal(day("2026-11-18")) || !c.LastClosable(after).Equal(day("2026-11-19")) {
		t.Fatalf("closable %v %v", c.LastClosable(before), c.LastClosable(after))
	}
	if !c.Day(time.Date(2026, 11, 19, 18, 0, 0, 0, time.UTC)).Equal(day("2026-11-20")) {
		t.Fatal("ICT day")
	}
}
