package contract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
)

// forbiddenRenderKeys are the keys that only the server may hold (QUESTION_BANK §2, F022-AC-22): answer key fields of
// the 8 types, scoring, explanation, hints, option feedback and raw LaTeX.
var forbiddenRenderKeys = map[string]bool{
	"answer_key": true, "scoring_rule": true, "explanation": true, "hints": true, "option_feedback": true, "feedback": true,
	"misconception_tag": true, "correct_option_id": true, "correct_option_ids": true, "statement_values": true,
	"accepted": true, "match": true, "value": true, "tolerance": true, "accept_equivalent_forms": true, "pairs": true,
	"order": true, "placements": true, "latex": true, "min_response_ms": true,
}

func keysOf(v any, out map[string]bool) {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			out[k] = true
			keysOf(e, out)
		}
	case []any:
		for _, e := range x {
			keysOf(e, out)
		}
	}
}

// F022-AC-22, AC-25 (contract of the render DTO): for the 8 types of QUESTION_BANK §3, the DTO sent to the client has
// none of the server-only keys, keeps every item id, and formulas reach it as math_ref + speech_text.
func TestRenderDTOHasNoAnswerData(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "..", "..", "internal", "domain", "question", "testdata", "qb_examples")
	reg := question.Base()
	for _, typ := range []question.Type{question.SingleChoice, question.MultipleChoice, question.TrueFalse, question.FillInBlank,
		question.Numeric, question.Matching, question.Ordering, question.DragDrop} {
		t.Run(string(typ), func(t *testing.T) {
			b, err := os.ReadFile(filepath.Join(dir, string(typ)+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var f struct {
				Stem          json.RawMessage      `json:"stem"`
				RenderPayload json.RawMessage      `json:"render_payload"`
				AnswerKey     json.RawMessage      `json:"answer_key"`
				ScoringRule   question.ScoringRule `json:"scoring_rule"`
				MaxScore      float64              `json:"max_score"`
			}
			if err := json.Unmarshal(b, &f); err != nil {
				t.Fatal(err)
			}
			render, _, err := question.SplitFeedback(f.RenderPayload)
			if err != nil {
				t.Fatal(err)
			}
			// A formula in the stem: the client receives math_ref + speech_text, never the LaTeX.
			stem := json.RawMessage(`{"blocks":[{"type":"paragraph","text":"Tính"},{"type":"math","latex":"\\frac{11}{2}"}]}`)
			c := question.Content{VersionID: uuid.New(), Type: typ, Stem: stem, Render: render, AnswerKey: f.AnswerKey, Scoring: f.ScoringRule,
				MaxScore: f.MaxScore, Language: "vi", MathAssets: []question.MathAsset{{Latex: `\frac{11}{2}`, SVGAsset: "media/0f/math.svg",
					SpeechText: "mười một phần hai"}}}
			out, _, err := reg.Render(c, 99)
			if err != nil {
				t.Fatal(err)
			}
			dto, _ := json.Marshal(out)
			var v any
			_ = json.Unmarshal(dto, &v)
			keys := map[string]bool{}
			keysOf(v, keys)
			for k := range keys {
				if forbiddenRenderKeys[k] {
					t.Errorf("render DTO of %s has %q: %s", typ, k, dto)
				}
			}
			if !keys["math_ref"] || !keys["speech_text"] {
				t.Errorf("formula of %s not rendered as math_ref + speech_text: %s", typ, dto)
			}
			want, have := map[string]bool{}, map[string]bool{}
			var payload any
			_ = json.Unmarshal(render, &payload)
			collectIDs(payload, want)
			collectIDs(v, have)
			for id := range want {
				if !have[id] {
					t.Errorf("render DTO of %s lost item id %s", typ, id)
				}
			}
		})
	}
}

func collectIDs(v any, out map[string]bool) {
	switch x := v.(type) {
	case map[string]any:
		if id, ok := x["id"].(string); ok {
			out[id] = true
		}
		for _, e := range x {
			collectIDs(e, out)
		}
	case []any:
		for _, e := range x {
			collectIDs(e, out)
		}
	}
}
