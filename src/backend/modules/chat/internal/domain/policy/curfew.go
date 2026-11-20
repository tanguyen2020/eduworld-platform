package policy

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

// Window is a daily curfew window [Start, End) in minutes after midnight of Loc (ADR-0078, CHILD_SAFETY §4.1); End <
// Start spans midnight (21:30–06:30). The zero value is "no curfew". Module chat keeps its own copy of the rule
// (modules never share internals, ADR-0074); the values come from the identity projections.
type Window struct {
	Start, End int
	Loc        *time.Location
}

// ErrWindow: a curfew bound is not HH:MM.
var ErrWindow = errors.New("policy: curfew must be HH:MM-HH:MM")

// ParseWindow parses "HH:MM-HH:MM" (EW_PLATFORM_CURFEW); "" is no curfew.
func ParseWindow(s string, loc *time.Location) (Window, error) {
	if s == "" {
		return Window{}, nil
	}
	a, b, ok := strings.Cut(strings.ReplaceAll(s, "–", "-"), "-")
	if !ok {
		return Window{}, ErrWindow
	}
	return NewWindow(strings.TrimSpace(a), strings.TrimSpace(b), loc)
}

// NewWindow builds a window from its bounds "HH:MM" (curfew_window {start, end}).
func NewWindow(start, end string, loc *time.Location) (Window, error) {
	s, err := minutes(start)
	if err != nil {
		return Window{}, err
	}
	e, err := minutes(end)
	if err != nil {
		return Window{}, err
	}
	if loc == nil {
		loc = time.UTC
	}
	return Window{Start: s, End: e, Loc: loc}, nil
}

func minutes(s string) (int, error) {
	h, m, ok := strings.Cut(s, ":")
	hh, err1 := strconv.Atoi(h)
	mm, err2 := strconv.Atoi(m)
	if !ok || err1 != nil || err2 != nil || hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return 0, ErrWindow
	}
	return hh*60 + mm, nil
}

// Zero reports the absence of a curfew.
func (w Window) Zero() bool { return w.Loc == nil || w.Start == w.End }

// Active reports whether now falls in the window and, if so, when it ends.
func (w Window) Active(now time.Time) (bool, time.Time) {
	if w.Zero() {
		return false, time.Time{}
	}
	t := now.In(w.Loc)
	minute := t.Hour()*60 + t.Minute()
	midnight := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, w.Loc)
	endToday := midnight.Add(time.Duration(w.End) * time.Minute)
	if w.Start < w.End {
		if minute >= w.Start && minute < w.End {
			return true, endToday
		}
		return false, time.Time{}
	}
	switch {
	case minute >= w.Start:
		return true, midnight.AddDate(0, 0, 1).Add(time.Duration(w.End) * time.Minute)
	case minute < w.End:
		return true, endToday
	}
	return false, time.Time{}
}

// Curfew is the effective curfew of a sender: the platform curfew united with the window of the school (a school may
// only extend it, ADR-0078). Same for every grade (R15).
type Curfew []Window

// Active reports whether now is in any window and when every overlapping window has ended (reopen_at).
func (c Curfew) Active(now time.Time) (bool, time.Time) {
	active, reopen := false, now
	for range len(c) + 1 {
		moved := false
		for _, w := range c {
			if ok, end := w.Active(reopen); ok && end.After(reopen) {
				active, reopen, moved = true, end, true
			}
		}
		if !moved {
			break
		}
	}
	if !active {
		return false, time.Time{}
	}
	return true, reopen
}
