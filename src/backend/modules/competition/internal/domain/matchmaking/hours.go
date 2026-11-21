package matchmaking

import (
	"encoding/json"
	"slices"
	"strings"
	"time"
)

// DefaultTimezone is the timezone of an allowed-hours window without one (Việt Nam).
const DefaultTimezone = "Asia/Ho_Chi_Minh"

// HourWindow is one window of allowed_hours (school.updated settings_subset): days mon…sun, from/to "HH:MM".
type HourWindow struct {
	Days []string `json:"days"`
	From string   `json:"from"`
	To   string   `json:"to"`
}

// AllowedHours is the allowed_hours of a school (H5: matchmaking only inside the access windows).
type AllowedHours struct {
	Timezone string       `json:"timezone"`
	Windows  []HourWindow `json:"windows"`
}

// ParseAllowedHours reads settings_subset.allowed_hours; nil when absent, null or unreadable (no window = no limit).
func ParseAllowedHours(settings []byte) *AllowedHours {
	var s struct {
		AllowedHours *AllowedHours `json:"allowed_hours"`
	}
	if len(settings) == 0 || json.Unmarshal(settings, &s) != nil || s.AllowedHours == nil || len(s.AllowedHours.Windows) == 0 {
		return nil
	}
	return s.AllowedHours
}

var weekdays = []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}

// Contains reports now inside one window (local time of the timezone; to is exclusive; to ≤ from spans midnight).
func (a AllowedHours) Contains(now time.Time) bool {
	tz := a.Timezone
	if tz == "" {
		tz = DefaultTimezone
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.FixedZone("ICT", 7*3600)
	}
	local := now.In(loc)
	minute := local.Hour()*60 + local.Minute()
	day := weekdays[local.Weekday()]
	prev := weekdays[(int(local.Weekday())+6)%7]
	for _, w := range a.Windows {
		from, ok1 := hhmm(w.From)
		to, ok2 := hhmm(w.To)
		if !ok1 || !ok2 {
			continue
		}
		if from < to {
			if slices.Contains(w.Days, day) && minute >= from && minute < to {
				return true
			}
			continue
		}
		if (slices.Contains(w.Days, day) && minute >= from) || (slices.Contains(w.Days, prev) && minute < to) {
			return true
		}
	}
	return false
}

func hhmm(s string) (int, bool) {
	h, m, ok := strings.Cut(s, ":")
	if !ok || len(h) != 2 || len(m) != 2 {
		return 0, false
	}
	hh := int(h[0]-'0')*10 + int(h[1]-'0')
	mm := int(m[0]-'0')*10 + int(m[1]-'0')
	if hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return 0, false
	}
	return hh*60 + mm, true
}
