package observability

import "github.com/prometheus/client_golang/prometheus"

// Efficacy study and experiment metrics of F035 (plan §Quan sát). Labels never carry ids, names, arms of a student nor
// scores (R14).
var (
	efficacyWaveSessions = prometheus.NewCounter(prometheus.CounterOpts{
		Name:        "eduworld_efficacy_wave_sessions_created_total",
		Help:        "Exam sessions created for efficacy waves.",
		ConstLabels: prometheus.Labels{"module": "learning"},
	})
	efficacyParticipants = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_efficacy_participants",
		Help: "Efficacy participants recorded by status change (active snapshot, withdrawn, excluded).",
	}, []string{"module", "status"})
	efficacyReportDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:        "eduworld_efficacy_report_duration_seconds",
		Help:        "Duration of one efficacy report computation.",
		ConstLabels: prometheus.Labels{"module": "learning"},
		Buckets:     []float64{0.5, 1, 5, 15, 30, 60, 120, 300},
	})
	experimentAssignments = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_experiment_assignments_total",
		Help: "Experiment assignments by kind of experiment.",
	}, []string{"module", "kind"})
	experimentGuardrail = prometheus.NewCounter(prometheus.CounterOpts{
		Name:        "eduworld_experiment_guardrail_breaches_total",
		Help:        "Experiments stopped by the guardrail.",
		ConstLabels: prometheus.Labels{"module": "learning"},
	})
	experimentShadow = prometheus.NewCounter(prometheus.CounterOpts{
		Name:        "eduworld_experiment_shadow_updates_total",
		Help:        "Mastery states of experiment arms updated from evidence.",
		ConstLabels: prometheus.Labels{"module": "learning"},
	})
)

// RegisterEfficacy adds the efficacy and experiment metrics to r; already registered collectors are ignored.
func RegisterEfficacy(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{efficacyWaveSessions, efficacyParticipants, efficacyReportDuration, experimentAssignments,
		experimentGuardrail, experimentShadow} {
		_ = r.Register(c)
	}
}

// EfficacyCounter adapts the metrics to command.EfficacyMetrics and command.ExperimentMetrics.
type EfficacyCounter struct{}

// WaveSessions counts the sessions of one scheduled wave.
func (EfficacyCounter) WaveSessions(n int) { efficacyWaveSessions.Add(float64(n)) }

// Participants counts participants entering a status.
func (EfficacyCounter) Participants(status string, n int) {
	efficacyParticipants.WithLabelValues("learning", status).Add(float64(n))
}

// ReportDuration observes one report computation.
func (EfficacyCounter) ReportDuration(seconds float64) { efficacyReportDuration.Observe(seconds) }

// Assigned counts one experiment assignment.
func (EfficacyCounter) Assigned(kind string) {
	experimentAssignments.WithLabelValues("learning", kind).Inc()
}

// GuardrailBreach counts one experiment stopped by the guardrail.
func (EfficacyCounter) GuardrailBreach() { experimentGuardrail.Inc() }

// ShadowUpdates counts arm mastery states updated.
func (EfficacyCounter) ShadowUpdates(n int) { experimentShadow.Add(float64(n)) }
