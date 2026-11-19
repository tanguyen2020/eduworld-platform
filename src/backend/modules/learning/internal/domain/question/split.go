package question

import (
	"encoding/json"
	"sort"
)

// optionLists are the lists of the render payload whose elements are options (question.schema.json#/$defs/option).
var optionLists = []string{"options", "left", "right", "items", "draggables"}

// SplitFeedback removes options[].feedback and options[].misconception_tag from an authoring render payload (F022-AC-06,
// research R4): the stored render payload never holds them, they go to the server-only option_feedback map keyed by
// option id.
func SplitFeedback(render json.RawMessage) (json.RawMessage, map[string]OptionFeedback, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(render, &payload); err != nil {
		return nil, nil, err
	}
	feedback := map[string]OptionFeedback{}
	for _, key := range optionLists {
		raw, ok := payload[key]
		if !ok {
			continue
		}
		var items []map[string]json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, nil, err
		}
		for _, it := range items {
			var id string
			_ = json.Unmarshal(it["id"], &id)
			fb := OptionFeedback{}
			if f, ok := it["feedback"]; ok {
				fb.Feedback = f
				delete(it, "feedback")
			}
			if m, ok := it["misconception_tag"]; ok {
				_ = json.Unmarshal(m, &fb.MisconceptionTag)
				delete(it, "misconception_tag")
			}
			if len(fb.Feedback) > 0 || fb.MisconceptionTag != "" {
				feedback[id] = fb
			}
		}
		b, err := json.Marshal(items)
		if err != nil {
			return nil, nil, err
		}
		payload[key] = b
	}
	out, err := json.Marshal(payload)
	if len(feedback) == 0 {
		feedback = nil
	}
	return out, feedback, err
}

// MergeFeedback puts the server-only option feedback back into a stored render payload (CMS read, F022-AC-10).
func MergeFeedback(render json.RawMessage, feedback map[string]OptionFeedback) (json.RawMessage, error) {
	if len(feedback) == 0 {
		return render, nil
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(render, &payload); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(payload))
	for k := range payload {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range optionLists {
		raw, ok := payload[key]
		if !ok {
			continue
		}
		var items []map[string]json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, err
		}
		for _, it := range items {
			var id string
			_ = json.Unmarshal(it["id"], &id)
			if fb, ok := feedback[id]; ok {
				if len(fb.Feedback) > 0 {
					it["feedback"] = fb.Feedback
				}
				if fb.MisconceptionTag != "" {
					b, _ := json.Marshal(fb.MisconceptionTag)
					it["misconception_tag"] = b
				}
			}
		}
		b, err := json.Marshal(items)
		if err != nil {
			return nil, err
		}
		payload[key] = b
	}
	return json.Marshal(payload)
}
