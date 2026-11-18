// Package metrics holds the Prometheus collectors of the SIS pipeline (SIS_INTEGRATION §4.5, §8).
package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
)

var (
	loginSync = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_sis_login_sync_total",
		Help: "Login sync by branch (fresh, blocking, background) and result."}, []string{"branch", "result"})
	loginSyncDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "eduworld_sis_login_sync_duration_seconds",
		Help: "Duration of login sync.", Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 3, 5}}, []string{"branch"})
	lastSuccess = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "eduworld_sis_last_success_timestamp",
		Help: "Unix time of the last successful sync run of a SIS connection."}, []string{"sis_connection_id"})
)

// Collectors returns the collectors to register in the worker/core-api registry.
func Collectors() []prometheus.Collector {
	return []prometheus.Collector{loginSync, loginSyncDuration, lastSuccess}
}

// LoginSync implements port.LoginSyncMetrics.
type LoginSync struct{}

var _ port.LoginSyncMetrics = LoginSync{}

// Observe records one login sync.
func (LoginSync) Observe(branch, result string, d time.Duration) {
	loginSync.WithLabelValues(branch, result).Inc()
	loginSyncDuration.WithLabelValues(branch).Observe(d.Seconds())
}

// RunSucceeded sets eduworld_sis_last_success_timestamp of a connection.
func RunSucceeded(connectionID string, at time.Time) {
	lastSuccess.WithLabelValues(connectionID).Set(float64(at.Unix()))
}

// LoginSyncCounter exposes eduworld_sis_login_sync_total (tests read it with prometheus/testutil).
func LoginSyncCounter() *prometheus.CounterVec { return loginSync }
