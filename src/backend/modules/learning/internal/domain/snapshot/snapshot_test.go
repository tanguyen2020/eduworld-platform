package snapshot

import (
	"encoding/json"
	"slices"
	"testing"
)

// F026-AC-12: no answer key survives Strip, at any depth.
func TestStrip(t *testing.T) {
	doc := json.RawMessage(`{"stem":{"blocks":[]},"answer_key":{"correct_option_id":"o_1"},"scoring_rule":{"mode":"x"},"hints":[],
		"render_payload":{"options":[{"id":"o_1","feedback":"đúng","misconception_tag":"m"}]},"explanation":{},"min_response_ms":100,
		"items":[{"accepted_answers":["a"],"statement_values":{"s":true}}]}`)
	if got := Leaks(doc); len(got) != 10 {
		t.Fatalf("leaks before: %v", got)
	}
	out, err := Strip(doc)
	if err != nil || len(Leaks(out)) != 0 {
		t.Fatalf("strip: %v %s", err, out)
	}
	if SHA256([]byte("a")) != "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb" {
		t.Fatal("sha")
	}
}

// F026-AC-21: diff by pointer; answer pointers are hidden without the review permission.
func TestDiff(t *testing.T) {
	a := json.RawMessage(`{"stem":{"text":"7 × 8"},"answer_key":{"v":"56"},"blocks":[1,2],"x":1}`)
	b := json.RawMessage(`{"stem":{"text":"7 × 9"},"answer_key":{"v":"63"},"blocks":[1],"y":2}`)
	cs := Diff(a, b)
	paths := []string{}
	for _, c := range cs {
		paths = append(paths, c.Path+":"+string(c.Op))
	}
	want := []string{"/answer_key/v:changed", "/blocks/1:removed", "/stem/text:changed", "/x:removed", "/y:added"}
	if !slices.Equal(paths, want) {
		t.Fatalf("diff %v", paths)
	}
	if len(HideAnswers(cs)) != 4 {
		t.Fatal("hide answers")
	}
	if len(Diff(a, a)) != 0 {
		t.Fatal("same doc")
	}
}
