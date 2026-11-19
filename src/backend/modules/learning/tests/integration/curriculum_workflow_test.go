//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
)

// F021-AC-13, AC-14, AC-15, AC-16, AC-17, AC-18 (US3; TC-CMS-001…004): workflow, publish in one transaction with
// idempotency, immutability, deep copy into a new draft.
func TestCurriculumWorkflow(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	math := c.subject(t, "math")
	cur, ver := c.curriculum(t, cmNoidung, "WF_MATH")
	topic := c.topic(t, cmNoidung, ver, math, "MATH.G3.T01", 3, "")
	a := must(t, c.objective(t, cmNoidung, topic, "MATH.G3.NUM.A", nil), http.StatusCreated, "LO A").str("objective_id")
	must(t, c.objective(t, cmNoidung, topic, "MATH.G3.NUM.B", map[string]any{"prerequisite_ids": []string{a}}), http.StatusCreated, "LO B")

	// AC-13: submit, self-approval refused, reject needs a note, wrong transitions refused, revise, approve.
	r := must(t, c.transition(t, cmNoidung, cur, 1, "submit_review", ""), http.StatusOK, "submit")
	if r.str("status") != "in_review" {
		t.Fatalf("submit %s", r.raw)
	}
	r = c.transition(t, cmNoidung, cur, 1, "approve", "")
	if must(t, r, http.StatusUnprocessableEntity, "self approve"); r.str("code") != "INVALID_STATE_TRANSITION" {
		t.Fatalf("self approve %s", r.raw)
	}
	r = c.transition(t, saAdmin, cur, 1, "reject", "")
	if must(t, r, http.StatusUnprocessableEntity, "reject without note"); !slices.Contains(r.errorCodes(), "note:REQUIRED") {
		t.Fatalf("reject %v", r.errorCodes())
	}
	tag := c.versionETag(t, saAdmin, cur, 1)
	r = c.do(t, http.MethodPost, "/cms/learning/curricula/"+cur+"/versions/1:publish", saAdmin, merge(idem(), ifm(tag)), nil)
	if must(t, r, http.StatusUnprocessableEntity, "publish in review"); r.str("code") != "INVALID_STATE_TRANSITION" {
		t.Fatalf("publish in review %s", r.raw)
	}
	// AC-18: content_reviewer has no curriculum.publish.
	must(t, c.do(t, http.MethodPost, "/cms/learning/curricula/"+cur+"/versions/1:transition", cmDuyet, merge(idem(), ifm(tag)),
		map[string]any{"action": "approve"}), http.StatusForbidden, "reviewer without curriculum.publish approves")
	must(t, c.do(t, http.MethodPost, "/cms/learning/curricula/"+cur+"/versions/1:publish", cmDuyet, merge(idem(), ifm(tag)), nil),
		http.StatusForbidden, "reviewer without curriculum.publish publishes")
	must(t, c.transition(t, saAdmin, cur, 1, "reject", "thiếu LO"), http.StatusOK, "reject")
	must(t, c.transition(t, cmNoidung, cur, 1, "revise", ""), http.StatusOK, "revise")
	must(t, c.transition(t, cmNoidung, cur, 1, "submit_review", ""), http.StatusOK, "resubmit")
	r = must(t, c.transition(t, saAdmin, cur, 1, "approve", ""), http.StatusOK, "approve")
	if r.str("status") != "approved" || r.str("reviewer_id") != c.users[saAdmin].UserID.String() {
		t.Fatalf("approve %s", r.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.content_transitions WHERE content_type = 'curriculum' AND content_version_id = $1`, ver); n != 5 {
		t.Fatalf("%d transitions", n)
	}
	for _, act := range []string{"content.submitted", "content.approved", "content.rejected", "content.revised"} {
		if c.audits(ctx, t, act, nil) == 0 {
			t.Errorf("no audit %s", act)
		}
	}

	// AC-15: publish in one transaction; the same Idempotency-Key replays the response, nothing twice.
	tag = c.versionETag(t, saAdmin, cur, 1)
	key := idem()
	r = must(t, c.do(t, http.MethodPost, "/cms/learning/curricula/"+cur+"/versions/1:publish", saAdmin, merge(key, ifm(tag)), nil), http.StatusOK, "publish")
	if r.str("status") != "published" || r.str("is_current") != "true" || r.str("published_at") == "" {
		t.Fatalf("published %s", r.raw)
	}
	again := must(t, c.do(t, http.MethodPost, "/cms/learning/curricula/"+cur+"/versions/1:publish", saAdmin, merge(key, ifm(tag)), nil), http.StatusOK, "replay")
	if again.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay %v", again.header)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.content_releases WHERE item_type = 'curriculum' AND item_id = $1
		AND to_version_id = $2 AND action = 'publish'`, cur, ver); n != 1 {
		t.Fatalf("%d releases", n)
	}
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'curriculum.version_published'
		AND envelope->'payload'->>'curriculum_version_id' = $1`, ver); n != 1 {
		t.Fatalf("%d curriculum.version_published", n)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM curriculum.curricula WHERE curriculum_id = $1 AND current_version_id = $2`, cur, ver); n != 1 {
		t.Fatal("release pointer not set")
	}
	if c.audits(ctx, t, "content.published", nil) != 1 {
		t.Fatal("audit content.published")
	}

	// AC-16: a published version never changes — use case (409) and trigger.
	r = c.do(t, http.MethodPatch, "/cms/learning/curricula/"+cur+"/versions/1", saAdmin, ifm(c.versionETag(t, saAdmin, cur, 1)),
		map[string]any{"change_note": "sửa"})
	if must(t, r, http.StatusConflict, "patch published"); r.str("code") != "CONTENT_NOT_EDITABLE" {
		t.Fatalf("patch published %s", r.raw)
	}
	if _, err := c.platform.ExecContext(ctx, `UPDATE curriculum.curriculum_versions SET version_label = '9.9' WHERE curriculum_version_id = $1`, ver); err == nil ||
		!strings.Contains(err.Error(), "CONTENT_NOT_EDITABLE") {
		t.Fatalf("trigger on published version: %v", err)
	}
	if _, err := c.platform.ExecContext(ctx, `UPDATE curriculum.topics SET name = 'x' WHERE topic_id = $1`, topic); err == nil {
		t.Fatal("trigger on a topic of a published version")
	}

	// AC-17 (+ AC-02): a new draft version deep-copies the structure; a second open draft is refused (409).
	r = must(t, c.do(t, http.MethodPost, "/cms/learning/curricula/"+cur+"/versions", cmNoidung, idem(),
		map[string]any{"from_version": 1, "version_label": "1.1", "change_note": "bổ sung"}), http.StatusCreated, "new version")
	v2 := r.str("curriculum_version_id")
	if r.str("status") != "draft" || r.str("version") != "2" || r.str("supersedes_version_id") != ver || len(r.items("subject_grades")) != 1 {
		t.Fatalf("new version %s", r.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM curriculum.learning_objectives n JOIN curriculum.learning_objectives o
		ON o.code = n.code AND o.stable_key = n.stable_key AND o.objective_id <> n.objective_id
		WHERE n.curriculum_version_id = $1 AND o.curriculum_version_id = $2`, v2, ver); n != 2 {
		t.Fatalf("%d LOs copied with code and stable_key", n)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM curriculum.learning_objectives n JOIN curriculum.learning_objectives p
		ON n.prerequisite_ids @> to_jsonb(ARRAY[p.objective_id::text]) WHERE n.curriculum_version_id = $1 AND p.curriculum_version_id = $1`, v2); n != 1 {
		t.Fatalf("prerequisites not remapped into the new version (%d)", n)
	}
	r = c.do(t, http.MethodPost, "/cms/learning/curricula/"+cur+"/versions", cmNoidung, idem(), map[string]any{"from_version": 1, "version_label": "1.2"})
	if must(t, r, http.StatusConflict, "second open draft"); r.str("code") != "CONFLICT" {
		t.Fatalf("second draft %s", r.raw)
	}
	l := must(t, c.do(t, http.MethodGet, "/cms/learning/curricula/"+cur+"/versions", cmNoidung, nil, nil), http.StatusOK, "versions")
	if len(l.items("data")) != 2 || l.items("data")[0]["status"] != "draft" || l.items("data")[1]["status"] != "published" {
		t.Fatalf("versions %s", l.raw)
	}

	// AC-14: GDPT2018 is published only when every LO has yccd_ref; the error lists the LO codes.
	g, gv := c.curriculum(t, cmNoidung, "GDPT2018")
	gt := c.topic(t, cmNoidung, gv, math, "MATH.G1.T01", 1, "")
	must(t, c.objective(t, cmNoidung, gt, "MATH.G1.NUM.COUNT_10", nil), http.StatusCreated, "LO without yccd")
	must(t, c.objective(t, cmNoidung, gt, "MATH.G1.NUM.COMPARE_10", map[string]any{"yccd_ref": "TOAN1.SO.1"}), http.StatusCreated, "LO with yccd")
	r = c.publishFlow(t, cmNoidung, g, 1)
	if must(t, r, http.StatusUnprocessableEntity, "GDPT2018 without yccd_ref"); !strings.Contains(r.raw, "MATH.G1.NUM.COUNT_10") ||
		strings.Contains(r.raw, "MATH.G1.NUM.COMPARE_10") {
		t.Fatalf("missing yccd %s", r.raw)
	}
	var codes []string
	_ = json.Unmarshal([]byte(r.str("objective_codes")), &codes)
	if c.audits(ctx, t, "content.published", nil) != 1 {
		t.Fatal("refused publish was audited as published")
	}
}
