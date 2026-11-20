package settings

import "time"

// KeyDailyPlayLimit is the school setting daily_play_limit_minutes (OQ-17, CHILD_SAFETY §4, F015-AC-30): the
// minutes of game sessions a student of the school may play per day (ICT); absent or null = off (the default).
const KeyDailyPlayLimit = "daily_play_limit_minutes"

// Bounds of daily_play_limit_minutes (minutes per day, CONFIG_DEFAULTS [Proposed] 15..300).
const (
	MinDailyPlayLimitMinutes = 15
	MaxDailyPlayLimitMinutes = 300
)

// ValidateDailyPlayLimit checks a daily_play_limit_minutes value.
func ValidateDailyPlayLimit(minutes int) error {
	if minutes < MinDailyPlayLimitMinutes || minutes > MaxDailyPlayLimitMinutes {
		return ErrInvalid{KeyDailyPlayLimit, "15..300 or null"}
	}
	return nil
}

// PlayDate is a calendar day in the platform time zone (ICT).
type PlayDate struct {
	Year  int
	Month time.Month
	Day   int
}

// String formats the day as YYYY-MM-DD.
func (d PlayDate) String() string {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC).Format(time.DateOnly)
}

// DateOf returns the day of t in loc.
func DateOf(t time.Time, loc *time.Location) PlayDate {
	y, m, d := t.In(loc).Date()
	return PlayDate{y, m, d}
}

// NextMidnight returns 00:00 of the day after now in loc (reopen_at of DAILY_PLAY_LIMIT_REACHED).
func NextMidnight(now time.Time, loc *time.Location) time.Time {
	y, m, d := now.In(loc).Date()
	return time.Date(y, m, d+1, 0, 0, 0, 0, loc)
}

// SplitByDay splits the play interval [end-duration, end] into the days of loc it covers, so a session over
// midnight counts on both days (the daily limit is per ICT day).
func SplitByDay(end time.Time, duration time.Duration, loc *time.Location) map[PlayDate]time.Duration {
	out := map[PlayDate]time.Duration{}
	if duration <= 0 {
		return out
	}
	start := end.Add(-duration)
	for start.Before(end) {
		next := NextMidnight(start, loc)
		if next.After(end) {
			next = end
		}
		out[DateOf(start, loc)] += next.Sub(start)
		start = next
	}
	return out
}

// DailyLimitReached reports whether played reaches a daily limit of limitMinutes (nil = no limit).
func DailyLimitReached(limitMinutes *int, played time.Duration) bool {
	return limitMinutes != nil && played >= time.Duration(*limitMinutes)*time.Minute
}
