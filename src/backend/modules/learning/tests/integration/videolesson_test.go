//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/storage"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// testSigner is the TEST CDN signer of the integration env (fixed key, never in a binary — R17).
var testSigner = storage.CdnSigner{BaseURL: "https://cdn.test.invalid", Key: "f065-integration-only", TTL: time.Hour}

// videoAsset inserts a ready (or processing) media asset of tenant A with the given variants — TEST helper: the row the
// media pipeline of F027 writes after transcoding, without the object storage.
func (c *curEnv) videoAsset(ctx context.Context, t *testing.T, kind, status string, variants string) string {
	t.Helper()
	id := ids.New()
	sha := strings.ReplaceAll(id.String()+id.String(), "-", "")[:64]
	mime := "video/mp4"
	if kind == "document" {
		mime = "text/csv"
	} else if kind == "audio" {
		mime = "audio/mpeg"
	}
	// tenancyguard:ok TEST helper under learning_platform: the row of the media pipeline
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO content.media_assets (asset_id, tenant_id, created_at, updated_at, owner_scope, kind,
		original_filename, mime_type, size_bytes, duration_ms, sha256, bucket, variants, license, status, quarantine_key, uploaded_by)
		VALUES ($1, $2, now(), now(), 'tenant', $3, 'bai-giang.mp4', $4, 1024, 100000, $5, 'ew-media-private', $6::jsonb, 'CC BY 4.0', $7, $8, $1)`,
		id, c.tenantA, kind, mime, sha, strings.ReplaceAll(variants, "{sha}", sha), status, "q/"+id.String()); err != nil {
		t.Fatal(err)
	}
	return id.String()
}

const theoraVariants = `{"ogv_720":{"key":"media/{sha}/ogv_720.ogv","bytes":1024,"mime":"video/ogg"},
	"ogv_480":{"key":"media/{sha}/ogv_480.ogv","bytes":512,"mime":"video/ogg"},"mp4_720":{"key":"media/{sha}/mp4_720.mp4","bytes":1024,"mime":"video/mp4"}}`

// videoConfig is a video_lesson config: 100 s video, one required cue at 30 s over the first question, a chapter, a vi
// subtitle; watched_only seek with rates 1 and 1.5; completion at 90 % with the required cues.
func videoConfig(video, subtitle, cueQuestion string) map[string]any {
	return map[string]any{
		"content": map[string]any{"video_asset_id": video, "duration_ms": 100000,
			"chapters":           []any{map[string]any{"chapter_id": "intro", "title": map[string]string{"vi": "Mở đầu"}, "start_ms": 0, "end_ms": 50000}},
			"subtitles":          []any{map[string]any{"language": "vi", "subtitle_asset_id": subtitle, "is_default": true}},
			"embedded_questions": []any{map[string]any{"cue_id": "q1", "at_ms": 30000, "question_version_id": cueQuestion}}},
		"interaction":     map[string]any{"allow_seek": "watched_only", "playback_rates": []float64{1, 1.5}},
		"evaluation":      map[string]any{"strategies": []string{"completion", "auto"}, "max_score": 1},
		"feedback":        map[string]any{"timing": "immediate"},
		"completion_rule": map[string]any{"mode": "min_watched_ratio", "min_watched_ratio": 0.9, "require_required_cues": true},
	}
}

type videoSetup struct {
	attSetup
	video, subtitle, activityID string
}

func (c *curEnv) videoSetup(ctx context.Context, t *testing.T, mutate func(map[string]any)) videoSetup {
	t.Helper()
	s := videoSetup{attSetup: c.attemptSetup(ctx, t, nil)}
	s.video = c.videoAsset(ctx, t, "video", "ready", theoraVariants)
	s.subtitle = c.videoAsset(ctx, t, "document", "ready", `{"vtt":{"key":"media/{sha}/sub.vtt","bytes":64,"mime":"text/vtt"}}`)
	cfg := videoConfig(s.video, s.subtitle, s.questions[0])
	if mutate != nil {
		mutate(cfg)
	}
	s.activityID = c.createActivity(t, qlcmA, activityBody(s.qc, "video_lesson", cfg))
	c.publishActivity(ctx, t, s.activityID, 1)
	return s
}

func (c *curEnv) heartbeat(t *testing.T, sub, attemptID string, seq, position int64, ranges [][2]int64) resp {
	t.Helper()
	rs := make([]any, len(ranges))
	for i, r := range ranges {
		rs[i] = map[string]int64{"start_ms": r[0], "end_ms": r[1]}
	}
	return c.do(t, http.MethodPost, "/learning/activity-attempts/"+attemptID+"/progress", sub, nil,
		map[string]any{"client_seq": seq, "position_ms": position, "watched_ranges": rs, "playback_rate": 1})
}

func (c *curEnv) learningEvents(ctx context.Context, t *testing.T, typ, attemptID string) int {
	t.Helper()
	if typ == "activity.completed" { // payload_activity_completed has no attempt_id: its idempotency key names the attempt
		return count(ctx, t, c.platform, `SELECT count(*) FROM learning.learning_events WHERE event_type = $1 AND idempotency_key = $2`,
			typ, "cmp:"+attemptID)
	}
	return count(ctx, t, c.platform, `SELECT count(*) FROM learning.learning_events WHERE event_type = $1 AND payload->>'attempt_id' = $2`,
		typ, attemptID)
}

// F065-AC-02…AC-12 (T013, T014, T015): start pins the cue as item 1 and serves signed Theora variants and the
// subtitle without answer data; heartbeats are plausibility-checked (trim + integrity signal), seek past the frontier
// is 422 VIDEO_SEEK_NOT_ALLOWED without change, old client_seq returns the state, video.progressed once per milestone;
// the cue is graded on the server (attempt_answers + video.cue_answered); submit needs the required cue
// (CUE_ANSWER_REQUIRED), then grades once with activity.completed; resume returns position and ranges.
func TestVideoLessonAttempt(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.videoSetup(ctx, t, nil)
	authztest.Cover(t, "reportActivityProgress", authztest.CrossTenant, authztest.RoleDenied)

	r := must(t, c.start(t, hsAn, s.activityID, nil, nil), http.StatusCreated, "start")
	id := r.str("attempt_id")
	if r.str("render", "activity_type") != "video_lesson" || r.str("progress", "position_ms") != "0" || r.str("resume_position_ms") != "0" {
		t.Fatalf("start: %s", r.raw)
	}
	items := r.items("items")
	if len(items) != 1 || items[0]["question_version_id"] != s.questions[0] || items[0]["position"] != float64(1) {
		t.Fatalf("items: %s", r.raw)
	}
	if leaked := answerKeysIn(r.raw); len(leaked) > 0 || strings.Contains(r.raw, "target_segment_id") {
		t.Fatalf("answer data in the attempt: %v %s", leaked, r.raw)
	}
	media := r.body["render"].(map[string]any)["media"].([]any)
	variants := map[string]string{}
	for _, m := range media {
		mm := m.(map[string]any)
		variants[mm["variant"].(string)] = mm["url"].(string)
	}
	if len(variants) != 3 || !strings.Contains(variants["ogv_720"], "expires=") || variants["vtt"] == "" || variants["mp4_720"] != "" {
		t.Fatalf("media: %v", variants)
	}
	cues := r.body["render"].(map[string]any)["render_payload"].(map[string]any)["cues"].([]any)
	if len(cues) != 1 || cues[0].(map[string]any)["position"] != float64(1) {
		t.Fatalf("cues: %s", r.raw)
	}

	// AC-05: a plausible heartbeat.
	c.clk.Advance(10 * time.Second)
	hb := must(t, c.heartbeat(t, hsAn, id, 1, 10000, [][2]int64{{0, 10000}}), http.StatusOK, "heartbeat 1")
	if hb.body["watched_ratio"] != 0.1 {
		t.Fatalf("heartbeat 1: %s", hb.raw)
	}
	// AC-06: a jump to the end with the position at the end → 422, nothing stored.
	c.clk.Advance(10 * time.Second)
	if r := c.heartbeat(t, hsAn, id, 2, 100000, [][2]int64{{0, 100000}}); r.status != 422 || r.str("code") != "VIDEO_SEEK_NOT_ALLOWED" {
		t.Fatalf("seek: %d %s", r.status, r.raw)
	}
	// AC-05: the same jump with a position inside the plausible part: trimmed to 1.5 × 10 s, integrity signal, 25 %.
	hb = must(t, c.heartbeat(t, hsAn, id, 3, 15000, [][2]int64{{0, 100000}}), http.StatusOK, "heartbeat 3")
	if hb.body["watched_ratio"] != 0.25 {
		t.Fatalf("trim: %s", hb.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.integrity_signals WHERE attempt_id = $1
		AND signal_type = 'video_progress_implausible'`, id); n != 1 {
		t.Fatalf("integrity signals: %d", n)
	}
	// Out of order: an old client_seq returns the stored state.
	if old := must(t, c.heartbeat(t, hsAn, id, 2, 0, nil), http.StatusOK, "old seq"); old.body["watched_ratio"] != 0.25 {
		t.Fatalf("old seq: %s", old.raw)
	}
	// AC-12: starting again resumes the open attempt with its position and ranges.
	again := must(t, c.start(t, hsAn, s.activityID, nil, nil), http.StatusCreated, "resume")
	if again.str("attempt_id") != id || again.str("resume_position_ms") != "15000" || len(again.body["progress"].(map[string]any)["watched_ranges"].([]any)) != 1 {
		t.Fatalf("resume: %s", again.raw)
	}
	// AC-10: the required cue is missing.
	if r := c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+":submit", hsAn, idem(), nil); r.status != 422 || r.str("code") != "CUE_ANSWER_REQUIRED" {
		t.Fatalf("submit without cue: %d %s", r.status, r.raw)
	}
	// AC-08: the cue answer is graded on the server.
	right, _ := correctOption()
	cue := must(t, c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+"/responses", hsAn, idem(),
		map[string]any{"kind": "cue_answer", "cue_id": "q1", "response": map[string]string{"option_id": right}}), http.StatusOK, "cue")
	if cue.body["is_correct"] != true || cue.str("position") != "1" {
		t.Fatalf("cue: %s", cue.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempt_answers WHERE attempt_id = $1`, id); n != 1 {
		t.Fatalf("answers: %d", n)
	}
	if n := c.learningEvents(ctx, t, "video.cue_answered", id); n != 1 {
		t.Fatalf("video.cue_answered: %d", n)
	}
	// Watch to the end in plausible heartbeats; AC-07: the 4 milestones once each.
	frontier := int64(25000)
	for seq := int64(4); frontier < 100000; seq++ {
		c.clk.Advance(10 * time.Second)
		frontier = min(frontier+15000, 100000)
		must(t, c.heartbeat(t, hsAn, id, seq, frontier, [][2]int64{{0, frontier}}), http.StatusOK, "watch")
	}
	if n := c.learningEvents(ctx, t, "video.progressed", id); n != 4 {
		t.Fatalf("video.progressed: %d", n)
	}
	// AC-10, AC-11: submit once, completed; same key replays, another key is 409.
	key := idem()
	sub := must(t, c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+":submit", hsAn, key, nil), http.StatusOK, "submit")
	if sub.str("status") != "graded" || sub.str("evaluation", "score") != "1" {
		t.Fatalf("submit: %s", sub.raw)
	}
	if n := c.learningEvents(ctx, t, "activity.completed", id); n != 1 {
		t.Fatalf("activity.completed: %d", n)
	}
	if r := c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+":submit", hsAn, key, nil); r.status != 200 || r.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+":submit", hsAn, idem(), nil); r.status != 409 || r.str("code") != "ATTEMPT_ALREADY_SUBMITTED" {
		t.Fatalf("second submit: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'activity.submitted'
		AND envelope->'payload'->>'attempt_id' = $1`, id); n != 1 {
		t.Fatalf("outbox activity.submitted: %d", n)
	}
	// Watching again after grading is accepted and keeps the evaluation.
	c.clk.Advance(10 * time.Second)
	must(t, c.heartbeat(t, hsAn, id, 100, 0, [][2]int64{{0, 1000}}), http.StatusOK, "rewatch")
}

