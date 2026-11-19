package question

import (
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

type fixture struct {
	Type          Type              `json:"type"`
	Stem          json.RawMessage   `json:"stem"`
	RenderPayload json.RawMessage   `json:"render_payload"`
	AnswerKey     json.RawMessage   `json:"answer_key"`
	ScoringRule   ScoringRule       `json:"scoring_rule"`
	Hints         []json.RawMessage `json:"hints"`
	MaxScore      float64           `json:"max_score"`
}

// load reads testdata/qb_examples/<type>.json (the QUESTION_BANK §3 examples, spec Assumptions).
func load(t *testing.T, typ Type) Authoring {
	t.Helper()
	b, err := os.ReadFile("testdata/qb_examples/" + string(typ) + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var f fixture
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	return Authoring{Type: f.Type, Stem: f.Stem, RenderPayload: f.RenderPayload, AnswerKey: f.AnswerKey, Scoring: f.ScoringRule,
		Hints: f.Hints, MaxScore: f.MaxScore, Language: "vi"}
}

// content stores an authoring version like the command does (feedback split off the render payload).
func content(t *testing.T, a Authoring) Content {
	t.Helper()
	render, _, err := SplitFeedback(a.RenderPayload)
	if err != nil {
		t.Fatal(err)
	}
	return Content{VersionID: uuid.New(), Type: a.Type, Stem: a.Stem, Render: render, AnswerKey: a.AnswerKey, Scoring: a.Scoring,
		MaxScore: a.MaxScore, Language: a.Language}
}

func base8() []Type {
	return []Type{SingleChoice, MultipleChoice, TrueFalse, FillInBlank, Numeric, Matching, Ordering, DragDrop}
}

func fields(err error) []string {
	var v contentmodel.Violations
	if !errors.As(err, &v) {
		return nil
	}
	out := make([]string, len(v))
	for i, x := range v {
		out[i] = x.Field
	}
	return out
}

func with(a Authoring, field, js string) Authoring {
	switch field {
	case "render":
		a.RenderPayload = json.RawMessage(js)
	case "key":
		a.AnswerKey = json.RawMessage(js)
	case "stem":
		a.Stem = json.RawMessage(js)
	}
	return a
}

// F022-AC-01…05: the 8 examples are valid; types of F023 are not enabled; semantic errors are reported as pointers.
func TestValidate(t *testing.T) {
	r := Base()
	lim := Limits{AssetsPerVersion: 50}
	for _, typ := range base8() {
		if err := r.Validate(load(t, typ), lim); err != nil {
			t.Errorf("%s fixture: %v", typ, err)
		}
	}
	for _, typ := range []Type{ShortAnswer, ImageBased, Interactive} {
		if err := r.Validate(Authoring{Type: typ, MaxScore: 1, Scoring: ScoringRule{Mode: ModeAllOrNothing}}, lim); !errors.Is(err, ErrTypeNotEnabled) {
			t.Errorf("%s: %v", typ, err)
		}
	}
	sc := load(t, SingleChoice)
	cases := []struct {
		name string
		a    Authoring
		want string
	}{
		{"single: two correct", with(sc, "key", `{"correct_option_id":"o_zzzz"}`), "/answer_key/correct_option_id"},
		{"single: id A", with(sc, "render", `{"options":[{"id":"A","content":{"blocks":[]}},{"id":"o_3b2e","content":{"blocks":[]}}]}`), "/render_payload/options/0/id"},
		{"single: duplicate id", with(sc, "render", `{"options":[{"id":"o_3b2e","content":{"blocks":[]}},{"id":"o_3b2e","content":{"blocks":[]}}]}`), "/render_payload/options/1/id"},
		{"multiple: unknown key", with(load(t, MultipleChoice), "key", `{"correct_option_ids":["o_1","o_9"]}`), "/answer_key/correct_option_ids/1"},
		{"ordering: 2 items", with(load(t, Ordering), "render", `{"items":[{"id":"i_1","content":{"blocks":[]}},{"id":"i_2","content":{"blocks":[]}}]}`), "/render_payload/items"},
		{"ordering: not a permutation", with(load(t, Ordering), "key", `{"order":["i_1","i_2","i_3","i_3"]}`), "/answer_key/order"},
		{"matching: unknown right", with(load(t, Matching), "key", `{"pairs":[["l_1","r_9"]]}`), "/answer_key/pairs/0/1"},
		{"fill: blank without answer", with(load(t, FillInBlank), "key", `{"blanks":{"b_1":{"accepted":["Hà Nội"]}}}`), "/answer_key/blanks/b_2"},
		{"fill: stem blank unknown", with(load(t, FillInBlank), "stem", `{"blocks":[{"type":"paragraph","text":"{{blank:b_1}} {{blank:b_2}} {{blank:b_9}}"}]}`), "/stem"},
		{"numeric: not a decimal", with(load(t, Numeric), "key", `{"value":"abc"}`), "/answer_key/value"},
		{"drag: unknown zone", with(load(t, DragDrop), "key", `{"placements":{"d_1":"z_none"}}`), "/answer_key/placements/d_1"},
		{"true_false: missing statement", with(load(t, TrueFalse), "key", `{"statement_values":{"s_1":true}}`), "/answer_key/statement_values"},
	}
	for _, c := range cases {
		if got := fields(r.Validate(c.a, lim)); !slices.Contains(got, c.want) {
			t.Errorf("%s: %v, want %s", c.name, got, c.want)
		}
	}
	// AC-04: partial_method of the type and max_score > 0.
	o := load(t, Ordering)
	o.Scoring = ScoringRule{Mode: ModePartial, PartialMethod: PartialRightMinusWrg}
	if got := fields(r.Validate(o, lim)); !slices.Contains(got, "/scoring_rule/partial_method") {
		t.Errorf("ordering right_minus_wrong: %v", got)
	}
	s := load(t, SingleChoice)
	s.Scoring = ScoringRule{Mode: ModePartial, PartialMethod: PartialProportional}
	s.MaxScore = 0
	if got := fields(r.Validate(s, lim)); !slices.Contains(got, "/scoring_rule/partial_method") || !slices.Contains(got, "/max_score") {
		t.Errorf("single partial + max 0: %v", got)
	}
	// AC-05: media limit, formula without asset, hints ≤ 5.
	m := with(sc, "stem", `{"blocks":[{"type":"math","latex":"\\frac{1}{2}"},{"type":"image","media_ref":"0193f0a2-7c1e-7000-8000-000000000001","alt":"x"}]}`)
	if got := fields(r.Validate(m, Limits{AssetsPerVersion: 0})); !slices.Contains(got, "/math_assets") {
		t.Errorf("formula without asset: %v", got)
	}
	m.MathAssets = []MathAsset{{Latex: `\frac{1}{2}`, SVGAsset: "math/" + strings.Repeat("a", 64) + ".svg", SpeechText: "một phần hai"}}
	if err := r.Validate(m, Limits{AssetsPerVersion: 50}); err != nil {
		t.Errorf("formula with asset: %v", err)
	}
	if got := fields(r.Validate(m, Limits{AssetsPerVersion: 0})); slices.Contains(got, "/media_refs") {
		t.Errorf("no limit: %v", got)
	}
	two := with(sc, "stem", `{"blocks":[{"type":"image","media_ref":"0193f0a2-7c1e-7000-8000-000000000001","alt":"x"},`+
		`{"type":"audio","media_ref":"0193f0a2-7c1e-7000-8000-000000000002"}]}`)
	if got := fields(r.Validate(two, Limits{AssetsPerVersion: 1})); !slices.Contains(got, "/media_refs") {
		t.Errorf("assets_per_version_max: %v", got)
	}
	h := load(t, SingleChoice)
	h.Hints = make([]json.RawMessage, 6)
	if got := fields(r.Validate(h, lim)); !slices.Contains(got, "/hints") {
		t.Errorf("6 hints: %v", got)
	}
}

// F022-AC-06: feedback and misconception tags leave the render payload and come back for the CMS.
func TestSplitFeedback(t *testing.T) {
	a := load(t, SingleChoice)
	render, fb, err := SplitFeedback(a.RenderPayload)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(render), "feedback") || strings.Contains(string(render), "misconception") || len(fb) != 4 ||
		fb["o_c7d0"].MisconceptionTag != "off_by_one_factor" {
		t.Fatalf("split %s %+v", render, fb)
	}
	back, err := MergeFeedback(render, fb)
	if err != nil || !strings.Contains(string(back), "off_by_one_factor") || !strings.Contains(string(back), "Đúng rồi") {
		t.Fatalf("merge %s %v", back, err)
	}
}
