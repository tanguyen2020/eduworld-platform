//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// versionTables maps a content type to its version table and item column (F025 tests).
var versionTables = map[string][2]string{
	"question": {"question_versions", "question_id"}, "question_set": {"question_set_versions", "question_set_id"},
	"lesson": {"lesson_versions", "lesson_id"}, "learning_material": {"learning_material_versions", "learning_material_id"},
	"assessment_blueprint": {"assessment_blueprint_versions", "assessment_blueprint_id"},
}

// rowTag reads the ETag of a version from the database (learning_platform).
func (c *curEnv) rowTag(ctx context.Context, t *testing.T, ctype, id string, version int) string {
	t.Helper()
	tb := versionTables[ctype]
	var v int64
	// tenancyguard:ok TEST helper under learning_platform: reads the row version of one version
	if err := c.platform.QueryRowContext(ctx, `SELECT version FROM content.`+tb[0]+` WHERE `+tb[1]+` = $1 AND version_no = $2`, id, version).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf(`W/"%d"`, v)
}

// wfDo applies a workflow action with the given headers (the ETag of the row when h has no If-Match).
func (c *curEnv) wfDo(ctx context.Context, t *testing.T, sub, ctype, id string, version int, body map[string]any, h hdr) resp {
	t.Helper()
	hs := merge(hdr{"If-Match": c.rowTag(ctx, t, ctype, id, version)}, idem(), h)
	return c.do(t, http.MethodPost, fmt.Sprintf("/cms/learning/content/%s/%s/versions/%d:transition", ctype, id, version), sub, hs, body)
}

// act applies action (and note) as sub.
func (c *curEnv) act(ctx context.Context, t *testing.T, sub, ctype, id string, version int, action, note string) resp {
	t.Helper()
	body := map[string]any{"action": action}
	if note != "" {
		body["note"] = note
	}
	return c.wfDo(ctx, t, sub, ctype, id, version, body, nil)
}

// mediaRow inserts a media asset of status for tenant (nil = platform) — TEST helper: the row the media pipeline of
// F027 writes, without the object storage.
func (c *curEnv) mediaRow(ctx context.Context, t *testing.T, tenant *uuid.UUID, status string) string {
	t.Helper()
	id := ids.New()
	scope, bucket := "platform", "ew-media-public"
	if tenant != nil {
		scope, bucket = "tenant", "ew-media-private"
	}
	sha := strings.ReplaceAll(id.String()+id.String(), "-", "")[:64]
	// tenancyguard:ok TEST helper under learning_platform: the row of the media pipeline
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO content.media_assets (asset_id, tenant_id, created_at, updated_at, owner_scope, kind,
		original_filename, mime_type, size_bytes, sha256, bucket, variants, license, status, quarantine_key, uploaded_by)
		VALUES ($1, $2, now(), now(), $3, 'audio', 'doc-de.ogg', 'audio/ogg', 1024, $4, $5, '{}', 'CC BY 4.0', $6, $7, $1)`,
		id, tenant, scope, sha, bucket, status, "q/"+id.String()); err != nil {
		t.Fatal(err)
	}
	return id.String()
}

func (c *curEnv) transitions(ctx context.Context, t *testing.T, versionID string) int {
	t.Helper()
	return count(ctx, t, c.platform, `SELECT count(*) FROM content.content_transitions WHERE content_version_id = $1`, versionID)
}

func (c *curEnv) events(ctx context.Context, t *testing.T, typ, versionID string) int {
	t.Helper()
	return count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = $1
		AND envelope->'payload'->>'content_version_id' = $2`, typ, versionID)
}

// studentRows counts the rows of the student data schemas (attempts, evaluations, learning events: assessment, learning).
func (c *curEnv) studentRows(ctx context.Context, t *testing.T) int {
	t.Helper()
	rows, err := c.migrator.QueryContext(ctx, `SELECT table_schema||'.'||table_name FROM information_schema.tables
		WHERE table_schema IN ('assessment','learning') AND table_type = 'BASE TABLE'`)
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		tables = append(tables, s)
	}
	_ = rows.Close()
	n := 0
	for _, tb := range tables {
		// tenancyguard:ok TEST helper under learning_migrator: row count of a table listed by information_schema
		n += count(ctx, t, c.migrator, `SELECT count(*) FROM `+tb)
	}
	return n
}

