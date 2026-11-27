package observability

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/battle"
)

// Metrics of the battle engine (F105 plan §Quan sát): never a school, student or match label (R14, cardinality).
var (
	btOnce = sync.Once{}

	btRejected = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_battle_answer_rejected_total",
		Help: "Refused battle answers by realtime error code."}, []string{"code"})
	btScore = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "eduworld_battle_score_call_seconds",
		Help: "Latency of scorePlanAnswer (competition → learning, TC-PERF-013).", Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2}},
		[]string{"result"})
	btAbandon = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_room_abandon_total",
		Help: "Players out of a match past their grace (abandoned or forfeit), by mode."}, []string{"mode"})
	btAborted = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_matches_aborted_total",
		Help: "Matches aborted without result, by reason."}, []string{"reason"})
	btReconnect = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_reconnect_total",
		Help: "battle.state.request (join or reconnect) by result."}, []string{"result"})
	btSignals = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_integrity_signals_total",
		Help: "Integrity signals recorded by the battle engine, by type."}, []string{"type"})
	btVoided = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_matches_voided_total",
		Help: "Matches voided for strong cheating evidence (ANTI_CHEAT §9 level 2), by mode."}, []string{"mode"})
	btResults = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_match_results_total",
		Help: "RecordMatchResult calls by mode and result (recorded, duplicate_rejected — alert DuplicateMatchResults)."},
		[]string{"mode", "result"})
)

// RegisterBattle registers the battle metrics once.
func RegisterBattle(reg prometheus.Registerer) {
	btOnce.Do(func() {
		if reg == nil {
			return
		}
		for _, c := range []prometheus.Collector{btRejected, btScore, btAbandon, btAborted, btReconnect, btSignals, btResults, btVoided} {
			_ = reg.Register(c)
		}
	})
}

// Battle implements port.BattleMetrics.
type Battle struct{}

var _ port.BattleMetrics = Battle{}

// Rooms implements port.BattleMetrics.
// The gauge eduworld_rooms_active is registered with the hub metrics (F091).
func (Battle) Rooms(roomType string, delta float64) { roomsActive.WithLabelValues(roomType).Add(delta) }

// AnswerRejected implements port.BattleMetrics.
func (Battle) AnswerRejected(code string) { btRejected.WithLabelValues(code).Inc() }

// ScoreCall implements port.BattleMetrics.
func (Battle) ScoreCall(d time.Duration, result string) {
	btScore.WithLabelValues(result).Observe(d.Seconds())
}

// Abandon implements port.BattleMetrics.
func (Battle) Abandon(mode string) { btAbandon.WithLabelValues(mode).Inc() }

// Aborted implements port.BattleMetrics.
func (Battle) Aborted(reason string) { btAborted.WithLabelValues(reason).Inc() }

// Reconnect implements port.BattleMetrics.
func (Battle) Reconnect(result string) { btReconnect.WithLabelValues(result).Inc() }

// Signal implements port.BattleMetrics.
func (Battle) Signal(kind string) { btSignals.WithLabelValues(kind).Inc() }

// MatchResult implements port.BattleMetrics.
func (Battle) MatchResult(mode, result string) { btResults.WithLabelValues(mode, result).Inc() }

// MatchResults exposes eduworld_match_results_total (tests).
func MatchResults() *prometheus.CounterVec { return btResults }

// VoidModes are the match modes of eduworld_matches_voided_total, exposed at 0 from start so an alert or a dashboard
// reads 0 instead of no data before the first void.
var VoidModes = []string{battle.Mode1v1, battle.ModeTeam, battle.ModeCoop, battle.ModeTimedQuiz, battle.ModeTournament}

func init() {
	for _, m := range VoidModes {
		btVoided.WithLabelValues(m)
	}
}

// MatchVoided implements port.BattleMetrics.
func (Battle) MatchVoided(mode string) { btVoided.WithLabelValues(mode).Inc() }

// MatchesVoided exposes eduworld_matches_voided_total (tests).
func MatchesVoided() *prometheus.CounterVec { return btVoided }
