package observability

import "github.com/prometheus/client_golang/prometheus"

// Attempt engine metrics (plan F030 "Quan sát"). Labels never carry ids, answers or names (R14).
var (
	attemptStarted = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_attempt_started_total",
		Help: "Attempts started by activity type and mode.",
	}, []string{"activity_type", "mode"})
	attemptSubmitted = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_attempts_submitted_total",
		Help: "Student submits of activity attempts by mode and result (ok, duplicate, late, rejected).",
	}, []string{"mode", "result"})
	activitiesCompleted = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_learning_activities_completed_total",
		Help: "Activities completed (graded) by a student submit, by activity type, context, subject code and grade band.",
	}, []string{"activity_type", "context", "subject", "grade_band"})
	attemptExpired = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_attempt_expired_total",
		Help: "Attempts closed by the attempt-lifecycle job (deadline, abandoned).",
	}, []string{"kind"})
	answerRateLimited = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_answer_rate_limited_total",
		Help: "Answers and submits refused with 429 by answer_rate, by kind (answer, submit).",
	}, []string{"kind"})
	answerRateFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_answer_rate_counter_failures_total",
		Help: "answer_rate counter calls that failed on Valkey and let the call through, by kind.",
	}, []string{"kind"})
)

// RegisterAttempts adds the attempt metrics to r; already registered collectors are ignored.
func RegisterAttempts(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{attemptStarted, attemptSubmitted, activitiesCompleted, attemptExpired, answerRateLimited, answerRateFailures} {
		_ = r.Register(c)
	}
}

// AttemptCounter adapts the attempt metrics to command.AttemptMetrics.
type AttemptCounter struct{}

// Started counts one start.
func (AttemptCounter) Started(activityType, mode string) {
	attemptStarted.WithLabelValues(activityType, mode).Inc()
}

// Submitted counts one student submit.
func (AttemptCounter) Submitted(mode, result string) {
	attemptSubmitted.WithLabelValues(mode, result).Inc()
}

// Completed counts one completed activity.
func (AttemptCounter) Completed(activityType, context, subject, gradeBand string) {
	activitiesCompleted.WithLabelValues(activityType, context, subject, gradeBand).Inc()
}

// AttemptsSubmitted exposes eduworld_attempts_submitted_total (tests).
func AttemptsSubmitted() *prometheus.CounterVec { return attemptSubmitted }

// ActivitiesCompleted exposes eduworld_learning_activities_completed_total (tests).
func ActivitiesCompleted() *prometheus.CounterVec { return activitiesCompleted }

// Expired counts one attempt closed by the job.
func (AttemptCounter) Expired(kind string) { attemptExpired.WithLabelValues(kind).Inc() }

// RateCounter adapts the answer_rate metrics to cache.RateMetrics.
type RateCounter struct{}

// RateLimited counts one call refused with 429.
func (RateCounter) RateLimited(kind string) { answerRateLimited.WithLabelValues(kind).Inc() }

// RateCounterFailed counts one counter call that failed (the call was let through).
func (RateCounter) RateCounterFailed(kind string) { answerRateFailures.WithLabelValues(kind).Inc() }
