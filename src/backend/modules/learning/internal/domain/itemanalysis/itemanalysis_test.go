package itemanalysis

import (
	"math"
	"testing"
	"time"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-4 }

func ms(n int) *int { return &n }

// TestComputeHandDataset checks F044-AC-16 on a dataset computed by hand: only first scored answers are given, rapid
// guesses are excluded from p_value/discrimination/distractors, answers with hints are separated, and the counts drive
// the threshold.
func TestComputeHandDataset(t *testing.T) {
	var rs []Response
	// 6 valid answers without hint: 4 correct (o_a), 2 wrong (o_b, o_c); totals 0.9, 0.8, 0.7, 0.6 / 0.3, 0.2.
	for i, tot := range []float64{0.9, 0.8, 0.7, 0.6} {
		rs = append(rs, Response{Correct: true, Total: tot, TimeMS: ms(1000 * (i + 1)), Options: []string{"o_a"}})
	}
	rs = append(rs, Response{Correct: false, Total: 0.3, TimeMS: ms(2000), Options: []string{"o_b"}},
		Response{Correct: false, Total: 0.2, TimeMS: ms(3000), Options: []string{"o_c"}})
	// One rapid guess (excluded) and one answer with a hint (separated).
	rs = append(rs, Response{Correct: true, Total: 1, TimeMS: ms(100), RapidGuess: true, Options: []string{"o_a"}},
		Response{Correct: true, Total: 0.5, TimeMS: ms(5000), HintsUsed: 1, Options: []string{"o_a"}})

	s := Compute(rs, Options{MinResponses: 6, OptionIDs: []string{"o_a", "o_b", "o_c", "o_d"}, CorrectOptionIDs: []string{"o_a"}, Difficulty: 3})
	if s.FirstAttempts != 8 || s.ExcludedRapidGuess != 1 || s.WithHint != 1 {
		t.Fatalf("counts = %+v", s)
	}
	if s.PValue == nil || !near(*s.PValue, 4.0/6) {
		t.Fatalf("p_value = %v", s.PValue)
	}
	if s.PValueWithHint == nil || !near(*s.PValueWithHint, 1) {
		t.Fatalf("p_value_with_hint = %v", s.PValueWithHint)
	}
	// Point-biserial: M1 = 0.75, M0 = 0.25, population SD of the six totals.
	mean := (0.9 + 0.8 + 0.7 + 0.6 + 0.3 + 0.2) / 6
	var ss float64
	for _, x := range []float64{0.9, 0.8, 0.7, 0.6, 0.3, 0.2} {
		ss += (x - mean) * (x - mean)
	}
	sd := math.Sqrt(ss / 6)
	want := (0.75 - 0.25) / sd * math.Sqrt(4.0/6*2.0/6)
	if s.Discrimination == nil || !near(*s.Discrimination, want) {
		t.Fatalf("discrimination = %v, want %v", s.Discrimination, want)
	}
	if !near(s.DistractorRates["o_b"], 1.0/6) || !near(s.DistractorRates["o_c"], 1.0/6) || s.DistractorRates["o_d"] != 0 {
		t.Fatalf("distractors = %v", s.DistractorRates)
	}
	if _, ok := s.DistractorRates["o_a"]; ok {
		t.Fatal("the correct option is not a distractor")
	}
	// avg time of the 7 non-rapid answers.
	if s.AvgTimeMS != (1000+2000+3000+4000+2000+3000+5000)/7 {
		t.Fatalf("avg_time_ms = %d", s.AvgTimeMS)
	}
	if !near(s.RapidGuessRate, 1.0/8) || !near(s.HintUsageRate, 1.0/8) {
		t.Fatalf("rates = %v %v", s.RapidGuessRate, s.HintUsageRate)
	}
	if !has(s.Flags, FlagUnusedOption) {
		t.Fatalf("flags = %v, want unused_option (o_d)", s.Flags)
	}
	// p = 0.667 → proposed difficulty 3 = declared: no calibration flag.
	if has(s.Flags, FlagNeedsCalibration) {
		t.Fatalf("flags = %v", s.Flags)
	}
}

