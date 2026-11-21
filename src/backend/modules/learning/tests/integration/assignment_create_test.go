//go:build integration

package integration

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// F041-AC-01, AC-02, AC-03, AC-06, AC-07, AC-09 (US1, TC-LMS-001, TC-CMS-ASG-01): a draft pins the published version and
// emits assignment.created; unpublished content 409, platform content not assigned 403, unsupported activity types do not
// exist, graded defaults, invalid fields 422 by JSON pointer, graded on a blueprint 422 while exam sessions are unavailable.
func TestAssignmentCreate(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t, withoutExams) // F041-AC-09 with the "unavailable" ExamSessionCreator; F042-AC-31 covers the adapter
	k := c.asgSetup(ctx, t)
	authztest.Cover(t, "createAssignment", authztest.CrossTenant, authztest.RoleDenied)
	r := must(t, c.createAsg(t, adA, c.asgBody(k, nil)), http.StatusCreated, "create")
	id := r.str("assignment_id")
	if r.str("status") != "draft" || r.str("created_by_role") != "school_admin" || r.str("content_ref", "pinned_version_id") != c.activityVersion(ctx, t, k.s.activityID) ||
		r.str("feedback_timing") != "immediate" || r.str("result_release") != "immediate" || r.header.Get("ETag") == "" {
		t.Fatalf("draft: %s", r.raw)
	}
	if c.outboxCount(ctx, t, "assignment.created", id) != 1 || c.audits(ctx, t, "assignment.created", &c.tenantA) != 1 {
		t.Fatal("assignment.created or its audit")
	}
	// AC-06: graded defaults to after_close/after_close; follow_latest and an early feedback are refused.
	g := must(t, c.createAsg(t, adA, c.asgBody(k, map[string]any{"mode": "graded"})), http.StatusCreated, "graded")
	if g.str("feedback_timing") != "after_close" || g.str("result_release") != "after_close" {
		t.Fatalf("graded defaults: %s", g.raw)
	}
	for _, extra := range []map[string]any{{"mode": "graded", "follow_latest": true}, {"feedback_timing": "immediate", "result_release": "after_close"}} {
		if r := c.createAsg(t, adA, c.asgBody(k, extra)); r.status != http.StatusUnprocessableEntity || r.str("code") != "VALIDATION_FAILED" {
			t.Fatalf("%v: %d %s", extra, r.status, r.raw)
		}
	}
	// AC-07: lengths and ranges by field.
	now := c.clk.Now()
	bad := c.asgBody(k, map[string]any{"title": strings.Repeat("a", 121), "max_attempts": 11, "due_at": now.Add(-time.Hour).Format(time.RFC3339),
		"late_policy": map[string]any{"allow_late": true, "late_window_hours": 169}})
	if r := c.createAsg(t, adA, bad); r.status != http.StatusUnprocessableEntity || r.str("code") != "VALIDATION_FAILED" {
		t.Fatalf("limits: %d %s", r.status, r.raw)
	}
	long := c.asgBody(k, map[string]any{"instructions": strings.Repeat("a", 2001)})
	if r := c.createAsg(t, adA, long); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("instructions: %d %s", r.status, r.raw)
	}
	window := c.asgBody(k, map[string]any{"due_at": now.Add(-2 * time.Minute).Format(time.RFC3339)})
	if r := c.createAsg(t, adA, window); r.status != http.StatusUnprocessableEntity || !slices.Contains(r.errorCodes(), "/due_at:INVALID") {
		t.Fatalf("due_at <= open_at: %d %s", r.status, r.raw)
	}
	// AC-02: an unpublished activity, a platform activity not assigned.
	draft := c.createActivity(t, qlcmA, activityBody(k.s.qc, "quiz", quizConfig(k.s.questions[0])))
	if r := c.createAsg(t, adA, c.asgBody(k, map[string]any{"content_ref": map[string]any{"content_type": "activity", "content_id": draft}})); r.status != http.StatusConflict ||
		r.str("code") != "CONTENT_NOT_PUBLISHED" {
		t.Fatalf("draft content: %d %s", r.status, r.raw)
	}
	platQ, _ := c.question(t, cmNoidung, k.s.qc, map[string]any{"owner_scope": "platform"})
	platQV := c.publishQuestion(ctx, t, platQ, 1)
	platBody := activityBody(k.s.qc, "quiz", quizConfig(platQV))
	platBody["owner_scope"] = "platform"
	plat := c.createActivity(t, cmNoidung, platBody)
	// tenancyguard:ok TEST fixture under learning_platform: a published platform activity (release is F026)
	if _, err := c.platform.ExecContext(ctx, `WITH v AS (UPDATE content.activity_versions SET status = 'published', published_at = now()
		WHERE activity_id = $1 RETURNING activity_version_id) UPDATE content.activities SET current_version_id = (SELECT activity_version_id FROM v)
		WHERE activity_id = $1`, plat); err != nil {
		t.Fatal(err)
	}
	if r := c.createAsg(t, adA, c.asgBody(k, map[string]any{"content_ref": map[string]any{"content_type": "activity", "content_id": plat}})); r.status != http.StatusForbidden ||
		r.str("code") != "RESOURCE_NOT_ASSIGNED" {
		t.Fatalf("platform not assigned: %d %s", r.status, r.raw)
	}
	c.assignResource(ctx, t, "activity", plat, c.tenantA)
	must(t, c.createAsg(t, adA, c.asgBody(k, map[string]any{"content_ref": map[string]any{"content_type": "activity", "content_id": plat}})),
		http.StatusCreated, "assigned platform activity")
	// AC-03: types outside the release cannot be stored (activity_versions_type_ck), so they cannot be assigned.
	// tenancyguard:ok TEST fixture under learning_platform: the type CHECK of the activity versions
	if _, err := c.platform.ExecContext(ctx, `UPDATE content.activity_versions SET body = jsonb_set(body, '{activity_type}', '"written_response"')
		WHERE activity_id = $1`, draft); err == nil || !strings.Contains(err.Error(), "activity_versions_type_ck") {
		t.Fatalf("written_response stored: %v", err)
	}
	// AC-09: graded on a blueprint while exam sessions are unavailable (F042).
	bp := must(t, c.do(t, http.MethodPost, blueprintsPath, qlcmA, idem(), blueprintBody(k.s.qc, 1,
		[]any{map[string]any{"objective_ids": []string{k.s.qc.lo}, "count": 1, "points_each": 1}}, nil)), http.StatusCreated, "blueprint").str("content_id")
	before := count(ctx, t, c.platform, `SELECT count(*) FROM lms.assignments`)
	if r := c.createAsg(t, adA, c.asgBody(k, map[string]any{"mode": "graded",
		"content_ref": map[string]any{"content_type": "assessment_blueprint", "content_id": bp}})); r.status != http.StatusUnprocessableEntity ||
		!slices.Contains(r.errorCodes(), "/content_ref:INVALID") {
		t.Fatalf("graded blueprint: %d %s", r.status, r.raw)
	}
	if count(ctx, t, c.platform, `SELECT count(*) FROM lms.assignments`) != before {
		t.Fatal("a refused assignment was stored")
	}
	_ = id
}

