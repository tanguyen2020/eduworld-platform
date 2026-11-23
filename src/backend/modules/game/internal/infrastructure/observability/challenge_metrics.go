package observability

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
)

// Metrics of the Challenge Engine (F076 T038, plan "Quan sát"); the room host metrics are pkg/roomhost.
var (
	challengeStarted = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_game_challenge_started_total", Help: "Challenge attempts started by type and difficulty (F076).",
	}, []string{"type", "difficulty"})
	challengeCompleted = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_game_challenge_completed_total", Help: "Challenge attempts ended by status (succeeded, failed, abandoned).",
	}, []string{"status"})
)

// RegisterChallenge registers the F076 metrics.
func RegisterChallenge(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{challengeStarted, challengeCompleted} {
		_ = r.Register(c)
	}
}

// ChallengeMetrics implements command.ChallengeMetrics.
type ChallengeMetrics struct{}

var _ command.ChallengeMetrics = ChallengeMetrics{}

// Started counts one attempt started.
func (ChallengeMetrics) Started(challengeType, difficulty string) {
	challengeStarted.WithLabelValues(challengeType, difficulty).Inc()
}

// Completed counts one attempt ended.
func (ChallengeMetrics) Completed(status string) { challengeCompleted.WithLabelValues(status).Inc() }
