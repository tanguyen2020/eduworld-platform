package observability

import "github.com/prometheus/client_golang/prometheus"

// Dashboard metrics of DASHBOARD_DATA §13 (F043 T039). Labels never carry ids or names (R14).
var (
	dashQueryDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "eduworld_dashboard_query_duration_seconds",
		Help:    "Duration of the dashboard queries by query id, grain and cache result.",
		Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10},
	}, []string{"module", "query_id", "grain", "cache"})
	dashCacheRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_dashboard_cache_requests_total",
		Help: "Dashboard cache lookups by result (hit, miss).",
	}, []string{"module", "result"})
	dashRejected = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_dashboard_rejected_total",
		Help: "Dashboard queries refused by reason (range, busy, timeout).",
	}, []string{"module", "reason"})
	rollupDayClose = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:        "eduworld_rollup_day_close_duration_seconds",
		Help:        "Duration of one run of the job dashboard-day-close.",
		ConstLabels: prometheus.Labels{"module": "learning"},
		Buckets:     []float64{1, 5, 30, 60, 300, 900, 1800, 3600, 7200},
	})
	rollupLate = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_rollup_late_events_total",
		Help: "Learning events that reached the rollups after their day was closed (bucket <=48h reclosed, >48h dropped).",
	}, []string{"module", "bucket"})
)

// RegisterDashboards adds the dashboard metrics to r; already registered collectors are ignored.
func RegisterDashboards(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{dashQueryDuration, dashCacheRequests, dashRejected, rollupDayClose, rollupLate} {
		_ = r.Register(c)
	}
}

// DashboardCounter adapts the metrics to query.DashboardMetrics, command.RollupMetrics and command.LateMetrics.
type DashboardCounter struct{}

// Query observes one dashboard query.
func (DashboardCounter) Query(queryID, grain, cache string, seconds float64) {
	dashQueryDuration.WithLabelValues("learning", queryID, grain, cache).Observe(seconds)
	dashCacheRequests.WithLabelValues("learning", cache).Inc()
}

// Rejected counts one refused query.
func (DashboardCounter) Rejected(reason string) {
	dashRejected.WithLabelValues("learning", reason).Inc()
}

// DayCloseDuration observes one run of the day close.
func (DashboardCounter) DayCloseDuration(seconds float64) { rollupDayClose.Observe(seconds) }

// LateEvent counts one late event.
func (DashboardCounter) LateEvent(bucket string) {
	rollupLate.WithLabelValues("learning", bucket).Inc()
}

// LateEvents exposes the late-event counter to the tests.
func LateEvents() *prometheus.CounterVec { return rollupLate }

// DashboardRejected exposes the refusal counter to the tests.
func DashboardRejected() *prometheus.CounterVec { return dashRejected }
