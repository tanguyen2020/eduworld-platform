// Package lesson is the domain of lessons and learning materials (F024 US2, CONTENT_MODEL §9, §9.2): their content
// blocks (rich text, media, worked examples, activities once F030 registers their validator) and the worked example
// value object shared by both types and referenced by questions (worked_example_ref). Structural rules are the JSON
// Schema lesson.schema.json; this package holds the semantic ones. Pure Go, no I/O.
package lesson

import (
	"encoding/json"
	"regexp"
	"slices"
	"strconv"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// Block kinds.
const (
	KindRichText      = "rich_text"
	KindMedia         = "media"
	KindWorkedExample = "worked_example"
	KindActivity      = "activity"
)

// Limits of a version (lesson.schema.json, CONTENT_MODEL §9.2).
const (
	MaxBlocks = 100
	MaxSteps  = 8
)

// SubBlockKindNotEnabled is errors[].code of a block kind without validator (activity before F030).
const SubBlockKindNotEnabled = "BLOCK_KIND_NOT_ENABLED"

var workedExampleID = regexp.MustCompile(`^we_[a-z0-9_]{1,40}$`)

// Step is one step of a worked example.
type Step struct {
	Content  json.RawMessage `json:"content"`
	AudioRef *uuid.UUID      `json:"audio_ref,omitempty"`
}

// Block is one content block.
type Block struct {
	Kind              string          `json:"kind"`
	ID                string          `json:"id,omitempty"`
	Content           json.RawMessage `json:"content,omitempty"`
	MediaRef          *uuid.UUID      `json:"media_ref,omitempty"`
	Problem           json.RawMessage `json:"problem,omitempty"`
	Steps             []Step          `json:"steps,omitempty"`
	ObjectiveIDs      []uuid.UUID     `json:"objective_ids,omitempty"`
	SelfExplainPrompt json.RawMessage `json:"self_explain_prompt,omitempty"`
	ActivityID        *uuid.UUID      `json:"activity_id,omitempty"`
}

// WorkedExample is the summary of a worked example block (id and its objectives).
type WorkedExample struct {
	ID           string
	ObjectiveIDs []uuid.UUID
}

// Options of the validation.
type Options struct {
	ActivityEnabled bool // F030 registered the activity block validator
}

// Result of a valid list of blocks.
type Result struct {
	WorkedExamples []WorkedExample
	MediaRefs      []uuid.UUID
}

// WorkedExampleIDs lists the ids of r.
func (r Result) WorkedExampleIDs() []string {
	out := make([]string, len(r.WorkedExamples))
	for i, w := range r.WorkedExamples {
		out[i] = w.ID
	}
	return out
}

// nestedForbidden are rich content block types that cannot appear in a worked example (no question or activity
// inside, F024-AC-07).
var nestedForbidden = []string{"question", "activity", "quiz", "question_set"}

// Validate applies the block rules (F024-AC-06, AC-07): 1–100 blocks of an allowed kind (activity only once enabled,
// BLOCK_KIND_NOT_ENABLED otherwise), worked examples with a unique we_ id, a problem, 1–8 steps, objectives and no
// nested question or activity.
func Validate(blocks []Block, o Options) (Result, contentmodel.Violations) {
	var vs contentmodel.Violations
	var res Result
	add := func(f, r string) { vs = append(vs, contentmodel.Violation{Field: f, Reason: r}) }
	if len(blocks) == 0 || len(blocks) > MaxBlocks {
		add("/blocks", "must have 1–100 blocks")
	}
	ids := map[string]int{}
	for i, b := range blocks {
		ptr := "/blocks/" + strconv.Itoa(i)
		switch b.Kind {
		case KindRichText:
			if isNull(b.Content) {
				add(ptr+"/content", "a rich_text block needs content")
			}
		case KindMedia:
			if b.MediaRef == nil {
				add(ptr+"/media_ref", "a media block needs media_ref")
			} else {
				res.MediaRefs = append(res.MediaRefs, *b.MediaRef)
			}
		case KindWorkedExample:
			if !workedExampleID.MatchString(b.ID) {
				add(ptr+"/id", "must match ^we_[a-z0-9_]{1,40}$")
			} else if j, ok := ids[b.ID]; ok {
				add(ptr+"/id", "duplicates block "+strconv.Itoa(j))
			}
			if isNull(b.Problem) {
				add(ptr+"/problem", "a worked example needs a problem")
			}
			if len(b.Steps) == 0 || len(b.Steps) > MaxSteps {
				add(ptr+"/steps", "must have 1–8 steps")
			}
			if len(b.ObjectiveIDs) == 0 {
				add(ptr+"/objective_ids", "a worked example needs objectives")
			}
			docs := []json.RawMessage{b.Problem, b.SelfExplainPrompt}
			for si, st := range b.Steps {
				if isNull(st.Content) {
					add(ptr+"/steps/"+strconv.Itoa(si)+"/content", "a step needs content")
				}
				docs = append(docs, st.Content)
				if st.AudioRef != nil {
					res.MediaRefs = append(res.MediaRefs, *st.AudioRef)
				}
			}
			for _, d := range docs {
				if nested(d) {
					add(ptr, "a worked example cannot contain a question or an activity")
					break
				}
			}
			res.WorkedExamples = append(res.WorkedExamples, WorkedExample{ID: b.ID, ObjectiveIDs: slices.Clone(b.ObjectiveIDs)})
		case KindActivity:
			if !o.ActivityEnabled {
				vs = append(vs, contentmodel.Violation{Field: ptr + "/kind", Reason: SubBlockKindNotEnabled + ": activity blocks come with F030"})
			}
		default:
			add(ptr+"/kind", "must be rich_text, media, worked_example or activity")
		}
		if b.ID != "" {
			if _, ok := ids[b.ID]; !ok {
				ids[b.ID] = i
			}
		}
	}
	return res, vs
}

func isNull(r json.RawMessage) bool { return len(r) == 0 || string(r) == "null" }

// nested reports a question or activity block inside a rich content.
func nested(doc json.RawMessage) bool {
	if isNull(doc) {
		return false
	}
	var v any
	if json.Unmarshal(doc, &v) != nil {
		return false
	}
	var rec func(any) bool
	rec = func(x any) bool {
		switch t := x.(type) {
		case map[string]any:
			if s, ok := t["type"].(string); ok && slices.Contains(nestedForbidden, s) {
				return true
			}
			if s, ok := t["kind"].(string); ok && slices.Contains(nestedForbidden, s) {
				return true
			}
			for k, val := range t {
				if k == "question_version_id" || k == "activity_id" || rec(val) {
					return true
				}
			}
		case []any:
			for _, val := range t {
				if rec(val) {
					return true
				}
			}
		}
		return false
	}
	return rec(v)
}

// Find returns the worked example block_id of blocks.
func Find(blocks []Block, blockID string) (WorkedExample, bool) {
	for _, b := range blocks {
		if b.Kind == KindWorkedExample && b.ID == blockID {
			return WorkedExample{ID: b.ID, ObjectiveIDs: slices.Clone(b.ObjectiveIDs)}, true
		}
	}
	return WorkedExample{}, false
}

// SharesObjective reports a common objective (F024-AC-08: a question references a worked example of the same LO).
func SharesObjective(a, b []uuid.UUID) bool {
	for _, x := range a {
		if slices.Contains(b, x) {
			return true
		}
	}
	return false
}
