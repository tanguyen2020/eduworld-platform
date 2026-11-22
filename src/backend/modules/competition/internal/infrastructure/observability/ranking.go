package observability

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
)

// Metrics of ranking and leaderboards (F106 plan §Quan sát): never a school, student or board label (R14, cardinality).
var (
	rkOnce = sync.Once{}

	rkRatings = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_rating_updates_total",
		Help: "Glicko-2 rating updates after rated matches, by mode."}, []string{"mode"})
	rkCapped = prometheus.NewCounter(prometheus.CounterOpts{Name: "eduworld_points_capped_total",
		Help: "Competition Points entries cut by the daily cap (competition_points_daily_cap)."})
	rkRebuild = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_ldb_rebuild_total",
		Help: "Leaderboard boards rebuilt from the ledgers, by reason (missing, drift)."}, []string{"reason"})
	rkDrift = prometheus.NewCounter(prometheus.CounterOpts{Name: "eduworld_ldb_drift_detected_total",
		Help: "Leaderboard boards whose Valkey values differed from the ledgers (alert)."})
	rkClose = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "eduworld_ldb_period_close_seconds",
		Help: "Duration of one run of leaderboard-period-close.", Buckets: []float64{.1, .5, 1, 5, 15, 60, 300}})
)

// RegisterRanking registers the ranking metrics once.
func RegisterRanking(reg prometheus.Registerer) {
	rkOnce.Do(func() {
		if reg == nil {
			return
		}
		for _, c := range []prometheus.Collector{rkRatings, rkCapped, rkRebuild, rkDrift, rkClose} {
			_ = reg.Register(c)
		}
	})
}

// Ranking implements port.RankingMetrics.
type Ranking struct{}

var _ port.RankingMetrics = Ranking{}

// RatingUpdated implements port.RankingMetrics.
func (Ranking) RatingUpdated(mode string) { rkRatings.WithLabelValues(mode).Inc() }

// PointsCapped implements port.RankingMetrics.
func (Ranking) PointsCapped() { rkCapped.Inc() }

// Rebuild implements port.RankingMetrics.
func (Ranking) Rebuild(reason string) { rkRebuild.WithLabelValues(reason).Inc() }

// Drift implements port.RankingMetrics.
func (Ranking) Drift() { rkDrift.Inc() }

// PeriodClose implements port.RankingMetrics.
func (Ranking) PeriodClose(d time.Duration) { rkClose.Observe(d.Seconds()) }
