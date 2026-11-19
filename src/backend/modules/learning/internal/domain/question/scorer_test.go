package question

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"testing/quick"
)

// F022-AC-14…16, AC-18: response → score for the 8 types, with the partial methods of QUESTION_BANK §3.
func TestGradeTable(t *testing.T) {
	r := Base()
	type row struct {
		typ      Type
		rule     *ScoringRule
		resp     string
		score    float64
		correct  bool
		language string
	}
	pm := func(method string) *ScoringRule { return &ScoringRule{Mode: ModePartial, PartialMethod: method} }
	aon := &ScoringRule{Mode: ModeAllOrNothing}
	rows := []row{
		{SingleChoice, nil, `{"option_id":"o_3b2e"}`, 1, true, ""},
		{SingleChoice, nil, `{"option_id":"o_a91c"}`, 0, false, ""},
		// multiple_choice: C=2 (o_1, o_3)
		{MultipleChoice, nil, `{"option_ids":["o_1","o_3"]}`, 1, true, ""},
		{MultipleChoice, nil, `{"option_ids":["o_1"]}`, 0.5, false, ""},
		{MultipleChoice, nil, `{"option_ids":["o_1","o_2"]}`, 0, false, ""},
		{MultipleChoice, nil, `{"option_ids":["o_1","o_3","o_2"]}`, 0.5, false, ""},
		{MultipleChoice, nil, `{"option_ids":["o_2","o_4"]}`, 0, false, ""},
		{MultipleChoice, pm(PartialProportional), `{"option_ids":["o_1"]}`, 0.5, false, ""},
		{MultipleChoice, pm(PartialProportional), `{"option_ids":["o_1","o_2"]}`, 0, false, ""},
		{MultipleChoice, aon, `{"option_ids":["o_1"]}`, 0, false, ""},
		{MultipleChoice, aon, `{"option_ids":["o_3","o_1"]}`, 1, true, ""},
		// true_false statements
		{TrueFalse, nil, `{"statement_values":{"s_1":true,"s_2":false}}`, 1, true, ""},
		{TrueFalse, nil, `{"statement_values":{"s_1":true,"s_2":true}}`, 0.5, false, ""},
		{TrueFalse, nil, `{"statement_values":{"s_1":true}}`, 0.5, false, ""},
		{TrueFalse, aon, `{"statement_values":{"s_1":true,"s_2":true}}`, 0, false, ""},
		// fill_in_blank: b_1 normalized "Hà Nội", b_2 exact "Hồng"
		{FillInBlank, nil, `{"blanks":{"b_1":"  hà   nội. ","b_2":"Hồng"}}`, 1, true, ""},
		{FillInBlank, nil, `{"blanks":{"b_1":"ha noi","b_2":"Hồng"}}`, 0.5, false, ""},
		{FillInBlank, nil, `{"blanks":{"b_1":"Hà Nội","b_2":"hồng"}}`, 0.5, false, ""},
		{FillInBlank, nil, `{"blanks":{"b_1":"Huế"}}`, 0, false, ""},
		// numeric 12.5 ± 0.01, equivalent forms
		{Numeric, nil, `{"value":"12,5"}`, 1, true, "vi"},
		{Numeric, nil, `{"value":"12.50"}`, 1, true, "vi"},
		{Numeric, nil, `{"value":"25/2"}`, 1, true, "vi"},
		{Numeric, nil, `{"value":"12,509"}`, 1, true, "vi"},
		{Numeric, nil, `{"value":"12,52"}`, 0, false, "vi"},
		{Numeric, nil, `{"value":"mười hai"}`, 0, false, "vi"},
		// matching: l_1→r_2, l_2→r_1
		{Matching, nil, `{"pairs":[["l_1","r_2"],["l_2","r_1"]]}`, 1, true, ""},
		{Matching, nil, `{"pairs":[["l_1","r_2"],["l_2","r_3"]]}`, 0.5, false, ""},
		{Matching, nil, `{"pairs":[["l_1","r_3"]]}`, 0, false, ""},
		// ordering i_1..i_4, adjacent_pairs default of the fixture
		{Ordering, nil, `{"order":["i_1","i_2","i_3","i_4"]}`, 1, true, ""},
		{Ordering, nil, `{"order":["i_2","i_3","i_4","i_1"]}`, 2.0 / 3.0, false, ""},
		{Ordering, nil, `{"order":["i_4","i_3","i_2","i_1"]}`, 0, false, ""},
		{Ordering, pm(PartialPosition), `{"order":["i_1","i_2","i_4","i_3"]}`, 0.5, false, ""},
		{Ordering, pm(PartialPosition), `{"order":["i_2","i_3","i_4","i_1"]}`, 0, false, ""},
		{Ordering, aon, `{"order":["i_1","i_2","i_4","i_3"]}`, 0, false, ""},
		// drag_drop: d_1 even, d_2 odd, d_3 even
		{DragDrop, nil, `{"placements":{"d_1":"z_even","d_2":"z_odd","d_3":"z_even"}}`, 1, true, ""},
		{DragDrop, nil, `{"placements":{"d_1":"z_even","d_2":"z_even","d_3":"z_even"}}`, 2.0 / 3.0, false, ""},
		{DragDrop, nil, `{"placements":{"d_1":"z_even"}}`, 1.0 / 3.0, false, ""},
	}
	for i, x := range rows {
		a := load(t, x.typ)
		if x.rule != nil {
			a.Scoring = *x.rule
		}
		if x.language != "" {
			a.Language = x.language
		}
		g, err := r.Grade(content(t, a), json.RawMessage(x.resp))
		if err != nil {
			t.Errorf("row %d %s %s: %v", i, x.typ, x.resp, err)
			continue
		}
		if fmt.Sprintf("%.4f", g.Score) != fmt.Sprintf("%.4f", x.score) || g.IsCorrect != x.correct || g.Score < 0 || g.Score > g.MaxScore {
			t.Errorf("row %d %s %s: %+v, want %.4f %v", i, x.typ, x.resp, g, x.score, x.correct)
		}
	}
	// min_score and max_score scale.
	a := load(t, MultipleChoice)
	a.MaxScore = 2
	floor := 0.5
	a.Scoring.MinScore = &floor
	if g, _ := r.Grade(content(t, a), json.RawMessage(`{"option_ids":["o_2"]}`)); g.Score != 0.5 || g.MaxScore != 2 {
		t.Fatalf("min_score %+v", g)
	}
	if g, _ := r.Grade(content(t, a), json.RawMessage(`{"option_ids":["o_1"]}`)); g.Score != 1 {
		t.Fatalf("scaled %+v", g)
	}
	// Edge cases: decoy draggable and zone capacity count as wrong.
	d := load(t, DragDrop)
	d.RenderPayload = json.RawMessage(`{"draggables":[{"id":"d_1","content":{"blocks":[]}},{"id":"d_2","content":{"blocks":[]}},{"id":"d_9","content":{"blocks":[]}}],
		"zones":[{"id":"z_a","label":"A","capacity":1},{"id":"z_b","label":"B"}]}`)
	d.AnswerKey = json.RawMessage(`{"placements":{"d_1":"z_a","d_2":"z_b"}}`)
	if g, _ := r.Grade(content(t, d), json.RawMessage(`{"placements":{"d_1":"z_a","d_2":"z_b","d_9":"z_b"}}`)); g.Score != 0.6667 || g.IsCorrect {
		t.Fatalf("decoy %+v", g)
	}
	if g, _ := r.Grade(content(t, d), json.RawMessage(`{"placements":{"d_1":"z_a","d_2":"z_a"}}`)); g.Score != 0.5 {
		t.Fatalf("capacity %+v", g)
	}
}

