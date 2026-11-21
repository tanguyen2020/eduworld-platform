package question

import "encoding/json"

// CorrectResponse returns the correct answer of a version in the shape of its response (question.schema.json#/$defs/
// response) for the types whose answer key is a fixed response without parameters: single_choice, multiple_choice,
// true_false (single statement), ordering and matching. Other types (parameterized numeric, text matches, zones…) return
// nil: their explanation carries the answer. Server side only, sent after a question closed (F105-AC-05, ADR-0016).
func CorrectResponse(t Type, key json.RawMessage) json.RawMessage {
	var k struct {
		One   string     `json:"correct_option_id"`
		Many  []string   `json:"correct_option_ids"`
		Value *bool      `json:"value"`
		Order []string   `json:"order"`
		Pairs [][]string `json:"pairs"`
	}
	if json.Unmarshal(key, &k) != nil {
		return nil
	}
	var v any
	switch {
	case t == SingleChoice && k.One != "":
		v = map[string]any{"option_id": k.One}
	case t == MultipleChoice && len(k.Many) > 0:
		v = map[string]any{"option_ids": k.Many}
	case t == TrueFalse && k.Value != nil:
		v = map[string]any{"value": *k.Value}
	case t == Ordering && len(k.Order) > 0:
		v = map[string]any{"order": k.Order}
	case t == Matching && len(k.Pairs) > 0:
		v = map[string]any{"pairs": k.Pairs}
	default:
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}
