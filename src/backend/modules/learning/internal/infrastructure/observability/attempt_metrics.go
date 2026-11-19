package observability

import "github.com/prometheus/client_golang/prometheus"

// Attempt engine metrics (plan F030 "Quan sát"). Labels never carry ids, answers or names (R14).
var (
	attemptStarted = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_attempt_started_total",
		Help: "Attempts started by activity type and mode.",
	}, []string{"activity_type", "mode"})
	attemptSubmitted = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_attempt_submitted_total",
		Help: "Attempts submitted and graded by submit reason (manual, timeout).",
	}, []string{"result"})
	attemptExpired = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_attempt_expired_total",
		Help: "Attempts closed by the attempt-lifecycle job (deadline, abandoned).",
	}, []string{"kind"})
)

// RegisterAttempts adds the attempt metrics to r; already registered collectors are ignored.
func RegisterAttempts(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{attemptStarted, attemptSubmitted, attemptExpired} {
		_ = r.Register(c)
	}
}

// AttemptCounter adapts the attempt metrics to command.AttemptMetrics.
type AttemptCounter struct{}

// Started counts one start.
func (AttemptCounter) Started(activityType, mode string) {
	attemptStarted.WithLabelValues(activityType, mode).Inc()
}

// Submitted counts one graded submit.
func (AttemptCounter) Submitted(result string) { attemptSubmitted.WithLabelValues(result).Inc() }

// Expired counts one attempt closed by the job.
func (AttemptCounter) Expired(kind string) { attemptExpired.WithLabelValues(kind).Inc() }