// F022-AC-19, AC-21: malformed responses are ErrInvalidResponse, never a panic; the scorer reads no score or answer
// from the response (extra fields are refused).
func TestGradeInvalidResponses(t *testing.T) {
	r := Base()
	bad := map[Type][]string{
		SingleChoice:   {``, `{}`, `null`, `[]`, `{"option_id":"o_unknown"}`, `{"option_id":1}`, `{"option_id":"o_3b2e","score":1}`, `{"option_id":"o_3b2e"} {}`},
		MultipleChoice: {`{"option_ids":[]}`, `{"option_ids":["o_1","o_1"]}`, `{"option_ids":["x"]}`, `{"option_ids":"o_1"}`, `{"option_ids":["o_1"],"is_correct":true}`},
		TrueFalse:      {`{"value":true}`, `{"statement_values":{}}`, `{"statement_values":{"s_9":true}}`, `{"statement_values":{"s_1":"yes"}}`},
		FillInBlank:    {`{"blanks":{}}`, `{"blanks":{"b_9":"x"}}`, `{"blanks":{"b_1":1}}`, `{"blanks":{"b_1":"` + strings.Repeat("x", 201) + `"}}`},
		Numeric:        {`{"value":""}`, `{"value":12.5}`, `{"value":"` + strings.Repeat("1", 65) + `"}`},
		Matching:       {`{"pairs":[]}`, `{"pairs":[["l_1"]]}`, `{"pairs":[["l_1","r_9"]]}`, `{"pairs":[["l_1","r_1"],["l_1","r_2"]]}`},
		Ordering:       {`{"order":["i_1","i_2"]}`, `{"order":["i_1","i_1","i_2","i_3"]}`, `{"order":["i_1","i_2","i_3","i_9"]}`},
		DragDrop:       {`{"placements":{}}`, `{"placements":{"d_9":"z_even"}}`, `{"placements":{"d_1":"z_none"}}`},
	}
	for typ, list := range bad {
		c := content(t, load(t, typ))
		for _, resp := range list {
			if _, err := r.Grade(c, json.RawMessage(resp)); !errors.Is(err, ErrInvalidResponse) {
				t.Errorf("%s %q: %v, want ErrInvalidResponse", typ, resp, err)
			}
		}
	}
	// Corrupt stored data never panics.
	c := content(t, load(t, Ordering))
	c.AnswerKey = json.RawMessage(`{"order":"broken"}`)
	if _, err := r.Grade(c, json.RawMessage(`{"order":["i_1","i_2","i_3","i_4"]}`)); err == nil {
		t.Fatal("corrupt key graded")
	}
	// Random bytes never panic.
	f := func(b []byte) bool {
		for _, typ := range base8() {
			_, _ = r.Grade(content(t, load(t, typ)), json.RawMessage(b))
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 200}); err != nil {
		t.Fatal(err)
	}
}

