// Package minigame is the plugin of activity_type minigame (F073, MINIGAME_ENGINE, ADR-0048): a presentation layer over
// question bank questions played with a published MiniGameConfigVersion of module game (pinned from the projection
// ref.minigame_config_versions). Each answer is graded by the question bank scorer; the plugin adds the game score
// computed from server times and the correct streak (MinigameScorer), the learning score of result_calculation and the
// completion rule of the configuration. The client receives the display part of the configuration only: never the
// question source, an answer, a scoring rule or an option feedback (F073-AC-07). Pure domain: no I/O.
package minigame

import "slices"

// Mini-game types of minigame-config.schema.json (MiniGameType of the public API).
const (
	MultipleChoice    = "multiple_choice"
	TrueFalse         = "true_false"
	FillInBlank       = "fill_in_blank"
	DragAndDrop       = "drag_and_drop"
	Matching          = "matching"
	Ordering          = "ordering"
	Calculation       = "calculation"
	Puzzle            = "puzzle"
	WordGame          = "word_game"
	ScienceSimulation = "science_simulation"
	TimedChallenge    = "timed_challenge"
)

// Types are the 11 mini-game types of the release (FR-MG-001).
var Types = []string{MultipleChoice, TrueFalse, FillInBlank, DragAndDrop, Matching, Ordering, Calculation, Puzzle, WordGame,
	ScienceSimulation, TimedChallenge}

// AutoGraded are the question types graded automatically by the question bank (QuestionType of the OpenAPI: every type
// of the release is auto-graded, ADR-0072).
var AutoGraded = []string{"single_choice", "multiple_choice", "true_false", "fill_in_blank", "short_answer", "numeric", "matching",
	"ordering", "drag_drop", "image_based", "interactive"}

// compat is the table of MINIGAME_ENGINE §2 (question types accepted by each mini-game type), with the question type names
// of the OpenAPI QuestionType. It is the only backend copy of the table: module game asks module learning through the
// facade operation validateMinigameQuestionSource (F073 Clarifications).
var compat = map[string][]string{
	MultipleChoice:    {"single_choice", "multiple_choice", "image_based"},
	TrueFalse:         {"true_false"},
	FillInBlank:       {"fill_in_blank", "short_answer"},
	DragAndDrop:       {"drag_drop", "image_based"},
	Matching:          {"matching"},
	Ordering:          {"ordering"},
	Calculation:       {"numeric"},
	Puzzle:            AutoGraded,
	WordGame:          {"fill_in_blank", "short_answer", "ordering"},
	ScienceSimulation: {"interactive"},
	TimedChallenge:    {"single_choice", "true_false", "numeric"},
}

// Valid reports a mini-game type of the release.
func Valid(minigameType string) bool { return slices.Contains(Types, minigameType) }

// Allowed returns the question types a mini-game type accepts (nil for an unknown type).
func Allowed(minigameType string) []string {
	ts, ok := compat[minigameType]
	if !ok {
		return nil
	}
	return slices.Clone(ts)
}

// Accepts reports whether a mini-game type accepts a question type.
func Accepts(minigameType, questionType string) bool {
	return slices.Contains(compat[minigameType], questionType)
}

// AcceptsEveryType reports a mini-game type that accepts every auto-graded question type (puzzle): a source without a
// question type filter is then compatible.
func AcceptsEveryType(minigameType string) bool {
	for _, t := range AutoGraded {
		if !Accepts(minigameType, t) {
			return false
		}
	}
	return Valid(minigameType)
}

// Reason codes of a question source check (MinigameQuestionSourceResult.reasons[].code).
const (
	ReasonIncompatibleType  = "incompatible_question_type"
	ReasonUnboundedTypes    = "unbounded_question_types"
	ReasonInsufficient      = "insufficient_questions"
	ReasonSetNotPublished   = "question_set_not_published"
	maxReasonMessageLength  = 500
	maxReportedQuestionType = 11
)

// SourceReason is one reason a question source is refused.
type SourceReason struct {
	Code    string
	Message string
}

// SourceCheck is what a question source can return: the question types it may deliver (Unbounded when a selection has
// no type filter), the number of questions available and whether its question set is published.
type SourceCheck struct {
	Types         []string
	Unbounded     bool
	Available     int
	SetNotFound   bool
	QuestionCount int
}

// Check applies the publish rules of F073-AC-02 to a question source of minigameType: every type it may return is in the
// compatibility table, a source without type filter only for a type that accepts every auto-graded type, enough
// questions for question_count and a published question set. It returns the reasons (none = compatible).
func Check(minigameType string, s SourceCheck) []SourceReason {
	var out []SourceReason
	if s.SetNotFound {
		out = append(out, SourceReason{Code: ReasonSetNotPublished, Message: "the question set is not published"})
	}
	for _, t := range s.Types {
		if !Accepts(minigameType, t) {
			out = append(out, SourceReason{Code: ReasonIncompatibleType, Message: truncate("question type " + t + " is not accepted by " + minigameType)})
		}
	}
	if s.Unbounded && !AcceptsEveryType(minigameType) {
		out = append(out, SourceReason{Code: ReasonUnboundedTypes, Message: "the source has no question type filter: it may return types not accepted by " + minigameType})
	}
	if !s.SetNotFound && s.Available < s.QuestionCount {
		out = append(out, SourceReason{Code: ReasonInsufficient, Message: "the source has fewer questions than question_count"})
	}
	return out
}

// SortTypes returns the distinct question types of ts in the order of AutoGraded (at most 11).
func SortTypes(ts []string) []string {
	out := make([]string, 0, maxReportedQuestionType)
	for _, t := range AutoGraded {
		if slices.Contains(ts, t) {
			out = append(out, t)
		}
	}
	return out
}

func truncate(s string) string {
	r := []rune(s)
	if len(r) > maxReasonMessageLength {
		return string(r[:maxReasonMessageLength])
	}
	return s
}
