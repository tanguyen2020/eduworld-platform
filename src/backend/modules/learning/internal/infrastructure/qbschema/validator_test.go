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
	for name, b := range map[string][]byte{"question.schema.json": QuestionSchemaJSON, "common/enums.schema.json": EnumsSchemaJSON,
		"question-set.schema.json": QuestionSetSchemaJSON, "assessment-blueprint.schema.json": BlueprintSchemaJSON, "lesson.schema.json": LessonSchemaJSON,
		"activities/activity-base.schema.json": ActivityBaseSchemaJSON, "activities/quiz.schema.json": QuizSchemaJSON,
		"activities/interactive-exercise.schema.json": InteractiveExerciseSchemaJSON,
		"activities/video-lesson.schema.json":         VideoLessonSchemaJSON, "activities/audio-lesson.schema.json": AudioLessonSchemaJSON, "activities/simulation.schema.json": SimulationSchemaJSON, "learning-event.schema.json": LearningEventSchemaJSON} {
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

// F024: the wrapper schemas compile and reject structural errors with JSON pointers.
func TestDocSchemas(t *testing.T) {
	v, err := NewDocs()
	if err != nil {
		t.Fatal(err)
	}
	if vs := v.ValidateSetDefinition(json.RawMessage(`{"kind":"static","purpose":"exercise","items":[{"question_version_id":"0193a5f4-6c1e-7b2a-9d1f-000000000101"}]}`)); len(vs) != 0 {
		t.Fatalf("set: %v", vs)
	}
	if vs := v.ValidateSetDefinition(json.RawMessage(`{"kind":"other","purpose":"exercise","x":1}`)); len(vs) == 0 {
		t.Fatal("bad set accepted")
	}
	if vs := v.ValidateBlocks(json.RawMessage(`[{"kind":"rich_text","content":{"blocks":[{"type":"paragraph","text":"x"}]}}]`)); len(vs) != 0 {
		t.Fatalf("blocks: %v", vs)
	}
	if vs := v.ValidateBlocks(json.RawMessage(`[{"kind":"worked_example","id":"we_a","problem":{"blocks":[]},"steps":[],"objective_ids":[]}]`)); len(vs) == 0 {
		t.Fatal("bad worked example accepted")
	}
	bp := `{"duration_sec":2400,"total_points":10,"attempt_policy":{"max_attempts":1,"score_policy":"best"},"result_release":{"mode":"manual"},
		"sections":[{"id":"sec_a","title":"Phần 1","rules":[{"objective_ids":["0193a5f4-6c1e-7b2a-9d1f-000000000101"],"count":10,"points_each":1}]}]}`
	if vs := v.ValidateBlueprintDefinition(json.RawMessage(bp)); len(vs) != 0 {
		t.Fatalf("blueprint: %v", vs)
	}
}
