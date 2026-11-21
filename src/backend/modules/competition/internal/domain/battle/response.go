package battle

import (
	"encoding/json"
	"errors"
)

// ErrInvalidResponse: the response is not an object of the shape of the question type (VALIDATION_FAILED, F105-AC-27).
// The room does not change; module learning still validates the full schema when it grades.
var ErrInvalidResponse = errors.New("battle: response does not fit the question type")

// responseKeys are the keys of question.schema.json#/$defs/response per question type (one shape per oneOf branch).
var responseKeys = map[string][][]string{
	"single_choice":   {{"option_id"}},
	"multiple_choice": {{"option_ids"}},
	"true_false":      {{"value"}, {"statement_values"}},
	"fill_in_blank":   {{"blanks"}},
	"short_answer":    {{"text"}},
	"numeric":         {{"value"}},
	"matching":        {{"pairs"}},
	"ordering":        {{"order"}},
	"drag_drop":       {{"placements"}},
	"image_based":     {{"points"}, {"label_placements"}},
	"interactive":     {{"state"}, {"state", "action_log"}},
}

// MaxResponseBytes bounds a raw response (attempt_answers.response ≤ 32 KB).
const MaxResponseBytes = 32768

// ValidResponse reports whether raw is an object with exactly the keys of one response shape of the question type.
func ValidResponse(questionType string, raw json.RawMessage) bool {
	if len(raw) == 0 || len(raw) > MaxResponseBytes {
		return false
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil || obj == nil {
		return false
	}
	shapes, ok := responseKeys[questionType]
	if !ok {
		return false
	}
	for _, keys := range shapes {
		if len(keys) != len(obj) {
			continue
		}
		match := true
		for _, k := range keys {
			if _, ok := obj[k]; !ok {
				match = false
			}
		}
		if match {
			return true
		}
	}
	return false
}
