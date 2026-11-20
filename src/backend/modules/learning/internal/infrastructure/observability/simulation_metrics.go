package observability

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Simulation metrics (plan F066 "Quan sát"). Labels carry the template code and status only, never ids or logs (R14).
var (
	simulationRuns = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_simulation_runs_total",
		Help: "Simulation runs by template and verification status (verified, rejected, not_verifiable).",
	}, []string{"template_code", "status"})
	simulationChecker = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "eduworld_simulation_checker_duration_seconds",
		Help:    "Duration of the server replay of a simulation run by its checker.",
		Buckets: []float64{0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
	}, []string{"template_code"})
)

// RegisterSimulation adds the simulation metrics to r; already registered collectors are ignored.
func RegisterSimulation(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{simulationRuns, simulationChecker} {
		_ = r.Register(c)
	}
}

// SimulationCounter adapts the simulation metrics to command.SimulationMetrics.
type SimulationCounter struct{}

// Run counts one stored run.
func (SimulationCounter) Run(templateCode, status string) {
	simulationRuns.WithLabelValues(templateCode, status).Inc()
}

// CheckerDuration observes one checker replay.
func (SimulationCounter) CheckerDuration(templateCode string, d time.Duration) {
	simulationChecker.WithLabelValues(templateCode).Observe(d.Seconds())
}
