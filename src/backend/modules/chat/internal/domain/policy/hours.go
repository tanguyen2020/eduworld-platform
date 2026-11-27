package policy

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// TimeWindow is one allowed_hours window of a rule (chat-policy.schema.json v5 time_window): a day range ("mon-sun",
// "mon-fri", "sat") and a daily span [From, To) in the policy timezone; To < From spans midnight.
type TimeWindow struct {
	Days string `json:"days"`
	From string `json:"from"`
	To   string `json:"to"`
}

// ErrHours: allowed_hours is not a list of valid windows.
var ErrHours = errors.New("policy: allowed_hours must be windows {days, from, to}")

var dayIndex = map[string]int{"mon": 0, "tue": 1, "wed": 2, "thu": 3, "fri": 4, "sat": 5, "sun": 6}

const minutesPerWeek = 7 * 24 * 60

var defaultLoc = time.FixedZone("ICT", 7*3600)

// WeekMask marks the allowed minutes of a week (Monday 00:00 = minute 0) in the policy timezone.
type WeekMask [minutesPerWeek]bool

// ParseHours decodes a raw allowed_hours value (nil/null: no window set at that level).
func ParseHours(raw json.RawMessage) ([]TimeWindow, bool, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, false, nil
	}
	var ws []TimeWindow
	if err := json.Unmarshal(raw, &ws); err != nil {
		return nil, false, ErrHours
	}
	for _, w := range ws {
		if _, err := w.days(); err != nil {
			return nil, false, err
		}
		if _, err := minutes(w.From); err != nil {
			return nil, false, ErrHours
		}
		if _, err := minutes(w.To); err != nil {
			return nil, false, ErrHours
		}
	}
	return ws, true, nil
}

func (w TimeWindow) days() ([]int, error) {
	a, b, rng := strings.Cut(w.Days, "-")
	from, ok1 := dayIndex[a]
	to := from
	ok2 := true
	if rng {
		to, ok2 = dayIndex[b]
	}
	if !ok1 || !ok2 {
		return nil, ErrHours
	}
	var out []int
	for d := from; ; d = (d + 1) % 7 {
		out = append(out, d)
		if d == to {
			break
		}
	}
	return out, nil
}

// Mask returns the allowed minutes of the windows (an empty list allows nothing).
func Mask(ws []TimeWindow) WeekMask {
	var m WeekMask
	for _, w := range ws {
		days, err := w.days()
		if err != nil {
			continue
		}
		from, err1 := minutes(w.From)
		to, err2 := minutes(w.To)
		if err1 != nil || err2 != nil || from == to {
			continue
		}
		for _, d := range days {
			start := d * 1440
			if from < to {
				for i := start + from; i < start+to; i++ {
					m[i] = true
				}
				continue
			}
			for i := start + from; i < start+1440; i++ {
				m[i] = true
			}
			next := ((d + 1) % 7) * 1440
			for i := next; i < next+to; i++ {
				m[i] = true
			}
		}
	}
	return m
}

// CurfewMask marks the minutes of the week inside the curfew windows (every day alike).
func CurfewMask(c Curfew) WeekMask {
	var m WeekMask
	for _, w := range c {
		if w.Zero() {
			continue
		}
		for d := range 7 {
			start := d * 1440
			if w.Start < w.End {
				for i := start + w.Start; i < start+w.End; i++ {
					m[i] = true
				}
				continue
			}
			for i := start + w.Start; i < start+1440; i++ {
				m[i] = true
			}
			for i := start; i < start+w.End; i++ {
				m[i] = true
			}
		}
	}
	return m
}

// Within reports that every allowed minute of m is allowed by outer and outside the curfew.
func (m WeekMask) Within(outer *WeekMask, curfew WeekMask) bool {
	for i := range m {
		if !m[i] {
			continue
		}
		if curfew[i] || (outer != nil && !outer[i]) {
			return false
		}
	}
	return true
}

// minuteOfWeek is the minute of t in loc (Monday 00:00 = 0).
func minuteOfWeek(t time.Time, loc *time.Location) int {
	if loc == nil {
		loc = defaultLoc // the policy timezone default Asia/Ho_Chi_Minh (UTC+7, no DST)
	}
	l := t.In(loc)
	d := (int(l.Weekday()) + 6) % 7 // Monday = 0
	return d*1440 + l.Hour()*60 + l.Minute()
}

// HoursOpen reports whether t falls in the allowed hours of every level that set some (an AND across levels, an OR across
// the windows of one level — CHAT_SYSTEM §3: hours take the intersection). No level with hours: open.
func HoursOpen(levels [][]TimeWindow, t time.Time, loc *time.Location) bool {
	i := minuteOfWeek(t, loc)
	for _, ws := range levels {
		m := Mask(ws)
		if !m[i] {
			return false
		}
	}
	return true
}

// NextOpen returns the first whole minute after t at which the allowed hours of every level let a message through and
// blocked (the curfew of the student) does not refuse it — the reopen_at of CHAT_NOT_ALLOWED outside the allowed hours
// (delta F112-6); false when no such minute exists within a week (an empty window list allows nothing).
func NextOpen(levels [][]TimeWindow, t time.Time, loc *time.Location, blocked func(time.Time) bool) (time.Time, bool) {
	if loc == nil {
		loc = defaultLoc
	}
	masks := make([]WeekMask, len(levels))
	for i, ws := range levels {
		masks[i] = Mask(ws)
	}
	start := t.In(loc)
	start = time.Date(start.Year(), start.Month(), start.Day(), start.Hour(), start.Minute(), 0, 0, loc)
	for i := 1; i <= minutesPerWeek; i++ {
		c := start.Add(time.Duration(i) * time.Minute)
		m := minuteOfWeek(c, loc)
		open := true
		for k := range masks {
			if !masks[k][m] {
				open = false
				break
			}
		}
		if open && (blocked == nil || !blocked(c)) {
			return c.UTC(), true
		}
	}
	return time.Time{}, false
}

// ReopenAt is NextOpen of the allowed hours of e.
func (e Effective) ReopenAt(t time.Time, loc *time.Location, blocked func(time.Time) bool) (time.Time, bool) {
	return NextOpen(e.Hours, t, loc, blocked)
}
