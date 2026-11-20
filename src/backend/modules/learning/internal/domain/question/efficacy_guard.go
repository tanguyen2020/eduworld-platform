package question

import (
	"encoding/json"

	"github.com/google/uuid"
)

// Efficacy instruments (F035 US1, LEARNING_EFFICACY_PLAN §3.2, QUESTION_BANK §8): the questions and the two parallel
// forms of the independent pre/post tests. They are platform content, exam_only, never parameterized, and only an
// efficacy blueprint (variants.strategy fixed, every rule a fixed list of efficacy questions) may reference them; no
// selection path (practice, retry_wrong, diagnostic, dynamic sets, question plans, ordinary blueprints) ever draws them.

// IsParameterized reports a render payload declaring params (F023 parameterized questions).
func IsParameterized(render json.RawMessage) bool { return paramsOf(render) != nil }

// EfficacyQuestion checks the flag on a question version (F035-AC-01): platform scope, exam_only, not parameterized.
func EfficacyQuestion(platform, examOnly bool, render json.RawMessage) []Violation {
	switch {
	case !platform:
		return []Violation{{Field: "/efficacy_instrument", Reason: "only platform questions are efficacy instruments"}}
	case !examOnly:
		return []Violation{{Field: "/efficacy_instrument", Reason: "an efficacy instrument must be exam_only"}}
	case IsParameterized(render):
		return []Violation{{Field: "/efficacy_instrument", Reason: "a parameterized question is not an efficacy instrument"}}
	}
	return nil
}

// EfficacyRule is the part of a blueprint rule the guard reads.
type EfficacyRule struct {
	Pointer string // JSON pointer of the rule
	Fixed   []uuid.UUID
}

// EfficacyBlueprint checks the flag on a blueprint version (F035-AC-01): platform scope, variants.strategy fixed and
// every rule a list of fixed question versions (whose flag the caller checks with EfficacyReference).
func EfficacyBlueprint(platform bool, strategy string, rules []EfficacyRule) []Violation {
	if !platform {
		return []Violation{{Field: "/efficacy_instrument", Reason: "only platform blueprints are efficacy instruments"}}
	}
	if strategy != "fixed" {
		return []Violation{{Field: "/efficacy_instrument", Reason: "an efficacy blueprint needs variants.strategy fixed"}}
	}
	var out []Violation
	for _, r := range rules {
		if len(r.Fixed) == 0 {
			out = append(out, Violation{Field: r.Pointer + "/fixed_question_version_ids", Reason: "every rule of an efficacy blueprint lists fixed questions",
				Required: true})
		}
	}
	return out
}

// EfficacyReference checks a reference from content to a question version (F035-AC-01, AC-03): an efficacy
// referrer references efficacy questions only; any other referrer (question set, lesson, activity, ordinary
// blueprint) never references one. It returns the reason of the refusal, "" when the reference is allowed.
func EfficacyReference(referrerEfficacy, questionEfficacy bool) string {
	switch {
	case referrerEfficacy && !questionEfficacy:
		return "an efficacy blueprint only references efficacy instruments"
	case !referrerEfficacy && questionEfficacy:
		return "an efficacy instrument is only referenced by an efficacy blueprint"
	}
	return ""
}
