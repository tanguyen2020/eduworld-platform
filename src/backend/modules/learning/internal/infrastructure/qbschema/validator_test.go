package qbschema

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
)

func repo(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "..", "..")
}

// The embedded copies equal specs/schemas (one source of truth, AGENTS R33).
func TestEmbeddedSchemas(t *testing.T) {
	for name, b := range map[string][]byte{"question.schema.json": QuestionSchemaJSON, "common/enums.schema.json": EnumsSchemaJSON} {
		src, err := os.ReadFile(filepath.Join(repo(t), "specs", "schemas", filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n")), bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))) {
			t.Errorf("embedded %s differs from specs/schemas: copy it again", name)
		}
	}
}

type fixture struct {
	Type          string            `json:"type"`
	Stem          json.RawMessage   `json:"stem"`
	RenderPayload json.RawMessage   `json:"render_payload"`
	AnswerKey     json.RawMessage   `json:"answer_key"`
	ScoringRule   json.RawMessage   `json:"scoring_rule"`
	Hints         []json.RawMessage `json:"hints"`
}

// F022-AC-01, AC-03, AC-05, AC-12, AC-19: the QUESTION_BANK §3 examples are valid; structural errors carry JSON
// pointers; responses are checked against $defs.<type>Response.
func TestValidate(t *testing.T) {
	v, err := New()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(repo(t), "src", "backend", "modules", "learning", "internal", "domain", "question", "testdata", "qb_examples")
	entries, _ := os.ReadDir(dir)
	if len(entries) != 8 {
		t.Fatalf("%d fixtures", len(entries))
	}
	for _, e := range entries {
		b, _ := os.ReadFile(filepath.Join(dir, e.Name()))
		var f fixture
		_ = json.Unmarshal(b, &f)
		doc := port.QuestionDoc{Type: f.Type, Stem: f.Stem, RenderPayload: f.RenderPayload, AnswerKey: f.AnswerKey, ScoringRule: f.ScoringRule, Hints: f.Hints}
		if got := v.ValidateAuthoring(doc); len(got) != 0 {
			t.Errorf("%s: %v", e.Name(), got)
		}
	}
	bad := port.QuestionDoc{Type: "single_choice", Stem: json.RawMessage(`{"blocks":[{"type":"worked_example"}]}`),
		RenderPayload: json.RawMessage(`{"options":[{"id":"A","content":{"blocks":[{"type":"paragraph","text":"x"}]}}],"extra":1}`),
		AnswerKey:     json.RawMessage(`{"correct_option_id":"o_1"}`), ScoringRule: json.RawMessage(`{"mode":"partial"}`)}
	got := v.ValidateAuthoring(bad)
	fields := map[string]bool{}
	for _, x := range got {
		fields[x.Field] = true
	}
	for _, want := range []string{"/stem/blocks/0/type", "/render_payload/options/0/id", "/scoring_rule"} {
		if !fields[want] {
			t.Errorf("missing %s in %v", want, got)
		}
	}
	long := port.QuestionDoc{Type: "fill_in_blank", Stem: json.RawMessage(`{"blocks":[{"type":"paragraph","text":"{{blank:b_1}}"}]}`),
		RenderPayload: json.RawMessage(`{"blanks":[{"id":"b_1","input":"text"}]}`),
		AnswerKey:     json.RawMessage(`{"blanks":{"b_1":{"accepted":["` + string(bytes.Repeat([]byte("x"), 300)) + `"]}}}`), ScoringRule: json.RawMessage(`{"mode":"all_or_nothing"}`)}
	if got := v.ValidateAuthoring(long); len(got) == 0 || got[0].Field != "/answer_key/blanks/b_1/accepted/0" {
		t.Errorf("accepted of 300 characters: %v", got)
	}
	if v.ValidateResponse("single_choice", json.RawMessage(`{"option_id":"o_1"}`)) != nil ||
		v.ValidateResponse("single_choice", json.RawMessage(`{"option_id":"o_1","score":1}`)) == nil ||
		v.ValidateResponse("numeric", json.RawMessage(`{"value":""}`)) == nil || v.ValidateResponse("x", nil) == nil {
		t.Fatal("responses")
	}
}
