// Package blueprint is the domain of assessment blueprints (F024 US3, EXAM_ENGINE §3, FR-QB-005): the exam matrix of
// sections and rules (LO/topic × difficulty × types × points), its consistency rules and the pool needed per rule.
// Drawing papers and variants belongs to F042. Structural rules are assessment-blueprint.schema.json. Pure Go.
package blueprint

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/questionset"
)

// Variant strategies.
const (
	StrategyFixed           = "fixed"
	StrategyIndependentDraw = "independent_draw"
	StrategyPerStudent      = "per_student"
)

// DefaultMaxMeanPDiff is variant_balance_max_mean_p_diff (CONFIG_DEFAULTS 0.05, owner EXAM_ENGINE §4.1).
const DefaultMaxMeanPDiff = 0.05

// Range is a difficulty range.
type Range struct {
	Min int `json:"min,omitempty"`
	Max int `json:"max,omitempty"`
}

// Rule is one rule of a section.
type Rule struct {
	ObjectiveIDs            []uuid.UUID `json:"objective_ids,omitempty"`
	TopicIDs                []uuid.UUID `json:"topic_ids,omitempty"`
	Difficulty              *Range      `json:"difficulty,omitempty"`
	Types                   []string    `json:"types,omitempty"`
	FixedQuestionVersionIDs []uuid.UUID `json:"fixed_question_version_ids,omitempty"`
	Count                   int         `json:"count"`
	PointsEach              float64     `json:"points_each"`
}

// Section is a section of the paper.
type Section struct {
	ID           string          `json:"id"`
	Title        json.RawMessage `json:"title,omitempty"`
	ShuffleItems *bool           `json:"shuffle_items,omitempty"`
	Rules        []Rule          `json:"rules"`
}

// Balance of the variants.
type Balance struct {
	Enabled      *bool    `json:"enabled,omitempty"`
	MaxMeanPDiff *float64 `json:"max_mean_p_diff,omitempty"`
}

// Variants of the paper.
type Variants struct {
	Strategy string   `json:"strategy"`
	Count    int      `json:"count,omitempty"`
	Balance  *Balance `json:"balance,omitempty"`
}

// ResultRelease of the paper.
type ResultRelease struct {
	Mode               string `json:"mode"`
	FeedbackTiming     string `json:"feedback_timing,omitempty"`
	ShowCorrectAnswers *bool  `json:"show_correct_answers,omitempty"`
	ShowExplanations   *bool  `json:"show_explanations,omitempty"`
}

// AttemptPolicy of the paper.
type AttemptPolicy struct {
	MaxAttempts int    `json:"max_attempts"`
	ScorePolicy string `json:"score_policy"`
}

// Definition is the matrix (the fields of assessment-blueprint.schema.json the rules read).
type Definition struct {
	Sections      []Section      `json:"sections"`
	TotalPoints   float64        `json:"total_points"`
	Variants      *Variants      `json:"variants,omitempty"`
	ResultRelease *ResultRelease `json:"result_release,omitempty"`
	DurationSec   int            `json:"duration_sec,omitempty"`
	AttemptPolicy *AttemptPolicy `json:"attempt_policy,omitempty"`
}

// Parse decodes a definition.
func Parse(raw json.RawMessage) (Definition, error) {
	var d Definition
	err := json.Unmarshal(raw, &d)
	return d, err
}

