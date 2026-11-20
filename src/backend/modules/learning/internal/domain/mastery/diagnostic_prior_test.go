package mastery

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// F032-AC-26, AC-28 (T041): M0 = clamp(0.15 + 0.6·acc; 0.15; 0.75) for objectives with ≥ 2 scored answers, an
// objective with one answer keeps the default, prerequisites get the highest prior of their dependents, objectives
// with evidence keep theirs; the prior does not change n nor the level and is the start of the first evidence.
func TestDiagnosticPriors(t *testing.T) {
	d := DefaultDiagnosticPrior()
	a, b, c, pre, preB, done := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for acc, want := range map[float64]float64{0: 0.15, 0.5: 0.45, 1: 0.75, 2.0 / 3: 0.55} {
		near(t, "M0", d.M0(acc), want)
	}
	g2, g7 := 2, 7
	if d.MaxQuestionsFor(&g2) != 15 || d.MaxQuestionsFor(&g7) != 20 || d.MaxQuestionsFor(nil) != 20 {
		t.Fatal("max questions")
	}
	out := d.Priors(PriorInput{
		Outcomes:      map[uuid.UUID][]float64{a: {1, 1, 0}, b: {1, 1}, c: {1}, done: {0, 0}},
		Prerequisites: map[uuid.UUID][]uuid.UUID{a: {pre}, b: {pre, a}, done: {preB}},
		HasEvidence:   func(id uuid.UUID) bool { return id == done || id == preB },
	})
	got := map[uuid.UUID]Prior{}
	for _, p := range out {
		got[p.ObjectiveID] = p
	}
	if len(got) != 3 {
		t.Fatalf("priors: %+v", out)
	}
	near(t, "a", got[a].Value, 0.55)
	near(t, "b", got[b].Value, 0.75)
	if got[a].Source != PriorDiagnostic || got[pre].Source != PriorPropagated {
		t.Fatalf("sources: %+v", got)
	}
	near(t, "prerequisite takes the highest dependent", got[pre].Value, 0.75)
	if _, ok := got[c]; ok {
		t.Fatal("one answer sets a prior")
	}

	m := WeightedEvidence{ModelVersion: "we-1.0", P: DefaultParams()}
	v := 0.55
	s := State{PriorSource: PriorDiagnostic, PriorValue: &v, Value: v}
	val, lvl := m.Effective(s, time.Now())
	if lvl != NotStarted || val != 0.55 || s.EvidenceCount != 0 {
		t.Fatalf("prior keeps not_started: %v %s", val, lvl)
	}
	ev := Evidence{EventID: uuid.New(), OccurredAt: day(1), Context: "practice", Difficulty: 2, Outcome: 1, QuestionType: "numeric"}
	after := m.Apply(s, ev)
	// α = α_cold = 1/3: 0.55 + (1 − 0.55)/3 = 0.70.
	near(t, "first evidence from the prior", after.Value, 0.70)
	// A replay (recompute) after the value moved still starts from the prior.
	moved := after
	near(t, "replay from the prior", Replay(m, moved, []Evidence{ev}).Value, 0.70)
}
