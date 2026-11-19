// Package metrics holds the Prometheus collectors of the SIS pipeline (SIS_INTEGRATION §4.5, §8).
package metrics

import (
	"time"

	"github.com/google/uuid"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
)

var (
	loginSync = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_sis_login_sync_total",
		Help: "Login sync by branch (fresh, blocking, background) and result."}, []string{"branch", "result"})
	loginSyncDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "eduworld_sis_login_sync_duration_seconds",
		Help: "Duration of login sync.", Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 3, 5}}, []string{"branch"})
	lastSuccess = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "eduworld_sis_last_success_timestamp",
		Help: "Unix time of the last successful sync run of a SIS connection."}, []string{"connection"})
	syncRuns = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_sis_sync_runs_total",
		Help: "SIS sync runs by connection, mode and status."}, []string{"connection", "mode", "status"})
)

// Collectors returns the collectors to register in the worker/core-api registry.
func Collectors() []prometheus.Collector {
	return []prometheus.Collector{loginSync, loginSyncDuration, lastSuccess, syncRuns}
}

// LoginSync implements port.LoginSyncMetrics.
type LoginSync struct{}

var _ port.LoginSyncMetrics = LoginSync{}

// Observe records one login sync.
func (LoginSync) Observe(branch, result string, d time.Duration) {
	loginSync.WithLabelValues(branch, result).Inc()
	loginSyncDuration.WithLabelValues(branch).Observe(d.Seconds())
}

// Runs implements port.SyncRunObserver.
type Runs struct{}

var _ port.SyncRunObserver = Runs{}

// RunFinished counts the run and sets the last success time of the connection for succeeded runs.
func (Runs) RunFinished(connectionID uuid.UUID, mode, status string, at time.Time) {
	syncRuns.WithLabelValues(connectionID.String(), mode, status).Inc()
	if status == "succeeded" {
		RunSucceeded(connectionID.String(), at)
	}
}

// RunSucceeded sets eduworld_sis_last_success_timestamp of a connection.
func RunSucceeded(connectionID string, at time.Time) {
	lastSuccess.WithLabelValues(connectionID).Set(float64(at.Unix()))
}

// LoginSyncCounter exposes eduworld_sis_login_sync_total (tests read it with prometheus/testutil).
func LoginSyncCounter() *prometheus.CounterVec { return loginSync }

// LastSuccess exposes eduworld_sis_last_success_timestamp (tests).
func LastSuccess() *prometheus.GaugeVec { return lastSuccess }
