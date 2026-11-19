package question

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

func adv(typ Type, render, key string) Content {
	return Content{VersionID: uuid.New(), Type: typ, Stem: json.RawMessage(`{"blocks":[]}`), Render: json.RawMessage(render),
		AnswerKey: json.RawMessage(key), Scoring: ScoringRule{Mode: ModeAllOrNothing}, MaxScore: 1, Language: "vi"}
}

func grade(t *testing.T, r *Registry, c Content, resp string) (GradeResult, error) {
	t.Helper()
	return r.Grade(c, json.RawMessage(resp))
}

func reasons(err error) string {
	var vs contentmodel.Violations
	var out []string
	if errors.As(err, &vs) {
		for _, v := range vs {
			out = append(out, v.Field+":"+v.Reason)
		}
	}
	return strings.Join(out, "|")
}

func authoring(typ Type, render, key string) Authoring {
	return Authoring{Type: typ, Stem: json.RawMessage(`{"blocks":[{"type":"paragraph","text":"x"}]}`), RenderPayload: json.RawMessage(render),
		AnswerKey: json.RawMessage(key), Scoring: ScoringRule{Mode: ModeAllOrNothing}, MaxScore: 1, Language: "vi"}
}

// F023-AC-01, AC-02, AC-04 (TC-LRN-003): short_answer validation and grading table.
func TestShortAnswer(t *testing.T) {
	r := Full()
	lim := Limits{AssetsPerVersion: 50}
	if err := r.Validate(authoring(ShortAnswer, `{"max_length":50}`, `{"accepted":["Hà Nội"]}`), lim); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		`{"accepted":[]}`: "/answer_key/accepted",
		`{"accepted":["` + strings.Repeat("x", 256) + `"]}`:                                  "/answer_key/accepted/0",
		`{"accepted":["abc"],"fuzzy":{"enabled":true,"max_edit_distance":3,"min_length":3}}`: "/answer_key/fuzzy/max_edit_distance",
		`{"accepted":["abc"],"fuzzy":{"enabled":true,"max_edit_distance":1,"min_length":2}}`: "/answer_key/fuzzy/min_length",
	} {
		if got := reasons(r.Validate(authoring(ShortAnswer, `{}`, key), lim)); !strings.Contains(got, want) {
			t.Errorf("%s: %s", key, got)
		}
	}
	cases := []struct {
		key, lang, text string
		ok              bool
	}{
		{`{"accepted":["Hà Nội"]}`, "vi", "hà nội", true},
		{`{"accepted":["Hà Nội"]}`, "vi", "Ha Noi", false},
		{`{"accepted":["Hà Nội"],"allow_diacritics_missing":true}`, "vi", "Ha Noi", true},
		{`{"accepted":["Hà Nội"],"match":"exact"}`, "vi", "hà nội", false},
		{`{"accepted":["photosynthesis"],"fuzzy":{"enabled":true,"max_edit_distance":1,"min_length":3}}`, "en", "photosynthesys", true},
		{`{"accepted":["cat"],"fuzzy":{"enabled":true,"max_edit_distance":1,"min_length":5}}`, "en", "cot", false},
		{`{"accepted":["Don't"]}`, "en", "don’t", true},
		{`{"accepted":["Apple"],"case_sensitive":true}`, "en", "apple", false},
		{`{"accepted":["hòa","hoà"]}`, "vi", "hoà", true},
	}
	for _, c := range cases {
		ct := adv(ShortAnswer, `{}`, c.key)
		ct.Language = c.lang
		g, err := grade(t, r, ct, `{"text":"`+c.text+`"}`)
		if err != nil || g.IsCorrect != c.ok {
			t.Errorf("%s %q: %v %v", c.key, c.text, g.IsCorrect, err)
		}
	}
	if _, err := grade(t, r, adv(ShortAnswer, `{"max_length":3}`, `{"accepted":["abc"]}`), `{"text":"abcd"}`); !errors.Is(err, ErrInvalidResponse) {
		t.Fatal("over max_length")
	}
	if !FuzzyAnywhere(json.RawMessage(`{"accepted":["a"],"fuzzy":{"enabled":true}}`)) || FuzzyAnywhere(json.RawMessage(`{"accepted":["a"]}`)) {
		t.Fatal("FuzzyAnywhere")
	}
}

