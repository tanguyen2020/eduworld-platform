package contract

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/plugins/video"
)

func videoConfig(t *testing.T) activity.Config {
	t.Helper()
	raw := `{"content":{"video_asset_id":"` + uuid.NewString() + `","duration_ms":450000,
	 "chapters":[{"chapter_id":"intro","title":{"vi":"Mở đầu","en":"Intro"},"start_ms":0,"end_ms":60000}],
	 "subtitles":[{"language":"vi","subtitle_asset_id":"` + uuid.NewString() + `","is_default":true}],
	 "embedded_questions":[{"cue_id":"q1","at_ms":90000,"question_version_id":"` + uuid.NewString() + `"}],
	 "segments":[{"segment_id":"water","video_asset_id":"` + uuid.NewString() + `","is_ending":true},
	   {"segment_id":"land","video_asset_id":"` + uuid.NewString() + `","is_ending":true}],
	 "branches":[{"branch_point_id":"bp1","at_ms":300000,"prompt":{"vi":"Con vịt đi đâu?"},
	   "options":[{"option_id":"swim","label":{"vi":"Xuống nước"},"target_segment_id":"water"},{"option_id":"walk","label":{"vi":"Lên bờ"},"target_segment_id":"land"}]}]},
	 "interaction":{"allow_seek":"watched_only","playback_rates":[0.75,1,1.25]},
	 "evaluation":{"strategies":["completion","auto"],"max_score":1},"feedback":{"timing":"immediate"},
	 "completion_rule":{"mode":"min_watched_ratio","min_watched_ratio":0.9,"require_required_cues":true}}`
	c, err := activity.ParseConfig(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "..")
}

// F065-AC-02, AC-03 (T005, T012, TC-ACT-001): the render payload of video_lesson matches
// video-lesson.schema.json#/$defs/render (closed) and never holds a branch target, a question version, an asset id or
// an answer.
func TestVideoLessonRenderPayload(t *testing.T) {
	p := video.New()
	v := activity.Version{ActivityID: uuid.New(), VersionID: uuid.New(), VersionNo: 1, Type: activity.VideoLesson, Config: videoConfig(t)}
	if vs := p.ValidateConfig(v.Config); len(vs) != 0 {
		t.Fatalf("valid config refused: %v", vs)
	}
	b, err := json.Marshal(p.BuildRenderPayload(v))
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range append([]string{"target_segment_id", "question_version_id", "video_asset_id", "subtitle_asset_id"}, answerKeys...) {
		if strings.Contains(string(b), `"`+leak+`"`) {
			t.Fatalf("render payload leaks %q: %s", leak, b)
		}
	}
	root := repoRoot(t)
	c := jsonschema.NewCompiler()
	for _, f := range []string{"activity-base.schema.json", "video-lesson.schema.json"} {
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
	s, err := c.Compile("https://schemas.eduworld.vn/content/activities/video-lesson.schema.json#/$defs/render")
	if err != nil {
		t.Fatal(err)
	}
	inst, _ := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err := s.Validate(inst); err != nil {
		t.Fatalf("render payload does not match $defs/render: %v\n%s", err, b)
	}
	cues, err := p.(activity.Timeline).CueQuestions(v)
	if err != nil || len(cues) != 1 {
		t.Fatalf("cue questions: %v %v", cues, err)
	}
	if r := p.CompletionRule(v); r.Mode != "min_watched_ratio" || r.MinWatchedRatio == nil || *r.MinWatchedRatio != 0.9 {
		t.Fatalf("completion rule: %+v", r)
	}
}
