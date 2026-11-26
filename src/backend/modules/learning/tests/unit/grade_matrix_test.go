package unit

import (
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/plugins/minigame"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/analytics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/attempt"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/mastery"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/readiness"
)

// Grade matrix 1–9 of module learning (TEST_STRATEGY §6.10): every grade-dependent rule of the learning domain runs for
// all nine grades (and the unknown grade, which always takes the most protective value) with the band boundaries 3→4,
// 4→5, 5→6 and 7→8 asserted exactly. Parameters are the CONFIG_DEFAULTS of the domain (Default*), never this file.

var grades = []int{1, 2, 3, 4, 5, 6, 7, 8, 9}

func gp(g int) *int { return &g }

// TestGradeMatrixAttemptRules (ACTIVITY_ENGINE §8, LEARNING_ANALYTICS §3.5, F032-AC-16): attempts_per_question 3 for
// grades 1–3 and 2 above; the rapid-guess threshold × 1.5 for grades 1–3; the diagnostic 15 questions for grades 1–3
// and 20 above. An unknown grade takes the grade 1–3 value where it protects the child (more tries).
func TestGradeMatrixAttemptRules(t *testing.T) {
	p := attempt.DefaultPolicy()
	rg := attempt.DefaultRapidGuess()
	d := mastery.DefaultDiagnosticPrior()
	base := rg.ByType["single_choice"]
	for _, g := range grades {
		young := g <= 3
		wantTries, wantMS, wantDiag := p.AttemptsPerQuestion, base, d.MaxQuestions
		if young {
			wantTries, wantMS, wantDiag = p.AttemptsPerQuestionG13, int(float64(base)*rg.G13Factor+0.5), d.MaxQuestionsG13
		}
		if got := p.AnswerLimit(gp(g)); got != wantTries {
			t.Errorf("grade %d attempts per question %d, want %d", g, got, wantTries)
		}
		in := attempt.GuessInput{QuestionType: "single_choice", Grade: gp(g), Scored: true}
		if got := rg.Threshold(in); got != wantMS {
			t.Errorf("grade %d rapid-guess threshold %d ms, want %d", g, got, wantMS)
		}
		in.ResponseMS = int64(base) // answered at the grade 4–9 threshold: rapid for grades 1–3 only
		if rg.Flag(in) != young {
			t.Errorf("grade %d answer at %d ms flagged %v", g, base, rg.Flag(in))
		}
		if got := d.MaxQuestionsFor(gp(g)); got != wantDiag {
			t.Errorf("grade %d diagnostic %d questions, want %d", g, got, wantDiag)
		}
	}
	if p.AttemptsPerQuestionG13 != 3 || p.AttemptsPerQuestion != 2 || rg.G13Factor != 1.5 || d.MaxQuestionsG13 != 15 || d.MaxQuestions != 20 {
		t.Fatalf("defaults: %+v %v %d/%d", p, rg.G13Factor, d.MaxQuestionsG13, d.MaxQuestions)
	}
	if p.AnswerLimit(nil) != p.AttemptsPerQuestion {
		t.Errorf("unknown grade attempts %d", p.AnswerLimit(nil))
	}
}

// TestGradeMatrixMasteryDisplay (LEARNING_EXPERIENCE §7, F032-AC-34): the student-facing band of each grade and the
// numeric mastery value shown to grades 6–9 only (an unknown grade gets the g1_3 display).
func TestGradeMatrixMasteryDisplay(t *testing.T) {
	want := map[int]string{1: "g1_3", 2: "g1_3", 3: "g1_3", 4: "g4_5", 5: "g4_5", 6: "g6_7", 7: "g6_7", 8: "g8_9", 9: "g8_9"}
	for _, g := range grades {
		b := mastery.GradeBand(gp(g))
		if b != want[g] || mastery.ShowsValue(b) != (g >= 6) {
			t.Errorf("grade %d band %s shows value %v", g, b, mastery.ShowsValue(b))
		}
	}
	for _, g := range []*int{nil, gp(0), gp(10)} {
		if b := mastery.GradeBand(g); b != mastery.BandG13 && !(g != nil && *g == 10) {
			t.Errorf("unknown grade band %s", b)
		}
	}
	if mastery.ShowsValue(mastery.GradeBand(nil)) {
		t.Error("unknown grade sees the numeric mastery")
	}
}

