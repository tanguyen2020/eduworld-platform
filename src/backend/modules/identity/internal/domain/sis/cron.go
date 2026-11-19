package sis

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

// Default schedules of sis_sync_schedule (CONFIG_DEFAULTS, tenant scope [Proposed]): full sync at 01:00 ICT,
// delta every 15 minutes during office hours (07:00–17:59 ICT, Monday–Saturday [Proposed]).
const (
	DefaultFullSchedule  = "0 1 * * *"
	DefaultDeltaSchedule = "*/15 7-17 * * 1-6"
)

// ScheduleZone is the zone of sync schedules (ICT, UTC+7, no daylight saving).
var ScheduleZone = time.FixedZone("ICT", 7*3600)

// Cron is a parsed 5-field cron expression (minute hour day-of-month month day-of-week).
type Cron struct{ fields [5]map[int]bool }

var cronRanges = [5][2]int{{0, 59}, {0, 23}, {1, 31}, {1, 12}, {0, 6}}

// ErrCron is an invalid cron expression.
var ErrCron = errors.New("sis: invalid cron expression")

// ParseCron parses "*", "n", "a-b", "*/s", "a-b/s" and comma lists in each of the 5 fields.
func ParseCron(expr string) (Cron, error) {
	parts := strings.Fields(expr)
	if len(parts) != 5 {
		return Cron{}, ErrCron
	}
	var c Cron
	for i, p := range parts {
		set := map[int]bool{}
		for _, item := range strings.Split(p, ",") {
			lo, hi := cronRanges[i][0], cronRanges[i][1]
			step := 1
			rng, s, hasStep := strings.Cut(item, "/")
			if hasStep {
				n, err := strconv.Atoi(s)
				if err != nil || n <= 0 {
					return Cron{}, ErrCron
				}
				step = n
			}
			if rng != "*" {
				a, b, isRange := strings.Cut(rng, "-")
				x, err := strconv.Atoi(a)
				if err != nil {
					return Cron{}, ErrCron
				}
				y := x
				if isRange {
					if y, err = strconv.Atoi(b); err != nil {
						return Cron{}, ErrCron
					}
				} else if hasStep {
					y = hi
				}
				if x < lo || y > hi || x > y {
					return Cron{}, ErrCron
				}
				lo, hi = x, y
			}
			for v := lo; v <= hi; v += step {
				set[v] = true
			}
		}
		c.fields[i] = set
	}
	return c, nil
}

// Matches reports whether the minute of t (in ScheduleZone) is scheduled.
func (c Cron) Matches(t time.Time) bool {
	t = t.In(ScheduleZone)
	v := [5]int{t.Minute(), t.Hour(), t.Day(), int(t.Month()), int(t.Weekday())}
	for i := range v {
		if !c.fields[i][v[i]] {
			return false
		}
	}
	return true
}
