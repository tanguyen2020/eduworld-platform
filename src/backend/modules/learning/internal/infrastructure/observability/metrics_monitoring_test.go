package observability

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// MONITORING §3 "Learning Completion" and "Exam Submission": the learning counters are registered under the names and
// labels of the catalogue (the former eduworld_attempt_submitted_total, eduworld_learning_hints_revealed_total and
// eduworld_learning_rapid_guess_total are gone) and move per call.
func TestLearningMonitoringMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	RegisterAttempts(reg)
	RegisterSelfStudy(reg)
	RegisterExams(reg)
	AttemptCounter{}.Submitted("practice", "ok")
	AttemptCounter{}.Submitted("practice", "duplicate")
	AttemptCounter{}.Completed("quiz", "practice", "math", "g4_5")
	SelfStudyCounter{}.HintRevealed("math", "g4_5")
	SelfStudyCounter{}.RapidGuess("unknown", "g1_3")
	ExamCounter{}.Submitted("mock", "late")
	ExamCounter{}.Submitted("mock", "auto_submitted")
	ExamCounter{}.SubmitDuration("mock", 250*time.Millisecond)
	want := `
# HELP eduworld_attempts_submitted_total Student submits of activity attempts by mode and result (ok, duplicate, late, rejected).
# TYPE eduworld_attempts_submitted_total counter
eduworld_attempts_submitted_total{mode="practice",result="duplicate"} 1
eduworld_attempts_submitted_total{mode="practice",result="ok"} 1
# HELP eduworld_learning_activities_completed_total Activities completed (graded) by a student submit, by activity type, context, subject code and grade band.
# TYPE eduworld_learning_activities_completed_total counter
eduworld_learning_activities_completed_total{activity_type="quiz",context="practice",grade_band="g4_5",subject="math"} 1
# HELP eduworld_hint_requests_total Hint levels opened by students, by subject code and grade band (F032-AC-10).
# TYPE eduworld_hint_requests_total counter
eduworld_hint_requests_total{grade_band="g4_5",subject="math"} 1
# HELP eduworld_rapid_guess_responses_total Scored answers flagged rapid_guess, by subject code and grade band (F032-AC-16).
# TYPE eduworld_rapid_guess_responses_total counter
eduworld_rapid_guess_responses_total{grade_band="g1_3",subject="unknown"} 1
# HELP eduworld_exam_submissions_total Exam submissions by exam type and result (ok, late, duplicate, rejected, auto_submitted).
# TYPE eduworld_exam_submissions_total counter
eduworld_exam_submissions_total{exam_type="mock",result="auto_submitted"} 1
eduworld_exam_submissions_total{exam_type="mock",result="late"} 1
`
	if err := testutil.GatherAndCompare(reg, strings.NewReader(want), "eduworld_attempts_submitted_total",
		"eduworld_learning_activities_completed_total", "eduworld_hint_requests_total", "eduworld_rapid_guess_responses_total",
		"eduworld_exam_submissions_total"); err != nil {
		t.Fatal(err)
	}
	if n, _ := testutil.GatherAndCount(reg, "eduworld_exam_submit_duration_seconds"); n != 1 {
		t.Fatalf("exam submit duration series %d", n)
	}
	for _, old := range []string{"eduworld_attempt_submitted_total", "eduworld_learning_hints_revealed_total", "eduworld_learning_rapid_guess_total"} {
		if n, _ := testutil.GatherAndCount(reg, old); n != 0 {
			t.Errorf("%s still exposed", old)
		}
	}
}
