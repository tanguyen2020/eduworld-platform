// Package selection picks questions from a pool by rule (QUESTION_BANK §6, F022-AC-26, AC-27): filters, count,
// difficulty distribution, exclusion list, reproducible by seed, relaxation (avoid_recent → difficulty ±1 → types)
// and QUESTION_POOL_INSUFFICIENT. The pool holds only current releases visible to the tenant (built by the caller).
package selection

import (
	"errors"
	"hash/fnv"
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/google/uuid"
)

// ErrPoolInsufficient: even relaxed, the pool cannot satisfy the rule (QUESTION_POOL_INSUFFICIENT).
var ErrPoolInsufficient = errors.New("selection: question pool insufficient")

// Candidate is one current published version of the pool.
type Candidate struct {
	QuestionID   uuid.UUID
	VersionID    uuid.UUID
	Type         string
	Difficulty   int
	SubjectID    uuid.UUID
	TopicID      uuid.UUID
	GradeLevels  []int
	ObjectiveIDs []uuid.UUID
	Tags         []string
	ExamOnly     bool
}

// Range is a difficulty range (inclusive, 0 = open).
type Range struct{ Min, Max int }

// Filters of a rule (question-set.schema.json#/$defs/selectionRule.filters).
type Filters struct {
	SubjectID          *uuid.UUID
	GradeLevels        []int
	TopicIDs           []uuid.UUID
	ObjectiveIDs       []uuid.UUID
	Difficulty         *Range
	Types              []string
	TagsAny            []string
	ExcludeQuestionIDs []uuid.UUID
}

// Bucket is one entry of the difficulty distribution.
type Bucket struct {
	Difficulty int
	Count      int
}

// Rule is a SelectionRule.
type Rule struct {
	Filters       Filters
	Count         int
	Distribution  []Bucket
	AllowExamOnly bool // exams only (exam_only questions never reach practice, mini-games or battles)
}

// Request is one selection: the rule, the seed of the attempt, the questions seen recently (computed by the caller
// from attempt history, F030), avoided first.
type Request struct {
	Rule   Rule
	Seed   uint64
	Recent []uuid.UUID
	// Strict keeps the difficulty band and the question types (challenge of a difficulty level, F076-AC-09): only the
	// recently seen questions may be relaxed, never the band (±1) nor the types.
	Strict bool
}

// Relaxation levels (QUESTION_BANK §6 step 4).
const (
	strict = iota
	allowRecent
	widenDifficulty
	anyType
)

// Select returns Count candidates (distribution respected), the same for the same pool, rule and seed.
func Select(pool []Candidate, req Request) ([]Candidate, error) {
	if req.Rule.Count <= 0 {
		return nil, nil
	}
	sorted := slices.Clone(pool)
	slices.SortFunc(sorted, func(a, b Candidate) int { return strings.Compare(a.VersionID.String(), b.VersionID.String()) })
	last := anyType
	if req.Strict {
		last = allowRecent
	}
	for level := strict; level <= last; level++ {
		if out, ok := attempt(sorted, req, level); ok {
			return out, nil
		}
	}
	return nil, ErrPoolInsufficient
}

func attempt(pool []Candidate, req Request, level int) ([]Candidate, bool) {
	cands := make([]Candidate, 0, len(pool))
	for _, c := range pool {
		if match(c, req, level) {
			cands = append(cands, c)
		}
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte("qb-select"))
	rng := rand.New(rand.NewPCG(req.Seed, h.Sum64())) //nolint:gosec // reproducible selection, not a secret
	perm := rng.Perm(len(cands))
	shuffled := make([]Candidate, len(cands))
	for i, p := range perm {
		shuffled[i] = cands[p]
	}
	taken := map[uuid.UUID]bool{}
	var out []Candidate
	pick := func(n int, ok func(Candidate) bool) bool {
		for _, c := range shuffled {
			if n == 0 {
				break
			}
			if !taken[c.QuestionID] && ok(c) {
				taken[c.QuestionID] = true
				out = append(out, c)
				n--
			}
		}
		return n == 0
	}
	if len(req.Rule.Distribution) > 0 {
		for _, b := range req.Rule.Distribution {
			d := b.Difficulty
			if !pick(b.Count, func(c Candidate) bool { return c.Difficulty == d }) {
				if level < widenDifficulty || !pick(b.Count-countDiff(out, d), func(c Candidate) bool { return abs(c.Difficulty-d) <= 1 }) {
					return nil, false
				}
			}
		}
		rest := req.Rule.Count - len(out)
		if rest > 0 && !pick(rest, func(Candidate) bool { return true }) {
			return nil, false
		}
		return out, len(out) == req.Rule.Count
	}
	return out, pick(req.Rule.Count, func(Candidate) bool { return true })
}

func countDiff(cs []Candidate, d int) int {
	n := 0
	for _, c := range cs {
		if c.Difficulty == d {
			n++
		}
	}
	return n
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func match(c Candidate, req Request, level int) bool {
	f := req.Rule.Filters
	switch {
	case c.ExamOnly && !req.Rule.AllowExamOnly:
		return false
	case slices.Contains(f.ExcludeQuestionIDs, c.QuestionID):
		return false
	case level < allowRecent && slices.Contains(req.Recent, c.QuestionID):
		return false
	case f.SubjectID != nil && c.SubjectID != *f.SubjectID:
		return false
	case len(f.GradeLevels) > 0 && !overlaps(f.GradeLevels, c.GradeLevels):
		return false
	case len(f.TopicIDs) > 0 && !slices.Contains(f.TopicIDs, c.TopicID):
		return false
	case len(f.ObjectiveIDs) > 0 && !overlapsID(f.ObjectiveIDs, c.ObjectiveIDs):
		return false
	case len(f.TagsAny) > 0 && !overlapsStr(f.TagsAny, c.Tags):
		return false
	case level < anyType && len(f.Types) > 0 && !slices.Contains(f.Types, c.Type):
		return false
	}
	if r := f.Difficulty; r != nil {
		lo, hi := r.Min, r.Max
		if level >= widenDifficulty {
			lo, hi = lo-1, hi+1
		}
		if (r.Min > 0 && c.Difficulty < lo) || (r.Max > 0 && c.Difficulty > hi) {
			return false
		}
	}
	return true
}

func overlaps(a, b []int) bool {
	for _, x := range a {
		if slices.Contains(b, x) {
			return true
		}
	}
	return false
}

func overlapsID(a, b []uuid.UUID) bool {
	for _, x := range a {
		if slices.Contains(b, x) {
			return true
		}
	}
	return false
}

func overlapsStr(a, b []string) bool {
	for _, x := range a {
		if slices.Contains(b, x) {
			return true
		}
	}
	return false
}

// Available counts the candidates that match a rule without relaxation (the pool size of a rule, F024-AC-04, AC-11).
func Available(pool []Candidate, r Rule) int {
	req := Request{Rule: r}
	n := 0
	for _, c := range pool {
		if match(c, req, strict) {
			n++
		}
	}
	return n
}
