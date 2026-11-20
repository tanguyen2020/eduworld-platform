//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

const oggVariants = `{"ogg":{"key":"media/{sha}/ogg.ogg","bytes":1024,"mime":"audio/ogg"},"mp3":{"key":"media/{sha}/mp3.mp3","bytes":1024,"mime":"audio/mpeg"}}`

// audioConfig is an audio_lesson config over a 100 s audio (videoAsset probes 100 000 ms): two segments, question qAfter
// asked after the audio and question qTimed at 30 s, two plays, transcript after completion, completion at 90 %.
func audioConfig(audio, transcript, qAfter, qTimed string) map[string]any {
	content := map[string]any{"audio_asset_id": audio,
		"segments": []any{map[string]any{"segment_id": "greet", "title": map[string]string{"vi": "Chào hỏi"}, "start_ms": 0, "end_ms": 40000},
			map[string]any{"segment_id": "name", "start_ms": 40000, "end_ms": 100000}},
		"questions": []any{map[string]any{"question_version_id": qAfter}, map[string]any{"question_version_id": qTimed, "at_ms": 30000}}}
	if transcript != "" {
		content["transcript_asset_id"] = transcript
	}
	return map[string]any{
		"content":         content,
		"interaction":     map[string]any{"max_plays": 2, "allow_speed_change": true, "show_transcript": "after_complete"},
		"evaluation":      map[string]any{"strategies": []string{"completion", "auto"}, "max_score": 2},
		"feedback":        map[string]any{"timing": "immediate"},
		"completion_rule": map[string]any{"mode": "min_watched_ratio", "min_watched_ratio": 0.9},
	}
}

type audioSetup struct {
	attSetup
	audio, transcript, activityID string
}

func (c *curEnv) audioSetup(ctx context.Context, t *testing.T, mutate func(map[string]any)) audioSetup {
	t.Helper()
	s := audioSetup{attSetup: c.attemptSetup(ctx, t, nil)}
	s.audio = c.videoAsset(ctx, t, "audio", "ready", oggVariants)
	s.transcript = c.videoAsset(ctx, t, "document", "ready", `{"original":{"key":"media/{sha}/transcript.txt","bytes":64,"mime":"text/plain"}}`)
	cfg := audioConfig(s.audio, s.transcript, s.questions[0], s.questions[1])
	if mutate != nil {
		mutate(cfg)
	}
	s.activityID = c.createActivity(t, qlcmA, activityBody(s.qc, "audio_lesson", cfg))
	c.publishActivity(ctx, t, s.activityID, 1)
	return s
}

func (c *curEnv) play(t *testing.T, sub, attemptID string, seq, position int64, ranges [][2]int64) resp {
	t.Helper()
	rs := make([]any, len(ranges))
	for i, r := range ranges {
		rs[i] = map[string]int64{"start_ms": r[0], "end_ms": r[1]}
	}
	return c.do(t, http.MethodPost, "/learning/activity-attempts/"+attemptID+"/progress", sub, nil,
		map[string]any{"client_seq": seq, "position_ms": position, "watched_ranges": rs, "play_started": true})
}

func mediaKinds(r resp) map[string]map[string]any {
	out := map[string]map[string]any{}
	render, _ := r.body["render"].(map[string]any)
	list, _ := render["media"].([]any)
	for _, m := range list {
		mm := m.(map[string]any)
		out[mm["kind"].(string)] = mm
	}
	return out
}

