// Package analytics holds the pure rules of Game Analytics (F080, GAME_ANALYTICS §2–§9, DASHBOARD_DATA §3–§9): the metric
// catalog (metric_code, metric_version), the fixed dimension sets of the rollups, the pseudonymous keys, the mapping of
// domain events and client telemetry to analytics events, histograms and percentiles, small-cell suppression and the range
// and grain of a dashboard query. No I/O.
package analytics

import (
	"errors"
	"time"
)

// Grain is the time granularity of a dashboard answer (DashboardGrain of the contract).
type Grain string

// Grains of DASHBOARD_DATA §8.
const (
	Day   Grain = "day"
	Week  Grain = "week"
	Month Grain = "month"
)

func (g Grain) rank() int {
	switch g {
	case Day:
		return 0
	case Week:
		return 1
	case Month:
		return 2
	}
	return -1
}

// Valid reports a known grain.
func (g Grain) Valid() bool { return g.rank() >= 0 }

// MaxPoints is the largest number of points of a series (DASHBOARD_DATA §8 rule 4).
const MaxPoints = 120

// Errors of a dashboard range (422 DASHBOARD_RANGE_TOO_LARGE, F080-AC-12).
var (
	ErrRangeMissing  = errors.New("from and to are required")
	ErrRangeInverted = errors.New("to is before from")
	ErrRangeTooLarge = errors.New("range is larger than dashboard_max_range_days")
	ErrGrainTooFine  = errors.New("grain is finer than the rule of the range")
)

// RangeRules are dashboard_max_range_days, dashboard_grain_daily_max_days and dashboard_grain_weekly_max_days.
type RangeRules struct {
	MaxDays, DailyMaxDays, WeeklyMaxDays int
}

// Range is a validated dashboard range: calendar dates [From, To] (both included) and the grain the server chose.
type Range struct {
	From, To time.Time
	Grain    Grain
}

// DateOf is the calendar date of t (its year, month and day at 00:00 UTC).
func DateOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// Days is the number of calendar days of [from, to], both included.
func Days(from, to time.Time) int {
	return int(DateOf(to).Sub(DateOf(from)).Hours()/24) + 1
}

// NewRange validates from/to and chooses the grain (≤ DailyMaxDays → day, ≤ WeeklyMaxDays → week, otherwise month); a
// hint may only be coarser than the rule (DASHBOARD_DATA §8 rule 3).
func NewRange(from, to time.Time, hint Grain, r RangeRules) (Range, error) {
	if from.IsZero() || to.IsZero() {
		return Range{}, ErrRangeMissing
	}
	from, to = DateOf(from), DateOf(to)
	if to.Before(from) {
		return Range{}, ErrRangeInverted
	}
	days := Days(from, to)
	if days > r.MaxDays {
		return Range{}, ErrRangeTooLarge
	}
	g := Month
	switch {
	case days <= r.DailyMaxDays:
		g = Day
	case days <= r.WeeklyMaxDays:
		g = Week
	}
	if hint != "" {
		if !hint.Valid() || hint.rank() < g.rank() {
			return Range{}, ErrGrainTooFine
		}
		g = hint
	}
	return Range{From: from, To: to, Grain: g}, nil
}

// BucketStart is the first day of the bucket of d: the day, the ISO Monday or the first of the month.
func BucketStart(d time.Time, g Grain) time.Time {
	d = DateOf(d)
	switch g {
	case Week:
		wd := (int(d.Weekday()) + 6) % 7 // Monday = 0
		return d.AddDate(0, 0, -wd)
	case Month:
		return time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	return d
}

// BucketEnd is the last day of the bucket starting at start.
func BucketEnd(start time.Time, g Grain) time.Time {
	switch g {
	case Week:
		return start.AddDate(0, 0, 6)
	case Month:
		return start.AddDate(0, 1, -1)
	}
	return start
}

// Buckets lists the bucket starts that intersect the range (at most MaxPoints).
func (r Range) Buckets() []time.Time {
	var out []time.Time
	for b := BucketStart(r.From, r.Grain); !b.After(r.To) && len(out) < MaxPoints; b = BucketEnd(b, r.Grain).AddDate(0, 0, 1) {
		out = append(out, b)
	}
	return out
}

// QueryFrom is the first bucket start of the range (lower bound of metric_date in the weekly / monthly tables).
func (r Range) QueryFrom() time.Time { return BucketStart(r.From, r.Grain) }

// Previous is the range of the same length just before r (comparison of two periods, GAME_ANALYTICS §7.1).
func (r Range) Previous() Range {
	n := Days(r.From, r.To)
	return Range{From: r.From.AddDate(0, 0, -n), To: r.From.AddDate(0, 0, -1), Grain: r.Grain}
}
