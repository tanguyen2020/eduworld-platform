package observability

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	configTransitions = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_game_config_transition_total",
		Help: "Game Operations Console actions by config type and action (create, update, submit_review, approve, reject, archive, publish, publish_scheduled, rollback, unpublish)."},
		[]string{"config_type", "action"})
	renderJobs = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_game_render_job_total",
		Help: "Render results recorded by the worker (done, failed = retry scheduled, dead = dead-lettered)."}, []string{"result"})
	renderDuration = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "eduworld_game_render_duration_seconds",
		Help: "Time from the render job to its recorded result.", Buckets: []float64{1, 5, 15, 30, 60, 120, 300, 600}})
)

// RegisterGameAdmin registers the F078 metrics (once per registry).
func RegisterGameAdmin(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{configTransitions, renderJobs, renderDuration} {
		_ = r.Register(c)
	}
}

// GameAdminMetrics implements command.GameAdminMetrics and command.RenderMetrics.
type GameAdminMetrics struct{}

// Transition counts one console action.
func (GameAdminMetrics) Transition(configType, action string) {
	configTransitions.WithLabelValues(configType, action).Inc()
}

// RenderJob counts one recorded render result.
func (GameAdminMetrics) RenderJob(result string) { renderJobs.WithLabelValues(result).Inc() }

// RenderDuration observes the render time.
func (GameAdminMetrics) RenderDuration(d time.Duration) { renderDuration.Observe(d.Seconds()) }
