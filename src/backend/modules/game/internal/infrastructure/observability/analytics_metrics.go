package observability

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
)

// Metrics of Game Analytics (F080, GAME_ANALYTICS §10, MONITORING). Labels never carry a user id, a tenant or content
// (F080-AC-18, R14).
var (
	telemetryEvents = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_telemetry_events_total", Help: "Client telemetry events by type and acceptance (F080).",
	}, []string{"type", "accepted", "reason"})
	analyticsDropped = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_analytics_events_dropped_total", Help: "Domain events not written to analytics by reason (qa_tenant, unknown_student…).",
	}, []string{"reason"})
	rollupLate = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "eduworld_rollup_late_events_total", Help: "Analytics events later than analytics_late_event_window (rollups unchanged).",
	})
	rawRows = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "eduworld_analytics_raw_rows", Help: "Rows written to analytics.events_raw (monotonic; size alert AnalyticsRawRowsHigh).",
	})
	rollupDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "eduworld_analytics_rollup_duration_seconds", Help: "Duration of the analytics rollup of one tenant day.",
		Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 300},
	}, []string{"step"})
	queryDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "eduworld_analytics_query_duration_seconds", Help: "Duration of the Game Analytics dashboard queries.",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5},
	}, []string{"query", "cache"})
	dashRejected = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_game_dashboard_rejected_total", Help: "Game Analytics queries rejected by reason (range, busy, timeout).",
	}, []string{"reason"})
	matchDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "eduworld_match_duration_seconds", Help: "Duration of completed matches by mode (FR-OPS-003).",
		Buckets: []float64{30, 60, 120, 180, 300, 600, 900, 1800},
	}, []string{"mode"})
)

// RegisterAnalytics registers the F080 metrics.
func RegisterAnalytics(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{telemetryEvents, analyticsDropped, rollupLate, rawRows, rollupDuration, queryDuration, dashRejected,
		matchDuration} {
		_ = r.Register(c)
	}
}

// AnalyticsMetrics implements port.AnalyticsMetrics.
type AnalyticsMetrics struct{}

var _ port.AnalyticsMetrics = AnalyticsMetrics{}

func boolLabel(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// Telemetry counts one telemetry event.
func (AnalyticsMetrics) Telemetry(typ string, accepted bool, reason string) {
	telemetryEvents.WithLabelValues(typ, boolLabel(accepted), reason).Inc()
}

// Dropped counts one domain event not written.
func (AnalyticsMetrics) Dropped(reason string) { analyticsDropped.WithLabelValues(reason).Inc() }

// LateEvent counts one event later than the late window.
func (AnalyticsMetrics) LateEvent() { rollupLate.Inc() }

// RawInserted counts the rows written.
func (AnalyticsMetrics) RawInserted(n int) { rawRows.Add(float64(max(n, 0))) }

// Rollup observes one rollup step.
func (AnalyticsMetrics) Rollup(step string, seconds float64) {
	rollupDuration.WithLabelValues(step).Observe(seconds)
}

// Query observes one dashboard query.
func (AnalyticsMetrics) Query(id string, seconds float64, cache string) {
	queryDuration.WithLabelValues(id, cache).Observe(seconds)
}

// Rejected counts one rejected dashboard query.
func (AnalyticsMetrics) Rejected(reason string) { dashRejected.WithLabelValues(reason).Inc() }

// MatchDuration observes one completed match.
func (AnalyticsMetrics) MatchDuration(mode string, seconds float64) {
	if mode == "" {
		mode = "unknown"
	}
	matchDuration.WithLabelValues(mode).Observe(seconds)
}
