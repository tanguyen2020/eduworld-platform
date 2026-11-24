package observability

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
)

// Metrics of the tournaments (F107 plan §Quan sát): never a school, student or tournament label (R14, cardinality).
var (
	trnOnce = sync.Once{}

	trnScheduled = prometheus.NewCounter(prometheus.CounterOpts{Name: "eduworld_tournament_matches_scheduled_total",
		Help: "Tournament matches proposed at their time (direct proposals of matchmaking)."})
	trnForfeits = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_tournament_forfeits_total",
		Help: "Tournament nodes decided without a played match, by reason (no_show, inactive, decision_*)."}, []string{"reason"})
	trnOverrides = prometheus.NewCounter(prometheus.CounterOpts{Name: "eduworld_tournament_overrides_total",
		Help: "Bracket nodes whose winner an organizer changed (audited)."})
	trnLag = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "eduworld_tournament_scheduler_lag_seconds",
		Help: "Delay between the scheduled time of a tournament match and its proposal.", Buckets: []float64{1, 5, 15, 30, 60, 120, 300}})
)

// RegisterTournament registers the tournament metrics once.
func RegisterTournament(reg prometheus.Registerer) {
	trnOnce.Do(func() {
		if reg == nil {
			return
		}
		for _, c := range []prometheus.Collector{trnScheduled, trnForfeits, trnOverrides, trnLag} {
			_ = reg.Register(c)
		}
	})
}

// Tournament implements port.TournamentMetrics.
type Tournament struct{}

var _ port.TournamentMetrics = Tournament{}

// MatchScheduled implements port.TournamentMetrics.
func (Tournament) MatchScheduled() { trnScheduled.Inc() }

// Forfeit implements port.TournamentMetrics.
func (Tournament) Forfeit(reason string) { trnForfeits.WithLabelValues(reason).Inc() }

// Override implements port.TournamentMetrics.
func (Tournament) Override() { trnOverrides.Inc() }

// SchedulerLag implements port.TournamentMetrics.
func (Tournament) SchedulerLag(seconds float64) { trnLag.Observe(max(seconds, 0)) }
