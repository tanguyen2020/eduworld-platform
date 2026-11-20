package audiolesson

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/videolesson"
)

func parse(t *testing.T, content, interaction string) Config {
	t.Helper()
	c, err := Parse(json.RawMessage(`{"content":` + content + `,"interaction":` + interaction + `}`))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func fields(vs interface{ Error() string }) string { return vs.Error() }

// F066-AC-01 (T010): segments start < end, inside the audio, no overlap, unique ids; at_ms before the end; a
// transcript only at publish.
func TestValidate(t *testing.T) {
	audio := uuid.NewString()
	ok := parse(t, `{"audio_asset_id":"`+audio+`","segments":[{"segment_id":"a","start_ms":0,"end_ms":1000},{"segment_id":"b","start_ms":1000,"end_ms":2000}],
	 "questions":[{"question_version_id":"`+uuid.NewString()+`","at_ms":1500}]}`, `{}`)
	if vs := ok.Validate(2000); len(vs) != 0 {
		t.Fatalf("valid: %v", vs)
	}
	if vs := ok.ValidatePublish(); len(vs) != 1 || vs[0].Field != "/config/content/transcript_asset_id" {
		t.Fatalf("publish without transcript: %v", vs)
	}
	bad := parse(t, `{"audio_asset_id":"`+audio+`","segments":[{"segment_id":"a","start_ms":0,"end_ms":1500},{"segment_id":"a","start_ms":1000,"end_ms":900},
	 {"segment_id":"c","start_ms":1600,"end_ms":2500}],"questions":[{"question_version_id":"`+uuid.NewString()+`","at_ms":2000}]}`, `{}`)
	vs := bad.Validate(2000)
	got := fields(vs)
	for _, want := range []string{"segments/1/end_ms", "segments/1/segment_id", "overlap", "segments/2/end_ms", "questions/0/at_ms"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
	// Unknown duration: only the relative rules.
	if vs := bad.Validate(0); strings.Contains(fields(vs), "questions/0/at_ms") || strings.Contains(fields(vs), "segments/2/end_ms") {
		t.Fatalf("duration rules without duration: %v", vs)
	}
}

// F066-AC-04 (T011): one play per accepted play_started, refused at max_plays, unlimited without it.
func TestAcceptPlay(t *testing.T) {
	two := 2
	used, err := AcceptPlay(0, &two)
	if err != nil || used != 1 {
		t.Fatal(used, err)
	}
	if used, err = AcceptPlay(used, &two); err != nil || used != 2 || PlaysLeft(used, &two) != 0 {
		t.Fatal(used, err)
	}
	if used, err = AcceptPlay(used, &two); !errors.Is(err, ErrPlayLimit) || used != 2 {
		t.Fatal(used, err)
	}
	if used, err = AcceptPlay(40, nil); err != nil || used != 41 || PlaysLeft(used, nil) != -1 {
		t.Fatal(used, err)
	}
}

// F066-AC-02, AC-07: the timeline orders the questions by at_ms then declaration order, questions are not required
// cues, the rates follow allow_speed_change and the transcript mode defaults to after_complete.
func TestTimeline(t *testing.T) {
	q1, q2, q3 := uuid.New(), uuid.New(), uuid.New()
	c := parse(t, `{"audio_asset_id":"`+uuid.NewString()+`","questions":[{"question_version_id":"`+q1.String()+`"},
	 {"question_version_id":"`+q2.String()+`","at_ms":5000},{"question_version_id":"`+q3.String()+`","at_ms":5000}]}`, `{"allow_speed_change":false}`)
	ids := c.QuestionIDs()
	if len(ids) != 3 || ids[0] != q2 || ids[1] != q3 || ids[2] != q1 {
		t.Fatalf("order: %v", ids)
	}
	tl := c.Timeline()
	if !tl.Audio() || tl.Seek() != videolesson.SeekFree || tl.MaxRate() != 1 || tl.ShowTranscript != TranscriptAfterComplete {
		t.Fatalf("timeline: %+v", tl)
	}
	if o := videolesson.Evaluate(tl, 0.95, nil, false); !o.Completed || len(o.MissingRequired) != 0 {
		t.Fatalf("questions must not block completion: %+v", o)
	}
	if pos := tl.CuePosition("q01"); pos != 3 {
		t.Fatalf("cue position: %d", pos)
	}
}
