package observability

import "github.com/prometheus/client_golang/prometheus"

// Export and quality metrics of F044 (plan §Quan sát). Labels never carry ids, names or scores (R14).
var (
	exportJobs = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_export_jobs_total",
		Help: "Export jobs finished by status (succeeded, failed).",
	}, []string{"module", "status"})
	exportDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:        "eduworld_export_jobs_duration_seconds",
		Help:        "Duration of one export job (read, write, upload).",
		ConstLabels: prometheus.Labels{"module": "learning"},
		Buckets:     []float64{1, 5, 15, 30, 60, 120, 300, 600, 1800},
	})
	itemAnalysed = prometheus.NewCounter(prometheus.CounterOpts{
		Name:        "eduworld_item_analysis_questions_total",
		Help:        "Question versions recomputed by the job item-analysis-nightly.",
		ConstLabels: prometheus.Labels{"module": "learning"},
	})
	calibrationProposals = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_calibration_proposals_total",
		Help: "Difficulty calibration proposals by status (pending created, accepted, dismissed).",
	}, []string{"module", "status"})
	reliabilityLow = prometheus.NewCounter(prometheus.CounterOpts{
		Name:        "eduworld_exam_reliability_low_total",
		Help:        "Official exam sessions whose reliability is below exam_reliability_warn.",
		ConstLabels: prometheus.Labels{"module": "learning"},
	})
)

// RegisterReporting adds the export and quality metrics to r; already registered collectors are ignored.
func RegisterReporting(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{exportJobs, exportDuration, itemAnalysed, calibrationProposals, reliabilityLow} {
		_ = r.Register(c)
	}
}

// ReportingCounter adapts the metrics to command.ReportMetrics and command.QualityMetrics.
type ReportingCounter struct{}

// Job observes one finished export job.
func (ReportingCounter) Job(status string, seconds float64) {
	exportJobs.WithLabelValues("learning", status).Inc()
	exportDuration.Observe(seconds)
}

// ItemAnalysed counts the versions of one run.
func (ReportingCounter) ItemAnalysed(n int) { itemAnalysed.Add(float64(n)) }

// Proposal counts one proposal event.
func (ReportingCounter) Proposal(status string) {
	calibrationProposals.WithLabelValues("learning", status).Inc()
}

// ReliabilityLow counts one low-reliability session.
func (ReportingCounter) ReliabilityLow() { reliabilityLow.Inc() }

// ExportJobs exposes the export counter to the tests.
func ExportJobs() *prometheus.CounterVec { return exportJobs }
