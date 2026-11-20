package observability

import "github.com/prometheus/client_golang/prometheus"

// Self-study metrics (plan F032 "Quan sát"). Labels are modes, question types and rule codes, never ids, answers nor
// hint texts (R14). The pool shortage of a selection is eduworld_qb_select_pool_insufficient_total (F022).
var (
	hintsRevealed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_learning_hints_revealed_total",
		Help: "Hint levels opened by students, by attempt mode (F032-AC-10).",
	}, []string{"mode"})
	rapidGuesses = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_learning_rapid_guess_total",
		Help: "Scored answers flagged rapid_guess, by question type (F032-AC-16).",
	}, []string{"question_type"})
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
func (SelfStudyCounter) HintRevealed(mode string) { hintsRevealed.WithLabelValues(mode).Inc() }

// RapidGuess counts one rapid guess.
func (SelfStudyCounter) RapidGuess(questionType string) {
	rapidGuesses.WithLabelValues(questionType).Inc()
}

// RecommendationGenerated counts one created recommendation.
func (SelfStudyCounter) RecommendationGenerated(ruleCode string) {
	recommendationsGenerated.WithLabelValues(ruleCode).Inc()
}

// RapidGuesses exposes the counter to the tests.
func RapidGuesses() *prometheus.CounterVec { return rapidGuesses }

// LMS metrics (plan F040 "Quan sát").
var (
	lmsCourses = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_lms_courses_provisioned_total",
		Help: "Courses of class.synced by result (created, exists, no_curriculum) (F040-AC-01).",
	}, []string{"result"})
	lmsUnlocks = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_lms_path_node_unlocks_total",
		Help: "Path nodes unlocked for a student by source (manual) (F040-AC-10).",
	}, []string{"source"})
)

// RegisterLMS adds the LMS metrics to r; already registered collectors are ignored.
func RegisterLMS(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{lmsCourses, lmsUnlocks} {
		_ = r.Register(c)
	}
}

// LMSCounter adapts the LMS metrics to command.LMSMetrics.
type LMSCounter struct{}

// CourseProvisioned counts one course of a class.synced.
func (LMSCounter) CourseProvisioned(result string) { lmsCourses.WithLabelValues(result).Inc() }

// NodeUnlocked counts one unlocked node.
func (LMSCounter) NodeUnlocked(source string) { lmsUnlocks.WithLabelValues(source).Inc() }
