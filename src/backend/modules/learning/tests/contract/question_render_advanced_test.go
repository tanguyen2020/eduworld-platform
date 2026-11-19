package contract

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
)

// F023-AC-06, AC-18, SC-001 (TC-LRN-007): the render DTO of the F023 types and of parameterized questions carries no
// region, placement, validator, params declaration, constraint or answer expression.
func TestRenderAdvancedHasNoAnswerData(t *testing.T) {
	img := uuid.New().String()
	cases := map[question.Type][2]string{
		question.ShortAnswer: {`{"max_length":30}`, `{"accepted":["Hà Nội"],"fuzzy":{"enabled":true,"max_edit_distance":1,"min_length":4}}`},
		question.ImageBased: {`{"interaction":"hotspot","image_media_ref":"` + img + `","image_size":{"w":10,"h":10}}`,
			`{"regions":[{"id":"r_a","shape":"circle","cx":5,"cy":5,"r":2}],"correct_region_ids":["r_a"]}`},
		question.Interactive: {`{"widget":"balance_scale","widget_version":"1.0","config":{"left_weights":[2],"available_weights":[1,1],"max_moves":3}}`,
			`{"validator":"balance_scale.equal","params":{"target":2}}`},
		question.Numeric: {`{"keyboard":"numeric","params":{"a":{"type":"int","min":2,"max":5},"constraints":["a > 2"]}}`, `{"value":"{{a}} * 3"}`},
	}
	forbidden := []string{"regions", "correct_region_ids", "placements", "validator", "params", "constraints", "accepted", "fuzzy", "{{", "target"}
	for typ, c := range cases {
		out, _, err := question.Full().Render(question.Content{VersionID: uuid.New(), Type: typ, Stem: json.RawMessage(`{"blocks":[{"type":"paragraph","text":"{{a}}?"}]}`),
			Render: json.RawMessage(c[0]), AnswerKey: json.RawMessage(c[1]), Scoring: question.ScoringRule{Mode: question.ModeAllOrNothing}, MaxScore: 1,
			Language: "vi"}, 5)
		if err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
		b, _ := json.Marshal(out)
		for _, k := range forbidden {
			if strings.Contains(string(b), k) && (typ == question.Numeric || k != "{{") {
				t.Errorf("render of %s has %q: %s", typ, k, b)
			}
		}
	}
}
