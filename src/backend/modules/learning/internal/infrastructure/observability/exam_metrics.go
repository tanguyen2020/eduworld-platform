package observability

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Exam engine metrics (plan F042 "Quan sát"). Labels never carry ids, answers or names (R14).
var (
	examStarted = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "eduworld_exam_attempts_started_total",
		Help: "Exam attempts started.",
	})
	examAutoSubmit = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_exam_autosubmit_total",
		Help: "Exam attempts submitted by the server (auto_timeout, window_closed).",
	}, []string{"reason"})
	examAnswerRejected = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_exam_answer_rejected_total",
		Help: "Exam answers refused by problem code (EXAM_TIME_OVER, EXAM_ALREADY_SUBMITTED).",
	}, []string{"code"})
	examRegraded = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_exam_regrade_attempts_total",
		Help: "Exam attempts regraded by action (accept_all, use_new_key, drop_item).",
	}, []string{"action"})
	examSubmissions = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_exam_submissions_total",
		Help: "Exam submissions by exam type and result (ok, late, duplicate, rejected, auto_submitted).",
	}, []string{"exam_type", "result"})
	examSubmitDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "eduworld_exam_submit_duration_seconds",
		Help:    "Server duration of an accepted student exam submit, by exam type (SLO latency submit: bucket 0.4 s).",
		Buckets: []float64{.025, .05, .1, .2, .3, .4, .6, 1, 2.5, 5},
	}, []string{"exam_type"})
)

// RegisterExams adds the exam engine metrics to r; already registered collectors are ignored.
func RegisterExams(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{examStarted, examAutoSubmit, examAnswerRejected, examRegraded, examSubmissions, examSubmitDuration} {
		_ = r.Register(c)
	}
}

// ExamCounter adapts the exam metrics to command.ExamMetrics.
type ExamCounter struct{}

// Started counts one exam attempt started.
func (ExamCounter) Started() { examStarted.Inc() }

// AutoSubmitted counts one attempt submitted by the server.
func (ExamCounter) AutoSubmitted(reason string) { examAutoSubmit.WithLabelValues(reason).Inc() }

// AnswerRejected counts one refused answer.
func (ExamCounter) AnswerRejected(code string) { examAnswerRejected.WithLabelValues(code).Inc() }

// Regraded counts one regraded attempt.
func (ExamCounter) Regraded(action string) { examRegraded.WithLabelValues(action).Inc() }

// Submitted counts one exam submission.
func (ExamCounter) Submitted(examType, result string) {
	examSubmissions.WithLabelValues(examType, result).Inc()
}

// SubmitDuration observes one accepted student submit.
func (ExamCounter) SubmitDuration(examType string, d time.Duration) {
	examSubmitDuration.WithLabelValues(examType).Observe(d.Seconds())
}

// ExamSubmissions exposes eduworld_exam_submissions_total (tests).
func ExamSubmissions() *prometheus.CounterVec { return examSubmissions }

// ExamSubmitDuration exposes eduworld_exam_submit_duration_seconds (tests).
func ExamSubmitDuration() *prometheus.HistogramVec { return examSubmitDuration }
