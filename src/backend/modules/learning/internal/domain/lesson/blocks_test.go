package lesson

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func blocks(t *testing.T, raw string) []Block {
	t.Helper()
	var b []Block
	if err := json.Unmarshal([]byte(raw), &b); err != nil {
		t.Fatal(err)
	}
	return b
}

func reasons(t *testing.T, raw string, o Options) string {
	t.Helper()
	_, vs := Validate(blocks(t, raw), o)
	var out []string
	for _, v := range vs {
		out = append(out, v.Field+":"+v.Reason)
	}
	return strings.Join(out, "|")
}

// F024-AC-06, AC-07: allowed kinds, activity not enabled, worked example rules.
func TestBlocks(t *testing.T) {
	lo := uuid.NewString()
	rich := `{"blocks":[{"type":"paragraph","text":"x"}]}`
	we := func(id string, steps int) string {
		s := make([]string, steps)
		for i := range s {
			s[i] = `{"content":` + rich + `}`
		}
		return `{"kind":"worked_example","id":"` + id + `","problem":` + rich + `,"steps":[` + strings.Join(s, ",") + `],"objective_ids":["` + lo + `"]}`
	}
	ok := `[{"kind":"rich_text","content":` + rich + `},{"kind":"media","media_ref":"` + uuid.NewString() + `"},` + we("we_mul7", 3) + `]`
	res, vs := Validate(blocks(t, ok), Options{})
	if len(vs) != 0 || len(res.WorkedExamples) != 1 || res.WorkedExampleIDs()[0] != "we_mul7" || len(res.MediaRefs) != 1 {
		t.Fatalf("valid: %v %+v", vs, res)
	}
	act := `[{"kind":"activity","activity_id":"` + uuid.NewString() + `"}]`
	if r := reasons(t, act, Options{}); !strings.Contains(r, SubBlockKindNotEnabled) {
		t.Fatalf("activity: %s", r)
	}
	if r := reasons(t, act, Options{ActivityEnabled: true}); r != "" {
		t.Fatalf("activity enabled: %s", r)
	}
	if r := reasons(t, `[`+we("we_a", 9)+`]`, Options{}); !strings.Contains(r, "/blocks/0/steps") {
		t.Fatalf("9 steps: %s", r)
	}
	if r := reasons(t, `[`+we("we_a", 1)+`,`+we("we_a", 1)+`]`, Options{}); !strings.Contains(r, "/blocks/1/id") {
		t.Fatalf("duplicate id: %s", r)
	}
	if r := reasons(t, `[`+we("WE-1", 1)+`]`, Options{}); !strings.Contains(r, "/blocks/0/id") {
		t.Fatalf("bad id: %s", r)
	}
	nestedQ := strings.Replace(we("we_b", 1), `"problem":`+rich, `"problem":{"blocks":[{"type":"question","question_version_id":"`+uuid.NewString()+`"}]}`, 1)
	if r := reasons(t, `[`+nestedQ+`]`, Options{}); !strings.Contains(r, "cannot contain") {
		t.Fatalf("nested question: %s", r)
	}
	noLO := strings.Replace(we("we_c", 1), `"objective_ids":["`+lo+`"]`, `"objective_ids":[]`, 1)
	if r := reasons(t, `[`+noLO+`]`, Options{}); !strings.Contains(r, "/blocks/0/objective_ids") {
		t.Fatalf("no LO: %s", r)
	}
	if r := reasons(t, `[]`, Options{}); !strings.Contains(r, "/blocks") {
		t.Fatalf("empty: %s", r)
	}
	if r := reasons(t, `[{"kind":"video"}]`, Options{}); !strings.Contains(r, "/blocks/0/kind") {
		t.Fatalf("unknown kind: %s", r)
	}
	w, found := Find(blocks(t, ok), "we_mul7")
	if !found || !SharesObjective(w.ObjectiveIDs, []uuid.UUID{uuid.MustParse(lo)}) || SharesObjective(w.ObjectiveIDs, []uuid.UUID{uuid.New()}) {
		t.Fatal("find/shares")
	}
}
