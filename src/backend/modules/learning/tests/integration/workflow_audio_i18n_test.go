//go:build integration

package integration

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"
)

// F025-AC-15, AC-16 (US3, quickstart 8; SC-002): g1_3 content needs the voice-over of its stem (question) or worked
// example steps (lesson); an exemption lifts it and must be acknowledged by the reviewer, who is recorded.
func TestWorkflowAudioG13(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	id, vid := c.question(t, cmNoidung, qc, map[string]any{"owner_scope": "platform", "grade_levels": []int{2}})
	r := c.act(ctx, t, cmNoidung, "question", id, 1, "submit_review", "")
	if r.status != http.StatusUnprocessableEntity || r.str("code") != "AUDIO_REQUIRED" || !slices.Contains(r.errorCodes(), "/stem/audio_ref:REQUIRED") {
		t.Fatalf("no audio: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.question_versions WHERE question_version_id = $1 AND status = 'draft'`, vid); n != 1 {
		t.Fatal("left draft without audio (SC-002)")
	}
	// Exemption declared on the draft (≤ 255 characters).
	long := strings.Repeat("x", 256)
	if r := c.do(t, http.MethodPatch, "/cms/learning/questions/"+id+"/versions/1", cmNoidung, ifm(c.rowTag(ctx, t, "question", id, 1)),
		map[string]any{"audio_exempt_reason": long}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("long exemption: %d %s", r.status, r.raw)
	}
	r = must(t, c.do(t, http.MethodPatch, "/cms/learning/questions/"+id+"/versions/1", cmNoidung, ifm(c.rowTag(ctx, t, "question", id, 1)),
		map[string]any{"audio_exempt_reason": "Câu chính tả: đọc to làm lộ đáp án"}), http.StatusOK, "exemption")
	if r.str("audio_exempt_reason") == "" {
		t.Fatalf("exemption not stored: %s", r.raw)
	}
	r = must(t, c.act(ctx, t, cmNoidung, "question", id, 1, "submit_review", ""), http.StatusOK, "submit with exemption")
	if len(r.items("warnings")) == 0 {
		t.Fatalf("no warning for choices without audio: %s", r.raw)
	}
	if r := c.act(ctx, t, cmDuyet, "question", id, 1, "approve", ""); r.status != http.StatusUnprocessableEntity ||
		!slices.Contains(r.errorCodes(), "audio_exemption_acknowledged:REQUIRED") {
		t.Fatalf("approve without acknowledgement: %d %s", r.status, r.raw)
	}
	r = must(t, c.wfDo(ctx, t, cmDuyet, "question", id, 1, map[string]any{"action": "approve", "audio_exemption_acknowledged": true}, nil),
		http.StatusOK, "approve acknowledged")
	if r.str("audio_exempt_approved_by") != c.users[cmDuyet].UserID.String() {
		t.Fatalf("exemption approver: %s", r.raw)
	}
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'content.version_approved'
		AND envelope->'payload'->>'audio_exemption_acknowledged' = 'true'`); n != 1 {
		t.Fatal("approved event without the acknowledgement")
	}
	// Grades {3, 4} with a ready voice-over pass.
	audio := c.mediaRow(ctx, t, nil, "ready")
	stem := map[string]any{"blocks": []any{map[string]any{"type": "paragraph", "text": "7 × 8 = ?"}}, "audio_ref": audio}
	id2, _ := c.question(t, cmNoidung, qc, map[string]any{"owner_scope": "platform", "grade_levels": []int{3, 4}, "stem": stem})
	must(t, c.act(ctx, t, cmNoidung, "question", id2, 1, "submit_review", ""), http.StatusOK, "g1_3 with audio")
	// A lesson of grade 2: every worked example step needs its audio.
	body := lessonBody(qc, workedExample("we_mul7", 2, qc.lo))
	body["grade_level"], body["owner_scope"] = 2, "platform"
	lid := must(t, c.do(t, http.MethodPost, lessonsPath, cmNoidung, idem(), body), http.StatusCreated, "lesson").str("content_id")
	r = c.act(ctx, t, cmNoidung, "lesson", lid, 1, "submit_review", "")
	if r.status != http.StatusUnprocessableEntity || r.str("code") != "AUDIO_REQUIRED" ||
		!slices.Equal(r.errorCodes(), []string{"/blocks/0/steps/0/audio_ref:REQUIRED", "/blocks/0/steps/1/audio_ref:REQUIRED"}) {
		t.Fatalf("lesson audio: %d %s", r.status, r.raw)
	}
}

// F025-AC-18, AC-19, AC-21 (US4, quickstart 12): translation_status is computed on save, the content_language text is
// mandatory on submit, texts are stored NFC and over-long texts are refused at their field.
func TestWorkflowLanguage(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	// AC-19: vi only → partial (a warning, the version can still be submitted).
	r := must(t, c.createQ(t, cmNoidung, qc.body(qbFixture(t, "single_choice"), map[string]any{"owner_scope": "platform", "grade_levels": []int{4}})), http.StatusCreated, "vi only")
	if r.str("translation_status") != "partial" {
		t.Fatalf("translation_status: %s", r.raw)
	}
	must(t, c.act(ctx, t, cmNoidung, "question", r.str("content_id"), 1, "submit_review", ""), http.StatusOK, "partial submits")
	// A bilingual true/false question is complete; editing the source makes it outdated.
	bi := func(vi, en string) map[string]any {
		return map[string]any{"blocks": []any{map[string]any{"type": "paragraph", "text": map[string]any{"vi": vi, "en": en}}}}
	}
	tf := qbFixture(t, "true_false")
	tf["stem"] = bi("Xét các mệnh đề sau", "Consider the statements")
	tf["render_payload"] = map[string]any{"statements": []any{map[string]any{"id": "s_1", "content": bi("Số 0 là số chẵn", "Zero is even")},
		map[string]any{"id": "s_2", "content": bi("Số 1 là số nguyên tố", "One is prime")}}}
	r = must(t, c.createQ(t, cmNoidung, qc.body(tf, map[string]any{"owner_scope": "platform", "grade_levels": []int{4}})), http.StatusCreated, "bilingual")
	if r.str("translation_status") != "complete" {
		t.Fatalf("bilingual: %s", r.raw)
	}
	id := r.str("content_id")
	r = must(t, c.do(t, http.MethodPatch, "/cms/learning/questions/"+id+"/versions/1", cmNoidung, ifm(c.rowTag(ctx, t, "question", id, 1)),
		map[string]any{"stem": bi("Xét các mệnh đề dưới đây", "Consider the statements")}), http.StatusOK, "edit source")
	if r.str("translation_status") != "outdated" || !strings.Contains(r.raw, `"outdated_fields":["/stem/blocks/0/text"]`) {
		t.Fatalf("outdated: %s", r.raw)
	}
	// AC-18: the vi text of the stem is missing.
	stem := map[string]any{"blocks": []any{map[string]any{"type": "paragraph", "text": map[string]any{"en": "7 x 8 = ?"}}}}
	id3, _ := c.question(t, cmNoidung, qc, map[string]any{"owner_scope": "platform", "grade_levels": []int{4}, "stem": stem})
	r = c.act(ctx, t, cmNoidung, "question", id3, 1, "submit_review", "")
	if r.status != http.StatusUnprocessableEntity || r.str("code") != "TRANSLATION_INCOMPLETE" ||
		!slices.Contains(r.errorCodes(), "/stem/blocks/0/text:REQUIRED") {
		t.Fatalf("translation incomplete: %d %s", r.status, r.raw)
	}
	// AC-21: NFC on save; an over-long text is refused at its pointer.
	nfd := norm.NFD.String("Tính nhẩm: 7 × 8")
	id4, vid4 := c.question(t, cmNoidung, qc, map[string]any{"owner_scope": "platform", "grade_levels": []int{4}, "stem": map[string]any{"blocks": []any{
		map[string]any{"type": "paragraph", "text": nfd}}}})
	_ = id4
	var stored string
	if err := c.platform.QueryRowContext(ctx, `SELECT stem->'blocks'->0->>'text' FROM content.question_versions WHERE question_version_id = $1`, vid4).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != norm.NFC.String(nfd) || stored == nfd {
		t.Fatalf("stored text is not NFC: %q", stored)
	}
	r = c.createQ(t, cmNoidung, qc.body(qbFixture(t, "single_choice"), map[string]any{"owner_scope": "platform", "grade_levels": []int{4}, "stem": map[string]any{"blocks": []any{
		map[string]any{"type": "paragraph", "text": strings.Repeat("a", 4001)}}}}))
	if r.status != http.StatusUnprocessableEntity || r.str("code") != "VALIDATION_FAILED" || !strings.Contains(r.raw, "/stem/blocks/0/text") {
		t.Fatalf("over-long text: %d %s", r.status, r.raw)
	}
}
