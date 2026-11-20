package mastery

import (
	"math"
	"slices"
	"strings"

	"github.com/google/uuid"
)

// DiagnosticPrior is diagnostic_prior (CONFIG_DEFAULTS, owner LEARNING_ANALYTICS §3.5) [Confirmed — Technical
// Architect 2026-11-15]: the size of a diagnostic and the formula of M0 per objective.
type DiagnosticPrior struct {
	MaxQuestionsG13 int     // 15 questions for grades 1–3
	MaxQuestions    int     // 20 questions for the other grades
	PerObjectiveMin int     // 2 questions per objective at least
	PerObjectiveMax int     // 3 questions per objective at most
	Base            float64 // M0 = clamp(Base + Slope × acc, Min, Max)
	Slope           float64
	Min             float64
	Max             float64
}

// DefaultDiagnosticPrior is M0 = clamp(0.15 + 0.6·acc; 0.15; 0.75); ≥ 2 questions per objective; 15/20 questions.
func DefaultDiagnosticPrior() DiagnosticPrior {
	return DiagnosticPrior{MaxQuestionsG13: 15, MaxQuestions: 20, PerObjectiveMin: 2, PerObjectiveMax: 3, Base: 0.15, Slope: 0.6, Min: 0.15,
		Max: 0.75}
}

// MaxQuestionsFor is the size of a diagnostic for a grade (nil = the other grades).
func (d DiagnosticPrior) MaxQuestionsFor(grade *int) int {
	if grade != nil && *grade >= 1 && *grade <= 3 {
		return d.MaxQuestionsG13
	}
	return d.MaxQuestions
}

// M0 is the prior of an objective with accuracy acc ∈ [0, 1].
func (d DiagnosticPrior) M0(acc float64) float64 {
	return math.Round(math.Min(d.Max, math.Max(d.Min, d.Base+d.Slope*acc))*1000) / 1000
}

// PriorInput is the graded diagnostic of one student: the outcomes o ∈ [0, 1] of the scored answers per objective
// (rapid_guess already left out), the direct prerequisites of the objectives and the objectives that already have
// evidence (their prior never changes, LEARNING_ANALYTICS §3.5 step 4).
type PriorInput struct {
	Outcomes      map[uuid.UUID][]float64
	Prerequisites map[uuid.UUID][]uuid.UUID
	HasEvidence   func(uuid.UUID) bool
}

// Prior is the M0 set on one objective.
type Prior struct {
	ObjectiveID uuid.UUID
	Value       float64
	Source      string // PriorDiagnostic or PriorPropagated
}

// Priors applies steps 2–4 of LEARNING_ANALYTICS §3.5 (F032-AC-26): each objective with at least PerObjectiveMin
// scored answers gets M0 from its accuracy; each direct prerequisite not diagnosed itself gets a prior not lower than
// the highest M0 of the diagnosed objectives depending on it; objectives with evidence are left out. The result is
// sorted by objective id.
func (d DiagnosticPrior) Priors(in PriorInput) []Prior {
	hasEvidence := in.HasEvidence
	if hasEvidence == nil {
		hasEvidence = func(uuid.UUID) bool { return false }
	}
	diagnosed := map[uuid.UUID]float64{}
	for lo, os := range in.Outcomes {
		if len(os) < d.PerObjectiveMin {
			continue
		}
		sum := 0.0
		for _, o := range os {
			sum += math.Max(0, math.Min(1, o))
		}
		diagnosed[lo] = d.M0(sum / float64(len(os)))
	}
	propagated := map[uuid.UUID]float64{}
	for lo, m0 := range diagnosed {
		for _, pre := range in.Prerequisites[lo] {
			if _, ok := diagnosed[pre]; ok {
				continue
			}
			if v, ok := propagated[pre]; !ok || m0 > v {
				propagated[pre] = m0
			}
		}
	}
	var out []Prior
	for lo, v := range diagnosed {
		if !hasEvidence(lo) {
			out = append(out, Prior{ObjectiveID: lo, Value: v, Source: PriorDiagnostic})
		}
	}
	for lo, v := range propagated {
		if !hasEvidence(lo) {
			out = append(out, Prior{ObjectiveID: lo, Value: v, Source: PriorPropagated})
		}
	}
	slices.SortFunc(out, func(a, b Prior) int { return strings.Compare(a.ObjectiveID.String(), b.ObjectiveID.String()) })
	return out
}
