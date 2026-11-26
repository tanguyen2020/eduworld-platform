// Package metrics exposes the ops-scheduler counters on the admin port (/metrics on :9090).
package metrics

import "github.com/prometheus/client_golang/prometheus"

// Results of one scale action (label result of eduworld_ops_scale_actions_total).
const (
	ResultOK       = "ok"
	ResultRejected = "rejected" // capacity check refused the scale-up (F130-AC-23)
	ResultError    = "error"    // Docker API failed or the service is missing
)

// Actions of one scale action (label action).
const (
	ActionScaleUp   = "scale_up"
	ActionScaleDown = "scale_down"
)

// Metrics holds the collectors of ops-scheduler.
type Metrics struct {
	ScaleActions *prometheus.CounterVec
	LastTick     prometheus.Gauge
}

// New registers the collectors on reg.
func New(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		ScaleActions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "eduworld_ops_scale_actions_total",
			Help: "Scheduled scale actions of ops-scheduler by Swarm service, action and result (F130-AC-22, AC-23).",
		}, []string{"service", "action", "result"}),
		LastTick: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "eduworld_ops_scheduler_last_tick_timestamp_seconds",
			Help: "Unix time of the last completed reconcile tick of ops-scheduler.",
		}),
	}
	reg.MustRegister(m.ScaleActions, m.LastTick)
	return m
}