// F065-AC-13, AC-14 (T016, T017): another student or tenant gets 404 on every attempt endpoint, a staff token 403
// GAME_ACCESS_STUDENT_ONLY; ids over 64 characters, more than 64 ranges or a rate outside [0.5, 2] are 422.
func TestVideoLessonIsolationAndValidation(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.videoSetup(ctx, t, nil)
	id := must(t, c.start(t, hsAn, s.activityID, nil, nil), http.StatusCreated, "start").str("attempt_id")
	base := "/learning/activity-attempts/" + id
	for _, sub := range []string{hsBinh, hsDung} {
		for _, call := range []resp{c.heartbeat(t, sub, id, 1, 0, nil), c.do(t, http.MethodGet, base, sub, nil, nil),
			c.do(t, http.MethodPost, base+"/responses", sub, idem(), map[string]any{"kind": "cue_answer", "cue_id": "q1", "response": map[string]string{"option_id": "o_x"}}),
			c.do(t, http.MethodPost, base+":submit", sub, idem(), nil)} {
			if call.status != http.StatusNotFound {
				t.Fatalf("%s: %d %s", sub, call.status, call.raw)
			}
		}
	}
	if r := c.heartbeat(t, qlcmA, id, 1, 0, nil); r.status != http.StatusForbidden || r.str("code") != "GAME_ACCESS_STUDENT_ONLY" {
		t.Fatalf("staff: %d %s", r.status, r.raw)
	}
	long := strings.Repeat("c", 65)
	if r := c.do(t, http.MethodPost, base+"/responses", hsAn, idem(), map[string]any{"kind": "cue_answer", "cue_id": long,
		"response": map[string]string{"option_id": "o_x"}}); r.status != 422 || r.str("code") != "VALIDATION_FAILED" {
		t.Fatalf("long cue_id: %d %s", r.status, r.raw)
	}
	many := make([][2]int64, 65)
	for i := range many {
		many[i] = [2]int64{int64(i) * 100, int64(i)*100 + 10}
	}
	if r := c.heartbeat(t, hsAn, id, 1, 0, many); r.status != 422 || r.str("code") != "VALIDATION_FAILED" {
		t.Fatalf("65 ranges: %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodPost, base+"/progress", hsAn, nil, map[string]any{"client_seq": 1, "position_ms": 0, "playback_rate": 3}); r.status != 422 ||
		r.str("code") != "VALIDATION_FAILED" {
		t.Fatalf("rate: %d %s", r.status, r.raw)
	}
}

// F065-AC-01, AC-04 (T013, T018): a config with a cue after the end is 422 ACTIVITY_CONFIG_INVALID on save; a video
// still processing is 422 VIDEO_NOT_READY at start and no attempt is created.
func TestVideoLessonConfigAndReadiness(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.videoSetup(ctx, t, nil)
	bad := videoConfig(s.video, s.subtitle, s.questions[0])
	bad["content"].(map[string]any)["embedded_questions"] = []any{map[string]any{"cue_id": "late", "at_ms": 200000, "question_version_id": s.questions[0]}}
	if r := c.do(t, http.MethodPost, "/cms/learning/activities", qlcmA, idem(), activityBody(s.qc, "video_lesson", bad)); r.status != 422 ||
		r.str("code") != "ACTIVITY_CONFIG_INVALID" || !strings.Contains(r.raw, "embedded_questions/0/at_ms") {
		t.Fatalf("config: %d %s", r.status, r.raw)
	}
	unknownQ := videoConfig(s.video, s.subtitle, uuid.NewString())
	if r := c.do(t, http.MethodPost, "/cms/learning/activities", qlcmA, idem(), activityBody(s.qc, "video_lesson", unknownQ)); r.status != 422 {
		t.Fatalf("unknown cue question: %d %s", r.status, r.raw)
	}
	processing := c.videoAsset(ctx, t, "video", "processing", `{}`)
	cfg := videoConfig(processing, s.subtitle, s.questions[0])
	id := c.createActivity(t, qlcmA, activityBody(s.qc, "video_lesson", cfg))
	c.publishActivity(ctx, t, id, 1)
	if r := c.start(t, hsAn, id, nil, nil); r.status != 422 || r.str("code") != "VIDEO_NOT_READY" {
		t.Fatalf("not ready: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempts WHERE activity_id = $1`, id); n != 0 {
		t.Fatalf("attempt created: %d", n)
	}
}

// F065-AC-24, AC-25 (T034): a branch choice is recorded once per branch point (same option replays, another is 409),
// is never graded, answers the next segment, and the attempt completes at the ending segment.
func TestVideoLessonBranches(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	var water, land string
	s := c.videoSetup(ctx, t, func(cfg map[string]any) {
		water = c.videoAsset(ctx, t, "video", "ready", theoraVariants)
		land = c.videoAsset(ctx, t, "video", "ready", theoraVariants)
		content := cfg["content"].(map[string]any)
		content["embedded_questions"] = []any{}
		content["segments"] = []any{map[string]any{"segment_id": "water", "video_asset_id": water, "is_ending": true},
			map[string]any{"segment_id": "land", "video_asset_id": land, "is_ending": true}}
		content["branches"] = []any{map[string]any{"branch_point_id": "bp1", "at_ms": 20000, "prompt": map[string]string{"vi": "Vịt đi đâu?"},
			"options": []any{map[string]any{"option_id": "swim", "label": map[string]string{"vi": "Xuống nước"}, "target_segment_id": "water"},
				map[string]any{"option_id": "walk", "label": map[string]string{"vi": "Lên bờ"}, "target_segment_id": "land"}}}}
	})
	r := must(t, c.start(t, hsAn, s.activityID, nil, nil), http.StatusCreated, "start")
	id := r.str("attempt_id")
	if strings.Contains(r.raw, "target_segment_id") || !strings.Contains(r.raw, `"segment_id":"water"`) {
		t.Fatalf("render: %s", r.raw)
	}
	choose := func(option string, h hdr) resp {
		return c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+"/responses", hsAn, h,
			map[string]any{"kind": "branch_choice", "branch_point_id": "bp1", "option_id": option})
	}
	got := must(t, choose("swim", idem()), http.StatusOK, "choose")
	if got.str("next_segment_id") != "water" || got.body["is_correct"] != nil {
		t.Fatalf("choice: %s", got.raw)
	}
	if again := must(t, choose("swim", idem()), http.StatusOK, "same option"); again.str("next_segment_id") != "water" {
		t.Fatalf("same option: %s", again.raw)
	}
	if other := choose("walk", idem()); other.status != http.StatusConflict {
		t.Fatalf("other option: %d %s", other.status, other.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.video_branch_choices WHERE attempt_id = $1`, id); n != 1 {
		t.Fatalf("choices: %d", n)
	}
	must(t, c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+":submit", hsAn, idem(), nil), http.StatusOK, "submit")
	if n := c.learningEvents(ctx, t, "activity.completed", id); n != 1 {
		t.Fatalf("activity.completed at the ending segment: %d", n)
	}
}
