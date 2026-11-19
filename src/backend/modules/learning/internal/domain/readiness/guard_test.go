package readiness

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

func pointers(vs []Violation) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v.Pointer
	}
	return out
}

const stemNoAudio = `{"stem":{"blocks":[{"type":"paragraph","text":"7 × 8 = ?"}]},"render_payload":{"options":[{"id":"o_1","content":{"blocks":[{"type":"paragraph","text":"56"}]}}]}}`
const stemAudio = `{"stem":{"blocks":[{"type":"paragraph","text":"7 × 8 = ?"}],"audio_ref":"0193a5f4-6c1e-7b2a-9d1f-000000000700"}}`

// F025-AC-15, AC-16: g1_3 needs the stem audio of a question, the instructions audio of an activity, the audio of every
// worked example step of a lesson; grades {3, 4} still count; an exemption lifts the rule; grades 4–9 need nothing.
func TestAudioRule(t *testing.T) {
	g := Default()
	q := Subject{Type: contentmodel.TypeQuestion, GradeLevels: []int{2}, ContentLanguage: "vi", Doc: json.RawMessage(stemNoAudio)}
	r := g.Check(q)
	if r.Code != CodeAudioRequired || !slices.Equal(pointers(r.Violations), []string{"/stem/audio_ref"}) {
		t.Fatalf("question g2 %+v", r)
	}
	if len(r.Warnings) != 1 || r.Warnings[0].Pointer != "/render_payload/options/0/content/audio_ref" {
		t.Fatalf("choice warning %+v", r.Warnings)
	}
	q.GradeLevels = []int{3, 4}
	if g.Check(q).Code != CodeAudioRequired {
		t.Fatal("grades 3,4 are g1_3")
	}
	q.GradeLevels = []int{4, 5}
	if r := g.Check(q); !r.OK() || len(r.Warnings) != 0 {
		t.Fatalf("grades 4,5 %+v", r)
	}
	q.GradeLevels, q.AudioExemptReason = []int{1}, "đọc to lộ đáp án chính tả"
	if !g.Check(q).OK() {
		t.Fatal("exemption")
	}
	q = Subject{Type: contentmodel.TypeQuestion, GradeLevels: []int{1}, ContentLanguage: "en", Doc: json.RawMessage(stemAudio)}
	if r := g.Check(q); r.Code == CodeAudioRequired {
		t.Fatalf("English subject with audio %+v", r)
	}
	act := Subject{Type: contentmodel.TypeActivity, GradeLevels: []int{1}, ContentLanguage: "vi", Doc: json.RawMessage(`{"instructions":{"vi":"Chọn đáp án"}}`)}
	if r := g.Check(act); !slices.Equal(pointers(r.Violations), []string{"/instructions_audio_ref"}) {
		t.Fatalf("activity %+v", r)
	}
	lesson := Subject{Type: contentmodel.TypeLesson, GradeLevels: []int{2}, ContentLanguage: "vi", Doc: json.RawMessage(`{"title":{"vi":"Bài 1"},
		"blocks":[{"kind":"rich_text"},{"kind":"worked_example","steps":[{"content":{}},{"content":{},"audio_ref":"0193a5f4-6c1e-7b2a-9d1f-000000000701"}]}]}`)}
	if r := g.Check(lesson); !slices.Equal(pointers(r.Violations), []string{"/blocks/1/steps/0/audio_ref"}) {
		t.Fatalf("lesson %+v", r)
	}
}

// F025-AC-02, AC-03, AC-18: the order of the codes and the asset ids of MEDIA_NOT_READY.
func TestGuardOrder(t *testing.T) {
	g := Default()
	a, b := uuid.New(), uuid.New()
	s := Subject{Type: contentmodel.TypeQuestion, GradeLevels: []int{2}, ContentLanguage: "vi",
		Doc:   json.RawMessage(`{"stem":{"blocks":[{"type":"paragraph","text":{"en":"x"}}]}}`),
		Meta:  contentmodel.Violations{{Field: "/grade_levels", Required: true, Reason: "is required"}},
		Media: map[uuid.UUID]string{a: "processing", b: "ready"}}
	if r := g.Check(s); r.Code != CodeValidation || r.Violations[0].Pointer != "/grade_levels" {
		t.Fatalf("schema first %+v", r)
	}
	s.Meta = nil
	if r := g.Check(s); r.Code != CodeMediaNotReady || len(r.Violations) != 1 || r.Violations[0].AssetID != a {
		t.Fatalf("media %+v", r)
	}
	s.Media[a] = "ready"
	if r := g.Check(s); r.Code != CodeAudioRequired {
		t.Fatalf("audio %+v", r)
	}
	s.AudioExemptReason = "x"
	if r := g.Check(s); r.Code != CodeTranslationIncomplete || !slices.Equal(pointers(r.Violations), []string{"/stem/blocks/0/text"}) {
		t.Fatalf("language %+v", r)
	}
	s.Media[a] = ""
	if r := g.Check(s); r.Code != CodeMediaNotReady {
		t.Fatalf("invisible asset %+v", r)
	}
}
