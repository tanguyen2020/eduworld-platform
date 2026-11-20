package retrywrong

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)

func day(t time.Time) string { return t.Format(time.DateOnly) }

// F032-AC-21 (T032, T036): due after 1 day, then 3, then 7; two correct answers on two different days clear the entry;
// a wrong answer restarts the schedule; two correct answers the same day count once.
func TestEntryLifecycle(t *testing.T) {
	s := DefaultSchedule()
	e := New(t0, s)
	if !e.NextDueAt.Equal(t0.Add(24*time.Hour)) || e.Due(t0) || !e.Due(t0.Add(25*time.Hour)) {
		t.Fatalf("new: %+v", e)
	}
	d1 := t0.Add(25 * time.Hour)
	if e.Correct(d1, day(d1), s) || e.Stage != 1 || !e.NextDueAt.Equal(d1.Add(72*time.Hour)) {
		t.Fatalf("first correct: %+v", e)
	}
	if e.Correct(d1.Add(time.Hour), day(d1), s) || e.CorrectStreak != 1 {
		t.Fatalf("same day twice: %+v", e)
	}
	e.Wrong(d1.Add(2*time.Hour), s)
	if e.Stage != 0 || e.CorrectStreak != 0 || e.WrongCount != 2 {
		t.Fatalf("wrong again: %+v", e)
	}
	d2, d3 := d1.Add(24*time.Hour), d1.Add(5*24*time.Hour)
	e.Correct(d2, day(d2), s)
	if !e.Correct(d3, day(d3), s) || e.Status != StatusCleared {
		t.Fatalf("cleared: %+v", e)
	}
	if e.Correct(d3.Add(24*time.Hour), "x", s) || e.Clear() {
		t.Fatal("cleared entry changed")
	}
	f := New(t0, s)
	if !f.Clear() || f.Status != StatusCleared {
		t.Fatal("re-evaluated")
	}
}
