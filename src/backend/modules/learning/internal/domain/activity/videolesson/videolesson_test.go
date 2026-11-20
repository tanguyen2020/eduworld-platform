package videolesson_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/videolesson"
)

func cfg(t *testing.T, raw string) videolesson.Config {
	t.Helper()
	c, err := videolesson.Parse(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

const sample = `{"content":{"video_asset_id":"0f650000-0000-4000-8000-000000000001","duration_ms":450000,
 "chapters":[{"chapter_id":"intro","title":{"vi":"Mở đầu"},"start_ms":0,"end_ms":60000},{"chapter_id":"main","title":{"vi":"Nội dung"},"start_ms":60000,"end_ms":450000}],
 "embedded_questions":[{"cue_id":"q2","at_ms":200000,"question_version_id":"0f650000-0000-4000-8000-0000000000b2","required":false},
  {"cue_id":"q1","at_ms":90000,"question_version_id":"0f650000-0000-4000-8000-0000000000b1","allow_skip_after_wrong":false}]},
 "interaction":{"allow_seek":"watched_only","playback_rates":[1,1.5]},"completion_rule":{"mode":"min_watched_ratio"}}`

// F065-AC-01 (T009): cue and branch times inside the video, chapters without overlap, unique ids, branch targets.
func TestValidate(t *testing.T) {
	if vs := cfg(t, sample).Validate(); len(vs) != 0 {
		t.Fatalf("valid config refused: %v", vs)
	}
	bad := `{"content":{"video_asset_id":"0f650000-0000-4000-8000-000000000001","duration_ms":100000,
	 "chapters":[{"chapter_id":"a","title":{"vi":"A"},"start_ms":0,"end_ms":60000},{"chapter_id":"b","title":{"vi":"B"},"start_ms":50000,"end_ms":40000},
	   {"chapter_id":"a","title":{"vi":"C"},"start_ms":70000,"end_ms":200000}],
	 "embedded_questions":[{"cue_id":"q","at_ms":100000,"question_version_id":"0f650000-0000-4000-8000-0000000000b1"}],
	 "segments":[{"segment_id":"land","video_asset_id":"0f650000-0000-4000-8000-000000000002"}],
	 "branches":[{"branch_point_id":"bp","at_ms":30000,"prompt":{"vi":"?"},"options":[{"option_id":"x","label":{"vi":"X"},"target_segment_id":"water"},
	   {"option_id":"x","label":{"vi":"Y"},"target_segment_id":"land"}]}]}}`
	got := map[string]bool{}
	for _, v := range cfg(t, bad).Validate() {
		got[v.Field] = true
	}
	for _, f := range []string{"/config/content/chapters/1/end_ms", "/config/content/chapters/2/end_ms", "/config/content/chapters",
		"/config/content/chapters/2", "/config/content/embedded_questions/0/at_ms", "/config/content/branches/0/options/0/target_segment_id",
		"/config/content/branches/0/options/1", "/config/content/segments"} {
		if !got[f] {
			t.Errorf("missing violation %s in %v", f, got)
		}
	}
}

func TestCueOrderAndDefaults(t *testing.T) {
	c := cfg(t, sample)
	cs := c.OrderedCues()
	if cs[0].CueID != "q1" || c.CuePosition("q2") != 2 || c.CuePosition("zz") != 0 {
		t.Fatalf("order: %+v", cs)
	}
	if !cs[0].IsRequired() || cs[0].SkipAfterWrong() || cs[1].IsRequired() || !cs[1].SkipAfterWrong() {
		t.Fatal("cue defaults")
	}
	if c.Seek() != videolesson.SeekWatchedOnly || c.MaxRate() != 1.5 || c.MinWatchedRatio() != 0.9 || !c.RequireRequiredCues() {
		t.Fatal("interaction/completion defaults")
	}
}

// F065-AC-02/03: the render payload has the cue positions and no branch target.
func TestRenderPayload(t *testing.T) {
	c := cfg(t, `{"content":{"video_asset_id":"0f650000-0000-4000-8000-000000000001","duration_ms":90000,
	 "segments":[{"segment_id":"water","video_asset_id":"0f650000-0000-4000-8000-000000000002","is_ending":true}],
	 "branches":[{"branch_point_id":"bp","at_ms":30000,"prompt":{"vi":"Đi đâu?"},"options":[{"option_id":"swim","label":{"vi":"Bơi"},"target_segment_id":"water"},
	   {"option_id":"walk","label":{"vi":"Đi bộ"},"target_segment_id":"water"}]}],
	 "embedded_questions":[{"cue_id":"q1","at_ms":20000,"question_version_id":"0f650000-0000-4000-8000-0000000000b1"}]}}`)
	b, _ := json.Marshal(videolesson.RenderPayload(c))
	s := string(b)
	if strings.Contains(s, "target_segment_id") || strings.Contains(s, "question_version_id") || strings.Contains(s, "video_asset_id") {
		t.Fatalf("leak: %s", s)
	}
	for _, want := range []string{`"position":1`, `"allow_seek":"watched_only"`, `"heartbeat_seconds":10`, `"is_ending":true`, `"min_watched_ratio":0.9`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in %s", want, s)
		}
	}
}

// F065-AC-05 (T010): merge, bound of 64 ranges, plausibility trim.
func TestMergeAndAccept(t *testing.T) {
	m := videolesson.Merge([]videolesson.Range{{5000, 9000}, {0, 5000}, {20000, 30000}, {25000, 26000}, {-10, 0}, {40000, 999999}}, 50000)
	if len(m) != 3 || m[0] != (videolesson.Range{0, 9000}) || m[2] != (videolesson.Range{40000, 50000}) {
		t.Fatalf("merge: %v", m)
	}
	var many []videolesson.Range
	for i := int64(0); i < 100; i++ {
		many = append(many, videolesson.Range{StartMS: i * 1000, EndMS: i*1000 + 500 + i})
	}
	if got := videolesson.Merge(many, 0); len(got) != videolesson.MaxRanges {
		t.Fatalf("bound: %d", len(got))
	}
	// 10 s of real time at 1.5× allows 15 s of new video: a jump to the end keeps the first 15 s only.
	a := videolesson.AcceptRanges([]videolesson.Range{{0, 30000}}, []videolesson.Range{{0, 450000}}, 10000, 1.5, 450000)
	if a.AddedMS != 15000 || a.TrimmedMS != 405000 || videolesson.Frontier(a.Ranges) != 45000 {
		t.Fatalf("accept: %+v", a)
	}
	ok := videolesson.AcceptRanges([]videolesson.Range{{0, 30000}}, []videolesson.Range{{30000, 40000}}, 10000, 1, 450000)
	if ok.TrimmedMS != 0 || ok.AddedMS != 10000 || videolesson.Ratio(ok.Ranges, 400000) != 0.1 {
		t.Fatalf("plausible: %+v", ok)
	}
}

// F065-AC-06 (T010): seek policies.
func TestCheckSeek(t *testing.T) {
	cases := []struct {
		policy    string
		pos       int64
		completed bool
		ok        bool
	}{
		{videolesson.SeekWatchedOnly, 40000, false, true}, {videolesson.SeekWatchedOnly, 40001, false, false},
		{videolesson.SeekBackwardOnly, 50000, false, false}, {videolesson.SeekFree, 400000, false, true},
		{videolesson.SeekWatchedOnly, 400000, true, true},
	}
	for _, c := range cases {
		err := videolesson.CheckSeek(c.policy, c.pos, 40000, c.completed)
		if (err == nil) != c.ok || (err != nil && !errors.Is(err, videolesson.ErrSeekNotAllowed)) {
			t.Errorf("%+v: %v", c, err)
		}
	}
}

// F065-AC-07: each milestone once.
func TestMilestones(t *testing.T) {
	ms, bits := videolesson.NewMilestones(0.6, 0)
	if len(ms) != 2 || bits != 3 {
		t.Fatalf("%v %b", ms, bits)
	}
	ms, bits = videolesson.NewMilestones(1, bits)
	if len(ms) != 2 || ms[1] != 1 || bits != 15 {
		t.Fatalf("%v %b", ms, bits)
	}
	if ms, _ := videolesson.NewMilestones(1, bits); len(ms) != 0 {
		t.Fatal("milestone emitted twice")
	}
}

// F065-AC-09, AC-10 (T011): cue processed after attempts_per_question wrong answers when skippable; completion rule.
func TestCompletion(t *testing.T) {
	c := cfg(t, sample)
	q1, _ := c.CueAt(1)
	q2, _ := c.CueAt(2)
	if videolesson.CueProcessed(q1, false, 5, 2) || !videolesson.CueProcessed(q1, true, 1, 2) {
		t.Fatal("q1 (no skip after wrong)")
	}
	if videolesson.CueProcessed(q2, false, 1, 2) || !videolesson.CueProcessed(q2, false, 2, 2) {
		t.Fatal("q2 (skip after 2 wrong)")
	}
	o := videolesson.Evaluate(c, 0.95, nil, false)
	if o.Completed || len(o.MissingRequired) != 1 || o.MissingRequired[0] != "q1" {
		t.Fatalf("missing required: %+v", o)
	}
	if o := videolesson.Evaluate(c, 0.85, []string{"q1"}, false); o.Completed {
		t.Fatal("completed below min_watched_ratio")
	}
	if o := videolesson.Evaluate(c, 0.9, []string{"q1"}, false); !o.Completed {
		t.Fatal("not completed at the ratio")
	}
	if p := videolesson.PendingCues(c, []string{"q1"}, false); len(p) != 1 || p[0] != "q2" {
		t.Fatalf("pending: %v", p)
	}
	branched := cfg(t, `{"content":{"video_asset_id":"`+uuid.NewString()+`","segments":[{"segment_id":"end","video_asset_id":"`+uuid.NewString()+`","is_ending":true}],
	 "branches":[{"branch_point_id":"bp","at_ms":1000,"prompt":{"vi":"?"},"options":[{"option_id":"a","label":{"vi":"A"},"target_segment_id":"end"},{"option_id":"b","label":{"vi":"B"},"target_segment_id":"end"}]}]}}`)
	if videolesson.Evaluate(branched, 0.1, nil, false).Completed || !videolesson.Evaluate(branched, 0.1, nil, true).Completed {
		t.Fatal("interactive completion at the ending segment")
	}
}
