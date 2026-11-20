package observability

import "github.com/prometheus/client_golang/prometheus"

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
)

// RegisterExams adds the exam engine metrics to r; already registered collectors are ignored.
func RegisterExams(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{examStarted, examAutoSubmit, examAnswerRejected, examRegraded} {
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
