// Package paper generates the immutable papers of an exam session (F042, EXAM_ENGINE §4, FR-QB-005): each rule of the
// blueprint draws its questions from the published pool (objective × topic × difficulty × type, exactly), the same
// questions for the same seed; independent_draw draws every variant with its own seed; per_student draws at start with
// the seed of the session, the student and the attempt. Balancing the variants is the extension point VariantBalancer
// (F044); the default keeps the draw. Pure Go.
package paper

import (
	"errors"
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/blueprint"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
)

// GeneratorVersion is stored with every paper (data-model §1) so a seed replays with the same algorithm.
const GeneratorVersion = "f042-v1"

// ErrPoolInsufficient: a rule cannot be filled from the pool (422 QUESTION_POOL_INSUFFICIENT).
var ErrPoolInsufficient = errors.New("paper: pool insufficient")

// PoolError names the rule that cannot be filled.
type PoolError struct {
	Section   int
	Rule      int
	Required  int
	Available int
}

func (e *PoolError) Error() string {
	return fmt.Sprintf("paper: section %d rule %d needs %d questions, the pool has %d", e.Section, e.Rule, e.Required, e.Available)
}

func (e *PoolError) Unwrap() error { return ErrPoolInsufficient }

// Pointer is the JSON pointer of the rule in the blueprint definition.
func (e *PoolError) Pointer() string {
	return fmt.Sprintf("/definition/sections/%d/rules/%d", e.Section, e.Rule)
}

// Item is one question of a paper (exam_papers.items[]).
type Item struct {
	SectionID         string      `json:"section_id"`
	Position          int         `json:"position"`
	QuestionVersionID uuid.UUID   `json:"question_version_id"`
	QuestionID        uuid.UUID   `json:"question_id"`
	Points            float64     `json:"points"`
	ObjectiveIDs      []uuid.UUID `json:"objective_ids,omitempty"`
	Difficulty        int         `json:"difficulty,omitempty"`
}

// Paper is one variant.
type Paper struct {
	VariantNo   int
	Seed        int64
	Items       []Item
	TotalPoints float64
	ExpectedP   *float64
}

// VariantBalancer balances the difficulty of the variants of independent_draw (EXAM_ENGINE §4.1, F044); it may swap
// questions of the same rule and sets ExpectedP. The papers it returns are stored as they are.
type VariantBalancer interface {
	Balance(def blueprint.Definition, pool []selection.Candidate, papers []Paper) []Paper
}

// NoopBalancer keeps the draw (F042: no balancing, F044 replaces it).
type NoopBalancer struct{}

// Balance returns the papers unchanged.
func (NoopBalancer) Balance(_ blueprint.Definition, _ []selection.Candidate, papers []Paper) []Paper {
	return papers
}

// SessionPapers generates the papers of a session at creation: one with fixed, Count with independent_draw (each with
// its own seed from the session seed), none with per_student (drawn at start).
func SessionPapers(def blueprint.Definition, pool []selection.Candidate, strategy string, count int, seed int64, official bool,
	b VariantBalancer) ([]Paper, error) {
	if strategy == blueprint.StrategyPerStudent {
		return nil, nil
	}
	if strategy != blueprint.StrategyIndependentDraw || count < 1 {
		count = 1
	}
	out := make([]Paper, 0, count)
	for v := 1; v <= count; v++ {
		s := VariantSeed(seed, v)
		p, err := Generate(def, pool, s, official)
		if err != nil {
			return nil, err
		}
		p.VariantNo = v
		out = append(out, p)
	}
	if b == nil {
		b = NoopBalancer{}
	}
	return b.Balance(def, pool, out), nil
}

// VariantSeed derives the seed of variant v from the session seed.
func VariantSeed(seed int64, v int) int64 {
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "%d:%d", seed, v)
	return int64(h.Sum64() & (1<<62 - 1))
}

// matches applies a rule exactly (no relaxation: an exam follows its matrix).
func matches(c selection.Candidate, r blueprint.Rule) bool {
	if len(r.ObjectiveIDs) > 0 && !overlaps(c.ObjectiveIDs, r.ObjectiveIDs) {
		return false
	}
	if len(r.TopicIDs) > 0 && !slices.Contains(r.TopicIDs, c.TopicID) {
		return false
	}
	if d := r.Difficulty; d != nil {
		if (d.Min > 0 && c.Difficulty < d.Min) || (d.Max > 0 && c.Difficulty > d.Max) {
			return false
		}
	}
	if len(r.Types) > 0 && !slices.Contains(r.Types, c.Type) {
		return false
	}
	return true
}