// Validate applies EXAM_ENGINE §3 (F024-AC-10): Σ count × points_each = total_points (±0.001), feedback timing not
// immediate, fixed questions as many as count and never twice across rules, ordered difficulty ranges.
func (d Definition) Validate(feedbackTiming string) contentmodel.Violations {
	var vs contentmodel.Violations
	add := func(f, r string) { vs = append(vs, contentmodel.Violation{Field: f, Reason: r}) }
	sum := 0.0
	fixed := map[uuid.UUID]string{}
	for si, s := range d.Sections {
		for ri, r := range s.Rules {
			ptr := "/definition/sections/" + strconv.Itoa(si) + "/rules/" + strconv.Itoa(ri)
			sum += float64(r.Count) * r.PointsEach
			if len(r.FixedQuestionVersionIDs) > 0 && len(r.FixedQuestionVersionIDs) != r.Count {
				add(ptr+"/fixed_question_version_ids", fmt.Sprintf("has %d questions, count is %d", len(r.FixedQuestionVersionIDs), r.Count))
			}
			for _, id := range r.FixedQuestionVersionIDs {
				if prev, ok := fixed[id]; ok {
					add(ptr+"/fixed_question_version_ids", "repeats a question of "+prev)
					break
				}
				fixed[id] = ptr
			}
			if dr := r.Difficulty; dr != nil && dr.Min > 0 && dr.Max > 0 && dr.Min > dr.Max {
				add(ptr+"/difficulty", "min is greater than max")
			}
		}
	}
	if math.Abs(sum-d.TotalPoints) > 0.001 {
		add("/definition/total_points", fmt.Sprintf("is %.2f but the rules give %.2f (count × points_each)", d.TotalPoints, sum))
	}
	ft := feedbackTiming
	if d.ResultRelease != nil && d.ResultRelease.FeedbackTiming != "" {
		ft = d.ResultRelease.FeedbackTiming
	}
	if ft == "immediate" {
		add("/feedback_timing", "an exam cannot show feedback immediately (EXAM_ENGINE §3)")
	}
	return vs
}

// References lists the LO and topic ids of the rules (checked against the curriculum version by the use case).
func (d Definition) References() (objectives, topics []uuid.UUID) {
	for _, s := range d.Sections {
		for _, r := range s.Rules {
			objectives = append(objectives, r.ObjectiveIDs...)
			topics = append(topics, r.TopicIDs...)
		}
	}
	return objectives, topics
}

// FixedQuestions lists every fixed question version.
func (d Definition) FixedQuestions() []uuid.UUID {
	var out []uuid.UUID
	for _, s := range d.Sections {
		for _, r := range s.Rules {
			out = append(out, r.FixedQuestionVersionIDs...)
		}
	}
	return out
}

// WithDefaults fills variants.balance.max_mean_p_diff with the configured default when absent (F024-AC-12) and
// returns the raw definition to store.
func WithDefaults(raw json.RawMessage, maxMeanPDiff float64) (json.RawMessage, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw, err
	}
	v, _ := m["variants"].(map[string]any)
	if v == nil {
		v = map[string]any{"strategy": StrategyFixed, "count": 1}
		m["variants"] = v
	}
	b, _ := v["balance"].(map[string]any)
	if b == nil {
		b = map[string]any{}
		v["balance"] = b
	}
	if _, ok := b["max_mean_p_diff"]; !ok {
		b["max_mean_p_diff"] = maxMeanPDiff
	}
	return json.Marshal(m)
}

// Stats counts the pool of every rule (F024-AC-11): required = count × variants.count with independent_draw, count
// otherwise; a fixed rule counts its fixed questions that are in the pool.
func (d Definition) Stats(pool []selection.Candidate) []questionset.PoolStat {
	mult := 1
	if d.Variants != nil && d.Variants.Strategy == StrategyIndependentDraw && d.Variants.Count > 1 {
		mult = d.Variants.Count
	}
	inPool := map[uuid.UUID]bool{}
	for _, c := range pool {
		inPool[c.VersionID] = true
	}
	var out []questionset.PoolStat
	for _, s := range d.Sections {
		for ri, r := range s.Rules {
			st := questionset.PoolStat{SectionID: s.ID, RuleIndex: ri, Count: r.Count, Required: r.Count * mult}
			if len(r.FixedQuestionVersionIDs) > 0 {
				st.Required = r.Count
				for _, id := range r.FixedQuestionVersionIDs {
					if inPool[id] {
						st.Available++
					}
				}
			} else {
				sel := selection.Rule{Count: r.Count, AllowExamOnly: true, Filters: selection.Filters{TopicIDs: r.TopicIDs, ObjectiveIDs: r.ObjectiveIDs,
					Types: r.Types}}
				if r.Difficulty != nil {
					sel.Filters.Difficulty = &selection.Range{Min: r.Difficulty.Min, Max: r.Difficulty.Max}
				}
				st.Available = selection.Available(pool, sel)
			}
			st.Insufficient = st.Available < st.Required
			out = append(out, st)
		}
	}
	return out
}