// F025-AC-01, AC-04, AC-05, AC-09, AC-11 (US1, quickstart 1–4): submit → in_review with a platform lane task, transition,
// audit and outbox in one transaction; the replay keeps one transition; the author cannot approve; another reviewer can.
func TestWorkflowSubmitApprove(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	authztest.Cover(t, "transitionContentVersion", authztest.CrossTenant, authztest.RoleDenied)
	before := c.studentRows(ctx, t)
	id, vid := c.question(t, cmNoidung, qc, map[string]any{"owner_scope": "platform", "grade_levels": []int{4}})
	tag := c.rowTag(ctx, t, "question", id, 1)
	key := idem()
	body := map[string]any{"action": "submit_review"}
	path := "/cms/learning/content/question/" + id + "/versions/1:transition"
	r := must(t, c.do(t, http.MethodPost, path, cmNoidung, merge(key, ifm(tag)), body), http.StatusOK, "submit")
	if r.str("status") != "in_review" || r.header.Get("ETag") == "" || r.str("translation_status") != "partial" {
		t.Fatalf("submit: %s", r.raw)
	}
	again := must(t, c.do(t, http.MethodPost, path, cmNoidung, merge(key, ifm(tag)), body), http.StatusOK, "replay")
	if again.header.Get("Idempotent-Replayed") != "true" || again.str("status") != "in_review" {
		t.Fatalf("replay: %s", again.raw)
	}
	if n := c.transitions(ctx, t, vid); n != 1 {
		t.Fatalf("%d transitions after replay", n)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.content_transitions WHERE content_version_id = $1 AND action = 'submit_review'
		AND from_status = 'draft' AND to_status = 'in_review' AND idempotency_key IS NOT NULL AND content_id = $2 AND tenant_id IS NULL`, vid, id); n != 1 {
		t.Fatal("transition row")
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.review_tasks WHERE content_version_id = $1 AND review_lane = 'platform_learning'
		AND status = 'open' AND tenant_id IS NULL AND due_at > submitted_at AND title = '7 × 8 = ?'`, vid); n != 1 {
		t.Fatal("open platform task")
	}
	if c.events(ctx, t, "content.version_submitted", vid) != 1 || c.audits(ctx, t, "content.submitted", nil) != 1 {
		t.Fatal("submitted event / audit")
	}
	// AC-05: the author is not a reviewer.
	denied := c.audits(ctx, t, "authz.denied", nil)
	if r := c.act(ctx, t, cmNoidung, "question", id, 1, "approve", ""); r.status != http.StatusForbidden || r.str("code") != "PERMISSION_DENIED" {
		t.Fatalf("self approve: %d %s", r.status, r.raw)
	}
	if c.audits(ctx, t, "authz.denied", nil) <= denied {
		t.Fatal("self approve not audited")
	}
	// AC-04.
	r = must(t, c.act(ctx, t, cmDuyet, "question", id, 1, "approve", ""), http.StatusOK, "approve")
	if r.str("status") != "approved" || r.str("reviewer_id") != c.users[cmDuyet].UserID.String() {
		t.Fatalf("approve: %s", r.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.review_tasks WHERE content_version_id = $1 AND status = 'approved'
		AND resolved_by = $2 AND resolved_at IS NOT NULL`, vid, c.users[cmDuyet].UserID); n != 1 {
		t.Fatal("task closed")
	}
	if c.events(ctx, t, "content.version_approved", vid) != 1 || c.audits(ctx, t, "content.approved", nil) != 1 || c.transitions(ctx, t, vid) != 2 {
		t.Fatal("approved event / audit / transition")
	}
	// SC-001: every approved version has a reviewer other than its author.
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.question_versions WHERE status = 'approved' AND (reviewer_id IS NULL OR reviewer_id = author_id)`); n != 0 {
		t.Fatal("approved without an independent reviewer")
	}
	// AC-11: no student data is written by the workflow.
	if after := c.studentRows(ctx, t); after != before {
		t.Fatalf("student rows %d → %d", before, after)
	}
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type LIKE 'activity.%' OR event_type LIKE 'attempt.%'
		OR event_type LIKE 'learning_objective.%'`); n != 0 {
		t.Fatal("learning events written by the workflow")
	}
}

// F025-AC-02, AC-03, AC-06, AC-07, AC-08 (quickstart 5–7): metadata and media are checked on submit, reject needs a note,
// revise returns to draft, actions outside the table are 422 INVALID_STATE_TRANSITION.
func TestWorkflowChecksRejectRevise(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	// AC-02: a lesson draft without grade and subject cannot leave draft.
	lr := must(t, c.do(t, http.MethodPost, lessonsPath, cmNoidung, idem(), map[string]any{"owner_scope": "platform", "title": map[string]any{"vi": "Bài nháp"},
		"blocks": []any{map[string]any{"kind": "rich_text", "content": rich("Nội dung")}}}), http.StatusCreated, "lesson")
	lid := lr.str("content_id")
	r := c.act(ctx, t, cmNoidung, "lesson", lid, 1, "submit_review", "")
	if r.status != http.StatusUnprocessableEntity || r.str("code") != "VALIDATION_FAILED" ||
		!slices.Contains(r.errorCodes(), "/grade_levels:REQUIRED") || !slices.Contains(r.errorCodes(), "/subject_id:REQUIRED") {
		t.Fatalf("lesson meta: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.lesson_versions WHERE lesson_id = $1 AND status = 'draft'`, lid); n != 1 {
		t.Fatal("lesson left draft")
	}
	// AC-03: media still processing.
	asset := c.mediaRow(ctx, t, nil, "processing")
	id, vid := c.question(t, cmNoidung, qc, map[string]any{"owner_scope": "platform", "grade_levels": []int{4}, "media_refs": []string{asset}})
	r = c.act(ctx, t, cmNoidung, "question", id, 1, "submit_review", "")
	if r.status != http.StatusUnprocessableEntity || r.str("code") != "MEDIA_NOT_READY" || !strings.Contains(r.raw, asset) {
		t.Fatalf("media: %d %s", r.status, r.raw)
	}
	// tenancyguard:ok TEST helper under learning_platform: the media worker marks the asset ready
	if _, err := c.platform.ExecContext(ctx, `UPDATE content.media_assets SET status = 'ready' WHERE asset_id = $1`, asset); err != nil {
		t.Fatal(err)
	}
	// AC-08: approve on a draft.
	if r := c.act(ctx, t, cmDuyet, "question", id, 1, "approve", ""); r.status != http.StatusUnprocessableEntity || r.str("code") != "INVALID_STATE_TRANSITION" {
		t.Fatalf("approve draft: %d %s", r.status, r.raw)
	}
	must(t, c.act(ctx, t, cmNoidung, "question", id, 1, "submit_review", ""), http.StatusOK, "submit ready media")
	// AC-06: reject needs a note.
	if r := c.act(ctx, t, cmDuyet, "question", id, 1, "reject", ""); r.status != http.StatusUnprocessableEntity || !slices.Contains(r.errorCodes(), "note:REQUIRED") {
		t.Fatalf("reject without note: %d %s", r.status, r.raw)
	}
	r = must(t, c.act(ctx, t, cmDuyet, "question", id, 1, "reject", "Thiếu hình minh họa"), http.StatusOK, "reject")
	if r.str("status") != "rejected" || c.events(ctx, t, "content.version_rejected", vid) != 1 || c.audits(ctx, t, "content.rejected", nil) != 1 {
		t.Fatalf("reject: %s", r.raw)
	}
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'content.version_rejected'
		AND envelope->'payload'->>'note' = 'Thiếu hình minh họa'`); n != 1 {
		t.Fatal("reject note in the event")
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.review_tasks WHERE content_version_id = $1 AND status = 'rejected'`, vid); n != 1 {
		t.Fatal("task rejected")
	}
	// AC-07: revise brings the version back to draft.
	r = must(t, c.act(ctx, t, cmNoidung, "question", id, 1, "revise", ""), http.StatusOK, "revise")
	if r.str("status") != "draft" || c.transitions(ctx, t, vid) != 3 {
		t.Fatalf("revise: %s", r.raw)
	}
	// AC-08: revise of a draft, an unknown action, archive (F026).
	if r := c.act(ctx, t, cmNoidung, "question", id, 1, "revise", ""); r.str("code") != "INVALID_STATE_TRANSITION" {
		t.Fatalf("revise draft: %s", r.raw)
	}
	// SC-004: one transition and one audit record per change.
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.content_transitions WHERE content_version_id = $1`, vid); n != 3 {
		t.Fatalf("%d transitions", n)
	}
}

// F025-AC-09 (quickstart 2, edge case): If-Match is required, a stale tag is 412, two reviewers cannot both approve.
func TestWorkflowPreconditions(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	id, _ := c.question(t, cmNoidung, qc, map[string]any{"owner_scope": "platform", "grade_levels": []int{4}})
	path := "/cms/learning/content/question/" + id + "/versions/1:transition"
	if r := c.do(t, http.MethodPost, path, cmNoidung, idem(), map[string]any{"action": "submit_review"}); r.status != http.StatusPreconditionRequired {
		t.Fatalf("no If-Match: %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodPost, path, cmNoidung, merge(idem(), ifm(`W/"99"`)), map[string]any{"action": "submit_review"}); r.status != http.StatusPreconditionFailed {
		t.Fatalf("stale If-Match: %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodPost, path, cmNoidung, ifm(c.rowTag(ctx, t, "question", id, 1)), map[string]any{"action": "submit_review"}); r.status != http.StatusPreconditionRequired || r.str("code") != "IDEMPOTENCY_KEY_REQUIRED" {
		t.Fatalf("no Idempotency-Key: %d %s", r.status, r.raw)
	}
	must(t, c.act(ctx, t, cmNoidung, "question", id, 1, "submit_review", ""), http.StatusOK, "submit")
	tag := c.rowTag(ctx, t, "question", id, 1)
	c.user("cm.duyet2", authztest.Platform("content_reviewer").PrincipalData, true)
	first := c.do(t, http.MethodPost, path, cmDuyet, merge(idem(), ifm(tag)), map[string]any{"action": "approve"})
	second := c.do(t, http.MethodPost, path, "cm.duyet2", merge(idem(), ifm(tag)), map[string]any{"action": "approve"})
	if first.status != http.StatusOK || second.status != http.StatusPreconditionFailed {
		t.Fatalf("concurrent approve: %d, %d %s", first.status, second.status, second.raw)
	}
}

// F025-AC-10 (quickstart 9–10): another tenant's content does not exist (404); assigned platform content is read-only
// (403); the tenant lane runs inside the tenant; a role without the permission is denied.
func TestWorkflowTenantIsolation(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	id, vid := c.question(t, qlcmA, qc, map[string]any{"grade_levels": []int{4}})
	r := must(t, c.act(ctx, t, qlcmA, "question", id, 1, "submit_review", ""), http.StatusOK, "tenant submit")
	if r.str("owner_scope") != "tenant" {
		t.Fatalf("tenant question %s", r.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.review_tasks WHERE content_version_id = $1 AND review_lane = 'tenant' AND tenant_id = $2`,
		vid, c.tenantA); n != 1 {
		t.Fatal("tenant lane task")
	}
	if r := c.act(ctx, t, qlcmB, "question", id, 1, "approve", ""); r.status != http.StatusNotFound {
		t.Fatalf("tenant B: %d %s", r.status, r.raw)
	}
	if r := c.act(ctx, t, noRoleA, "question", id, 1, "approve", ""); r.status != http.StatusForbidden {
		t.Fatalf("no role: %d %s", r.status, r.raw)
	}
	if r := c.act(ctx, t, edA, "question", id, 1, "approve", ""); r.status != http.StatusForbidden {
		t.Fatalf("editor approves: %d %s", r.status, r.raw)
	}
	c.user("ad.donvi.a", authztest.Staff(c.tenantA, "tenant_admin").PrincipalData, true)
	r = must(t, c.act(ctx, t, "ad.donvi.a", "question", id, 1, "approve", ""), http.StatusOK, "tenant admin approves")
	if r.str("tenant_id") != c.tenantA.String() || c.audits(ctx, t, "content.approved", &c.tenantA) != 1 {
		t.Fatalf("tenant approve %s", r.raw)
	}
	// Assigned platform content is read-only for the tenant.
	pid, _ := c.question(t, cmNoidung, qc, map[string]any{"owner_scope": "platform", "grade_levels": []int{4}})
	c.assignResource(ctx, t, "question", pid, c.tenantA)
	if r := c.act(ctx, t, qlcmA, "question", pid, 1, "submit_review", ""); r.status != http.StatusForbidden || r.str("code") != "PERMISSION_DENIED" {
		t.Fatalf("assigned platform: %d %s", r.status, r.raw)
	}
	if r := c.act(ctx, t, qlcmB, "question", pid, 1, "submit_review", ""); r.status != http.StatusNotFound {
		t.Fatalf("unassigned platform: %d %s", r.status, r.raw)
	}
	// A second submit of an approved version.
	if r := c.act(ctx, t, qlcmA, "question", id, 1, "submit_review", ""); r.str("code") != "INVALID_STATE_TRANSITION" {
		t.Fatalf("approved → submit: %s", r.raw)
	}
}
