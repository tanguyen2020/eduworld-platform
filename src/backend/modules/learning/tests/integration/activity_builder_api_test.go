//go:build integration

package integration

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// F058 T005-T006 contract deltas (owner F030): getCmsActivity returns the authoring fields the Activity Builder edits
// again (description, instructions, topic, timer, author, assigned) so an autosave PATCH never drops them.
func TestActivityDefinitionAuthoringFields(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil)
	body := activityBody(s.qc, "quiz", quizConfig(s.questions...))
	body["time_limit_seconds"] = 600
	id := c.createActivity(t, qlcmA, body)
	g := must(t, c.do(t, http.MethodGet, "/cms/learning/activities/"+id, qlcmA, nil, nil), http.StatusOK, "get")
	if g.str("description", "vi") != "Ôn tập phép nhân" || g.str("instructions", "vi") != "Chọn đáp án đúng" || g.str("topic_id") != s.qc.topic ||
		g.str("time_limit_seconds") != "600" || g.str("author_id") != c.users[qlcmA].UserID.String() || g.body["assigned"] != false {
		t.Fatalf("authoring fields: %s", g.raw)
	}
}

// F058-AC-22, AC-23 (T006, owner F031; TC-LRN-015): previewRegradeRequest counts the attempts a request would
// re-evaluate without writing anything; listRegradeRequests returns the history of a version in the caller's scope,
// newest first; both need learning_content.re_evaluate and a version visible in scope.
func TestRegradePreviewAndHistory(t *testing.T) {
	authztest.Cover(t, "previewRegradeRequest", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listRegradeRequests", authztest.CrossTenant, authztest.RoleDenied)
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil)
	right, wrong := correctOption()
	c.quizAnswers(t, hsAn, s.activityID, right, right, right)
	c.quizAnswers(t, hsBinh, s.activityID, wrong, right, right)
	_, corrected := c.correctedVersion(ctx, t, s.questions[0])
	body := map[string]any{"scope": "question_version", "question_version_id": s.questions[0], "action": "use_new_key",
		"corrected_version_id": corrected, "reason": "Đáp án câu 7 × 8 bị nhập sai, sửa theo version 2"}
	requests := `SELECT count(*) FROM assessment.regrade_requests`

	p := must(t, c.do(t, http.MethodPost, regradePath+":preview", qlcmA, nil, body), http.StatusOK, "preview")
	if p.str("affected_attempts") != "2" || count(ctx, t, c.platform, requests) != 0 {
		t.Fatalf("preview: %s", p.raw)
	}
	for _, tc := range []struct {
		sub  string
		want int
	}{{adA, 403}, {qlcmB, 404}, {cmNoidung, 404}} {
		if r := c.do(t, http.MethodPost, regradePath+":preview", tc.sub, nil, body); r.status != tc.want {
			t.Errorf("preview as %s: %d, want %d (%s)", tc.sub, r.status, tc.want, r.raw)
		}
	}
	short := map[string]any{"scope": "question_version", "question_version_id": s.questions[0], "action": "accept_all", "reason": "ngắn"}
	if r := c.do(t, http.MethodPost, regradePath+":preview", qlcmA, nil, short); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("short reason: %d %s", r.status, r.raw)
	}

	history := regradePath + "?question_version_id=" + s.questions[0]
	if r := must(t, c.do(t, http.MethodGet, history, qlcmA, nil, nil), http.StatusOK, "empty history"); len(r.items("data")) != 0 {
		t.Fatalf("history before: %s", r.raw)
	}
	id := must(t, c.do(t, http.MethodPost, regradePath, qlcmA, idem(), body), http.StatusAccepted, "create").str("regrade_request_id")
	h := must(t, c.do(t, http.MethodGet, history, qlcmA, nil, nil), http.StatusOK, "history")
	if items := h.items("data"); len(items) != 1 || items[0]["regrade_request_id"] != id || items[0]["status"] != "queued" {
		t.Fatalf("history: %s", h.raw)
	}
	// API_DESIGN §5: cursor pagination (default limit 20), the last page has no cursor.
	if pg := h.obj("page"); pg["has_more"] != false || pg["next_cursor"] != nil || pg["limit"] != float64(20) {
		t.Fatalf("history page: %s", h.raw)
	}
	if r := c.do(t, http.MethodGet, history+"&cursor=bogus", qlcmA, nil, nil); r.status != http.StatusBadRequest {
		t.Fatalf("bad cursor: %d %s", r.status, r.raw)
	}
	if r := must(t, c.do(t, http.MethodGet, history, qlcmB, nil, nil), http.StatusOK, "tenant B"); len(r.items("data")) != 0 {
		t.Fatalf("tenant B sees tenant A requests: %s", r.raw)
	}
	if r := c.do(t, http.MethodGet, regradePath, qlcmA, nil, nil); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("history without version: %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodGet, history, adA, nil, nil); r.status != http.StatusForbidden {
		t.Fatalf("history as school admin: %d", r.status)
	}
}

// F058-AC-09 (T020, delta owner F030; CONTENT_MODEL §8.1, TC-CNT-010): an activity of grades 1–3 needs the voice-over of
// its instructions (instructions_audio_ref) or an exemption before review; both are stored and returned by
// getCmsActivity so the builder can edit them again.
func TestActivityInstructionsAudio(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil)
	body := activityBody(s.qc, "quiz", quizConfig(s.questions...))
	body["grade_levels"] = []int{2}
	id := c.createActivity(t, qlcmA, body)
	r := c.act(ctx, t, qlcmA, "activity", id, 1, "submit_review", "")
	if r.status != http.StatusUnprocessableEntity || r.str("code") != "AUDIO_REQUIRED" || !slices.Contains(r.errorCodes(), "/instructions_audio_ref:REQUIRED") {
		t.Fatalf("no voice-over: %d %s", r.status, r.raw)
	}
	audio := c.mediaRow(ctx, t, &c.tenantA, "ready")
	body["instructions_audio_ref"] = audio
	must(t, c.do(t, http.MethodPatch, "/cms/learning/activities/"+id, qlcmA, merge(hdr{"Content-Type": "application/json"}, ifm(c.rowTag(ctx, t, "activity", id, 1))), body), http.StatusOK, "voice-over")
	g := must(t, c.do(t, http.MethodGet, "/cms/learning/activities/"+id, qlcmA, nil, nil), http.StatusOK, "get")
	if g.str("instructions_audio_ref") != audio {
		t.Fatalf("voice-over not returned: %s", g.raw)
	}
	must(t, c.act(ctx, t, qlcmA, "activity", id, 1, "submit_review", ""), http.StatusOK, "submit with voice-over")

	body2 := activityBody(s.qc, "quiz", quizConfig(s.questions...))
	body2["grade_levels"], body2["audio_exempt_reason"] = []int{1}, "Bài nhận biết chữ: đọc to làm lộ đáp án"
	id2 := c.createActivity(t, qlcmA, body2)
	if g := must(t, c.do(t, http.MethodGet, "/cms/learning/activities/"+id2, qlcmA, nil, nil), http.StatusOK, "get exempt"); g.str("audio_exempt_reason") == "" {
		t.Fatalf("exemption not returned: %s", g.raw)
	}
	must(t, c.act(ctx, t, qlcmA, "activity", id2, 1, "submit_review", ""), http.StatusOK, "submit with exemption")
}
