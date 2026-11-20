package examquality

import (
	"math"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/blueprint"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/exam/paper"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

// pool returns n candidates per difficulty 1..5 of one topic (deterministic ids).
func pool(n int) []selection.Candidate {
	topic := uuid.MustParse("00000000-0000-0000-0000-0000000000aa")
	var out []selection.Candidate
	for d := 1; d <= 5; d++ {
		for i := 0; i < n; i++ {
			q := uuid.NewSHA1(uuid.NameSpaceOID, []byte{byte(d), byte(i), 'q'})
			v := uuid.NewSHA1(uuid.NameSpaceOID, []byte{byte(d), byte(i), 'v'})
			out = append(out, selection.Candidate{QuestionID: q, VersionID: v, Type: "single_choice", Difficulty: d, TopicID: topic})
		}
	}
	return out
}

func def(enabled bool) blueprint.Definition {
	e := enabled
	return blueprint.Definition{TotalPoints: 10, Sections: []blueprint.Section{{ID: "s1", Rules: []blueprint.Rule{
		{Count: 6, PointsEach: 1}, {Count: 2, PointsEach: 2, Difficulty: &blueprint.Range{Min: 3}}}}},
		Variants: &blueprint.Variants{Strategy: blueprint.StrategyIndependentDraw, Count: 4, Balance: &blueprint.Balance{Enabled: &e}}}
}

func spread(ps []paper.Paper) float64 {
	lo, hi := 1.0, 0.0
	for _, p := range ps {
		lo, hi = math.Min(lo, *p.ExpectedP), math.Max(hi, *p.ExpectedP)
	}
	return hi - lo
}

// TestBalancerReachesThreshold checks F044-AC-22 (TC-EXM-001): 4 variants of independent_draw are balanced within
// variant_balance_max_mean_p_diff, every item still satisfies its rule, no question twice in a paper, reproducible.
func TestBalancerReachesThreshold(t *testing.T) {
	d, pl := def(true), pool(12)
	run := func() []paper.Paper {
		ps, err := paper.SessionPapers(d, pl, blueprint.StrategyIndependentDraw, 4, 42, false, Balancer{})
		if err != nil {
			t.Fatal(err)
		}
		return ps
	}
	ps := run()
	for _, p := range ps {
		if p.ExpectedP == nil || p.BalanceWarning {
			t.Fatalf("paper %d: expected_p = %v warning = %v", p.VariantNo, p.ExpectedP, p.BalanceWarning)
		}
		seen := map[uuid.UUID]bool{}
		for i, it := range p.Items {
			if seen[it.QuestionID] {
				t.Fatalf("question twice in paper %d", p.VariantNo)
			}
			seen[it.QuestionID] = true
			if i >= 6 && it.Difficulty < 3 {
				t.Fatalf("item %d of paper %d breaks its rule (difficulty %d)", i, p.VariantNo, it.Difficulty)
			}
		}
	}
	if s := spread(ps); s > blueprint.DefaultMaxMeanPDiff+1e-9 {
		t.Fatalf("spread %v > %v", s, blueprint.DefaultMaxMeanPDiff)
	}
	again := run()
	for i := range ps {
		if *ps[i].ExpectedP != *again[i].ExpectedP || ps[i].Items[0].QuestionVersionID != again[i].Items[0].QuestionVersionID {
			t.Fatal("the balancer is not reproducible")
		}
	}
}

// TestBalancerUsesPValues: a measured p_value replaces p̂ of the declared difficulty.
func TestBalancerUsesPValues(t *testing.T) {
	d, pl := def(false), pool(3)
	ps, err := paper.SessionPapers(d, pl, blueprint.StrategyIndependentDraw, 1, 7, false, Balancer{})
	if err != nil {
		t.Fatal(err)
	}
	pv := map[uuid.UUID]float64{}
	var want float64
	for _, it := range ps[0].Items {
		pv[it.QuestionVersionID] = 0.5
		want += 0.5 * it.Points
	}
	got := Balancer{}.WithPValues(pv).Balance(d, pl, ps)
	if !near(*got[0].ExpectedP, want/10) {
		t.Fatalf("expected_p = %v, want %v", *got[0].ExpectedP, want/10)
	}
}

// TestBalancerWarnsWhenImpossible: a pool too narrow to balance still yields the papers, with the warning and the
// expected_p of every variant.
func TestBalancerWarnsWhenImpossible(t *testing.T) {
	topic := uuid.New()
	d := blueprint.Definition{TotalPoints: 2, Sections: []blueprint.Section{{ID: "s1", Rules: []blueprint.Rule{
		{Count: 1, PointsEach: 1, FixedQuestionVersionIDs: nil}, {Count: 1, PointsEach: 1}}}}}
	easy := selection.Candidate{QuestionID: uuid.New(), VersionID: uuid.New(), Difficulty: 1, TopicID: topic}
	hard := selection.Candidate{QuestionID: uuid.New(), VersionID: uuid.New(), Difficulty: 5, TopicID: topic}
	papers := []paper.Paper{
		{VariantNo: 1, TotalPoints: 2, Items: []paper.Item{{QuestionID: easy.QuestionID, QuestionVersionID: easy.VersionID, Points: 1, Difficulty: 1},
			{QuestionID: hard.QuestionID, QuestionVersionID: hard.VersionID, Points: 1, Difficulty: 5}}},
		{VariantNo: 2, TotalPoints: 2, Items: []paper.Item{{QuestionID: easy.QuestionID, QuestionVersionID: easy.VersionID, Points: 1, Difficulty: 1},
			{QuestionID: easy.QuestionID, QuestionVersionID: easy.VersionID, Points: 1, Difficulty: 1}}},
	}
	out := Balancer{}.Balance(d, []selection.Candidate{easy}, papers)
	if len(out) != 2 || out[0].ExpectedP == nil || out[1].ExpectedP == nil || !out[0].BalanceWarning || !out[1].BalanceWarning {
		t.Fatalf("papers = %+v", out)
	}
}

// TestKR20AndSEM checks F044-AC-23 on a hand-computed 0/1 matrix: Σpq = 0.8, σ² = 2, KR-20 = 4/3 × (1 − 0.4) = 0.8;
// SEM = SD × √(1 − r) on the 10-point scale.
func TestKR20AndSEM(t *testing.T) {
	rows := [][]float64{{1, 1, 1, 1}, {1, 1, 1, 0}, {1, 1, 0, 0}, {1, 0, 0, 0}, {0, 0, 0, 0}}
	var sheets []Sheet
	items := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	for _, r := range rows {
		s := Sheet{StudentID: uuid.New(), VariantNo: 1, Max: 4}
		for j, x := range r {
			s.Items = append(s.Items, ItemScore{QuestionVersionID: items[j], Score: x, Max: 1, Correct: x == 1})
			s.Total += x
		}
		sheets = append(sheets, s)
	}
	cfg := DefaultConfig()
	cfg.MinStudents, cfg.MinItemResponses = 5, 5
	rep := Analyze(sheets, nil, cfg)
	v := rep.Variants[0]
	if v.Method != MethodKR20 || v.Reliability == nil || !near(*v.Reliability, 0.8) {
		t.Fatalf("variant = %+v", v)
	}
	sd := math.Sqrt(2) / 4 * 10
	if !near(v.SD10, sd) || v.SEM == nil || !near(*v.SEM, sd*math.Sqrt(0.2)) || !near(v.Mean10, 5) {
		t.Fatalf("sd = %v sem = %v mean = %v", v.SD10, v.SEM, v.Mean10)
	}
	if rep.ReliabilityWarning {
		t.Fatal("0.8 is not below 0.7")
	}
	if rep.Median != 5 || !near(rep.PassRate, 0.6) || rep.Distribution[10] != 1 || rep.Distribution[20] != 1 || rep.Distribution[0] != 1 {
		t.Fatalf("median %v pass %v distribution %v", rep.Median, rep.PassRate, rep.Distribution)
	}
	// Item 1 (p = 0.8) and item 4 (p = 0.2) are within bounds; nothing to drop.
	for _, it := range rep.Items {
		if it.PValue == nil {
			t.Fatalf("item %v without p_value", it.QuestionVersionID)
		}
	}
	// Below the minimum number of students: no reliability ("not enough data").
	cfg.MinStudents = 30
	if v := Analyze(sheets, nil, cfg).Variants[0]; v.Reliability != nil || v.SEM != nil || v.Enough {
		t.Fatalf("variant under the minimum = %+v", v)
	}
}

// TestAlphaGapAndProblemItems: partial credit gives Cronbach alpha; two variants whose means differ by ≥ 1 point are
// flagged; a reliability under exam_reliability_warn sets the warning; a constant item is a problem item (drop_item).
func TestAlphaGapAndProblemItems(t *testing.T) {
	rows := [][]float64{{2, 2, 1}, {2, 1, 1}, {1, 1, 0}, {1, 0, 0}}
	items := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	var sheets []Sheet
	for _, r := range rows {
		s := Sheet{StudentID: uuid.New(), VariantNo: 1, Max: 6}
		for j, x := range r {
			s.Items = append(s.Items, ItemScore{QuestionVersionID: items[j], Score: x, Max: 2})
			s.Total += x
		}
		sheets = append(sheets, s)
	}
	constant := uuid.New()
	for i := 0; i < 4; i++ {
		s := Sheet{StudentID: uuid.New(), VariantNo: 2, Max: 2, Total: 2}
		s.Items = []ItemScore{{QuestionVersionID: constant, Score: 1, Max: 1, Correct: true}, {QuestionVersionID: uuid.New(), Score: 1, Max: 1, Correct: true}}
		sheets = append(sheets, s)
	}
	cfg := DefaultConfig()
	cfg.MinStudents, cfg.MinItemResponses = 4, 4
	rep := Analyze(sheets, map[int]*float64{1: ptr(0.6)}, cfg)
	// alpha = k/(k−1) × (1 − Σσᵢ²/σ²): σᵢ² = 0.25, 0.5, 0.25; totals 5, 4, 2, 1 → σ² = 2.5.
	want := 1.5 * (1 - 1.0/2.5)
	v1 := rep.Variants[0]
	if v1.Method != MethodAlpha || v1.Reliability == nil || !near(*v1.Reliability, want) || v1.ExpectedP == nil || *v1.ExpectedP != 0.6 {
		t.Fatalf("variant 1 = %+v", v1)
	}
	if !rep.VariantGap {
		t.Fatalf("means %v and %v differ by ≥ 1 point", rep.Variants[0].Mean10, rep.Variants[1].Mean10)
	}
	if rep.ReliabilityWarning || rep.Variants[1].Reliability != nil {
		t.Fatalf("variant 1 is 0.9 and variant 2 has no variance (no reliability): %+v", rep.Variants)
	}
	// KR-20 = 0 (Σpq = σ² = 0.75) → below exam_reliability_warn.
	var low []Sheet
	for _, r := range [][]float64{{1, 0, 1}, {0, 1, 1}, {1, 1, 0}, {0, 0, 0}} {
		s := Sheet{StudentID: uuid.New(), VariantNo: 1, Max: 3}
		for j, x := range r {
			s.Items = append(s.Items, ItemScore{QuestionVersionID: items[j], Score: x, Max: 1, Correct: x == 1})
			s.Total += x
		}
		low = append(low, s)
	}
	if r := Analyze(low, nil, cfg); !r.ReliabilityWarning || !r.Variants[0].Warning || !near(*r.Variants[0].Reliability, 0) {
		t.Fatalf("low reliability = %+v", r.Variants)
	}
	var problem bool
	for _, it := range rep.Items {
		if it.QuestionVersionID == constant {
			problem = it.Problem && it.Suggestion == SuggestDropItem
		}
	}
	if !problem {
		t.Fatalf("the constant item (p = 1) is a problem item: %+v", rep.Items)
	}
}

func ptr(f float64) *float64 { return &f }
