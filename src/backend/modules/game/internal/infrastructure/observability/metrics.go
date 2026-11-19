// Package observability holds the metrics of module game (MONITORING §3, plan F014 "Quan sát"): game logins,
// active sessions, takeovers, ends and access denials (names of MONITORING), the createGameSession outcome and
// duration (login_slo), projection lag and consumer rejections. The projection and consumer metrics carry the
// module in their name because every binary registers all modules in one registry and module learning already
// registers eduworld_projection_lag_seconds{table} (OQ F014-6).
package observability

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
)

var (
	logins = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_game_logins_total",
		Help: "Entries into the game (game sessions created, ADR-0031).",
	}, []string{"tenant", "platform"})
	active = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "eduworld_game_sessions_active",
		Help: "Active game sessions per platform (refreshed by the job session-timeout).",
	}, []string{"platform"})
	takeovers = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_game_session_takeovers_total",
		Help: "Game sessions replaced by a confirmed takeover on another device, by the new platform (ADR-0031).",
	}, []string{"platform"})
	ended = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_game_session_ended_total",
		Help: "Game sessions ended by end reason.",
	}, []string{"end_reason"})
	denied = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_game_access_denied_total",
		Help: "Refused game entries of accounts that are not active students.",
	}, []string{"reason"})
	sessionStart = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_game_session_start_total",
		Help: "createGameSession outcomes (created, takeover, conflict, denied, curfew, client_version_unsupported, maintenance, not_projected, error).",
	}, []string{"result"})
	sessionStartDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "eduworld_game_session_start_duration_seconds",
		Help:    "Duration of createGameSession (login_slo).",
		Buckets: []float64{.01, .025, .05, .1, .25, .5, 1, 2.5, 5},
	})
	projectionLag = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "eduworld_game_projection_lag_seconds",
		Help:    "Delay between an identity event and its projection into ref.* of eduworld_game.",
		Buckets: []float64{.05, .1, .25, .5, 1, 2, 5, 10, 30, 60, 300},
	}, []string{"table"})
	consumerRejected = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_game_consumer_rejected_total",
		Help: "Events refused by a consumer of module game (sent to DLQ_GAME) by reason.",
	}, []string{"consumer", "reason"})
)

// ProjectionLag is eduworld_game_projection_lag_seconds{table}.
func ProjectionLag() *prometheus.HistogramVec { return projectionLag }

// ConsumerRejected is eduworld_game_consumer_rejected_total{consumer,reason}.
func ConsumerRejected() *prometheus.CounterVec { return consumerRejected }

// SessionStarts is eduworld_game_session_start_total{result} (tests).
func SessionStarts() *prometheus.CounterVec { return sessionStart }

// SessionEnds is eduworld_game_session_ended_total{end_reason} (tests).
func SessionEnds() *prometheus.CounterVec { return ended }

// ActiveSessions is eduworld_game_sessions_active{platform} (tests).
func ActiveSessions() *prometheus.GaugeVec { return active }

// Register adds the metrics to r (composition root); already registered collectors are ignored.
func Register(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{logins, active, takeovers, ended, denied, sessionStart, sessionStartDuration,
		projectionLag, consumerRejected} {
		_ = r.Register(c)
	}
}

// SessionMetrics implements port.SessionMetrics.
type SessionMetrics struct{}

var _ port.SessionMetrics = SessionMetrics{}

// Started records one createGameSession outcome.
func (SessionMetrics) Started(result string, d time.Duration) {
	sessionStart.WithLabelValues(result).Inc()
	sessionStartDuration.Observe(d.Seconds())
}

// Login records one entry into the game.
func (SessionMetrics) Login(tenant, platform string) { logins.WithLabelValues(tenant, platform).Inc() }

// Takeover records one takeover.
func (SessionMetrics) Takeover(platform string) { takeovers.WithLabelValues(platform).Inc() }

// Ended records one ended session.
func (SessionMetrics) Ended(reason string) { ended.WithLabelValues(reason).Inc() }

// Denied records one refused entry.
func (SessionMetrics) Denied(reason string) { denied.WithLabelValues(reason).Inc() }

var activeMu sync.Mutex

// Active sets the active sessions per platform; platforms without sessions go to 0.
func (SessionMetrics) Active(byPlatform map[string]int) {
	activeMu.Lock()
	defer activeMu.Unlock()
	for _, p := range []string{"windows", "android", "ios", "web", "macos", "linux"} {
		active.WithLabelValues(p).Set(float64(byPlatform[p]))
	}
}
