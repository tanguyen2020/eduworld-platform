package observability

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
)

// Metrics of live-ops (F077 T038, plan "Quan sát").
var (
	runTransitions = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_game_event_run_transition_total", Help: "Event run transitions by status reached (F077).",
	}, []string{"to"})
	schedulerLag = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "eduworld_game_event_scheduler_lag_seconds", Help: "Delay between starts_at/ends_at and the transition (F077, ±1 minute).",
		Buckets: []float64{1, 5, 15, 30, 45, 60, 120, 300},
	})
	goalContributions = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_game_cgoal_contribution_total", Help: "Community goal contributions by result (counted, capped, duplicate).",
	}, []string{"result"})
	goalCacheRebuilds = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "eduworld_game_cgoal_cache_rebuild_total", Help: "Community goal counters rewritten from PostgreSQL (goal-recompute).",
	})
	seasonPoints = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_game_season_points_total", Help: "Season point credits by capped flag (F077).",
	}, []string{"capped"})
	liveOpsClaims = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_game_event_claim_total", Help: "Live-ops rewards given by kind (event_milestone, season_tier, auto grants).",
	}, []string{"kind"})
)

// RegisterLiveOps registers the F077 metrics.
func RegisterLiveOps(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{runTransitions, schedulerLag, goalContributions, goalCacheRebuilds, seasonPoints, liveOpsClaims} {
		_ = r.Register(c)
	}
}

// LiveOpsMetrics implements command.LiveOpsMetrics.
type LiveOpsMetrics struct{}

var _ command.LiveOpsMetrics = LiveOpsMetrics{}

// RunTransition counts one run transition.
func (LiveOpsMetrics) RunTransition(to string) { runTransitions.WithLabelValues(to).Inc() }

// SchedulerLag observes the delay of a scheduled transition.
func (LiveOpsMetrics) SchedulerLag(seconds float64) { schedulerLag.Observe(max(0, seconds)) }

// Contribution counts one contribution by result.
func (LiveOpsMetrics) Contribution(result string) { goalContributions.WithLabelValues(result).Inc() }

// CacheRebuild counts one counter rewritten.
func (LiveOpsMetrics) CacheRebuild() { goalCacheRebuilds.Inc() }

// SeasonPoints counts one season point credit.
func (LiveOpsMetrics) SeasonPoints(capped bool) {
	seasonPoints.WithLabelValues(strconv.FormatBool(capped)).Inc()
}

// Claim counts one reward given.
func (LiveOpsMetrics) Claim(kind string) { liveOpsClaims.WithLabelValues(kind).Inc() }
