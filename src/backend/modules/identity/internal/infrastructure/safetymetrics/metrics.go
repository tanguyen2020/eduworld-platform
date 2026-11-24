// Package safetymetrics holds the Prometheus metrics of the enforcement of the Safety Team in module identity (F124,
// MONITORING §3): account locks by source and the staff emails by template and status. Fixed label sets only: never a
// user, an address or a case (R14). Alerts EmailDeliveryFailing and SchoolSafetyNotificationNoRecipient read them.
package safetymetrics

import (
	"errors"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
)

// Metrics implements command.LockMetrics and command.MailMetrics.
type Metrics struct {
	locks       *prometheus.CounterVec
	deliveries  *prometheus.CounterVec
	noRecipient *prometheus.CounterVec
}

var (
	_ command.LockMetrics = (*Metrics)(nil)
	_ command.MailMetrics = (*Metrics)(nil)
)

// New builds the collectors and registers them on reg (nil = unregistered, tests); an already registered collector is
// reused.
func New(reg prometheus.Registerer) (*Metrics, error) {
	m := &Metrics{
		locks: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_account_locks_total",
			Help: "Account locks applied or lifted by source (moderation_sanction: sanction account_lock of the Safety Team)."},
			[]string{"source", "action"}),
		deliveries: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_email_deliveries_total",
			Help: "Staff emails by template and status (sent, failed, skipped, retry); alert EmailDeliveryFailing on failed."},
			[]string{"template", "status"}),
		noRecipient: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_school_safety_notification_no_recipient_total",
			Help: "School safety notifications without any School Admin or Tenant Admin to email (alert SchoolSafetyNotificationNoRecipient)."},
			[]string{"template"}),
	}
	if reg == nil {
		return m, nil
	}
	for _, c := range []**prometheus.CounterVec{&m.locks, &m.deliveries, &m.noRecipient} {
		if err := reg.Register(*c); err != nil {
			var are prometheus.AlreadyRegisteredError
			if !errors.As(err, &are) {
				return nil, err
			}
			existing, ok := are.ExistingCollector.(*prometheus.CounterVec)
			if !ok {
				return nil, err
			}
			*c = existing
		}
	}
	return m, nil
}

// Lock implements command.LockMetrics.
func (m *Metrics) Lock(source, action string) { m.locks.WithLabelValues(source, action).Inc() }

// Delivery implements command.MailMetrics.
func (m *Metrics) Delivery(template, status string) {
	m.deliveries.WithLabelValues(template, status).Inc()
}

// NoRecipient implements command.MailMetrics.
func (m *Metrics) NoRecipient(template string) { m.noRecipient.WithLabelValues(template).Inc() }

// NoRecipients returns the no-recipient counter (tests).
func (m *Metrics) NoRecipients() *prometheus.CounterVec { return m.noRecipient }

// Deliveries returns the delivery counter (tests).
func (m *Metrics) Deliveries() *prometheus.CounterVec { return m.deliveries }