func overlaps(a, b []uuid.UUID) bool {
	for _, x := range a {
		if slices.Contains(b, x) {
			return true
		}
	}
	return false
}

// Generate draws one paper: rules in order, positions across sections, a question never twice in a paper; exam_only
// questions first for an official exam (EXAM_ENGINE §4).
func Generate(def blueprint.Definition, pool []selection.Candidate, seed int64, official bool) (Paper, error) {
	sorted := slices.Clone(pool)
	slices.SortFunc(sorted, func(a, b selection.Candidate) int { return strings.Compare(a.VersionID.String(), b.VersionID.String()) })
	byVersion := map[uuid.UUID]selection.Candidate{}
	for _, c := range sorted {
		byVersion[c.VersionID] = c
	}
	used := map[uuid.UUID]bool{}
	p := Paper{Seed: seed}
	pos := 0
	for si, s := range def.Sections {
		for ri, r := range s.Rules {
			var picked []selection.Candidate
			if len(r.FixedQuestionVersionIDs) > 0 {
				for _, id := range r.FixedQuestionVersionIDs {
					c, ok := byVersion[id]
					if !ok || used[c.QuestionID] {
						return Paper{}, &PoolError{Section: si, Rule: ri, Required: r.Count, Available: len(picked)}
					}
					picked = append(picked, c)
					used[c.QuestionID] = true
				}
			} else {
				var cands []selection.Candidate
				for _, c := range sorted {
					if !used[c.QuestionID] && matches(c, r) {
						cands = append(cands, c)
					}
				}
				h := fnv.New64a()
				_, _ = fmt.Fprintf(h, "rule:%d:%d", si, ri)
				rng := rand.New(rand.NewPCG(uint64(seed), h.Sum64())) //nolint:gosec // reproducible draw, not a secret
				rng.Shuffle(len(cands), func(i, j int) { cands[i], cands[j] = cands[j], cands[i] })
				if official {
					slices.SortStableFunc(cands, func(a, b selection.Candidate) int {
						switch {
						case a.ExamOnly == b.ExamOnly:
							return 0
						case a.ExamOnly:
							return -1
						}
						return 1
					})
				}
				for _, c := range cands {
					if len(picked) == r.Count {
						break
					}
					if !used[c.QuestionID] {
						picked = append(picked, c)
						used[c.QuestionID] = true
					}
				}
				if len(picked) < r.Count {
					return Paper{}, &PoolError{Section: si, Rule: ri, Required: r.Count, Available: len(picked)}
				}
			}
			for _, c := range picked {
				pos++
				p.Items = append(p.Items, Item{SectionID: s.ID, Position: pos, QuestionVersionID: c.VersionID, QuestionID: c.QuestionID,
					Points: r.PointsEach, ObjectiveIDs: c.ObjectiveIDs, Difficulty: c.Difficulty})
				p.TotalPoints += r.PointsEach
			}
		}
	}
	return p, nil
}

// Order is the order of the items of a paper for one attempt (EXAM_ENGINE §4): sections keep their order; the items of a
// section are shuffled by the attempt seed unless the section says shuffle_items false. Positions are renumbered.
func Order(def blueprint.Definition, items []Item, seed int64) []Item {
	shuffle := map[string]bool{}
	for _, s := range def.Sections {
		shuffle[s.ID] = s.ShuffleItems == nil || *s.ShuffleItems
	}
	out := make([]Item, 0, len(items))
	for start := 0; start < len(items); {
		end := start
		for end < len(items) && items[end].SectionID == items[start].SectionID {
			end++
		}
		part := slices.Clone(items[start:end])
		if shuffle[items[start].SectionID] {
			h := fnv.New64a()
			_, _ = h.Write([]byte(items[start].SectionID))
			rng := rand.New(rand.NewPCG(uint64(seed), h.Sum64())) //nolint:gosec // reproducible order, not a secret
			rng.Shuffle(len(part), func(i, j int) { part[i], part[j] = part[j], part[i] })
		}
		out = append(out, part...)
		start = end
	}
	for i := range out {
		out[i].Position = i + 1
	}
	return out
}
