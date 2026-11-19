//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// sideEffects counts what a preview must never create (F027-AC-17): outbox events other than audit, attempts and
// learning records (fixture tables of the attempt shape), game sessions do not exist in this database.
func (c *curEnv) sideEffects(ctx context.Context, t *testing.T) int {
	t.Helper()
	return count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type <> 'audit.recorded'`) +
		count(ctx, t, c.platform, `SELECT count(*) FROM content.question_versions`) +
		count(ctx, t, c.platform, `SELECT count(*) FROM content.media_usages`)
}

// F027-AC-16, AC-17, AC-18 (US-CMS-002): a reviewer previews a draft as a student sees it (no answer, chosen
// language), tries it with server-side grading, explanation, option feedback and hints, and nothing is stored.
func TestPreviewSession(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t, withMedia)
	qc := c.qbSetup(ctx, t)
	fx := qbFixture(t, "single_choice")
	fx["stem"] = map[string]any{"blocks": []any{map[string]any{"type": "paragraph", "text": map[string]any{"vi": "Bảy nhân tám bằng?", "en": "Seven times eight?"}}}}
	fx["explanation"] = map[string]any{"blocks": []any{map[string]any{"type": "paragraph", "text": "7 × 8 = 56"}}}
	q := must(t, c.createQ(t, qlcmA, qc.body(fx, nil)), http.StatusCreated, "draft")
	id := q.str("content_id")
	before := c.sideEffects(ctx, t)
	authztest.Cover(t, "createLearningPreviewSession", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "submitPreviewResponse", authztest.CrossTenant, authztest.RoleDenied)
	p := must(t, c.do(t, http.MethodPost, "/cms/learning/preview/sessions", qlcmA, idem(), map[string]any{"content_type": "question",
		"content_id": id, "version": 1, "display_language": "en-US"}), http.StatusCreated, "preview draft")
	items := p.items("items")
	if p.str("preview_session_id") == "" || p.str("expires_at") == "" || len(items) != 1 {
		t.Fatalf("session %s", p.raw)
	}
	for _, leak := range []string{"correct_option_id", "answer_key", "feedback", "misconception", "hints", "explanation"} {
		if strings.Contains(p.raw, leak) {
			t.Fatalf("preview leaks %s: %s", leak, p.raw)
		}
	}
	if !strings.Contains(p.raw, "Seven times eight?") || strings.Contains(p.raw, "Bảy nhân") {
		t.Fatalf("display_language en-US not applied: %s", p.raw)
	}
	sid := p.str("preview_session_id")
	path := "/cms/learning/preview/sessions/" + sid + "/responses"
	right := must(t, c.do(t, http.MethodPost, path, qlcmA, nil, map[string]any{"position": 1, "response": map[string]any{"option_id": "o_3b2e"}}),
		http.StatusOK, "right")
	if right.body["is_correct"] != true || right.body["score"] != float64(1) || !strings.Contains(right.raw, "7 × 8 = 56") ||
		len(right.items("hint_levels")) != 2 {
		t.Fatalf("right: %s", right.raw)
	}
	wrong := must(t, c.do(t, http.MethodPost, path, qlcmA, nil, map[string]any{"position": 1, "response": map[string]any{"option_id": "o_c7d0"}}),
		http.StatusOK, "wrong")
	fb := wrong.items("feedback")
	if wrong.body["is_correct"] != false || len(fb) != 1 || fb[0]["option_id"] != "o_c7d0" || strings.Contains(wrong.raw, "off_by_one") {
		t.Fatalf("wrong: %s", wrong.raw)
	}
	r := c.do(t, http.MethodPost, path, qlcmA, nil, map[string]any{"position": 1, "response": map[string]any{"option_ids": []string{"o_3b2e"}}})
	if r.status != http.StatusUnprocessableEntity {
		t.Fatalf("invalid response: %d %s", r.status, r.raw)
	}
	must(t, c.do(t, http.MethodPost, path, qlcmA, nil, map[string]any{"position": 2, "response": map[string]any{"option_id": "o_3b2e"}}),
		http.StatusUnprocessableEntity, "position 2")
	if after := c.sideEffects(ctx, t); after != before {
		t.Fatalf("preview wrote data: %d → %d", before, after)
	}
	// Published versions preview too (cm.duyet is platform staff: platform scope, sees platform content only).
	c.publishQuestion(ctx, t, id, 1)
	must(t, c.do(t, http.MethodPost, "/cms/learning/preview/sessions", qlcmA, idem(), map[string]any{"content_type": "question", "content_id": id,
		"version": 1}), http.StatusCreated, "preview published")
	// AC-18: an expired session is 404.
	if err := c.media.store.C.Do(ctx, c.media.store.C.B().Del().Key("ew:learning:preview:"+sid).Build()).Error(); err != nil {
		t.Fatal(err)
	}
	must(t, c.do(t, http.MethodPost, path, qlcmA, nil, map[string]any{"position": 1, "response": map[string]any{"option_id": "o_3b2e"}}),
		http.StatusNotFound, "expired")
	if ttl, err := c.media.store.C.Do(ctx, c.media.store.C.B().Ttl().Key("ew:learning:preview:"+sid).Build()).AsInt64(); err != nil || ttl != -2 {
		t.Fatalf("ttl %d %v", ttl, err)
	}
}

// F027-AC-19 (TC-TEN-001): another tenant's content and another user's session are 404; no learning_content.read is
// 403; a game token is refused.
func TestPreviewAuthorization(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t, withMedia)
	qc := c.qbSetup(ctx, t)
	id := must(t, c.createQ(t, qlcmA, qc.body(qbFixture(t, "single_choice"), nil)), http.StatusCreated, "draft").str("content_id")
	body := map[string]any{"content_type": "question", "content_id": id, "version": 1}
	must(t, c.do(t, http.MethodPost, "/cms/learning/preview/sessions", adB, idem(), body), http.StatusNotFound, "tenant B")
	must(t, c.do(t, http.MethodPost, "/cms/learning/preview/sessions", noRoleA, idem(), body), http.StatusForbidden, "no role")
	must(t, c.do(t, http.MethodPost, "/cms/learning/preview/sessions", hsAn, idem(), body), http.StatusForbidden, "game token")
	must(t, c.do(t, http.MethodPost, "/cms/learning/preview/sessions", qlcmA, idem(), map[string]any{"content_type": "question",
		"content_id": uuid.NewString(), "version": 1}), http.StatusNotFound, "unknown")
	must(t, c.do(t, http.MethodPost, "/cms/learning/preview/sessions", qlcmA, idem(), map[string]any{"content_type": "lesson",
		"content_id": id, "version": 1}), http.StatusUnprocessableEntity, "lesson not yet")
	p := must(t, c.do(t, http.MethodPost, "/cms/learning/preview/sessions", qlcmA, idem(), body), http.StatusCreated, "own")
	path := "/cms/learning/preview/sessions/" + p.str("preview_session_id") + "/responses"
	answer := map[string]any{"position": 1, "response": map[string]any{"option_id": "o_3b2e"}}
	must(t, c.do(t, http.MethodPost, path, edA, nil, answer), http.StatusNotFound, "other user of tenant A")
	must(t, c.do(t, http.MethodPost, path, adB, nil, answer), http.StatusNotFound, "tenant B")
	must(t, c.do(t, http.MethodPost, path, noRoleA, nil, answer), http.StatusForbidden, "no role")
	_ = time.Second
}