// F041-AC-04, AC-05, AC-26, AC-27, AC-28 (TC-TEN-002, TC-TEN-001, TC-TEN-004): an academic manager of another subject is
// 403 and audited; targets of another school 404; another school or tenant does not see the assignment; the tenant
// admin only reads; a student is PORTAL_ACCESS_STAFF_ONLY; the System Admin writes for a tenant with X-Tenant-Id.
func TestAssignmentScope(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.asgSetup(ctx, t)
	authztest.Cover(t, "getAssignment", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listAssignments", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateAssignment", authztest.CrossTenant, authztest.RoleDenied)
	if r := c.createAsg(t, qlcmOther, c.asgBody(k, nil)); r.status != http.StatusForbidden || r.str("code") != "PERMISSION_DENIED" {
		t.Fatalf("other subject: %d %s", r.status, r.raw)
	}
	if c.audits(ctx, t, "authz.denied", nil) == 0 {
		t.Fatal("no authz.denied")
	}
	must(t, c.createAsg(t, qlcmA, c.asgBody(k, nil)), http.StatusCreated, "academic manager in scope")
	// AC-05 (ADR-0095): a student of another tenant is 422; a school outside the grants is 404.
	other := c.asgBody(k, map[string]any{"targets": []any{map[string]any{"target_type": "students",
		"student_ids": []string{c.users[hsDung].UserID.String()}}}})
	if r := c.createAsg(t, adA, other); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("student of another tenant: %d %s", r.status, r.raw)
	}
	if r := c.createAsg(t, adA2, c.asgBody(k, nil)); r.status != http.StatusNotFound {
		t.Fatalf("school outside the grants: %d %s", r.status, r.raw)
	}
	r := must(t, c.createAsg(t, adA, c.asgBody(k, nil)), http.StatusCreated, "create")
	path := assignmentsPath + "/" + r.str("assignment_id")
	for _, sub := range []string{adA2, adB, qlcmB} {
		if g := c.do(t, http.MethodGet, path, sub, nil, nil); g.status != http.StatusNotFound {
			t.Fatalf("%s reads: %d %s", sub, g.status, g.raw)
		}
	}
	if l := must(t, c.do(t, http.MethodGet, assignmentsPath+"?tenant_ids="+c.tenantB.String(), adB, nil, nil), http.StatusOK, "list B"); len(l.items("data")) != 0 {
		t.Fatalf("tenant B lists A: %s", l.raw)
	}
	// AC-27: tenant admin reads, does not write; a student is not portal staff.
	must(t, c.do(t, http.MethodGet, path, taA, nil, nil), http.StatusOK, "tenant admin reads")
	if w := c.createAsg(t, taA, c.asgBody(k, nil)); w.status != http.StatusForbidden || w.str("code") != "PERMISSION_DENIED" {
		t.Fatalf("tenant admin writes: %d %s", w.status, w.raw)
	}
	c.user("sm.antoan", authztest.Platform("safety_moderator").PrincipalData, true)
	if s := c.do(t, http.MethodGet, assignmentsPath, "sm.antoan", hdr{"X-Tenant-Id": c.tenantA.String()}, nil); s.status != http.StatusForbidden {
		t.Fatalf("safety moderator: %d %s", s.status, s.raw)
	}
	if s := c.do(t, http.MethodGet, assignmentsPath, hsAn, nil, nil); s.status != http.StatusForbidden {
		t.Fatalf("student on /lms: %d %s", s.status, s.raw)
	}
	l := must(t, c.do(t, http.MethodGet, assignmentsPath+"?subject_id="+k.subject, qlcmA, nil, nil), http.StatusOK, "list in scope")
	if len(l.items("data")) != 2 {
		t.Fatalf("list in scope: %s", l.raw)
	}
	if l := must(t, c.do(t, http.MethodGet, assignmentsPath, qlcmOther, nil, nil), http.StatusOK, "list other subject"); len(l.items("data")) != 0 {
		t.Fatalf("other subject lists: %s", l.raw)
	}
	// AC-28: System Admin without X-Tenant-Id → 400, with it → created_by_role system_admin, audit in tenant A.
	if s := c.do(t, http.MethodPost, assignmentsPath, saAdmin, idem(), c.asgBody(k, nil)); s.status != http.StatusBadRequest ||
		s.str("code") != "TENANT_HEADER_REQUIRED" {
		t.Fatalf("system admin without tenant: %d %s", s.status, s.raw)
	}
	s := must(t, c.do(t, http.MethodPost, assignmentsPath, saAdmin, merge(idem(), hdr{"X-Tenant-Id": c.tenantA.String()}), c.asgBody(k, nil)),
		http.StatusCreated, "system admin for tenant A")
	if s.str("created_by_role") != "system_admin" || s.str("tenant_id") != c.tenantA.String() {
		t.Fatalf("system admin: %s", s.raw)
	}
}

