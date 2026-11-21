package observability

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
)

// Metrics of matchmaking (MATCHMAKING §10, F104 T041): never a school or student label (R14, cardinality).
var (
	mmOnce = sync.Once{}

	mmTickets = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_mm_tickets_total",
		Help: "createMatchmakingTicket by mode and result (ok or the problem code)."}, []string{"mode", "result"})
	mmQueueSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "eduworld_mm_queue_size",
		Help: "Queued tickets seen by the last matchmaker pass of a queue."}, []string{"mode", "grade_key", "subject"})
	mmWait = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "eduworld_mm_wait_seconds",
		Help:    "Wait of a ticket until it ends (matched, cancelled, expired).",
		Buckets: []float64{1, 2, 5, 10, 15, 20, 30, 45, 60, 90, 120, 180, 300}}, []string{"mode", "result"})
	mmProposals = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_mm_proposals_total",
		Help: "Match proposals by result (accepted, declined, timeout, participant_unavailable)."}, []string{"result"})
	mmRatingGap = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "eduworld_mm_rating_gap",
		Help: "Rating gap (μ) between the two sides of a match.", Buckets: []float64{25, 50, 100, 150, 200, 300, 400, 600, 1000}})
	mmMatches = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_mm_matches_created_total",
		Help: "Matches created by matchmaking, duels and practice bots, by mode."}, []string{"mode"})
	mmLoop = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "eduworld_mm_loop_duration_seconds",
		Help: "Duration of one matchmaker pass (target < 1 s).", Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5}})
)

// RegisterMatchmaking registers the matchmaking metrics once.
func RegisterMatchmaking(reg prometheus.Registerer) {
	mmOnce.Do(func() {
		if reg == nil {
			return
		}
		for _, c := range []prometheus.Collector{mmTickets, mmQueueSize, mmWait, mmProposals, mmRatingGap, mmMatches, mmLoop} {
			_ = reg.Register(c)
		}
	})
}

// Matchmaking implements port.MatchmakingMetrics.
type Matchmaking struct{}

var _ port.MatchmakingMetrics = Matchmaking{}

// Ticket implements port.MatchmakingMetrics.
func (Matchmaking) Ticket(mode, result string) { mmTickets.WithLabelValues(mode, result).Inc() }

// QueueSize implements port.MatchmakingMetrics.
func (Matchmaking) QueueSize(mode, gradeKey, subject string, n int) {
	mmQueueSize.WithLabelValues(mode, gradeKey, subject).Set(float64(n))
}

// Wait implements port.MatchmakingMetrics.
func (Matchmaking) Wait(mode, result string, d time.Duration) {
	mmWait.WithLabelValues(mode, result).Observe(d.Seconds())
}

// Proposal implements port.MatchmakingMetrics.
func (Matchmaking) Proposal(result string) { mmProposals.WithLabelValues(result).Inc() }

// RatingGap implements port.MatchmakingMetrics.
func (Matchmaking) RatingGap(gap float64) { mmRatingGap.Observe(gap) }

// MatchCreated implements port.MatchmakingMetrics.
func (Matchmaking) MatchCreated(mode string) { mmMatches.WithLabelValues(mode).Inc() }

// Loop implements port.MatchmakingMetrics.
func (Matchmaking) Loop(d time.Duration) { mmLoop.Observe(d.Seconds()) }
