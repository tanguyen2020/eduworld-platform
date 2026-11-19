// Package question is the domain of the question bank (F022, QUESTION_BANK §2–§8): question types, the semantic
// validation of authoring data, the server-side scorers with partial credit, the render DTO built from a
// white-list of fields with a reproducible shuffle, and the separation of the server-only parts (option feedback,
// misconception tags) from the render payload. A registry of TypeHandler lets F023 add types without touching this
// code. Pure Go, no I/O.
package question

import (
	"encoding/json"
	"errors"
	"regexp"
	"slices"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// Type is a QuestionType (specs/schemas/common/enums.schema.json#/$defs/QuestionType).
type Type string

// The eleven question types (QUESTION_BANK §3).
const (
	SingleChoice   Type = "single_choice"
	MultipleChoice Type = "multiple_choice"
	TrueFalse      Type = "true_false"
	FillInBlank    Type = "fill_in_blank"
	ShortAnswer    Type = "short_answer"
	Numeric        Type = "numeric"
	Matching       Type = "matching"
	Ordering       Type = "ordering"
	DragDrop       Type = "drag_drop"
	ImageBased     Type = "image_based"
	Interactive    Type = "interactive"
)

var allTypes = []Type{SingleChoice, MultipleChoice, TrueFalse, FillInBlank, ShortAnswer, Numeric, Matching, Ordering, DragDrop,
	ImageBased, Interactive}

// Known reports one of the eleven types.
func (t Type) Known() bool { return slices.Contains(allTypes, t) }

// Errors of the question domain.
var (
	// ErrInvalidResponse: the response does not match $defs.<type>Response (422 INVALID_RESPONSE, F022-AC-19).
	ErrInvalidResponse = errors.New("question: invalid response")
	// ErrTypeNotEnabled: no handler is registered for the type yet (F022-AC-02: short_answer, image_based,
	// interactive come with F023).
	ErrTypeNotEnabled = errors.New("question: type not enabled")
)

// SubTypeNotEnabled is errors[].code of a type without handler.
const SubTypeNotEnabled = "TYPE_NOT_ENABLED"

// Scoring modes and partial methods (QUESTION_BANK §3).
const (
	ModeAllOrNothing     = "all_or_nothing"
	ModePartial          = "partial"
	PartialRightMinusWrg = "right_minus_wrong"
	PartialProportional  = "proportional"
	PartialPosition      = "position"
	PartialAdjacentPairs = "adjacent_pairs"
)

// ScoringRule is the scoring_rule of a version.
type ScoringRule struct {
	Mode             string   `json:"mode"`
	PartialMethod    string   `json:"partial_method,omitempty"`
	MinScore         *float64 `json:"min_score,omitempty"`
	RequireActionLog bool     `json:"require_action_log,omitempty"`
}

// MathAsset is a rendered formula (ADR-0055, question.schema.json#/$defs/mathDisplay).
type MathAsset struct {
	Latex      string `json:"latex"`
	SVGAsset   string `json:"svg_asset"`
	SpeechText string `json:"speech_text"`
}

// OptionFeedback is the server-only part of an option (QUESTION_BANK §2): feedback and misconception tag.
type OptionFeedback struct {
	Feedback         json.RawMessage `json:"feedback,omitempty"`
	MisconceptionTag string          `json:"misconception_tag,omitempty"`
}

// Content is what the scorers and renderers read of a version: the render payload is the stored one (server-only
// parts already removed).
type Content struct {
	VersionID  uuid.UUID
	Type       Type
	Stem       json.RawMessage
	Render     json.RawMessage
	AnswerKey  json.RawMessage
	Scoring    ScoringRule
	MaxScore   float64
	Language   string
	MathAssets []MathAsset
}

// Authoring is the authoring form of a version (question.schema.json): render payload with option feedback.
type Authoring struct {
	Type          Type
	Stem          json.RawMessage
	RenderPayload json.RawMessage
	AnswerKey     json.RawMessage
	Scoring       ScoringRule
	MaxScore      float64
	Explanation   json.RawMessage
	Hints         []json.RawMessage
	MathAssets    []MathAsset
	MinResponseMS *int
	Language      string
}

// Part is the result of one part of a response (option, statement, blank, pair, item, draggable).
type Part struct {
	ID      string `json:"id"`
	Correct bool   `json:"correct"`
}

// GradeResult is the server-side grade of one response (F022-AC-14).
type GradeResult struct {
	Score             float64  `json:"score"`
	MaxScore          float64  `json:"max_score"`
	IsCorrect         bool     `json:"is_correct"`
	Parts             []Part   `json:"parts,omitempty"`
	SelectedOptionIDs []string `json:"selected_option_ids,omitempty"`
}

// Presentation is how the server showed a question in one attempt (attempt_items.presentation): the seed and the
// order of every shuffled list.
type Presentation struct {
	QuestionVersionID uuid.UUID           `json:"question_version_id"`
	Seed              uint64              `json:"seed"`
	Orders            map[string][]string `json:"orders,omitempty"`
}

// Rendered is the render DTO of a question for a student (F022-AC-22): no answer, no feedback, no hints.
type Rendered struct {
	QuestionVersionID uuid.UUID      `json:"question_version_id"`
	Type              Type           `json:"type"`
	Stem              any            `json:"stem"`
	Payload           map[string]any `json:"render_payload"`
	MaxScore          float64        `json:"max_score"`
}

var itemIDRe = regexp.MustCompile(`^[a-z]_[a-z0-9_]{1,24}$`)

// ValidItemID reports an itemId (QUESTION_BANK §2: short, stable, not A–D nor an index).
func ValidItemID(id string) bool { return itemIDRe.MatchString(id) }

// Violations is the list of invalid fields of a validation (JSON pointers).
type Violations = contentmodel.Violations

type violations struct{ v contentmodel.Violations }

func (b *violations) add(ptr, reason string) {
	b.v = append(b.v, contentmodel.Violation{Field: ptr, Reason: reason})
}

func (b *violations) err() error {
	if len(b.v) == 0 {
		return nil
	}
	return b.v
}