const hotspot = `{"interaction":"hotspot","image_media_ref":"%s","image_size":{"w":800,"h":600},"max_selections":2}`

// F023-AC-05, AC-06, AC-07, AC-08: image_based validation, geometry, grading, render.
func TestImageBased(t *testing.T) {
	r := Full()
	img := uuid.New()
	render := strings.Replace(hotspot, "%s", img.String(), 1)
	key := `{"regions":[{"id":"r_heart","shape":"circle","cx":100,"cy":100,"r":50},{"id":"r_lung","shape":"rect","x":300,"y":100,"w":100,"h":50},` +
		`{"id":"r_liver","shape":"polygon","points":[[500,400],[700,400],[700,500],[600,450],[500,500]]}],"correct_region_ids":["r_heart","r_liver"]}`
	a := authoring(ImageBased, render, key)
	a.MediaRefs = []uuid.UUID{img}
	if err := r.Validate(a, Limits{AssetsPerVersion: 50}); err != nil {
		t.Fatal(err)
	}
	a.MediaRefs = nil
	if got := reasons(r.Validate(a, Limits{})); !strings.Contains(got, "/render_payload/image_media_ref") {
		t.Fatalf("media ref: %s", got)
	}
	a.MediaRefs = []uuid.UUID{img}
	for name, k := range map[string]string{
		"radius":     strings.Replace(key, `"r":50`, `"r":0`, 1),
		"outside":    strings.Replace(key, `"x":300`, `"x":790`, 1),
		"two points": strings.Replace(key, `[[500,400],[700,400],[700,500],[600,450],[500,500]]`, `[[1,1],[2,2]]`, 1),
		"self-cross": strings.Replace(key, `[[500,400],[700,400],[700,500],[600,450],[500,500]]`, `[[500,400],[700,500],[700,400],[500,500]]`, 1),
		"unknown":    strings.Replace(key, `"correct_region_ids":["r_heart","r_liver"]`, `"correct_region_ids":["r_x"]`, 1),
		"selections": strings.Replace(key, `"correct_region_ids":["r_heart","r_liver"]`, `"correct_region_ids":["r_heart","r_liver","r_lung"]`, 1),
	} {
		b := a
		b.AnswerKey = json.RawMessage(k)
		if r.Validate(b, Limits{}) == nil {
			t.Errorf("%s accepted", name)
		}
	}
	c := adv(ImageBased, render, key)
	for resp, ok := range map[string]bool{
		`{"points":[{"x":100,"y":120},{"x":600,"y":420}]}`:                   true,  // circle + concave polygon
		`{"points":[{"x":100,"y":100},{"x":110,"y":110}]}`:                   false, // same region twice: liver missing
		`{"points":[{"x":600,"y":470}]}`:                                     false, // in the notch of the concave polygon
		`{"points":[{"x":100,"y":100},{"x":600,"y":420},{"x":320,"y":120}]}`: false, // over max_selections
	} {
		g, err := grade(t, r, c, resp)
		if err != nil || g.IsCorrect != ok {
			t.Errorf("%s: %v %v", resp, g.IsCorrect, err)
		}
	}
	for _, bad := range []string{`{"points":[{"x":900,"y":10}]}`, `{"points":[]}`, `{"points":[` + strings.TrimSuffix(strings.Repeat(`{"x":1,"y":1},`, 11), ",") + `]}`,
		`{"label_placements":{"l_a":"d_1"}}`} {
		if _, err := grade(t, r, c, bad); !errors.Is(err, ErrInvalidResponse) {
			t.Errorf("%s: %v", bad, err)
		}
	}
	out, _, err := r.Render(c, 1)
	b, _ := json.Marshal(out)
	if err != nil || strings.Contains(string(b), "regions") || strings.Contains(string(b), "r_heart") || !strings.Contains(string(b), "max_selections") {
		t.Fatalf("render %s %v", b, err)
	}
	// label
	lr := `{"interaction":"label","image_media_ref":"` + img.String() + `","image_size":{"w":800,"h":600},` +
		`"labels":[{"id":"l_a","content":{"blocks":[]}},{"id":"l_b","content":{"blocks":[]}}],"drop_points":[{"id":"d_1","x":10,"y":10},{"id":"d_2","x":20,"y":20}]}`
	lk := `{"placements":{"l_a":"d_1","l_b":"d_2"}}`
	la := authoring(ImageBased, lr, lk)
	la.MediaRefs = []uuid.UUID{img}
	if err := r.Validate(la, Limits{}); err != nil {
		t.Fatal(err)
	}
	la.AnswerKey = json.RawMessage(`{"placements":{"l_a":"d_9"}}`)
	if r.Validate(la, Limits{}) == nil {
		t.Fatal("unknown drop point accepted")
	}
	lc := adv(ImageBased, lr, lk)
	lc.Scoring = ScoringRule{Mode: ModePartial, PartialMethod: PartialProportional}
	g, err := grade(t, r, lc, `{"label_placements":{"l_a":"d_1","l_b":"d_1"}}`)
	if err != nil || g.Score != 0.5 {
		t.Fatalf("label partial %+v %v", g, err)
	}
	if _, err := grade(t, r, lc, `{"label_placements":{"l_z":"d_1"}}`); !errors.Is(err, ErrInvalidResponse) {
		t.Fatal("unknown label")
	}
	out, _, _ = r.Render(lc, 2)
	b, _ = json.Marshal(out)
	if strings.Contains(string(b), "placements") || !strings.Contains(string(b), "drop_points") {
		t.Fatalf("label render %s", b)
	}
}

