package observability

import "github.com/prometheus/client_golang/prometheus"

var (
	minigameStart = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_game_minigame_start_total",
		Help: "startMiniGame calls by mini-game type and result (ok, refused, unavailable)."}, []string{"type", "result"})
	minigameCompleted = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_game_minigame_completed_total",
		Help: "Mini-game plays recorded (minigame.completed, once per play) by mini-game type."}, []string{"type"})
)

// RegisterMiniGame registers the F073 mini-game metrics (once per registry).
func RegisterMiniGame(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{minigameStart, minigameCompleted} {
		_ = r.Register(c)
	}
}

// MiniGameMetrics implements command.MiniGameMetrics.
type MiniGameMetrics struct{}

// Start counts one startMiniGame.
func (MiniGameMetrics) Start(minigameType, result string) {
	minigameStart.WithLabelValues(minigameType, result).Inc()
}

// Completed counts one recorded play.
func (MiniGameMetrics) Completed(minigameType string) {
	minigameCompleted.WithLabelValues(minigameType).Inc()
}
