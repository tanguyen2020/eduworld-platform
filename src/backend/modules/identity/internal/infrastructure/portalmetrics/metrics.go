// Package portalmetrics implements port.PortalAuthMetrics with Prometheus (plan F012 "Quan sát"):
// eduworld_portal_login_total{result} and eduworld_portal_csrf_rejected_total{operation}. Labels are fixed
// sets; no user, device or token value is ever a label.
package portalmetrics

import (
	"errors"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
)

// Metrics holds the counters.
type Metrics struct {
	login *prometheus.CounterVec
	csrf  *prometheus.CounterVec
}

var _ port.PortalAuthMetrics = (*Metrics)(nil)

// New builds the counters and registers them on reg (nil = unregistered, tests).
func New(reg prometheus.Registerer) (*Metrics, error) {
	m := &Metrics{
		login: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_portal_login_total",
			Help: "Admin Portal logins through the backend by result (ADR-0091)."}, []string{"result"}),
		csrf: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_portal_csrf_rejected_total",
			Help: "Portal refresh/logout requests rejected by the CSRF double-submit check."}, []string{"operation"}),
	}
	if reg == nil {
		return m, nil
	}
	for _, c := range []*prometheus.CounterVec{m.login, m.csrf} {
		if err := reg.Register(c); err != nil {
			var are prometheus.AlreadyRegisteredError
			if !errors.As(err, &are) {
				return nil, err
			}
			if c == m.login {
				m.login = are.ExistingCollector.(*prometheus.CounterVec)
			} else {
				m.csrf = are.ExistingCollector.(*prometheus.CounterVec)
			}
		}
	}
	return m, nil
}

// Login implements port.PortalAuthMetrics.
func (m *Metrics) Login(result string) { m.login.WithLabelValues(result).Inc() }

// CSRFRejected implements port.PortalAuthMetrics.
func (m *Metrics) CSRFRejected(operation string) { m.csrf.WithLabelValues(operation).Inc() }

// LoginCount returns the counter of one result (tests, dashboards).
func (m *Metrics) LoginCount(result string) prometheus.Counter {
	return m.login.WithLabelValues(result)
}

// CSRFCount returns the counter of one operation.
func (m *Metrics) CSRFCount(operation string) prometheus.Counter {
	return m.csrf.WithLabelValues(operation)
}
