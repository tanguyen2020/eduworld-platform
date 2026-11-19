package observability

import "github.com/prometheus/client_golang/prometheus"

// Review workflow metrics (plan F025 "Quan sát").
var (
	contentTransitions = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_content_transitions_total",
		Help: "Workflow actions on learning content versions by content type, action and result (success, rejected, error).",
	}, []string{"content_type", "action", "result"})
	reviewTasksOpen = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "eduworld_review_tasks_open",
		Help: "Open review tasks by lane (set by the worker from content.review_tasks).",
	}, []string{"lane"})
)

// RegisterWorkflow adds the workflow metrics to r; already registered collectors are ignored.
func RegisterWorkflow(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{contentTransitions, reviewTasksOpen} {
		_ = r.Register(c)
	}
}

// ReviewTasksOpen is eduworld_review_tasks_open{lane}.
func ReviewTasksOpen() *prometheus.GaugeVec { return reviewTasksOpen }

// WorkflowCounter adapts eduworld_content_transitions_total to command.WorkflowMetrics.
type WorkflowCounter struct{}

// Transition counts one workflow action.
func (WorkflowCounter) Transition(contentType, action, result string) {
	contentTransitions.WithLabelValues(contentType, action, result).Inc()
}