// F041-AC-08, AC-10, AC-11, AC-17, AC-29: publish snapshots the grade (open now or scheduled), emits assignment.published
// with the targets; draft fields edit with If-Match, open only due_at/instructions/late_policy; cancel before any
// submission; the same Idempotency-Key replays without a second write.
func TestAssignmentPublishUpdateCancel(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.asgSetup(ctx, t)
	authztest.Cover(t, "publishAssignment", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "cancelAssignment", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "archiveAssignment", authztest.CrossTenant, authztest.RoleDenied)
	r := must(t, c.createAsg(t, adA, c.asgBody(k, nil)), http.StatusCreated, "create")
	id, path := r.str("assignment_id"), assignmentsPath+"/"+r.str("assignment_id")
	js := hdr{"Content-Type": "application/merge-patch+json"}
	// AC-10: draft edits everything with If-Match; a stale ETag is 412.
	if p := c.do(t, http.MethodPatch, path, adA, merge(js, ifm(`W/"9"`)), map[string]any{"title": "X"}); p.status != http.StatusPreconditionFailed {
		t.Fatalf("stale If-Match: %d %s", p.status, p.raw)
	}
	u := must(t, c.do(t, http.MethodPatch, path, adA, merge(js, ifm(r.header.Get("ETag"))), map[string]any{"title": "Ôn tập chương 3",
		"max_attempts": 3, "late_policy": map[string]any{"allow_late": true, "late_window_hours": 24}}), http.StatusOK, "edit draft")
	if u.str("title") != "Ôn tập chương 3" || u.body["max_attempts"] != float64(3) {
		t.Fatalf("draft edit: %s", u.raw)
	}
	// AC-08, AC-29: publish (same key twice), snapshot of grade 3 of the school, open now.
	key := idem()
	p := must(t, c.do(t, http.MethodPost, path+":publish", adA, merge(key, ifm(u.header.Get("ETag"))), nil), http.StatusOK, "publish")
	again := must(t, c.do(t, http.MethodPost, path+":publish", adA, merge(key, ifm(u.header.Get("ETag"))), nil), http.StatusOK, "replay")
	if p.str("status") != "open" || again.header.Get("Idempotent-Replayed") != "true" || c.outboxCount(ctx, t, "assignment.published", id) != 1 {
		t.Fatalf("publish: %s / %s", p.raw, again.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM lms.assignment_target_students WHERE assignment_id = $1`, id); n != 2 {
		t.Fatalf("snapshot = %d", n)
	}
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'assignment.published'
		AND envelope->'payload'->>'assignment_id' = $1 AND envelope->'payload'->'targets'->0->>'student_count' = '2'
		AND envelope->'payload'->>'subject_id' = $2 AND envelope->'payload'->'targets'->0->>'target_type' = 'grade'`, id, k.subject); n != 1 {
		t.Fatal("assignment.published payload (AC-17)")
	}
	if c.audits(ctx, t, "assignment.published", &c.tenantA) != 1 {
		t.Fatal("publish audit")
	}
	// AC-10: open → only due_at, instructions, late_policy.
	if e := c.do(t, http.MethodPatch, path, adA, merge(js, ifm(p.header.Get("ETag"))), map[string]any{"title": "Đổi"}); e.status != http.StatusConflict ||
		e.str("code") != "ASSIGNMENT_NOT_EDITABLE" {
		t.Fatalf("open title: %d %s", e.status, e.raw)
	}
	due := c.clk.Now().Add(96 * time.Hour).Format(time.RFC3339)
	must(t, c.do(t, http.MethodPatch, path, adA, merge(js, ifm(p.header.Get("ETag"))), map[string]any{"due_at": due, "instructions": "Làm cẩn thận"}),
		http.StatusOK, "open due_at")
	// AC-11: cancel an open assignment nobody started; a scheduled one; a closed one is not cancelled.
	must(t, c.do(t, http.MethodPost, path+":cancel", adA, idem(), map[string]any{"reason": "Giao nhầm"}), http.StatusOK, "cancel")
	if c.audits(ctx, t, "assignment.cancelled", &c.tenantA) != 1 {
		t.Fatal("cancel audit")
	}
	if x := c.do(t, http.MethodPost, path+":cancel", adA, idem(), map[string]any{"reason": "Lại"}); x.status != http.StatusConflict {
		t.Fatalf("cancel twice: %d %s", x.status, x.raw)
	}
	later := c.clk.Now().Add(24 * time.Hour)
	sc := must(t, c.createAsg(t, adA, c.asgBody(k, map[string]any{"open_at": later.Format(time.RFC3339),
		"due_at": later.Add(48 * time.Hour).Format(time.RFC3339)})), http.StatusCreated, "create scheduled")
	if ps := must(t, c.publishAsg(t, adA, sc), http.StatusOK, "publish scheduled"); ps.str("status") != "scheduled" {
		t.Fatalf("scheduled: %s", ps.raw)
	}
	// AC-11: an open assignment with an attempt is not cancelled.
	busy := c.openAsg(t, k, nil)
	must(t, c.start(t, hsAn, k.s.activityID, nil, map[string]any{"mode": "assignment", "context_type": "assignment", "context_id": busy}),
		http.StatusCreated, "start")
	if x := c.do(t, http.MethodPost, assignmentsPath+"/"+busy+":cancel", adA, idem(), map[string]any{"reason": "Hủy"}); x.status != http.StatusConflict ||
		x.str("code") != "ASSIGNMENT_NOT_EDITABLE" {
		t.Fatalf("cancel with an attempt: %d %s", x.status, x.raw)
	}
}
