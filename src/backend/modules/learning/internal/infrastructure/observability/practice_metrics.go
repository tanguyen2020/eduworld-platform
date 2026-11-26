package observability

import "github.com/prometheus/client_golang/prometheus"

// Self-study metrics (plan F032 "Quan sát"; names and labels of the first two owned by MONITORING §3). Labels are subject
// codes of the platform catalog, grade bands and rule codes, never ids, answers nor hint texts (R14). The pool shortage of a selection is eduworld_qb_select_pool_insufficient_total (F022).
var (
	hintsRevealed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_hint_requests_total",
		Help: "Hint levels opened by students, by subject code and grade band (F032-AC-10).",
	}, []string{"subject", "grade_band"})
	rapidGuesses = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_rapid_guess_responses_total",
		Help: "Scored answers flagged rapid_guess, by subject code and grade band (F032-AC-16).",
	}, []string{"subject", "grade_band"})
	recommendationsGenerated = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_learning_recommendations_generated_total",
		Help: "Recommendations created by the rule engine, by rule (F032-AC-30).",
	}, []string{"rule_code"})
)

// RegisterSelfStudy adds the self-study metrics to r; already registered collectors are ignored.
func RegisterSelfStudy(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{hintsRevealed, rapidGuesses, recommendationsGenerated} {
		_ = r.Register(c)
	}
}

// SelfStudyCounter adapts the self-study metrics to command.SelfStudyMetrics.
type SelfStudyCounter struct{}

// HintRevealed counts one opened hint level.
func (SelfStudyCounter) HintRevealed(subject, gradeBand string) {
	hintsRevealed.WithLabelValues(subject, gradeBand).Inc()
}

// RapidGuess counts one rapid guess.
func (SelfStudyCounter) RapidGuess(subject, gradeBand string) {
	rapidGuesses.WithLabelValues(subject, gradeBand).Inc()
}

// RecommendationGenerated counts one created recommendation.
func (SelfStudyCounter) RecommendationGenerated(ruleCode string) {
	recommendationsGenerated.WithLabelValues(ruleCode).Inc()
}

// RapidGuesses exposes the counter to the tests.
func RapidGuesses() *prometheus.CounterVec { return rapidGuesses }

// HintRequests exposes eduworld_hint_requests_total (tests).
func HintRequests() *prometheus.CounterVec { return hintsRevealed }
