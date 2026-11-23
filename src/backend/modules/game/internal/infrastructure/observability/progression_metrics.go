package observability

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics of the Reward Engine and the progression (F074 T037, plan "Quan sát").
var (
	xpAwarded = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_game_xp_awarded_total", Help: "XP postings of the Reward Engine by source code and cap flag (F074).",
	}, []string{"source_code", "capped"})
	xpAmount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_game_xp_awarded_amount_total", Help: "XP posted by the Reward Engine by source code (F074).",
	}, []string{"source_code"})
	levelReached = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "eduworld_game_level_reached_total", Help: "Levels reached (one per level.reached, F074).",
	})
	engineDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "eduworld_game_reward_engine_duration_seconds", Help: "Seconds the Reward Engine spends on one event (F074).",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5},
	})
	ledgerMismatch = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_game_ledger_mismatch_total", Help: "Profiles whose cached total differs from their ledger (xp-reconcile, F074-AC-03).",
	}, []string{"ledger"})
	tenantMismatch = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_game_consumer_student_unknown_total",
		Help: "Events naming a student the projection of the event tenant does not hold (retried, then DLQ; F074-AC-11).",
	}, []string{"consumer"})
)

// RegisterProgression registers the F074 metrics.
func RegisterProgression(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{xpAwarded, xpAmount, levelReached, engineDuration, ledgerMismatch, tenantMismatch} {
		_ = r.Register(c)
	}
}

// ProgressionMetrics implements command.ProgressionMetrics.
type ProgressionMetrics struct{}

// XPAwarded counts one ledger posting.
func (ProgressionMetrics) XPAwarded(sourceCode string, capped bool, amount int) {
	xpAwarded.WithLabelValues(sourceCode, strconv.FormatBool(capped)).Inc()
	if amount > 0 {
		xpAmount.WithLabelValues(sourceCode).Add(float64(amount))
	}
}

// LevelReached counts n levels reached.
func (ProgressionMetrics) LevelReached(n int) { levelReached.Add(float64(n)) }

// TenantMismatch counts one event whose student is not in the projection of its tenant.
func (ProgressionMetrics) TenantMismatch(consumer string) {
	tenantMismatch.WithLabelValues(consumer).Inc()
}

// EngineDuration observes one Reward Engine run.
func (ProgressionMetrics) EngineDuration(seconds float64) { engineDuration.Observe(seconds) }

// LedgerMismatch counts one profile whose xp_total differs from Σ xp_ledger.
func (ProgressionMetrics) LedgerMismatch() { ledgerMismatch.WithLabelValues("xp").Inc() }

// StudentUnknown is eduworld_game_consumer_student_unknown_total{consumer} (tests).
func StudentUnknown() *prometheus.CounterVec { return tenantMismatch }

// LedgerMismatches is eduworld_game_ledger_mismatch_total{ledger} (tests).
func LedgerMismatches() *prometheus.CounterVec { return ledgerMismatch }
