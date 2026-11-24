package liveops

import "time"

// CalendarWindowMaxDays is the longest window of the events calendar of the console (listGameConfigs from/to,
// F079-AC-14): two months of ICT calendar days, both bounds inclusive.
const CalendarWindowMaxDays = 62

// CalendarWindow is a window of ICT calendar days [from, to] (both inclusive) as a half-open instant interval: Start is
// 00:00 ICT of from, End is 00:00 ICT of the day after to (exclusive).
type CalendarWindow struct {
	Start, End time.Time
}

// ParseCalendarWindow reads the from/to query of the events calendar (dates YYYY-MM-DD in ICT). Both empty = no window
// (nil); one bound alone, an invalid date, to before from or more than CalendarWindowMaxDays days is a FieldError naming
// from or to.
func ParseCalendarWindow(from, to string) (*CalendarWindow, error) {
	switch {
	case from == "" && to == "":
		return nil, nil
	case from == "":
		return nil, invalid("from", "from and to go together")
	case to == "":
		return nil, invalid("to", "from and to go together")
	}
	f, err := time.ParseInLocation(time.DateOnly, from, ICT)
	if err != nil {
		return nil, invalid("from", "a date YYYY-MM-DD is required")
	}
	t, err := time.ParseInLocation(time.DateOnly, to, ICT)
	if err != nil {
		return nil, invalid("to", "a date YYYY-MM-DD is required")
	}
	end := t.AddDate(0, 0, 1)
	switch {
	case end.Before(f.AddDate(0, 0, 1)):
		return nil, invalid("to", "must not be before from")
	case end.After(f.AddDate(0, 0, CalendarWindowMaxDays)):
		return nil, invalid("to", "the window is at most %d days", CalendarWindowMaxDays)
	}
	return &CalendarWindow{Start: f.UTC(), End: end.UTC()}, nil
}

// Overlaps reports whether a schedule [start, end) intersects the window.
func (w CalendarWindow) Overlaps(start, end time.Time) bool {
	return start.Before(w.End) && end.After(w.Start)
}
