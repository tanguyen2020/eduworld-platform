// Package gameauth holds the infrastructure of the game login that is neither Keycloak, Valkey nor SQL
// (F013): Prometheus metrics and the platform curfew source. Labels are fixed sets; no user, device or
// token value is ever a label.
package gameauth

import (
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
)

// Metrics implements port.GameAuthMetrics (plan F013 "Quan sát", MONITORING).
type Metrics struct {
	login    *prometheus.CounterVec
	duration prometheus.Histogram
	refresh  *prometheus.CounterVec
	revoked  *prometheus.CounterVec
	limited  *prometheus.CounterVec
}

var _ port.GameAuthMetrics = (*Metrics)(nil)

// NewMetrics builds the collectors and registers them on reg (nil = unregistered, tests).
func NewMetrics(reg prometheus.Registerer) (*Metrics, error) {
	m := &Metrics{
		login: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_auth_game_login_total",
			Help: "Game logins through the backend by result (F013)."}, []string{"result"}),
		duration: prometheus.NewHistogram(prometheus.HistogramOpts{Name: "eduworld_auth_game_login_duration_seconds",
			Help: "Duration of gameLogin including Keycloak (login_slo).", Buckets: []float64{.05, .1, .25, .5, 1, 2, 3, 5}}),
		refresh: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_auth_game_refresh_total",
			Help: "Game token refreshes by mode and result (ADR-0086)."}, []string{"mode", "result"}),
		revoked: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_auth_game_sessions_revoked_total",
			Help: "Game login sessions revoked by reason (ADR-0086)."}, []string{"reason"}),
		limited: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_auth_ratelimit_rejected_total",
			Help: "Logins refused by login_rate: scope global (LOGIN_QUEUE_BUSY), device or username (RATE_LIMITED)."}, []string{"scope"}),
	}
	if reg == nil {
		return m, nil
	}
	for _, c := range []prometheus.Collector{m.login, m.duration, m.refresh, m.revoked, m.limited} {
		if err := reg.Register(c); err != nil {
			var are prometheus.AlreadyRegisteredError
			if !errors.As(err, &are) {
				return nil, err
			}
		}
	}
	return m, nil
}

// Login implements port.GameAuthMetrics.
func (m *Metrics) Login(result string, d time.Duration) {
	m.login.WithLabelValues(result).Inc()
	m.duration.Observe(d.Seconds())
}

// Refresh implements port.GameAuthMetrics.
func (m *Metrics) Refresh(mode, result string) { m.refresh.WithLabelValues(mode, result).Inc() }

// Revoked implements port.GameAuthMetrics.
func (m *Metrics) Revoked(reason string) { m.revoked.WithLabelValues(reason).Inc() }

// QueueRejected implements port.GameAuthMetrics.
func (m *Metrics) QueueRejected() { m.limited.WithLabelValues("global").Inc() }

// RateLimited implements port.GameAuthMetrics.
func (m *Metrics) RateLimited(scope string) { m.limited.WithLabelValues(scope).Inc() }

// LoginCount, RefreshCount and RevokedCount expose one series (tests, dashboards).
func (m *Metrics) LoginCount(result string) prometheus.Counter {
	return m.login.WithLabelValues(result)
}

// RefreshCount returns one refresh series.
func (m *Metrics) RefreshCount(mode, result string) prometheus.Counter {
	return m.refresh.WithLabelValues(mode, result)
}

// RevokedCount returns one revocation series.
func (m *Metrics) RevokedCount(reason string) prometheus.Counter {
	return m.revoked.WithLabelValues(reason)
}

// QueueCount returns eduworld_auth_ratelimit_rejected_total{scope="global"}.
func (m *Metrics) QueueCount() prometheus.Counter { return m.limited.WithLabelValues("global") }
