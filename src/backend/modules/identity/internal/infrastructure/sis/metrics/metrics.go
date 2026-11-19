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
	return []prometheus.Collector{loginSync, loginSyncDuration, lastSuccess, syncRuns, unknownFields, httpRequests, webhooks, runsSkipped, runDuration}
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

// F017 collectors (SIS_INTEGRATION §8): unknown fields of contract responses, HTTP calls of the contract_v1
// client, webhook outcomes, skipped scheduled runs and run durations.
var (
	unknownFields = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_sis_contract_unknown_fields_total",
		Help: "Fields outside the EduWorld SIS Contract dropped from facade responses (never stored)."}, []string{"connection", "entity"})
	httpRequests = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_sis_http_requests_total",
		Help: "Requests of the contract_v1 client by connection, contract operation and HTTP status class."}, []string{"connection", "operation", "status"})
	webhooks = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_sis_webhook_total",
		Help: "SIS webhook deliveries by result (accepted, replay, unauthorized, not_found, invalid, unavailable)."}, []string{"result"})
	runsSkipped = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_sis_sync_runs_skipped_total",
		Help: "Scheduled SIS runs skipped (another run holds the lock of the connection)."}, []string{"connection", "reason"})
	runDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "eduworld_sis_sync_duration_seconds",
		Help: "Duration of SIS sync runs.", Buckets: []float64{1, 5, 15, 30, 60, 120, 300, 600, 900, 1800}}, []string{"connection", "mode"})
)

// UnknownFields counts dropped fields of one entity kind.
func UnknownFields(connectionID uuid.UUID, entity string, n int) {
	if n > 0 {
		unknownFields.WithLabelValues(connectionID.String(), entity).Add(float64(n))
	}
}

// HTTPRequest counts one call of the contract_v1 client (status "2xx", "4xx", "429", "5xx", "error").
func HTTPRequest(connectionID uuid.UUID, operation, status string) {
	httpRequests.WithLabelValues(connectionID.String(), operation, status).Inc()
}

// UnknownFieldsCounter exposes eduworld_sis_contract_unknown_fields_total (tests).
func UnknownFieldsCounter() *prometheus.CounterVec { return unknownFields }

// WebhookCounter exposes eduworld_sis_webhook_total (tests).
func WebhookCounter() *prometheus.CounterVec { return webhooks }

// SkippedCounter exposes eduworld_sis_sync_runs_skipped_total (tests).
func SkippedCounter() *prometheus.CounterVec { return runsSkipped }

// Sync implements port.SisSyncMetrics.
type Sync struct{}

var _ port.SisSyncMetrics = Sync{}

// RunSkipped counts a skipped scheduled run.
func (Sync) RunSkipped(connectionID uuid.UUID, reason string) {
	runsSkipped.WithLabelValues(connectionID.String(), reason).Inc()
}

// Webhook counts a webhook delivery outcome.
func (Sync) Webhook(result string) { webhooks.WithLabelValues(result).Inc() }

// RunDuration observes the duration of a run.
func (Sync) RunDuration(connectionID uuid.UUID, mode string, d time.Duration) {
	runDuration.WithLabelValues(connectionID.String(), mode).Observe(d.Seconds())
}
