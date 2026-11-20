// Package examquality holds the exam quality rules of F044 US6: the variant balancer of independent_draw (EXAM_ENGINE §4.1,
// implementation of the F042 extension point paper.VariantBalancer) and the reliability report of an official session
// (EXAM_ENGINE §8.1: KR-20 or Cronbach alpha, SEM, variant gap, problem items, score distribution). Pure Go.
package examquality

import (
	"math"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/blueprint"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/exam/paper"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/itemanalysis"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
)

// MaxIterations bounds the swaps/redraws of one balancing (EXAM_ENGINE §4.1: at most 200).
const MaxIterations = 200

// Balancer implements paper.VariantBalancer (F044-AC-22): p̂ of an item is its measured p_value (item analysis) when known,
// else the p̂ of its declared difficulty; P_v = Σ(p̂ × points) / total_points; while max − min of P_v exceeds
// max_mean_p_diff, the move (swap of the same rule position between the easiest and the hardest variant, or redraw of
// that position from the pool within the rule) that narrows the spread most is applied, in a fixed order (same input →
// same papers). Unreachable within MaxIterations → the papers are kept with BalanceWarning. Every paper gets ExpectedP.
type Balancer struct {
	PValues map[uuid.UUID]float64 // p_value per question version (versions with enough data only)
}

var _ paper.VariantBalancer = Balancer{}

// WithPValues returns the balancer with the measured p_values of the pool (loaded per session by the caller).
func (b Balancer) WithPValues(pv map[uuid.UUID]float64) paper.VariantBalancer {
	return Balancer{PValues: pv}
}

func (b Balancer) phat(versionID uuid.UUID, difficulty int) float64 {
	if p, ok := b.PValues[versionID]; ok {
		return p
	}
	return itemanalysis.PHat(difficulty)
}

// settings returns whether balancing is enabled (default true, assessment-blueprint.schema.json) and max_mean_p_diff.
func settings(def blueprint.Definition) (bool, float64) {
	maxDiff := blueprint.DefaultMaxMeanPDiff
	if def.Variants == nil || def.Variants.Balance == nil {
		return true, maxDiff
	}
	bal := def.Variants.Balance
	if bal.MaxMeanPDiff != nil {
		maxDiff = *bal.MaxMeanPDiff
	}
	return bal.Enabled == nil || *bal.Enabled, maxDiff
}

// ruleOf maps the item index of a paper to its rule (papers are generated rule by rule, EXAM_ENGINE §4); nil when the
// papers do not have that shape.
func ruleOf(def blueprint.Definition, n int) []blueprint.Rule {
	var out []blueprint.Rule
	for _, s := range def.Sections {
		for _, r := range s.Rules {
			for i := 0; i < r.Count; i++ {
				out = append(out, r)
			}
		}
	}
	if len(out) != n {
		return nil
	}
	return out
}

func (b Balancer) expected(p paper.Paper) float64 {
	total := p.TotalPoints
	var sum, pts float64
	for _, it := range p.Items {
		sum += b.phat(it.QuestionVersionID, it.Difficulty) * it.Points
		pts += it.Points
	}
	if total <= 0 {
		total = pts
	}
	if total <= 0 {
		return 0
	}
	return sum / total
}

func spreadOf(ps []float64) (float64, int, int) {
	hi, lo := 0, 0
	for i, p := range ps {
		if p > ps[hi] {
			hi = i
		}
		if p < ps[lo] {
			lo = i
		}
	}
	return ps[hi] - ps[lo], hi, lo
}

type move struct {
	variant, index int
	item           paper.Item
	other          int // swap partner (−1 = redraw)
	otherItem      paper.Item
	spread         float64
}

// Balance implements paper.VariantBalancer.
func (b Balancer) Balance(def blueprint.Definition, pool []selection.Candidate, papers []paper.Paper) []paper.Paper {
	out := make([]paper.Paper, len(papers))
	for i, p := range papers {
		out[i] = p
		out[i].Items = slices.Clone(p.Items)
	}
	enabled, maxDiff := settings(def)
	if enabled && len(out) > 1 {
		b.balance(def, pool, out, maxDiff)
	}
	ps := make([]float64, len(out))
	for i := range out {
		ps[i] = b.expected(out[i])
	}
	warn := false
	if enabled && len(out) > 1 {
		s, _, _ := spreadOf(ps)
		warn = s > maxDiff+1e-12
	}
	for i := range out {
		e := math.Round(ps[i]*1e4) / 1e4
		out[i].ExpectedP, out[i].BalanceWarning = &e, warn
	}
	return out
}

