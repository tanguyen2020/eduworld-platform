package analytics

import "time"

// Zone is the calendar of every analytics day: Vietnam time (UTC+7, no daylight saving, GAME_ANALYTICS §4).
var Zone = time.FixedZone("ICT", 7*3600)

// Window is a daily clock window [Start, End) in Zone; End before Start crosses midnight (analytics_late_night_window).
type Window struct {
	Start, End time.Duration // offsets from midnight
}

// Config holds the analytics and dashboard parameters of CONFIG_DEFAULTS (owner GAME_ANALYTICS, DASHBOARD_DATA). Module
// game reads them from its configuration (infrastructure/config); DefaultConfig holds the [Confirmed] defaults.
type Config struct {
	Range                RangeRules
	MinCellSize          int           // analytics_min_cell_size
	ActiveSessionMin     time.Duration // analytics_active_session_min_seconds
	LateNight            Window        // analytics_late_night_window
	LateWindow           time.Duration // analytics_late_event_window
	FutureSkew           time.Duration // analytics_telemetry_future_skew (occurred_at in the future beyond it → dropped)
	ProductTelemetry     bool          // analytics_product_telemetry_enabled (platform default)
	TelemetryRatePerMin  int           // telemetry_rate_limit_per_minute (API_DESIGN §8)
	RawRetention         time.Duration // analytics_raw_retention_days
	PlayerDaysRetention  time.Duration // analytics_player_days_retention_days
	StatementTimeout     time.Duration // dashboard_statement_timeout
	MaxConcurrentPerUser int           // dashboard_max_concurrent_per_user
	MaxConcurrentTenant  int           // dashboard_max_concurrent_per_tenant
	CacheTTLToday        time.Duration // dashboard_cache_ttl_today
	CacheTTLClosed       time.Duration // dashboard_cache_ttl_closed
	DayCloseTime         time.Duration // dashboard_day_close_time (offset from midnight in Zone)
	PageSizeMax          int           // dashboard_page_size_max
	PageSizeDefault      int
	ExportMaxRows        int           // dashboard_export_max_rows
	ExportMaxConcurrent  int           // dashboard_export_max_concurrent_per_user
	ExportLinkTTL        time.Duration // dashboard_export_link_ttl_minutes
	ExportRetention      time.Duration // dashboard_export_file_retention_days
	QuestAbandonReview   float64       // analytics_quest_abandon_review_rate (GAME_ANALYTICS §4.2: > 40 % → review list)
}

// DefaultConfig returns the CONFIG_DEFAULTS values.
func DefaultConfig() Config {
	return Config{
		Range:                RangeRules{MaxDays: 730, DailyMaxDays: 31, WeeklyMaxDays: 366},
		MinCellSize:          5,
		ActiveSessionMin:     60 * time.Second,
		LateNight:            Window{Start: 22 * time.Hour, End: 6 * time.Hour},
		LateWindow:           48 * time.Hour,
		FutureSkew:           5 * time.Minute,
		ProductTelemetry:     true,
		TelemetryRatePerMin:  30,
		RawRetention:         90 * 24 * time.Hour,
		PlayerDaysRetention:  400 * 24 * time.Hour,
		StatementTimeout:     5 * time.Second,
		MaxConcurrentPerUser: 4,
		MaxConcurrentTenant:  30,
		CacheTTLToday:        300 * time.Second,
		CacheTTLClosed:       86400 * time.Second,
		DayCloseTime:         2*time.Hour + 30*time.Minute,
		PageSizeMax:          100,
		PageSizeDefault:      50,
		ExportMaxRows:        1_000_000,
		ExportMaxConcurrent:  2,
		ExportLinkTTL:        15 * time.Minute,
		ExportRetention:      7 * 24 * time.Hour,
		QuestAbandonReview:   0.40,
	}
}

// DayOf is the analytics day (ICT calendar date) of the instant t.
func DayOf(t time.Time) time.Time { return DateOf(t.In(Zone)) }

// LastClosable is the last day the close job may finalise at now: yesterday once DayCloseTime has passed, the day before
// otherwise.
func (c Config) LastClosable(now time.Time) time.Time {
	n := now.In(Zone)
	today := DateOf(n)
	since := time.Duration(n.Hour())*time.Hour + time.Duration(n.Minute())*time.Minute + time.Duration(n.Second())*time.Second
	if since >= c.DayCloseTime {
		return today.AddDate(0, 0, -1)
	}
	return today.AddDate(0, 0, -2)
}

// PageSize clamps a requested page size to [1, PageSizeMax].
func (c Config) PageSize(n int) int {
	switch {
	case n <= 0:
		return c.PageSizeDefault
	case n > c.PageSizeMax:
		return c.PageSizeMax
	}
	return n
}

// Intersects reports whether the play interval [start, end] meets the window on any day (late_night_play_rate, §4.6).
func (w Window) Intersects(start, end time.Time) bool {
	if end.Before(start) {
		return false
	}
	s, e := start.In(Zone), end.In(Zone)
	for d := DateOf(s).AddDate(0, 0, -1); !d.After(DateOf(e)); d = d.AddDate(0, 0, 1) {
		ws := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, Zone).Add(w.Start)
		we := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, Zone).Add(w.End)
		if w.End <= w.Start {
			we = we.AddDate(0, 0, 1)
		}
		if s.Before(we) && e.After(ws) {
			return true
		}
	}
	return false
}

// LateAction is what ingestion does with an event of a given day (DASHBOARD_DATA §5, F080-AC-09).
type LateAction int

// Actions of Late.
const (
	Apply        LateAction = iota // day not closed yet: counted at the next rollup
	ApplyReclose                   // day closed, event within the late window: counted and the day is closed again
	Drop                           // later than the window: rollups unchanged, metric only
)

// Late decides the action for an event that occurred at occurredAt, ingested at now, on a day whose closing state is final.
func Late(occurredAt, now time.Time, window time.Duration, dayFinal bool) LateAction {
	if now.Sub(occurredAt) > window {
		return Drop
	}
	if dayFinal {
		return ApplyReclose
	}
	return Apply
}