// F022-AC-20 (property): the score of a response does not depend on the presentation order.
func TestGradeIndependentOfShuffle(t *testing.T) {
	r := Base()
	for _, typ := range base8() {
		c := content(t, load(t, typ))
		resp := map[Type]string{SingleChoice: `{"option_id":"o_3b2e"}`, MultipleChoice: `{"option_ids":["o_1","o_2"]}`,
			TrueFalse: `{"statement_values":{"s_1":true,"s_2":true}}`, FillInBlank: `{"blanks":{"b_1":"Hà Nội","b_2":"Mekong"}}`,
			Numeric: `{"value":"12,5"}`, Matching: `{"pairs":[["l_1","r_2"],["l_2","r_3"]]}`, Ordering: `{"order":["i_2","i_1","i_3","i_4"]}`,
			DragDrop: `{"placements":{"d_1":"z_even","d_2":"z_even"}}`}[typ]
		want, err := r.Grade(c, json.RawMessage(resp))
		if err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
		check := func(seed uint64) bool {
			_, p, err := r.Render(c, seed)
			if err != nil {
				return false
			}
			_ = p
			got, err := r.Grade(c, json.RawMessage(resp))
			return err == nil && got.Score == want.Score
		}
		if err := quick.Check(check, &quick.Config{MaxCount: 50}); err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
	}
}