func (b Balancer) balance(def blueprint.Definition, pool []selection.Candidate, out []paper.Paper, maxDiff float64) {
	rules := ruleOf(def, len(out[0].Items))
	for _, p := range out {
		if rules == nil || len(p.Items) != len(rules) {
			return
		}
	}
	sorted := slices.Clone(pool)
	slices.SortFunc(sorted, func(a, c selection.Candidate) int { return strings.Compare(a.VersionID.String(), c.VersionID.String()) })
	matching := make([][]selection.Candidate, len(rules))
	for i, r := range rules {
		if len(r.FixedQuestionVersionIDs) > 0 {
			continue
		}
		for _, c := range sorted {
			if paper.RuleMatches(c, r) {
				matching[i] = append(matching[i], c)
			}
		}
	}
	ps := make([]float64, len(out))
	for i := range out {
		ps[i] = b.expected(out[i])
	}
	for iter := 0; iter < MaxIterations; iter++ {
		cur, hi, lo := spreadOf(ps)
		if cur <= maxDiff {
			return
		}
		best := move{spread: cur}
		found := false
		try := func(m move, deltas map[int]float64) {
			next := slices.Clone(ps)
			for v, d := range deltas {
				next[v] += d
			}
			if s, _, _ := spreadOf(next); s < best.spread-1e-12 {
				m.spread, best, found = s, m, true
			}
		}
		total := func(v int) float64 {
			if out[v].TotalPoints > 0 {
				return out[v].TotalPoints
			}
			return 1
		}
		for i := range rules {
			a, c := out[hi].Items[i], out[lo].Items[i]
			if len(rules[i].FixedQuestionVersionIDs) == 0 && a.QuestionID != c.QuestionID &&
				!hasAt(out[lo], a.QuestionID, i) && !hasAt(out[hi], c.QuestionID, i) {
				pa, pc := b.phat(a.QuestionVersionID, a.Difficulty), b.phat(c.QuestionVersionID, c.Difficulty)
				try(move{variant: hi, index: i, item: with(a, c), other: lo, otherItem: with(c, a)},
					map[int]float64{hi: (pc - pa) * a.Points / total(hi), lo: (pa - pc) * c.Points / total(lo)})
			}
			for _, v := range []int{hi, lo} {
				it := out[v].Items[i]
				pi := b.phat(it.QuestionVersionID, it.Difficulty)
				for _, cand := range matching[i] {
					if cand.QuestionID == it.QuestionID || hasAt(out[v], cand.QuestionID, i) {
						continue
					}
					pc := b.phat(cand.VersionID, cand.Difficulty)
					try(move{variant: v, index: i, item: fromCandidate(it, cand), other: -1}, map[int]float64{v: (pc - pi) * it.Points / total(v)})
				}
			}
		}
		if !found {
			return
		}
		out[best.variant].Items[best.index] = best.item
		if best.other >= 0 {
			out[best.other].Items[best.index] = best.otherItem
		}
		for _, v := range []int{best.variant, best.other} {
			if v >= 0 {
				ps[v] = b.expected(out[v])
			}
		}
	}
}

// hasAt reports whether paper p holds question q at a position other than skip.
func hasAt(p paper.Paper, q uuid.UUID, skip int) bool {
	for j, it := range p.Items {
		if j != skip && it.QuestionID == q {
			return true
		}
	}
	return false
}

// with returns slot (section, position, points) holding the question of src.
func with(slot, src paper.Item) paper.Item {
	slot.QuestionVersionID, slot.QuestionID, slot.ObjectiveIDs, slot.Difficulty = src.QuestionVersionID, src.QuestionID, src.ObjectiveIDs, src.Difficulty
	return slot
}

func fromCandidate(slot paper.Item, c selection.Candidate) paper.Item {
	slot.QuestionVersionID, slot.QuestionID, slot.ObjectiveIDs, slot.Difficulty = c.VersionID, c.QuestionID, c.ObjectiveIDs, c.Difficulty
	return slot
}
