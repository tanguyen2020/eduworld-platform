// Package questionset is the domain of question sets (F024 US1, QUESTION_BANK §6, CONTENT_MODEL §2): fixed items that
// reference immutable question versions and selection rules checked for consistency. Purpose exercise is the
// "Exercise" of FR-CNT-001. Structural rules are the JSON Schema question-set.schema.json; this package holds the
// semantic ones. Pure Go, no I/O.
package questionset

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
)

// Kinds of a set.
const (
	KindStatic    = "static"
	KindRuleBased = "rule_based"
	KindMixed     = "mixed"
)

// MaxRules is the largest number of selection rules (question-set.schema.json).
const MaxRules = 20

// Item is one fixed question of a version.
type Item struct {
	QuestionVersionID uuid.UUID `json:"question_version_id"`
	MaxScore          *float64  `json:"max_score,omitempty"`
	Position          int       `json:"position,omitempty"`
}

// Range is a difficulty range.
type Range struct {
	Min int `json:"min,omitempty"`
	Max int `json:"max,omitempty"`
}

// Bucket is one entry of a difficulty distribution.
type Bucket struct {
	Difficulty int `json:"difficulty"`
	Count      int `json:"count"`
}

// Filters are the filters of a selection rule.
type Filters struct {
	SubjectID          *uuid.UUID  `json:"subject_id,omitempty"`
	GradeLevels        []int       `json:"grade_levels,omitempty"`
	TopicIDs           []uuid.UUID `json:"topic_ids,omitempty"`
	ObjectiveIDs       []uuid.UUID `json:"objective_ids,omitempty"`
	Difficulty         *Range      `json:"difficulty,omitempty"`
	Types              []string    `json:"types,omitempty"`
	TagsAny            []string    `json:"tags_any,omitempty"`
	ExcludeQuestionIDs []uuid.UUID `json:"exclude_question_ids,omitempty"`
	IncludeExamOnly    bool        `json:"include_exam_only,omitempty"`
}

// Rule is a selection rule.
type Rule struct {
	Filters      Filters  `json:"filters"`
	Count        int      `json:"count"`
	Distribution []Bucket `json:"distribution,omitempty"`
}

// Definition is the body of a question set version (QuestionSetAuthoringInput.definition).
type Definition struct {
	Kind           string `json:"kind"`
	Purpose        string `json:"purpose"`
	Items          []Item `json:"items,omitempty"`
	SelectionRules []Rule `json:"selection_rules,omitempty"`
}

// Parse decodes a definition (already checked by the JSON Schema).
func Parse(raw json.RawMessage) (Definition, error) {
	var d Definition
	if err := json.Unmarshal(raw, &d); err != nil {
		return d, err
	}
	return d, nil
}

// Validate applies the semantic rules (F024-AC-01, AC-02, AC-03): the kind has its parts, items are unique, at most
// 20 rules, each rule's distribution adds up to its count and its difficulty range is ordered.
func (d Definition) Validate() contentmodel.Violations {
	var vs contentmodel.Violations
	add := func(f, r string) { vs = append(vs, contentmodel.Violation{Field: f, Reason: r}) }
	switch d.Kind {
	case KindStatic:
		if len(d.Items) == 0 {
			add("/definition/items", "a static set needs items")
		}
	case KindRuleBased:
		if len(d.SelectionRules) == 0 {
			add("/definition/selection_rules", "a rule_based set needs selection_rules")
		}
	case KindMixed:
		if len(d.Items) == 0 || len(d.SelectionRules) == 0 {
			add("/definition", "a mixed set needs items and selection_rules")
		}
	default:
		add("/definition/kind", "must be static, rule_based or mixed")
	}
	seen := map[uuid.UUID]int{}
	for i, it := range d.Items {
		if j, ok := seen[it.QuestionVersionID]; ok {
			add("/definition/items/"+strconv.Itoa(i)+"/question_version_id", fmt.Sprintf("duplicates item %d", j))
		}
		seen[it.QuestionVersionID] = i
	}
	if len(d.SelectionRules) > MaxRules {
		add("/definition/selection_rules", "has more than 20 rules")
	}
	for i, r := range d.SelectionRules {
		ptr := "/definition/selection_rules/" + strconv.Itoa(i)
		if len(r.Distribution) > 0 {
			sum := 0
			for _, b := range r.Distribution {
				sum += b.Count
			}
			if sum != r.Count {
				add(ptr+"/distribution", fmt.Sprintf("adds up to %d, not count %d", sum, r.Count))
			}
		}
		if dr := r.Filters.Difficulty; dr != nil && dr.Min > 0 && dr.Max > 0 && dr.Min > dr.Max {
			add(ptr+"/filters/difficulty", "min is greater than max")
		}
	}
	return vs
}

// QuestionVersionIDs lists the fixed items in order.
func (d Definition) QuestionVersionIDs() []uuid.UUID {
	out := make([]uuid.UUID, len(d.Items))
	for i, it := range d.Items {
		out[i] = it.QuestionVersionID
	}
	return out
}

// Selection maps a rule to the selection rule of the question bank (pool counting and drawing).
func (r Rule) Selection() selection.Rule {
	f := r.Filters
	out := selection.Rule{Count: r.Count, AllowExamOnly: f.IncludeExamOnly, Filters: selection.Filters{SubjectID: f.SubjectID,
		GradeLevels: slices.Clone(f.GradeLevels), TopicIDs: slices.Clone(f.TopicIDs), ObjectiveIDs: slices.Clone(f.ObjectiveIDs),
		Types: slices.Clone(f.Types), TagsAny: slices.Clone(f.TagsAny), ExcludeQuestionIDs: slices.Clone(f.ExcludeQuestionIDs)}}
	if f.Difficulty != nil {
		out.Filters.Difficulty = &selection.Range{Min: f.Difficulty.Min, Max: f.Difficulty.Max}
	}
	for _, b := range r.Distribution {
		out.Distribution = append(out.Distribution, selection.Bucket{Difficulty: b.Difficulty, Count: b.Count})
	}
	return out
}

// PoolStat is the pool of one rule (F024-AC-04): insufficient when available < count, warning when < 3 × count.
type PoolStat struct {
	SectionID    string `json:"section_id,omitempty"`
	RuleIndex    int    `json:"rule_index"`
	Count        int    `json:"count"`
	Required     int    `json:"required"`
	Available    int    `json:"available"`
	Insufficient bool   `json:"insufficient"`
	Warning      bool   `json:"warning"`
}

// Stats counts the pool of every rule of d.
func (d Definition) Stats(pool []selection.Candidate) []PoolStat {
	out := make([]PoolStat, 0, len(d.SelectionRules))
	for i, r := range d.SelectionRules {
		n := selection.Available(pool, r.Selection())
		out = append(out, PoolStat{RuleIndex: i, Count: r.Count, Required: r.Count, Available: n, Insufficient: n < r.Count, Warning: n < 3*r.Count})
	}
	return out
}