// TestGradeMatrixMiniGameScoring (F073-AC-08/09, MINIGAME_ENGINE §6, NFR-A11Y-002): a fast correct answer earns the speed
// bonus from grade 6 only and a wrong answer costs the penalty from grade 6 only; grades 1–5 and an unknown grade never.
func TestGradeMatrixMiniGameScoring(t *testing.T) {
	r := minigame.Rules{Scoring: minigame.Scoring{PointsPerCorrect: 10, WrongPenalty: 2,
		TimeBonus: &minigame.TimeBonus{Enabled: true, MaxBonus: 5, FullBonusWithinMS: 3000}}}
	if minigame.YoungGrade != 5 {
		t.Fatalf("young grade %d", minigame.YoungGrade)
	}
	for _, g := range append([]int{0}, grades...) {
		grade := gp(g)
		if g == 0 {
			grade = nil
		}
		old := g >= 6
		right := r.Points(activity.GameAnswer{Correct: true, Scored: true, ServerMS: 1000, Score: 20}, grade)
		wantRight := 10
		if old {
			wantRight = 15
		}
		if right.Points != wantRight || r.TimeBonusEnabled(grade) != old {
			t.Errorf("grade %d fast correct: %+v, want %d", g, right, wantRight)
		}
		wrong := r.Points(activity.GameAnswer{Correct: false, Scored: true, Score: 20}, grade)
		wantWrong := 0
		if old {
			wantWrong = -2
		}
		if wrong.Points != wantWrong {
			t.Errorf("grade %d wrong: %+v, want %d", g, wrong, wantWrong)
		}
	}
}

// TestGradeMatrixAudioRequired (F025-AC-15/16, CONTENT_MODEL §8.1): content for any grade 1–3 needs its voice-over; a
// version of grades 4–9 only does not; a version spanning 3→4 does.
func TestGradeMatrixAudioRequired(t *testing.T) {
	for _, g := range grades {
		if readiness.G13([]int{g}) != (g <= 3) {
			t.Errorf("grade %d g1_3 %v", g, readiness.G13([]int{g}))
		}
	}
	if !readiness.G13([]int{3, 4}) || readiness.G13([]int{4, 5, 6, 7, 8, 9}) || readiness.G13(nil) {
		t.Error("mixed grades")
	}
}

// TestGradeMatrixGradeSuppression (F043-AC-25, FR-ANL-016, analytics_min_cell_size): the grades of one school on the
// School Learning Dashboard — for every grade, fewer than 5 students hides the grade and, when it is the only hidden
// one, the smallest other grade too (so the school total cannot reveal it); 5 or more shows it.
func TestGradeMatrixGradeSuppression(t *testing.T) {
	k := analytics.DefaultConfig().MinCellSize
	if k != 5 {
		t.Fatalf("analytics_min_cell_size %d", k)
	}
	for _, small := range grades {
		sizes := map[int]int{}
		for _, g := range grades {
			sizes[g] = 30 + g
		}
		sizes[small] = k - 1
		m := analytics.SuppressGrades(sizes, k)
		smallestOther := 1
		if small == 1 {
			smallestOther = 2
		}
		for _, g := range grades {
			if want := g == small || g == smallestOther; m.Hidden(g) != want {
				t.Errorf("grade %d below %d: grade %d hidden %v", small, k, g, m.Hidden(g))
			}
		}
		sizes[small] = k
		if m := analytics.SuppressGrades(sizes, k); len(m.Visible()) != 9 {
			t.Errorf("grade %d with %d students: visible %v", small, k, m.Visible())
		}
	}
	if !analytics.SuppressGrades(map[int]int{0: 100, 3: 40}, k).Hidden(0) {
		t.Error("the unknown grade is shown")
	}
}