// forbidden are the answer fields that never reach a client (F022-AC-22, QUESTION_BANK §8).
var forbidden = []string{"answer_key", "scoring_rule", "explanation", "feedback", "misconception_tag", "hints", "min_response_ms",
	"accepted", "correct_option_id", "correct_option_ids", "statement_values", "placements", "pairs", "order", "latex"}

// F022-AC-22, AC-23, AC-25: the render DTO has white-listed fields only, a reproducible shuffle keeping ids, an
// ordering never shown in the right order, formulas as math_ref + speech_text.
func TestRender(t *testing.T) {
	r := Base()
	for _, typ := range base8() {
		a := load(t, typ)
		c := content(t, a)
		dto, p, err := r.Render(c, 42)
		if err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
		b, _ := json.Marshal(dto)
		for _, f := range forbidden {
			if strings.Contains(string(b), `"`+f+`"`) {
				t.Errorf("%s: render DTO has %q: %s", typ, f, b)
			}
		}
		again, p2, _ := r.Render(c, 42)
		b2, _ := json.Marshal(again)
		if string(b) != string(b2) || fmt.Sprint(p.Orders) != fmt.Sprint(p2.Orders) {
			t.Errorf("%s: same seed, different render", typ)
		}
	}
	// ids kept, order differs between seeds, ordering never in the right order.
	c := content(t, load(t, Ordering))
	shapes := map[string]bool{}
	for seed := uint64(0); seed < 200; seed++ {
		_, p, _ := r.Render(c, seed)
		got := p.Orders["items"]
		if slices.Equal(got, []string{"i_1", "i_2", "i_3", "i_4"}) {
			t.Fatalf("seed %d shows the right order", seed)
		}
		sorted := slices.Clone(got)
		slices.Sort(sorted)
		if !slices.Equal(sorted, []string{"i_1", "i_2", "i_3", "i_4"}) {
			t.Fatalf("ids changed: %v", got)
		}
		shapes[fmt.Sprint(got)] = true
	}
	if len(shapes) < 5 {
		t.Fatalf("only %d presentation orders", len(shapes))
	}
	// Formulas: LaTeX replaced by the SVG asset and its speech text.
	m := load(t, SingleChoice)
	m.Stem = json.RawMessage(`{"blocks":[{"type":"math","latex":"\\frac{11}{2}"}]}`)
	m.MathAssets = []MathAsset{{Latex: `\frac{11}{2}`, SVGAsset: "math/" + strings.Repeat("b", 64) + ".svg", SpeechText: "mười một phần hai"}}
	mc := content(t, m)
	mc.MathAssets = m.MathAssets
	dto, _, _ := r.Render(mc, 1)
	b, _ := json.Marshal(dto)
	if strings.Contains(string(b), "latex") || !strings.Contains(string(b), `"math_ref"`) || !strings.Contains(string(b), "mười một phần hai") {
		t.Fatalf("math render %s", b)
	}
	// show_correct_count is allowed display information.
	mcq := load(t, MultipleChoice)
	mcq.RenderPayload = json.RawMessage(strings.Replace(string(mcq.RenderPayload), `"show_correct_count": false`, `"show_correct_count": true`, 1))
	dto, _, _ = r.Render(content(t, mcq), 1)
	if dto.Payload["correct_count"] != float64(2) {
		t.Fatalf("correct_count %v", dto.Payload)
	}
	_ = rand.Int
}

func BenchmarkGrade(b *testing.B) {
	r := Base()
	t := &testing.T{}
	c := content(t, load(t, Ordering))
	resp := json.RawMessage(`{"order":["i_2","i_1","i_3","i_4"]}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = r.Grade(c, resp)
	}
}
