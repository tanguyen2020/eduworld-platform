package main

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/render"
)

// Metrics of the container, served on the admin port (/metrics). The worker of module game owns the job-level metrics
// (retries, DLQ: eduworld_game_render_*); these count what this container rendered.
type Metrics struct {
	jobs     *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// NewMetrics registers the metrics on reg (nil = not exported, tests).
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		jobs: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_game_renderer_jobs_total",
			Help: "Render jobs handled by the game-renderer container, by kind and status."}, []string{"kind", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "eduworld_game_renderer_job_duration_seconds",
			Help:    "Duration of one render job (download, Godot, upload), by kind.",
			Buckets: []float64{1, 2, 5, 10, 20, 30, 60, 120, 300}}, []string{"kind"}),
	}
	if reg != nil {
		reg.MustRegister(m.jobs, m.duration)
	}
	return m
}

func (m *Metrics) observe(kind, status string, d time.Duration) {
	if m == nil {
		return
	}
	if kind != render.KindThumbnail && kind != render.KindTurntable {
		kind = "invalid"
	}
	m.jobs.WithLabelValues(kind, status).Inc()
	m.duration.WithLabelValues(kind).Observe(d.Seconds())
}
