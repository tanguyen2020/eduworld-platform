package contract

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/mathx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
)

// mathBlocks collects every {"type":"math"} block of a decoded payload.
func mathBlocks(v any, out *[]map[string]any) {
	switch t := v.(type) {
	case map[string]any:
		if t["type"] == "math" {
			*out = append(*out, t)
		}
		for _, x := range t {
			mathBlocks(x, out)
		}
	case []any:
		for _, x := range t {
			mathBlocks(x, out)
		}
	}
}

// F027-AC-14 (TC-MATH-003): every formula of a render payload — stem and option contents — reaches the client as
// the stored SVG key (math/<sha256>.svg) and its speech text, so the game needs no MathJax; the LaTeX is not sent.
func TestMathRenderPayload(t *testing.T) {
	svgKey := mathx.Key(mathx.Hash([]byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`)))
	optKey := mathx.Key(mathx.Hash([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><g></g></svg>`)))
	c := question.Content{VersionID: uuid.New(), Type: question.SingleChoice, MaxScore: 1, Language: "vi",
		Stem: json.RawMessage(`{"blocks":[{"type":"paragraph","text":"Phân số nào lớn hơn?"},{"type":"math","latex":"\\frac{3}{4}"}]}`),
		Render: json.RawMessage(`{"options":[{"id":"o_a","content":{"blocks":[{"type":"math","latex":"\\frac{1}{2}"}]}},` +
			`{"id":"o_b","content":{"blocks":[{"type":"paragraph","text":"Bằng nhau"}]}}],"shuffle_options":false}`),
		AnswerKey: json.RawMessage(`{"correct_option_id":"o_b"}`), Scoring: question.ScoringRule{Mode: question.ModeAllOrNothing},
		MathAssets: []question.MathAsset{{Latex: `\frac{3}{4}`, SVGAsset: svgKey, SpeechText: "ba phần tư"},
			{Latex: `\frac{1}{2}`, SVGAsset: optKey, SpeechText: "một phần hai"}}}
	out, _, err := question.Base().Render(c, 7)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(out)
	var v any
	_ = json.Unmarshal(b, &v)
	var blocks []map[string]any
	mathBlocks(v, &blocks)
	if len(blocks) != 2 {
		t.Fatalf("math blocks %d: %s", len(blocks), b)
	}
	for _, m := range blocks {
		ref, _ := m["math_ref"].(string)
		speech, _ := m["speech_text"].(string)
		if !mathx.SVGKeyPattern.MatchString(ref) || speech == "" || m["latex"] != nil {
			t.Errorf("math block %v", m)
		}
	}
	if strings.Contains(string(b), `\\frac`) {
		t.Errorf("LaTeX reached the client: %s", b)
	}
}
