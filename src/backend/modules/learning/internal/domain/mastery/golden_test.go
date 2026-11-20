package mastery

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
)

const tol = 0.0005

func model() WeightedEvidence { return WeightedEvidence{ModelVersion: "we-1.0", P: DefaultParams()} }

// day returns 09:00 ICT of day n (day 1 = 2026-09-07).
func day(n int) time.Time {
	return time.Date(2026, 9, 7, 2, 0, 0, 0, time.UTC).AddDate(0, 0, n-1)
}

func near(t *testing.T, what string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.4f, want %.4f", what, got, want)
	}
}

// F031-AC-09 (T017, TC-LRN-010): every row of the example table of LEARNING_ANALYTICS §3.3 — numeric questions (c = 0),
// no hint, no rapid_guess — and the decay after 104 days.
func TestGoldenTable(t *testing.T) {
	m := model()
	rows := []struct {
		day      int
		ctx      string
		d        int
		o        float64
		w, aw, a float64
		m        float64
		level    Level
		n, ndays int
	}{
		{1, "practice", 2, 1, 0.63, 0.2205, 0.3333, 0.5333, Developing, 1, 1},
		{1, "practice", 2, 0, 0.77, 0.2695, 0.2695, 0.3896, Beginning, 2, 1},
		{1, "practice", 3, 1, 0.70, 0.2450, 0.2450, 0.5391, Developing, 3, 1},
		{3, "quest", 2, 1, 0.54, 0.1890, 0.1890, 0.6262, Proficient, 4, 2},
		{5, "exam", 3, 1, 1.00, 0.3500, 0.3500, 0.7570, Proficient, 5, 3},
		{5, "exam", 4, 1, 1.10, 0.3850, 0.3850, 0.8506, Mastered, 6, 3},
	}
	s := State{}
	if v, l := m.Effective(s, day(1)); l != NotStarted || math.Abs(v-0.300) > tol {
		t.Fatalf("prior: %v %v", v, l)
	}
	mastered := 0
	for i, r := range rows {
		ev := Evidence{EventID: uuid.New(), OccurredAt: day(r.day).Add(time.Duration(i) * time.Minute), Context: r.ctx, Difficulty: r.d,
			Outcome: r.o, QuestionType: "numeric"}
		near(t, "w", m.Weight(ev), r.w)
		near(t, "α_w", math.Min(m.P.AlphaMax, m.P.AlphaBase*m.Weight(ev)), r.aw)
		near(t, "α", m.Alpha(s.EvidenceCount, ev), r.a)
		c := Step(m, s, ev)
		s = c.After
		near(t, "M", s.Value, r.m)
		if c.LevelAfter != r.level || s.EvidenceCount != r.n || s.DistinctDays != r.ndays {
			t.Errorf("row %d: level %s n %d days %d, want %s %d %d", i+1, c.LevelAfter, s.EvidenceCount, s.DistinctDays, r.level, r.n, r.ndays)
		}
		if c.FirstMastered {
			mastered++
		}
	}
	if mastered != 1 || s.FirstMasteredAt == nil {
		t.Fatalf("first mastered %d times", mastered)
	}
	// 104 days without evidence: M_eff = 0.8506 × 0.75 = 0.638 → proficient; the stored value is unchanged (F031-AC-11).
	v, l := m.Effective(s, s.LastEvidenceAt.Add(104*24*time.Hour))
	near(t, "M_eff", v, 0.638)
	if l != Proficient {
		t.Fatalf("decayed level %s", l)
	}
	near(t, "stored M", s.Value, 0.8506)
}

// F031-AC-09: the additional example — true_false correct after one hint; the same answer flagged rapid_guess is no
// evidence (the caller drops it: M stays at the prior).
func TestGoldenAdditional(t *testing.T) {
	m := model()
	ev := Evidence{EventID: uuid.New(), OccurredAt: day(1), Context: "practice", Difficulty: 2, Outcome: 1, QuestionType: "true_false", HintsUsed: 1}
	near(t, "w", m.Weight(ev), 0.189)
	near(t, "α_w", m.P.AlphaBase*m.Weight(ev), 0.0662)
	near(t, "α", m.Alpha(0, ev), 0.3333)
	s := m.Apply(State{}, ev)
	near(t, "M", s.Value, 0.5333)
	// rapid_guess: not an evidence (Replay of no evidence keeps the prior).
	r := Replay(m, State{}, nil)
	v, l := m.Effective(r, day(1))
	near(t, "M rapid", v, 0.300)
	if l != NotStarted || r.EvidenceCount != 0 {
		t.Fatalf("rapid guess counted: %+v", r)
	}
}