func has(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// TestComputeBelowThreshold: fewer valid first answers than item_analysis_min_responses → counts only, no statistics.
func TestComputeBelowThreshold(t *testing.T) {
	rs := []Response{{Correct: true, Total: 1}, {Correct: false, Total: 0}, {Correct: true, RapidGuess: true}}
	s := Compute(rs, Options{MinResponses: 30, Difficulty: 2})
	if s.Sufficient || s.PValue != nil || s.Discrimination != nil || len(s.Flags) != 0 || s.FirstAttempts != 3 {
		t.Fatalf("stats = %+v", s)
	}
}

// TestFlagsCalibrationAndFeedback: a declared difficulty two levels away is flagged; a wrong option chosen by ≥ 25 % suggests
// writing its feedback; a non-discriminating item is flagged.
func TestFlagsCalibrationAndFeedback(t *testing.T) {
	var rs []Response
	for i := 0; i < 40; i++ {
		rs = append(rs, Response{Correct: i < 16, Total: 0.5, Options: []string{map[bool]string{true: "o_a", false: "o_b"}[i < 16]}})
	}
	s := Compute(rs, Options{MinResponses: 30, OptionIDs: []string{"o_a", "o_b"}, CorrectOptionIDs: []string{"o_a"}, Difficulty: 1})
	if !has(s.Flags, FlagNeedsCalibration) || !has(s.Flags, FlagSuggestOptionFeedback) {
		t.Fatalf("flags = %v", s.Flags)
	}
	if s.Discrimination != nil || !has(s.Flags, FlagLowDiscrimination) {
		t.Fatalf("constant totals: discrimination = %v flags = %v", s.Discrimination, s.Flags)
	}
}

// TestCalibrationTable checks the p_value → difficulty table and p̂ (QUESTION_BANK §9.1).
func TestCalibrationTable(t *testing.T) {
	for _, c := range []struct {
		p    float64
		want int
	}{{0.95, 1}, {0.85, 1}, {0.84, 2}, {0.70, 2}, {0.69, 3}, {0.5, 3}, {0.49, 4}, {0.40, 4}, {0.30, 4}, {0.29, 5}, {0, 5}} {
		if got := ProposedDifficulty(c.p); got != c.want {
			t.Errorf("ProposedDifficulty(%v) = %d, want %d", c.p, got, c.want)
		}
	}
	for d, want := range map[int]float64{1: 0.90, 2: 0.77, 3: 0.60, 4: 0.40, 5: 0.20} {
		if PHat(d) != want {
			t.Errorf("PHat(%d) = %v", d, PHat(d))
		}
	}
}

// TestPropose checks F044-AC-19: ≥ 100 answers and ≥ 1 level apart; a pending proposal blocks a new one; two consecutive
// dismissals suppress the question for 90 days.
func TestPropose(t *testing.T) {
	cfg := DefaultCalibrationConfig()
	now := time.Date(2026, 12, 7, 0, 0, 0, 0, time.UTC)
	if d, ok := Propose(2, 0.40, 100, PriorState{}, cfg, now); !ok || d != 4 {
		t.Fatalf("Propose = %d %v, want 4 true", d, ok)
	}
	if _, ok := Propose(2, 0.40, 99, PriorState{}, cfg, now); ok {
		t.Fatal("below the sample size")
	}
	if _, ok := Propose(3, 0.55, 500, PriorState{}, cfg, now); ok {
		t.Fatal("same level")
	}
	if _, ok := Propose(2, 0.40, 100, PriorState{Pending: true}, cfg, now); ok {
		t.Fatal("pending proposal")
	}
	st := Dismiss(PriorState{}, cfg, now)
	if st.ConsecutiveDismissals != 1 || st.SuppressedUntil != nil {
		t.Fatalf("first dismissal = %+v", st)
	}
	st = Dismiss(st, cfg, now)
	if st.ConsecutiveDismissals != 2 || st.SuppressedUntil == nil || !st.SuppressedUntil.Equal(now.AddDate(0, 0, 90)) {
		t.Fatalf("second dismissal = %+v", st)
	}
	if _, ok := Propose(2, 0.40, 100, st, cfg, now.AddDate(0, 0, 89)); ok {
		t.Fatal("suppressed for 90 days")
	}
	if _, ok := Propose(2, 0.40, 100, st, cfg, now.AddDate(0, 0, 90)); !ok {
		t.Fatal("proposed again after 90 days")
	}
	if st := Accept(st); st.ConsecutiveDismissals != 0 || st.SuppressedUntil != nil {
		t.Fatalf("accept resets the state: %+v", st)
	}
}
