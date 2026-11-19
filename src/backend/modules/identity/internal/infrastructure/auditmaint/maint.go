// Package auditmaint implements write.AuditMaintenance and port.AuditMetrics (F015 US1): it calls the
// SECURITY DEFINER functions of the F015 migration with the pool of identity_partition_admin, which has no
// privilege on the tables themselves, and exposes the Prometheus metrics of the audit sink.
package auditmaint

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
)

// Maint implements write.AuditMaintenance.
type Maint struct {
	DB *sql.DB // identity_partition_admin
}

var _ write.AuditMaintenance = Maint{}

// EnsurePartitions implements write.AuditMaintenance.
func (m Maint) EnsurePartitions(ctx context.Context, from time.Time, months int) (int, error) {
	var n int
	if err := m.DB.QueryRowContext(ctx, `SELECT audit.ensure_monthly_partitions($1::date, $2)`, from.UTC().Format("2006-01-02"), months).Scan(&n); err != nil {
		return 0, fmt.Errorf("ensure audit partitions: %w", err)
	}
	return n, nil
}

// MonthsAhead implements write.AuditMaintenance.
func (m Maint) MonthsAhead(ctx context.Context, table string, from time.Time) (int, error) {
	var n int
	if err := m.DB.QueryRowContext(ctx, `SELECT audit.monthly_partitions_ahead($1, $2::date)`, table, from.UTC().Format("2006-01-02")).Scan(&n); err != nil {
		return 0, fmt.Errorf("partitions ahead: %w", err)
	}
	return n, nil
}

// Anonymize implements write.AuditMaintenance.
func (m Maint) Anonymize(ctx context.Context, before time.Time) (int64, error) {
	var n int64
	if err := m.DB.QueryRowContext(ctx, `SELECT audit.anonymize_expired($1)`, before).Scan(&n); err != nil {
		return 0, fmt.Errorf("anonymize audit: %w", err)
	}
	return n, nil
}

// Metrics implements port.AuditMetrics.
type Metrics struct {
	records   *prometheus.CounterVec
	sanitized prometheus.Counter
	missing   *prometheus.CounterVec
}

var _ port.AuditMetrics = (*Metrics)(nil)

// NewMetrics builds the collectors and registers them on reg (nil = unregistered, tests).
func NewMetrics(reg prometheus.Registerer) (*Metrics, error) {
	m := &Metrics{
		records: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_audit_records_total",
			Help: "audit.recorded events handled by the audit sink by producing module and result."}, []string{"service", "result"}),
		sanitized: prometheus.NewCounter(prometheus.CounterOpts{Name: "eduworld_audit_sanitized_fields_total",
			Help: "Audit metadata fields dropped or truncated by the sink (forbidden key, size)."}),
		missing: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_partition_missing_total",
			Help: "Monthly partitions missing ahead of time after a partition-maintainer run."}, []string{"table"}),
	}
	if reg == nil {
		return m, nil
	}
	for _, c := range []prometheus.Collector{m.records, m.sanitized, m.missing} {
		if err := reg.Register(c); err != nil {
			var are prometheus.AlreadyRegisteredError
			if !errors.As(err, &are) {
				return nil, err
			}
		}
	}
	return m, nil
}

// Recorded implements port.AuditMetrics.
func (m *Metrics) Recorded(service, result string) { m.records.WithLabelValues(service, result).Inc() }

// Sanitized implements port.AuditMetrics.
func (m *Metrics) Sanitized(n int) { m.sanitized.Add(float64(n)) }

// PartitionMissing implements port.AuditMetrics.
func (m *Metrics) PartitionMissing(table string) { m.missing.WithLabelValues(table).Inc() }

// RecordedCount returns one eduworld_audit_records_total series (tests, dashboards).
func (m *Metrics) RecordedCount(service, result string) prometheus.Counter {
	return m.records.WithLabelValues(service, result)
}

// SanitizedCount returns eduworld_audit_sanitized_fields_total.
func (m *Metrics) SanitizedCount() prometheus.Counter { return m.sanitized }

// MissingCount returns one eduworld_partition_missing_total series.
func (m *Metrics) MissingCount(table string) prometheus.Counter {
	return m.missing.WithLabelValues(table)
}
