package config

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/analytics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// Intervals of the Game Analytics jobs (F080 T002): the provisional rollup runs hourly (GAME_ANALYTICS §5), the day close
// is checked every 15 minutes (it finalises a day once dashboard_day_close_time has passed, like F043), the export queue
// every 15 seconds, the retention once a day.
const (
	AnalyticsRollupInterval    = time.Hour
	AnalyticsDayCloseInterval  = 15 * time.Minute
	AnalyticsExportInterval    = 15 * time.Second
	AnalyticsRetentionInterval = 24 * time.Hour
	AnalyticsRetentionBatch    = 10000
	// ExportBucket is the object storage bucket of the exports (ADR-0044, DASHBOARD_DATA §10).
	ExportBucket = "ew-exports"
)

// Analytics is the Game Analytics configuration.
type Analytics struct {
	Config        analytics.Config
	Keys          analytics.KeyRing
	PrometheusURL string
	Thresholds    query.KPIThresholds
}

// AnalyticsFrom reads EW_ANALYTICS_* (CONFIG_DEFAULTS owners GAME_ANALYTICS / DASHBOARD_DATA; never hard-coded — R3, R33):
// the player keys EW_ANALYTICS_PLAYER_KEYS(_FILE) as "version:secret,…" (secret store; empty = analytics ingestion
// disabled), EW_PROMETHEUS_URL (Live dashboard), and the dashboard and live-ops thresholds.
func AnalyticsFrom(c *config.Config) (Analytics, error) {
	d := analytics.DefaultConfig()
	d.MinCellSize = c.Int("EW_GAME_ANALYTICS_MIN_CELL_SIZE", d.MinCellSize)
	d.ActiveSessionMin = time.Duration(c.Int("EW_ANALYTICS_ACTIVE_SESSION_MIN_SECONDS", int(d.ActiveSessionMin.Seconds()))) * time.Second
	d.LateWindow = c.Duration("EW_ANALYTICS_LATE_EVENT_WINDOW", d.LateWindow)
	d.ProductTelemetry = c.Bool("EW_ANALYTICS_PRODUCT_TELEMETRY_ENABLED", d.ProductTelemetry)
	d.TelemetryRatePerMin = c.Int("EW_TELEMETRY_RATE_PER_MINUTE", d.TelemetryRatePerMin)
	d.RawRetention = time.Duration(c.Int("EW_ANALYTICS_RAW_RETENTION_DAYS", 90)) * 24 * time.Hour
	d.PlayerDaysRetention = time.Duration(c.Int("EW_ANALYTICS_PLAYER_DAYS_RETENTION_DAYS", 400)) * 24 * time.Hour
	d.Range.MaxDays = c.Int("EW_DASHBOARD_MAX_RANGE_DAYS", d.Range.MaxDays)
	d.StatementTimeout = c.Duration("EW_DASHBOARD_STATEMENT_TIMEOUT", d.StatementTimeout)
	d.MaxConcurrentPerUser = c.Int("EW_DASHBOARD_MAX_CONCURRENT_PER_USER", d.MaxConcurrentPerUser)
	d.MaxConcurrentTenant = c.Int("EW_DASHBOARD_MAX_CONCURRENT_PER_TENANT", d.MaxConcurrentTenant)
	d.ExportMaxRows = c.Int("EW_DASHBOARD_EXPORT_MAX_ROWS", d.ExportMaxRows)
	d.ExportMaxConcurrent = c.Int("EW_DASHBOARD_EXPORT_MAX_CONCURRENT_PER_USER", d.ExportMaxConcurrent)
	d.ExportLinkTTL = time.Duration(c.Int("EW_DASHBOARD_EXPORT_LINK_TTL_MINUTES", 15)) * time.Minute
	d.ExportRetention = time.Duration(c.Int("EW_DASHBOARD_EXPORT_FILE_RETENTION_DAYS", 7)) * 24 * time.Hour
	out := Analytics{Config: d, PrometheusURL: c.StringOr("EW_PROMETHEUS_URL", ""), Thresholds: query.DefaultKPIThresholds()}
	out.Thresholds.TargetCCU = float64(c.Int("EW_TARGET_CCU", int(out.Thresholds.TargetCCU)))
	switch {
	case d.MinCellSize < 5:
		return out, errors.New("game config: EW_GAME_ANALYTICS_MIN_CELL_SIZE must be ≥ 5 (GAME_ANALYTICS A-5)")
	case d.ActiveSessionMin < 0 || d.LateWindow < time.Hour || d.LateWindow > 7*24*time.Hour:
		return out, errors.New("game config: EW_ANALYTICS_LATE_EVENT_WINDOW must be 1h..168h")
	case d.Range.MaxDays < 31 || d.Range.MaxDays > 3650 || d.MaxConcurrentPerUser < 1 || d.ExportMaxConcurrent < 1 || d.ExportMaxRows < 1:
		return out, errors.New("game config: invalid dashboard limits")
	case d.TelemetryRatePerMin < 1:
		return out, errors.New("game config: EW_TELEMETRY_RATE_PER_MINUTE must be positive")
	}
	if raw := strings.TrimSpace(c.String("EW_ANALYTICS_PLAYER_KEYS")); raw != "" {
		keys, err := ParsePlayerKeys(raw)
		if err != nil {
			return out, err
		}
		if out.Keys, err = analytics.NewKeyRing(keys...); err != nil {
			return out, err
		}
	}
	return out, nil
}

// ParsePlayerKeys parses "version:secret,version:secret".
func ParsePlayerKeys(raw string) ([]analytics.Key, error) {
	var out []analytics.Key
	for _, part := range strings.Split(raw, ",") {
		v, s, ok := strings.Cut(strings.TrimSpace(part), ":")
		n, err := strconv.Atoi(v)
		if !ok || err != nil {
			return nil, errors.New("game config: EW_ANALYTICS_PLAYER_KEYS must be version:secret pairs")
		}
		out = append(out, analytics.Key{Version: n, Secret: []byte(s)})
	}
	return out, nil
}