// F023-AC-09, AC-10, AC-11, AC-12, SC-002: interactive widget validation and replay.
func TestInteractive(t *testing.T) {
	r := Full()
	render := `{"widget":"balance_scale","widget_version":"1.0","config":{"left_weights":[5,3],"available_weights":[1,2,4,8],"max_moves":6}}`
	key := `{"validator":"balance_scale.equal","params":{"target":8}}`
	if err := r.Validate(authoring(Interactive, render, key), Limits{}); err != nil {
		t.Fatal(err)
	}
	for name, a := range map[string]Authoring{
		"widget":    authoring(Interactive, strings.Replace(render, `"1.0"`, `"9.0"`, 1), key),
		"validator": authoring(Interactive, render, `{"validator":"balance_scale.sum"}`),
		"config":    authoring(Interactive, strings.Replace(render, `"max_moves":6`, `"max_moves":99`, 1), key),
		"params":    authoring(Interactive, render, `{"validator":"balance_scale.equal","params":{"evil":"os.exit"}}`),
	} {
		if r.Validate(a, Limits{}) == nil {
			t.Errorf("%s accepted", name)
		}
	}
	c := adv(Interactive, render, key)
	c.Scoring.RequireActionLog = true
	place := func(ws ...int) string {
		var out []string
		for i, w := range ws {
			out = append(out, `{"t_ms":`+string(rune('1'+i))+`00,"action":"place","args":{"w":`+itoa(w)+`}}`)
		}
		return "[" + strings.Join(out, ",") + "]"
	}
	g, err := grade(t, r, c, `{"state":{"right_weights":[8]},"action_log":`+place(8)+`}`)
	if err != nil || !g.IsCorrect || g.IntegrityMismatch {
		t.Fatalf("honest: %+v %v", g, err)
	}
	g, err = grade(t, r, c, `{"state":{"right_weights":[8]},"action_log":`+place(4)+`}`)
	if err != nil || g.IsCorrect || !g.IntegrityMismatch || g.Score != 0 {
		t.Fatalf("forged: %+v %v", g, err)
	}
	g, _ = grade(t, r, c, `{"state":{"right_weights":[8,8]},"action_log":`+place(8, 8)+`}`)
	if g.IsCorrect || !g.IntegrityMismatch {
		t.Fatalf("weight used twice: %+v", g)
	}
	g, _ = grade(t, r, c, `{"state":{"right_weights":[1,2,4]},"action_log":`+place(1, 2, 4)+`}`)
	if g.IsCorrect || g.IntegrityMismatch {
		t.Fatalf("honest wrong: %+v", g)
	}
	long := make([]string, 501)
	for i := range long {
		long[i] = `{"t_ms":1,"action":"place","args":{"w":1}}`
	}
	if _, err := grade(t, r, c, `{"state":{"right_weights":[]},"action_log":[`+strings.Join(long, ",")+`]}`); !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("501 actions: %v", err)
	}
	out, _, _ := r.Render(c, 1)
	b, _ := json.Marshal(out)
	if strings.Contains(string(b), "validator") || strings.Contains(string(b), "target") {
		t.Fatalf("render %s", b)
	}
	if len(r.Widgets().List()) != 1 || r.Widgets().List()[0].Validators[0] != "balance_scale.equal" {
		t.Fatal("widgets list")
	}
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// F023-AC-14, AC-16, AC-17, AC-18: a parameterized numeric question is rendered with its values and graded on them.
func TestParameterized(t *testing.T) {
	r := Full()
	render := `{"keyboard":"numeric","params":{"a":{"type":"int","min":6,"max":9},"b":{"type":"int","min":2,"max":9},"constraints":["a != b"]}}`
	a := authoring(Numeric, render, `{"value":"{{a}} * {{b}}"}`)
	a.Stem = json.RawMessage(`{"blocks":[{"type":"paragraph","text":"{{a}} × {{b}} = ?"}]}`)
	if err := r.Validate(a, Limits{}); err != nil {
		t.Fatal(err)
	}
	for name, b := range map[string]Authoring{
		"undeclared": func() Authoring {
			x := a
			x.Stem = json.RawMessage(`{"blocks":[{"type":"paragraph","text":"{{c}}"}]}`)
			return x
		}(),
		"function": func() Authoring { x := a; x.AnswerKey = json.RawMessage(`{"value":"sqrt({{a}})"}`); return x }(),
		"infeasible": func() Authoring {
			x := a
			x.RenderPayload = json.RawMessage(`{"params":{"a":{"type":"int","min":1,"max":2},"constraints":["a > 3"]}}`)
			x.Stem = json.RawMessage(`{"blocks":[]}`)
			x.AnswerKey = json.RawMessage(`{"value":"{{a}}"}`)
			return x
		}(),
		"no params": func() Authoring { x := a; x.RenderPayload = json.RawMessage(`{"keyboard":"numeric"}`); return x }(),
		"tf params": authoring(TrueFalse, `{"statements":[],"params":{"a":{"type":"int","min":1,"max":2}}}`, `{}`),
	} {
		if r.Validate(b, Limits{}) == nil {
			t.Errorf("%s accepted", name)
		}
	}
	c := adv(Numeric, render, `{"value":"{{a}} * {{b}}"}`)
	c.Stem = a.Stem
	for seed := range uint64(50) {
		out, pres, err := r.Render(c, seed)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(out)
		if strings.Contains(string(b), "params") || strings.Contains(string(b), "{{") || strings.Contains(string(b), "constraints") {
			t.Fatalf("render leaks %s", b)
		}
		av, bv := pres.Params["a"], pres.Params["b"]
		if av == "" || av == bv || !strings.Contains(string(b), av+" × "+bv) {
			t.Fatalf("stem %s params %v", b, pres.Params)
		}
		want := itoa(atoi(av) * atoi(bv))
		g, err := r.GradeIn(c, pres, json.RawMessage(`{"value":"`+want+`"}`))
		if err != nil || !g.IsCorrect {
			t.Fatalf("seed %d: %s × %s = %s: %+v %v", seed, av, bv, want, g, err)
		}
		g, _ = r.GradeIn(c, Presentation{Seed: seed, QuestionVersionID: c.VersionID}, json.RawMessage(`{"value":"`+want+`"}`))
		if !g.IsCorrect {
			t.Fatal("regenerated values differ from the presentation")
		}
	}
	sc := adv(SingleChoice, `{"options":[{"id":"o_a","content":{"blocks":[{"type":"paragraph","text":"{{a}}"}]}},{"id":"o_b","content":{"blocks":[]}}],`+
		`"shuffle_options":false,"params":{"a":{"type":"choice","values":[10,20]}}}`, `{"correct_option_id":"o_a"}`)
	out, pres, err := r.Render(sc, 3)
	b, _ := json.Marshal(out)
	if err != nil || !slices.Contains([]string{"10", "20"}, pres.Params["a"]) || !strings.Contains(string(b), `"text":"`+pres.Params["a"]+`"`) {
		t.Fatalf("single choice %s %v %v", b, pres.Params, err)
	}
}

func atoi(s string) int {
	var n int
	_ = json.Unmarshal([]byte(s), &n)
	return n
}
