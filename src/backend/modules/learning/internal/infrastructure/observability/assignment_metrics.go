package observability

import "github.com/prometheus/client_golang/prometheus"

// School assignment metrics (plan F041 "Quan sát"). Labels never carry ids or names (R14).
var (
	lmsAssignments = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_lms_assignments_total",
		Help: "School assignments entering a status (draft, scheduled, open, closed, results_released, archived, cancelled).",
	}, []string{"status"})
	lmsSubmissions = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_lms_submissions_updated_total",
		Help: "Assignment submissions updated by status.",
	}, []string{"status"})
	lmsReminders = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_lms_assignment_reminders_total",
		Help: "Assignment reminders by result (sent, limited).",
	}, []string{"result"})
)

// RegisterAssignments adds the school assignment metrics to r; already registered collectors are ignored.
func RegisterAssignments(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{lmsAssignments, lmsSubmissions, lmsReminders} {
		_ = r.Register(c)
	}
}

// SchoolAssignmentCounter adapts the school assignment metrics to command.SchoolAssignmentMetrics.
type SchoolAssignmentCounter struct{}

// AssignmentStatus counts one assignment entering status.
func (SchoolAssignmentCounter) AssignmentStatus(status string) {
	lmsAssignments.WithLabelValues(status).Inc()
}

// SubmissionUpdated counts one submission update.
func (SchoolAssignmentCounter) SubmissionUpdated(status string) {
	lmsSubmissions.WithLabelValues(status).Inc()
}

// Reminder counts one reminder request.
func (SchoolAssignmentCounter) Reminder(result string) { lmsReminders.WithLabelValues(result).Inc() }
