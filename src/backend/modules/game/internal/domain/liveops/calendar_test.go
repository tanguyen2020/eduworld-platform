package liveops

import (
	"errors"
	"testing"
	"time"
)

// F079-AC-14 (events calendar, listGameConfigs from/to): the window is [from, to] as ICT calendar days, both inclusive,
// at most 62 days; both bounds go together; anything else names the field.
func TestParseCalendarWindow(t *testing.T) {
	w, err := ParseCalendarWindow("", "")
	if err != nil || w != nil {
		t.Fatalf("no window: %v %v", w, err)
	}
	w, err = ParseCalendarWindow("2026-12-01", "2026-12-31")
	if err != nil || w == nil {
		t.Fatalf("december: %v", err)
	}
	if want := time.Date(2026, 11, 30, 17, 0, 0, 0, time.UTC); !w.Start.Equal(want) {
		t.Fatalf("start %s, want %s (00:00 ICT)", w.Start, want)
	}
	if want := time.Date(2026, 12, 31, 17, 0, 0, 0, time.UTC); !w.End.Equal(want) {
		t.Fatalf("end %s, want %s (the day after to, 00:00 ICT, exclusive)", w.End, want)
	}
	if w, err := ParseCalendarWindow("2026-12-05", "2026-12-05"); err != nil || w.End.Sub(w.Start) != 24*time.Hour {
		t.Fatalf("one day: %v %v", w, err)
	}
	if _, err := ParseCalendarWindow("2026-12-01", "2027-01-31"); err != nil {
		t.Fatalf("62 days: %v", err)
	}
	for _, c := range []struct{ from, to, field string }{
		{"2026-12-01", "", "to"},
		{"", "2026-12-01", "from"},
		{"2026-12-1", "2026-12-31", "from"},
		{"2026-12-01", "2026-13-01", "to"},
		{"2026-12-31", "2026-12-01", "to"},
		{"2026-12-01", "2027-02-01", "to"}, // 63 days
	} {
		_, err := ParseCalendarWindow(c.from, c.to)
		var fe *FieldError
		if !errors.As(err, &fe) || fe.Field != c.field || !errors.Is(err, ErrInvalid) {
			t.Errorf("%s..%s: %v, want field %s", c.from, c.to, err, c.field)
		}
	}
}

// The schedule of an event intersects the window when it starts before the end of the window and ends after its start
// (half-open intervals: an event ending at 00:00 ICT of from is outside).
func TestCalendarWindowOverlaps(t *testing.T) {
	w, _ := ParseCalendarWindow("2026-12-01", "2026-12-07")
	ict := func(d, h int) time.Time { return time.Date(2026, 12, d, h, 0, 0, 0, ICT) }
	for _, c := range []struct {
		start, end time.Time
		want       bool
	}{
		{ict(1, 0), ict(2, 0), true},
		{ict(7, 23), ict(9, 0), true},
		{time.Date(2026, 11, 20, 0, 0, 0, 0, ICT), ict(20, 0), true},
		{time.Date(2026, 11, 20, 0, 0, 0, 0, ICT), ict(1, 0), false},
		{ict(8, 0), ict(9, 0), false},
	} {
		if got := w.Overlaps(c.start, c.end); got != c.want {
			t.Errorf("%s..%s: %v, want %v", c.start, c.end, got, c.want)
		}
	}
}