// F031-AC-10 (T018): ≥ 0.80 without n_min evidence on d_min days is not mastered, even with full marks.
func TestMasteredNeedsEvidenceOnDays(t *testing.T) {
	m := model()
	s := State{}
	for i := 0; i < 10; i++ { // 10/10 in one exam on one day
		s = m.Apply(s, Evidence{EventID: uuid.New(), OccurredAt: day(1).Add(time.Duration(i) * time.Minute), Context: "exam", Difficulty: 5,
			Outcome: 1, QuestionType: "numeric"})
	}
	v, l := m.Effective(s, day(1))
	if v < 0.8 || l != Proficient || s.DistinctDays != 1 {
		t.Fatalf("one day: value %.3f level %s days %d", v, l, s.DistinctDays)
	}
	s = m.Apply(s, Evidence{EventID: uuid.New(), OccurredAt: day(2), Context: "exam", Difficulty: 5, Outcome: 1, QuestionType: "numeric"})
	if _, l := m.Effective(s, day(2)); l != Mastered {
		t.Fatalf("two days: %s", l)
	}
	// Four evidence on two days: still not mastered (n < n_min).
	s2 := State{}
	for i := 0; i < 4; i++ {
		s2 = m.Apply(s2, Evidence{EventID: uuid.New(), OccurredAt: day(1 + i%2).Add(time.Duration(i) * time.Minute), Context: "exam", Difficulty: 5,
			Outcome: 1, QuestionType: "numeric"})
	}
	if v, l := m.Effective(s2, day(2)); v < 0.8 || l != Proficient {
		t.Fatalf("n=4: value %.3f level %s", v, l)
	}
}

// F031-AC-11: decay is applied when reading only, starts after the grace period and never goes below half.
func TestEffectiveDecay(t *testing.T) {
	m := model()
	at := day(1)
	s := State{Value: 0.9, EvidenceCount: 6, DistinctDays: 3, LastEvidenceAt: &at}
	if v, _ := m.Effective(s, at.Add(14*24*time.Hour)); math.Abs(v-0.9) > tol {
		t.Fatalf("within grace: %.4f", v)
	}
	if v, _ := m.Effective(s, at.Add(10000*24*time.Hour)); math.Abs(v-0.45) > tol {
		t.Fatalf("floor: %.4f", v)
	}
	if s.Value != 0.9 {
		t.Fatal("stored value changed")
	}
}

// Weights: repeat within 24 h halves w; ≥ 2 hints → 0.3; wrong answers ignore hint/guess; single_choice uses 1/k.
func TestWeights(t *testing.T) {
	m := model()
	base := Evidence{Context: "practice", Difficulty: 3, Outcome: 1, QuestionType: "numeric"}
	near(t, "base", m.Weight(base), 0.7)
	rep := base
	rep.Repeat = true
	near(t, "repeat", m.Weight(rep), 0.35)
	h := base
	h.HintsUsed = 4
	near(t, "hints", m.Weight(h), 0.21)
	wrong := base
	wrong.Outcome, wrong.HintsUsed, wrong.QuestionType = 0.4, 2, "true_false"
	near(t, "wrong", m.Weight(wrong), 0.7)
	sc := base
	sc.QuestionType, sc.OptionCount = "single_choice", 4
	near(t, "single_choice k=4", m.Weight(sc), 0.7*0.75)
	mc := base
	mc.QuestionType = "multiple_choice"
	near(t, "multiple_choice", m.Weight(mc), 0.63)
	tc := base
	tc.Context = "timed_challenge"
	near(t, "timed_challenge", m.Weight(tc), 0.4)
	if err := m.P.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := m.P
	bad.Thresholds.Mastered = 0.5
	if bad.Validate() == nil {
		t.Fatal("bad thresholds accepted")
	}
}

// Replay sorts by occurred_at and keeps first_mastered_at of the prior (no second mastered after a recompute).
func TestReplay(t *testing.T) {
	m := model()
	var evs []Evidence
	for i, d := range []int{5, 1, 3, 1, 5, 1} {
		evs = append(evs, Evidence{EventID: uuid.New(), OccurredAt: day(d).Add(time.Duration(i) * time.Minute), Context: "exam", Difficulty: 4,
			Outcome: 1, QuestionType: "numeric"})
	}
	s := Replay(m, State{}, evs)
	if s.EvidenceCount != 6 || s.DistinctDays != 3 || s.FirstMasteredAt == nil {
		t.Fatalf("replay: %+v", s)
	}
	first := *s.FirstMasteredAt
	again := Replay(m, State{FirstMasteredAt: &first}, evs)
	if !again.FirstMasteredAt.Equal(first) {
		t.Fatal("first mastered moved")
	}
	c := Step(m, again, Evidence{EventID: uuid.New(), OccurredAt: day(6), Context: "exam", Difficulty: 4, Outcome: 1, QuestionType: "numeric"})
	if c.FirstMastered {
		t.Fatal("mastered emitted twice")
	}
}
