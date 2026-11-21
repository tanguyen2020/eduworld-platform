package analytics

import "time"

// RangeRules are dashboard_max_range_days, dashboard_grain_daily_max_days and dashboard_grain_weekly_max_days.
type RangeRules struct {
	MaxDays, DailyMaxDays, WeeklyMaxDays int
}

// Config is the set of dashboard parameters of CONFIG_DEFAULTS (DASHBOARD_DATA §4–§9, SCHOOL_LMS §8.1). Module learning
// has no reader of platform/tenant settings for these keys yet: DefaultConfig holds the [Confirmed] defaults and the
// composition root passes them (same pattern as answer_rate, F030 T053) [Proposed until the settings reader exists].
type Config struct {
	Range                RangeRules
	PageSizeMax          int           // dashboard_page_size_max
	PageSizeDefault      int           // default of the DashboardPageSize parameter of the contract
	StatementTimeout     time.Duration // dashboard_statement_timeout
	MaxConcurrentPerUser int           // dashboard_max_concurrent_per_user
	MaxConcurrentTenant  int           // dashboard_max_concurrent_per_tenant
	CacheTTLToday        time.Duration // dashboard_cache_ttl_today
	CacheTTLClosed       time.Duration // dashboard_cache_ttl_closed
	DayCloseTime         time.Duration // dashboard_day_close_time (offset from midnight in Zone)
	LateWindow           time.Duration // dashboard_late_window_hours
	Refresh              time.Duration // school_dashboard_refresh
	MinCellSize          int           // analytics_min_cell_size
	Attention            Thresholds    // dashboard_attention_thresholds (tenant setting; platform default here)
	ReplicaLagAlert      time.Duration // dashboard_replica_lag_alert_seconds (above: replica_lag_warning, F043 T042)
	Zone                 *time.Location
}

// Zone is the calendar of bucket_date: Vietnam time (UTC+7, no daylight saving), as progress_daily of F031.
var Zone = time.FixedZone("ICT", 7*3600)

// DefaultConfig returns the CONFIG_DEFAULTS values.
func DefaultConfig() Config {
	return Config{
		Range:                RangeRules{MaxDays: 730, DailyMaxDays: 31, WeeklyMaxDays: 366},
		PageSizeMax:          100,
		PageSizeDefault:      50,
		StatementTimeout:     5 * time.Second,
		MaxConcurrentPerUser: 4,
		MaxConcurrentTenant:  30,
		CacheTTLToday:        300 * time.Second,
		CacheTTLClosed:       86400 * time.Second,
		DayCloseTime:         2*time.Hour + 30*time.Minute,
		LateWindow:           48 * time.Hour,
		Refresh:              15 * time.Minute,
		MinCellSize:          5,
		Attention:            DefaultThresholds(),
		ReplicaLagAlert:      60 * time.Second,
		Zone:                 Zone,
	}
}

func (c Config) zone() *time.Location {
	if c.Zone == nil {
		return Zone
	}
	return c.Zone
}

// PageSize clamps a requested page size to [1, PageSizeMax] (F043-AC-16: larger values are limited, not refused).
func (c Config) PageSize(n int) int {
	switch {
	case n <= 0:
		return c.PageSizeDefault
	case n > c.PageSizeMax:
		return c.PageSizeMax
	}
	return n
}

// Day is the calendar day (bucket_date) of the instant t.
func (c Config) Day(t time.Time) time.Time { return DateOf(t.In(c.zone())) }

// LastClosable is the last day the close job may finalise at now: yesterday once DayCloseTime has passed, the day before
// otherwise.
func (c Config) LastClosable(now time.Time) time.Time {
	n := now.In(c.zone())
	today := DateOf(n)
	since := time.Duration(n.Hour())*time.Hour + time.Duration(n.Minute())*time.Minute + time.Duration(n.Second())*time.Second
	if since >= c.DayCloseTime {
		return today.AddDate(0, 0, -1)
	}
	return today.AddDate(0, 0, -2)
}
