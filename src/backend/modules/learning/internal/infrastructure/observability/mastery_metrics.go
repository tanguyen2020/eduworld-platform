package observability

import "github.com/prometheus/client_golang/prometheus"

// Mastery and learning record metrics (plan F031 "Quan sát"). Labels never carry ids or answers (R14).
var (
	masteryUpdates = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_mastery_updates_total",
		Help: "Mastery recomputations of a (student, objective) by result (updated, unchanged, conflict).",
	}, []string{"result"})
	masteryLag = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "eduworld_mastery_update_lag_seconds",
		Help:    "Seconds between activity.graded and the mastery update (SC-002: p95 ≤ 5 s).",
		Buckets: []float64{0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60, 300},
	})
	partitionMonthsAhead = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "eduworld_learning_events_partition_months_ahead",
		Help: "Future monthly partitions of the partitioned learning tables (alert LearningPartitionsMissing below 2).",
	}, []string{"table"})
)

// RegisterMastery adds the mastery metrics to r; already registered collectors are ignored.
func RegisterMastery(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{masteryUpdates, masteryLag, partitionMonthsAhead} {
		_ = r.Register(c)
	}
}

// MasteryCounter adapts the mastery metrics to command.MasteryMetrics and command.PartitionMetrics.
type MasteryCounter struct{}

// Updated counts one recomputation.
func (MasteryCounter) Updated(result string) { masteryUpdates.WithLabelValues(result).Inc() }

// Lag observes the delay of an update.
func (MasteryCounter) Lag(seconds float64) { masteryLag.Observe(max(seconds, 0)) }

// MonthsAhead sets the future partitions of a table.
func (MasteryCounter) MonthsAhead(table string, months int) {
	partitionMonthsAhead.WithLabelValues(table).Set(float64(months))
}

// PartitionMonthsAhead exposes the gauge to the tests.
func PartitionMonthsAhead() *prometheus.GaugeVec { return partitionMonthsAhead }

var regradeAttempts = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "eduworld_regrade_attempts_total",
	Help: "Attempts re-evaluated by the job regrade-runner by action (accept_all, use_new_key, drop_item).",
}, []string{"action"})

// RegisterRegrade adds the re-evaluation metric to r; already registered collectors are ignored.
func RegisterRegrade(r prometheus.Registerer) { _ = r.Register(regradeAttempts) }

// RegradeCounter adapts the re-evaluation metric to command.RegradeMetrics.
type RegradeCounter struct{}

// ReEvaluated counts one re-evaluated attempt.
func (RegradeCounter) ReEvaluated(action string) { regradeAttempts.WithLabelValues(action).Inc() }

var lessonCompletions = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "eduworld_lesson_completions_total",
	Help: "Lesson sessions completed by trigger (activity_graded, complete_lesson).",
}, []string{"trigger"})

// RegisterLessons adds the lesson metric to r; already registered collectors are ignored.
func RegisterLessons(r prometheus.Registerer) { _ = r.Register(lessonCompletions) }

// LessonCounter adapts the lesson metric to command.LessonMetrics.
type LessonCounter struct{}

// LessonCompleted counts one completed lesson session.
func (LessonCounter) LessonCompleted(trigger string) {
	lessonCompletions.WithLabelValues(trigger).Inc()
}
