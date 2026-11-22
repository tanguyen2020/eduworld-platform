package leaderboard

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ICT is Asia/Ho_Chi_Minh (UTC+7, no daylight saving): the zone of every period key (§9.5). A fixed zone needs no tzdata.
var ICT = time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)

// Season is the season a season-period key refers to.
type Season struct {
	ID             uuid.UUID
	StartsAt, Ends time.Time
}

// SeasonKey is the period key of a season.
func SeasonKey(id uuid.UUID) string { return "s-" + id.String() }

// AllTimeKey is the period key of all_time.
const AllTimeKey = "all"

// Key is the period key of t (weekly: ISO week, Monday 00:00 ICT).
func Key(period string, t time.Time, season *Season) string {
	l := t.In(ICT)
	switch period {
	case PeriodDaily:
		return l.Format("2006-01-02")
	case PeriodWeekly:
		y, w := l.ISOWeek()
		return fmt.Sprintf("%04d-W%02d", y, w)
	case PeriodMonthly:
		return l.Format("2006-01")
	case PeriodSeason:
		if season == nil {
			return ""
		}
		return SeasonKey(season.ID)
	}
	return AllTimeKey
}

// DayKey is the Asia/Ho_Chi_Minh day of t at midnight UTC (the date column day_key).
func DayKey(t time.Time) time.Time {
	l := t.In(ICT)
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, time.UTC)
}

// Bounds returns [start, end) of a period key; season keys take the bounds of season. ok false for a malformed key.
func Bounds(period, key string, season *Season) (start, end time.Time, ok bool) {
	switch period {
	case PeriodDaily:
		d, err := time.ParseInLocation("2006-01-02", key, ICT)
		if err != nil {
			return time.Time{}, time.Time{}, false
		}
		return d, d.AddDate(0, 0, 1), true
	case PeriodWeekly:
		y, w, ok := parseWeek(key)
		if !ok {
			return time.Time{}, time.Time{}, false
		}
		// ISO week 1 holds 4 January; its Monday starts the week.
		jan4 := time.Date(y, 1, 4, 0, 0, 0, 0, ICT)
		monday := jan4.AddDate(0, 0, -((int(jan4.Weekday()) + 6) % 7))
		s := monday.AddDate(0, 0, 7*(w-1))
		if yy, ww := s.ISOWeek(); yy != y || ww != w {
			return time.Time{}, time.Time{}, false
		}
		return s, s.AddDate(0, 0, 7), true
	case PeriodMonthly:
		m, err := time.ParseInLocation("2006-01", key, ICT)
		if err != nil {
			return time.Time{}, time.Time{}, false
		}
		return m, m.AddDate(0, 1, 0), true
	case PeriodSeason:
		if season == nil || key != SeasonKey(season.ID) {
			return time.Time{}, time.Time{}, false
		}
		return season.StartsAt, season.Ends, true
	case PeriodAllTime:
		if key != AllTimeKey {
			return time.Time{}, time.Time{}, false
		}
		return time.Date(2020, 1, 1, 0, 0, 0, 0, ICT), time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC), true
	}
	return time.Time{}, time.Time{}, false
}

func parseWeek(key string) (int, int, bool) {
	y, w, found := strings.Cut(key, "-W")
	if !found || len(y) != 4 || len(w) != 2 {
		return 0, 0, false
	}
	yy, err1 := strconv.Atoi(y)
	ww, err2 := strconv.Atoi(w)
	if err1 != nil || err2 != nil || ww < 1 || ww > 53 {
		return 0, 0, false
	}
	return yy, ww, true
}

// Previous is the key of the period before the one containing t (the period the close job snapshots).
func Previous(period string, t time.Time) string {
	l := t.In(ICT)
	switch period {
	case PeriodDaily:
		return Key(period, l.AddDate(0, 0, -1), nil)
	case PeriodWeekly:
		return Key(period, l.AddDate(0, 0, -7), nil)
	case PeriodMonthly:
		first := time.Date(l.Year(), l.Month(), 1, 0, 0, 0, 0, ICT)
		return Key(period, first.AddDate(0, 0, -1), nil)
	}
	return ""
}

// Tick is the tie-break part of a sorted-set score: the earlier a value is reached in the period, the higher the score
// (§9.3: equal points → who got there first ranks above). It fits below TickSpan.
func Tick(start, at time.Time) float64 {
	s := at.Sub(start).Seconds()
	if s < 0 {
		s = 0
	}
	if s >= TickSpan {
		// long periods (all_time) count minutes.
		s = at.Sub(start).Minutes()
	}
	return TickSpan - 1 - min(s, TickSpan-1)
}

// TickSpan is the multiplier of the value in a sorted-set score: score = value·TickSpan + Tick.
const TickSpan = 1e7

// Score is the sorted-set score of a value reached at the given tick.
func Score(value float64, tick float64) float64 { return value*TickSpan + tick }

// ValueOf returns the value of a sorted-set score (values are never negative).
func ValueOf(score float64) float64 { return math.Floor(score / TickSpan) }
