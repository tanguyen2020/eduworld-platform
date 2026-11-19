// Package lifecyclemetrics holds the Prometheus metrics of the account lifecycle (F016 T034, MONITORING):
// the inactive-student cleanup and the saga engine. Labels are fixed sets plus the tenant id; no user is ever
// a label. The alerts SagaStuck (eduworld_saga_stuck > 0 for 30 min) and StudentCleanupLargeBatch
// (increase of eduworld_student_cleanup_large_batch_total) are defined in MONITORING.
package lifecyclemetrics

import (
	"errors"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/saga"
)

// Metrics implements command.CleanupMetrics and saga.Metrics.
type Metrics struct {
	cleanup    *prometheus.CounterVec
	largeBatch *prometheus.CounterVec
	sagaRuns   *prometheus.CounterVec
	sagaStuck  prometheus.Gauge
}

var (
	_ command.CleanupMetrics = (*Metrics)(nil)
	_ saga.Metrics           = (*Metrics)(nil)
)

// New builds the collectors and registers them on reg (nil = unregistered, tests).
func New(reg prometheus.Registerer) (*Metrics, error) {
	m := &Metrics{
		cleanup: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_student_cleanup_total",
			Help: "Students handled by the inactive cleanup by result (cleaned, skipped_active; ADR-0032)."}, []string{"tenant", "result"}),
		largeBatch: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_student_cleanup_large_batch_total",
			Help: "Cleanup runs held for approval above the threshold (alert StudentCleanupLargeBatch)."}, []string{"tenant"}),
		sagaRuns: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_saga_runs_total",
			Help: "Sagas of module identity finished by type and result."}, []string{"saga", "result"}),
		sagaStuck: prometheus.NewGauge(prometheus.GaugeOpts{Name: "eduworld_saga_stuck",
			Help: "Sagas still running past the stuck threshold (alert SagaStuck)."}),
	}
	if reg == nil {
		return m, nil
	}
	for _, c := range []prometheus.Collector{m.cleanup, m.largeBatch, m.sagaRuns, m.sagaStuck} {
		if err := reg.Register(c); err != nil {
			var are prometheus.AlreadyRegisteredError
			if !errors.As(err, &are) {
				return nil, err
			}
		}
	}
	return m, nil
}

// Cleanup implements command.CleanupMetrics.
func (m *Metrics) Cleanup(tenant uuid.UUID, result string, n int) {
	m.cleanup.WithLabelValues(tenant.String(), result).Add(float64(n))
}

// LargeBatch implements command.CleanupMetrics.
func (m *Metrics) LargeBatch(tenant uuid.UUID) { m.largeBatch.WithLabelValues(tenant.String()).Inc() }

// Finished implements saga.Metrics.
func (m *Metrics) Finished(sagaType, status string) {
	m.sagaRuns.WithLabelValues(sagaType, status).Inc()
}

// Stuck implements saga.Metrics.
func (m *Metrics) Stuck(n int) { m.sagaStuck.Set(float64(n)) }
