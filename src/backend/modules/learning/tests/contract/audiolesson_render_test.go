package contract

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/plugins/audio"
)

// renderSchema compiles specs/schemas/activities/<file>#/$defs/render.
func renderSchema(t *testing.T, file string) *jsonschema.Schema {
	t.Helper()
	root := repoRoot(t)
	c := jsonschema.NewCompiler()
	for _, f := range []string{"activity-base.schema.json", file} {
		raw, err := os.ReadFile(filepath.Join(root, "specs", "schemas", "activities", f))
		if err != nil {
			t.Fatal(err)
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		if err := c.AddResource("https://schemas.eduworld.vn/content/activities/"+f, doc); err != nil {
			t.Fatal(err)
		}
	}
	enums, err := os.ReadFile(filepath.Join(root, "specs", "schemas", "common", "enums.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	ed, _ := jsonschema.UnmarshalJSON(bytes.NewReader(enums))
	_ = c.AddResource("https://schemas.eduworld.vn/common/enums.schema.json", ed)
	s, err := c.Compile("https://schemas.eduworld.vn/content/activities/" + file + "#/$defs/render")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func audioConfig(t *testing.T, extra string) activity.Config {
	t.Helper()
	raw := `{"content":{"audio_asset_id":"` + uuid.NewString() + `","transcript_asset_id":"` + uuid.NewString() + `",
	 "segments":[{"segment_id":"intro","title":{"vi":"Chào hỏi","en":"Greeting"},"start_ms":0,"end_ms":20000},
	   {"segment_id":"name","start_ms":20000,"end_ms":45000}],
	 "questions":[{"question_version_id":"` + uuid.NewString() + `"},{"question_version_id":"` + uuid.NewString() + `","at_ms":21000}]},
	 "interaction":{"max_plays":2,"allow_speed_change":true,"show_transcript":"after_complete"` + extra + `},
	 "evaluation":{"strategies":["completion","auto"],"max_score":2},"feedback":{"timing":"immediate"},
	 "completion_rule":{"mode":"min_watched_ratio","min_watched_ratio":0.9}}`
	c, err := activity.ParseConfig(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// F066-AC-02, AC-03 (T005, T012, TC-ACT-001): the render payload of audio_lesson matches
// audio-lesson.schema.json#/$defs/render (closed), the timed question comes first, and the payload never holds a
// question version, an asset id or an answer; Evaluate is idempotent.
func TestAudioLessonRenderPayload(t *testing.T) {
	p := audio.New()
	v := activity.Version{ActivityID: uuid.New(), VersionID: uuid.New(), VersionNo: 1, Type: activity.AudioLesson, Config: audioConfig(t, "")}
	if vs := p.ValidateConfig(v.Config); len(vs) != 0 {
		t.Fatalf("valid config refused: %v", vs)
	}
	payload := p.BuildRenderPayload(v)
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range append([]string{"question_version_id", "audio_asset_id", "transcript_asset_id"}, answerKeys...) {
		if strings.Contains(string(b), `"`+leak+`"`) {
			t.Fatalf("render payload leaks %q: %s", leak, b)
		}
	}
	inst, _ := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err := renderSchema(t, "audio-lesson.schema.json").Validate(inst); err != nil {
		t.Fatalf("render payload does not match $defs/render: %v\n%s", err, b)
	}
	var got struct {
		Questions []struct {
			CueID    string `json:"cue_id"`
			Position int    `json:"position"`
			AtMS     *int64 `json:"at_ms"`
		} `json:"questions"`
		Interaction struct {
			MaxPlays      *int      `json:"max_plays"`
			PlaybackRates []float64 `json:"playback_rates"`
		} `json:"interaction"`
	}
	_ = json.Unmarshal(b, &got)
	if len(got.Questions) != 2 || got.Questions[0].CueID != "q02" || got.Questions[0].AtMS == nil || *got.Questions[0].AtMS != 21000 ||
		got.Questions[1].CueID != "q01" || got.Questions[1].AtMS != nil || got.Questions[1].Position != 2 {
		t.Fatalf("questions by at_ms then after the end: %s", b)
	}
	if got.Interaction.MaxPlays == nil || *got.Interaction.MaxPlays != 2 || len(got.Interaction.PlaybackRates) != 2 {
		t.Fatalf("interaction: %s", b)
	}
	items := []activity.ItemResult{{Position: 1, MaxScore: 1, Score: 1, IsCorrect: true, Answered: true}, {Position: 2, MaxScore: 1}}
	r1, r2 := p.Evaluate(v, items), p.Evaluate(v, items)
	if r1.Score != 1 || r1.MaxScore != 2 || r1.Score != r2.Score || r1.CorrectCount != r2.CorrectCount {
		t.Fatalf("evaluate: %+v %+v", r1, r2)
	}
	if r := p.CompletionRule(v); r.Mode != "min_watched_ratio" {
		t.Fatalf("completion rule: %+v", r)
	}
}