// F066-AC-02…AC-07 (T013, T014, T015; TC-ACT-005, TC-AUD-001, TC-LRN-003, TC-LRN-008, TC-LRN-009): start pins the
// questions by at_ms (timed first), serves the signed Ogg Vorbis variant without answer data or transcript; plays are
// counted once per client_seq and the third is 422 AUDIO_PLAY_LIMIT_REACHED without change; a jump is trimmed with an
// audio integrity signal; answers are graded on the server after the plays are used (question.answered, no
// video.cue_answered); resume keeps position, plays and answers; submit grades once with activity.completed and the
// transcript appears after completion.
func TestAudioLessonAttempt(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.audioSetup(ctx, t, nil)
	// The question after the audio is a dictation (fill_in_blank, normalized match — QUESTION_BANK §4).
	fill := must(t, c.createQ(t, qlcmA, s.qc.body(qbFixture(t, "fill_in_blank"), nil)), http.StatusCreated, "dictation").str("content_id")
	dictation := c.publishQuestion(ctx, t, fill, 1)
	s.activityID = c.createActivity(t, qlcmA, activityBody(s.qc, "audio_lesson", audioConfig(s.audio, s.transcript, dictation, s.questions[1])))
	c.publishActivity(ctx, t, s.activityID, 1)

	r := must(t, c.start(t, hsAn, s.activityID, nil, nil), http.StatusCreated, "start")
	id := r.str("attempt_id")
	items := r.items("items")
	if len(items) != 2 || items[0]["question_version_id"] != s.questions[1] || items[1]["question_version_id"] != dictation {
		t.Fatalf("items: %s", r.raw)
	}
	if leaked := answerKeysIn(r.raw); len(leaked) > 0 {
		t.Fatalf("answer data in the attempt: %v %s", leaked, r.raw)
	}
	media := mediaKinds(r)
	if a := media["audio"]; a == nil || a["variant"] != "ogg" || !strings.Contains(a["url"].(string), "expires=") || media["transcript"] != nil || len(media) != 1 {
		t.Fatalf("media: %s", r.raw)
	}
	if r.str("render", "render_payload", "plays_used") != "0" || r.str("render", "render_payload", "duration_ms") != "100000" ||
		r.str("render", "render_payload", "interaction", "max_plays") != "2" || r.str("progress", "plays_used") != "0" || r.str("progress", "max_plays") != "2" {
		t.Fatalf("render: %s", r.raw)
	}

	// AC-04: two plays accepted, an old client_seq returns the state without counting, the third play is refused.
	c.clk.Advance(10 * time.Second)
	if p := must(t, c.play(t, hsAn, id, 1, 10000, [][2]int64{{0, 10000}}), http.StatusOK, "play 1"); p.str("plays_used") != "1" {
		t.Fatalf("play 1: %s", p.raw)
	}
	if p := must(t, c.play(t, hsAn, id, 1, 10000, nil), http.StatusOK, "play 1 again"); p.str("plays_used") != "1" {
		t.Fatalf("same seq counted twice: %s", p.raw)
	}
	c.clk.Advance(10 * time.Second)
	// A jump to the end with a plausible position: trimmed, audio integrity signal.
	if hb := must(t, c.heartbeat(t, hsAn, id, 2, 20000, [][2]int64{{0, 100000}}), http.StatusOK, "jump"); hb.body["watched_ratio"] != 0.2 {
		t.Fatalf("trim: %s", hb.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.integrity_signals WHERE attempt_id = $1
		AND signal_type = 'audio_progress_implausible'`, id); n != 1 {
		t.Fatalf("integrity signals: %d", n)
	}
	c.clk.Advance(10 * time.Second)
	if p := must(t, c.play(t, hsAn, id, 3, 30000, [][2]int64{{0, 30000}}), http.StatusOK, "play 2"); p.str("plays_used") != "2" {
		t.Fatalf("play 2: %s", p.raw)
	}
	c.clk.Advance(10 * time.Second)
	if p := c.play(t, hsAn, id, 4, 0, [][2]int64{{0, 40000}}); p.status != 422 || p.str("code") != "AUDIO_PLAY_LIMIT_REACHED" {
		t.Fatalf("play 3: %d %s", p.status, p.raw)
	}
	g := must(t, c.do(t, http.MethodGet, "/learning/activity-attempts/"+id, hsAn, nil, nil), http.StatusOK, "get")
	if g.str("progress", "plays_used") != "2" || g.str("progress", "position_ms") != "30000" || g.str("render", "render_payload", "plays_used") != "2" {
		t.Fatalf("state changed by the refused play: %s", g.raw)
	}

	// AC-05: answering still works after the plays are used; graded on the server.
	right, _ := correctOption()
	ans := must(t, c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+"/responses", hsAn, idem(),
		map[string]any{"kind": "question", "position": 1, "response": map[string]string{"option_id": right}}), http.StatusOK, "answer")
	if ans.body["is_correct"] != true {
		t.Fatalf("answer: %s", ans.raw)
	}
	dict := must(t, c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+"/responses", hsAn, idem(),
		map[string]any{"kind": "question", "position": 2, "response": map[string]any{"blanks": map[string]string{"b_1": "  hà   nội. ", "b_2": "Hồng"}}}),
		http.StatusOK, "dictation")
	if dict.body["is_correct"] != true {
		t.Fatalf("dictation normalized: %s", dict.raw)
	}
	if n := c.learningEvents(ctx, t, "question.answered", id); n != 2 {
		t.Fatalf("question.answered: %d", n)
	}
	if n := c.learningEvents(ctx, t, "video.cue_answered", id); n != 0 {
		t.Fatalf("video.cue_answered for audio: %d", n)
	}

	// AC-07: starting again resumes the open attempt with its position, plays and answer.
	again := must(t, c.start(t, hsAn, s.activityID, nil, nil), http.StatusCreated, "resume")
	if again.str("attempt_id") != id || again.str("resume_position_ms") != "30000" || again.str("progress", "plays_used") != "2" ||
		again.str("render", "render_payload", "resume_position_ms") != "30000" {
		t.Fatalf("resume: %s", again.raw)
	}
	if kept := must(t, c.do(t, http.MethodGet, "/learning/activity-attempts/"+id, hsAn, nil, nil), http.StatusOK, "get resumed"); len(kept.items("saved_responses")) != 2 {
		t.Fatalf("answers kept: %s", kept.raw)
	}

	// Listen to the end in plausible heartbeats (no play_started inside the play).
	for seq, pos := int64(5), int64(30000); pos < 100000; seq++ {
		c.clk.Advance(10 * time.Second)
		pos = min(pos+10000, 100000)
		must(t, c.heartbeat(t, hsAn, id, seq, pos, [][2]int64{{0, pos}}), http.StatusOK, "listen")
	}
	// AC-06: submit once, completed, both questions right; the transcript is served after completion.
	key := idem()
	sub := must(t, c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+":submit", hsAn, key, nil), http.StatusOK, "submit")
	if sub.str("status") != "graded" || sub.str("evaluation", "score") != "2" || sub.str("evaluation", "max_score") != "2" {
		t.Fatalf("submit: %s", sub.raw)
	}
	if r := c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+":submit", hsAn, key, nil); r.status != 200 || r.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+":submit", hsAn, idem(), nil); r.status != 409 || r.str("code") != "ATTEMPT_ALREADY_SUBMITTED" {
		t.Fatalf("second submit: %d %s", r.status, r.raw)
	}
	if n := c.learningEvents(ctx, t, "activity.completed", id); n != 1 {
		t.Fatalf("activity.completed: %d", n)
	}
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'activity.submitted'
		AND envelope->'payload'->>'attempt_id' = $1`, id); n != 1 {
		t.Fatalf("outbox activity.submitted: %d", n)
	}
	done := must(t, c.do(t, http.MethodGet, "/learning/activity-attempts/"+id, hsAn, nil, nil), http.StatusOK, "get graded")
	if tr := mediaKinds(done)["transcript"]; tr == nil || !strings.Contains(tr["url"].(string), "expires=") {
		t.Fatalf("transcript after completion: %s", done.raw)
	}
}

// F066-AC-01, AC-03, AC-08 (T013, T016; TC-ACT-003, TC-AUD-004): overlapping segments and a segment past the probed
// audio are 422 ACTIVITY_CONFIG_INVALID on save; a draft may miss the transcript but its publish is 422; an audio still
// processing is 422 MEDIA_NOT_READY at start without attempt; speaking is refused and never listed.
func TestAudioLessonConfigAndReadiness(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.audioSetup(ctx, t, nil)
	post := func(cfg map[string]any) resp {
		return c.do(t, http.MethodPost, "/cms/learning/activities", qlcmA, idem(), activityBody(s.qc, "audio_lesson", cfg))
	}
	overlap := audioConfig(s.audio, s.transcript, s.questions[0], s.questions[1])
	overlap["content"].(map[string]any)["segments"] = []any{map[string]any{"segment_id": "a", "start_ms": 0, "end_ms": 50000},
		map[string]any{"segment_id": "b", "start_ms": 40000, "end_ms": 60000}}
	if r := post(overlap); r.status != 422 || r.str("code") != "ACTIVITY_CONFIG_INVALID" || !strings.Contains(r.raw, "overlap") {
		t.Fatalf("overlap: %d %s", r.status, r.raw)
	}
	late := audioConfig(s.audio, s.transcript, s.questions[0], s.questions[1])
	late["content"].(map[string]any)["segments"] = []any{map[string]any{"segment_id": "a", "start_ms": 0, "end_ms": 150000}}
	if r := post(late); r.status != 422 || r.str("code") != "ACTIVITY_CONFIG_INVALID" || !strings.Contains(r.raw, "segments/0/end_ms") {
		t.Fatalf("past the audio: %d %s", r.status, r.raw)
	}

	noTranscript := c.createActivity(t, qlcmA, activityBody(s.qc, "audio_lesson", audioConfig(s.audio, "", s.questions[0], s.questions[1])))
	c.approve(ctx, t, qlcmA, "activity", noTranscript, 1)
	if r := c.publishV(ctx, t, qlcmA, "activity", noTranscript, 1, nil, nil); r.status != 422 || r.str("code") != "ACTIVITY_CONFIG_INVALID" ||
		!strings.Contains(r.raw, "transcript_asset_id") {
		t.Fatalf("publish without transcript: %d %s", r.status, r.raw)
	}

	processing := c.videoAsset(ctx, t, "audio", "processing", `{}`)
	id := c.createActivity(t, qlcmA, activityBody(s.qc, "audio_lesson", audioConfig(processing, s.transcript, s.questions[0], s.questions[1])))
	c.publishActivity(ctx, t, id, 1)
	if r := c.start(t, hsAn, id, nil, nil); r.status != 422 || r.str("code") != "MEDIA_NOT_READY" {
		t.Fatalf("not ready: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempts WHERE activity_id = $1`, id); n != 0 {
		t.Fatalf("attempt created: %d", n)
	}

	if r := c.do(t, http.MethodPost, "/cms/learning/activities", qlcmA, idem(), activityBody(s.qc, "speaking", audioConfig(s.audio, "", s.questions[0], s.questions[1]))); r.status != 422 {
		t.Fatalf("speaking: %d %s", r.status, r.raw)
	}
	types := must(t, c.do(t, http.MethodGet, "/cms/learning/activity-types", qlcmA, nil, nil), http.StatusOK, "types")
	if strings.Contains(types.raw, `"speaking"`) || !strings.Contains(types.raw, `"audio_lesson"`) {
		t.Fatalf("types: %s", types.raw)
	}
}

// F066-AC-27, AC-28 (T046 audio part; TC-TEN-001, TC-AUTH-014): another student or tenant gets 404 on every attempt
// endpoint of an audio lesson, a staff token 403 GAME_ACCESS_STUDENT_ONLY.
func TestAudioLessonIsolation(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.audioSetup(ctx, t, nil)
	authztest.Cover(t, "reportActivityProgress", authztest.CrossTenant, authztest.RoleDenied)
	id := must(t, c.start(t, hsAn, s.activityID, nil, nil), http.StatusCreated, "start").str("attempt_id")
	base := "/learning/activity-attempts/" + id
	for _, sub := range []string{hsBinh, hsDung} {
		for _, call := range []resp{c.play(t, sub, id, 1, 0, nil), c.do(t, http.MethodGet, base, sub, nil, nil),
			c.do(t, http.MethodPost, base+"/responses", sub, idem(), map[string]any{"kind": "question", "position": 1, "response": map[string]string{"option_id": "o_x"}}),
			c.do(t, http.MethodPost, base+":submit", sub, idem(), nil)} {
			if call.status != http.StatusNotFound {
				t.Fatalf("%s: %d %s", sub, call.status, call.raw)
			}
		}
	}
	if r := c.play(t, qlcmA, id, 1, 0, nil); r.status != http.StatusForbidden || r.str("code") != "GAME_ACCESS_STUDENT_ONLY" {
		t.Fatalf("staff: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT plays_used FROM assessment.video_progress WHERE attempt_id = $1`, id); n != 0 {
		t.Fatalf("plays counted by another user: %d", n)
	}
}
