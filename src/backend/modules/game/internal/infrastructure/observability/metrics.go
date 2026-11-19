// Package observability holds the metrics of module game (plan F014 "Quan sát", MONITORING): game session starts,
// takeovers, timeouts and ends, projection lag and consumer rejections. Names carry the module (game) because the
// binaries register every module in one registry.
package observability

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
)

var (
	sessionStart = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_game_session_start_total",
		Help: "createGameSession outcomes (created, takeover, conflict, denied, curfew, client_version_unsupported, maintenance, not_projected, error).",
	}, []string{"result"})
	sessionStartDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "eduworld_game_session_start_duration_seconds",
		Help:    "Duration of createGameSession (login_slo).",
		Buckets: []float64{.01, .025, .05, .1, .25, .5, 1, 2.5, 5},
	})
	sessionTakeover = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "eduworld_game_session_takeover_total",
		Help: "Game sessions replaced by a confirmed takeover on another device (ADR-0031).",
	})
	sessionTimeout = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "eduworld_game_session_timeout_total",
		Help: "Game sessions ended by the job session-timeout (no heartbeat).",
	})
	sessionEnded = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_game_session_ended_total",
		Help: "Game sessions ended by end reason.",
	}, []string{"reason"})
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

// SessionEnds is eduworld_game_session_ended_total{reason} (tests).
func SessionEnds() *prometheus.CounterVec { return sessionEnded }

// Register adds the metrics to r (composition root); already registered collectors are ignored.
func Register(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{sessionStart, sessionStartDuration, sessionTakeover, sessionTimeout, sessionEnded,
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
	if result == port.StartTakeover {
		sessionTakeover.Inc()
	}
}

// Ended records one ended session.
func (SessionMetrics) Ended(reason string) {
	sessionEnded.WithLabelValues(reason).Inc()
	if reason == "timeout" {
		sessionTimeout.Inc()
	}
}
