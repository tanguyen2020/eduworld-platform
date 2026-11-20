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

// F042-AC-01…AC-08 (US1, TC-EXM-001, TC-EXM-005, TC-TEN-002, TC-TEN-003, TC-TEN-024): a session from a published
// blueprint pins its version, locks independent_draw variants on the LO matrix, takes the defaults of its type, audits
// exam_session.created and replays; the preview is for the staff who create exams; scope by school and subject; invalid
// fields and pools are named.
func TestExamSessionCreate(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.examSetup(ctx, t, 5, map[string]any{"strategy": "independent_draw", "count": 3})
	authztest.Cover(t, "createExamSession", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listExamSessions", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getExamSession", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listExamSessionPapers", authztest.CrossTenant, authztest.RoleDenied)
	key := idem()
	body := c.sessionBody(k, map[string]any{"type": "chapter", "accommodations": []any{map[string]any{"student_id": c.users[hsAn].UserID,
		"extra_time_percent": 50}}})
	r := must(t, c.do(t, http.MethodPost, examSessionsPath, qlcmA, key, body), http.StatusCreated, "create")
	id := r.str("exam_session_id")
	// AC-01, AC-02: defaults of chapter (official, 1 attempt, after_close), pinned version, open now, 3 variants.
	if r.str("status") != "open" || r.raw == "" || r.str("is_official") != "true" || r.str("result_release") != "manual" ||
		r.str("feedback_timing") != "after_close" || r.num("max_attempts") != 1 || r.str("assessment_blueprint_version_id") != k.version ||
		r.num("variant_count") != 3 || r.str("variant_strategy") != "independent_draw" || r.num("question_count") != 2 || r.str("server_time") == "" {
		t.Fatalf("session: %s", r.raw)
	}
	// AC-07: the same key replays the same session, one audit with the blueprint version.
	again := must(t, c.do(t, http.MethodPost, examSessionsPath, qlcmA, key, body), http.StatusCreated, "replay")
	if again.str("exam_session_id") != id || again.header.Get("Idempotent-Replayed") != "true" ||
		count(ctx, t, c.platform, `SELECT count(*) FROM assessment.exam_sessions WHERE exam_session_id = $1`, id) != 1 ||
		c.audits(ctx, t, "exam_session.created", &c.tenantA) != 1 {
		t.Fatalf("replay: %s", again.raw)
	}
	// AC-02: the variants are locked on the matrix (2 questions of the LO, 2 points each); a paper never changes.
	papers := must(t, c.do(t, http.MethodGet, examSessionsPath+"/"+id+"/papers", qlcmA, nil, nil), http.StatusOK, "papers").items("data")
	if len(papers) != 3 {
		t.Fatalf("papers: %v", papers)
	}
	for i, p := range papers {
		secs := p["sections"].([]any)
		items := secs[0].(map[string]any)["items"].([]any)
		if p["variant_no"] != float64(i+1) || p["total_points"] != float64(4) || len(items) != 2 || items[0].(map[string]any)["points"] != float64(2) ||
			p["seed"] == "" {
			t.Fatalf("paper %d: %v", i, p)
		}
	}
	// tenancyguard:ok TEST check of the immutability trigger under learning_platform
	if _, err := c.platform.ExecContext(ctx, `UPDATE assessment.exam_papers SET items = '[]' WHERE exam_session_id = $1`, id); err == nil {
		t.Fatal("a paper was changed")
	}
	// AC-04: a student cannot preview the papers.
	if x := c.do(t, http.MethodGet, examSessionsPath+"/"+id+"/papers", hsAn, nil, nil); x.status != http.StatusForbidden ||
		x.str("code") != "PERMISSION_DENIED" {
		t.Fatalf("student papers: %d %s", x.status, x.raw)
	}
	// AC-05: the effective duration of the student with extra time (900 s + 50 %); the others keep 900 s.
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.exam_participations WHERE exam_session_id = $1
		AND ((student_id = $2 AND effective_duration_sec = 1350 AND extra_time_percent = 50) OR (student_id = $3 AND effective_duration_sec = 900))`,
		id, c.users[hsAn].UserID, c.users[hsBinh].UserID); n != 2 {
		t.Fatalf("participations: %d", n)
	}
	mine := must(t, c.do(t, http.MethodGet, examSessionsPath+"/"+id, hsAn, nil, nil), http.StatusOK, "student session")
	if mine.num("effective_duration_seconds") != 1350 || mine.raw == "" || strings.Contains(mine.raw, "accommodations") {
		t.Fatalf("student view: %s", mine.raw)
	}
	if l := must(t, c.do(t, http.MethodGet, examSessionsPath, hsBinh, nil, nil), http.StatusOK, "student list").items("data"); len(l) != 1 {
		t.Fatalf("student list: %v", l)
	}
	if l := must(t, c.do(t, http.MethodGet, examSessionsPath, adA, nil, nil), http.StatusOK, "staff list").items("data"); len(l) != 1 ||
		l[0]["accommodations"] == nil {
		t.Fatalf("staff list: %v", l)
	}
	// AC-06: another subject 403, another school or tenant 404, a student without the class 404, no permission 403.
	if x := c.do(t, http.MethodPost, examSessionsPath, qlcmOther, idem(), c.sessionBody(k, nil)); x.status != http.StatusForbidden ||
		x.str("code") != "PERMISSION_DENIED" {
		t.Fatalf("other subject: %d %s", x.status, x.raw)
	}
	if x := c.do(t, http.MethodPost, examSessionsPath, adA2, idem(), c.sessionBody(k, nil)); x.status != http.StatusNotFound {
		t.Fatalf("other school: %d %s", x.status, x.raw)
	}
	if x := c.do(t, http.MethodPost, examSessionsPath, adB, idem(), c.sessionBody(k, nil)); x.status != http.StatusNotFound {
		t.Fatalf("tenant B: %d %s", x.status, x.raw)
	}
	if x := c.do(t, http.MethodGet, examSessionsPath+"/"+id, hsDung, nil, nil); x.status != http.StatusNotFound {
		t.Fatalf("tenant B student: %d", x.status)
	}
	if x := c.do(t, http.MethodGet, examSessionsPath+"/"+id+"/papers", adA2, nil, nil); x.status != http.StatusNotFound {
		t.Fatalf("other school papers: %d", x.status)
	}
	if x := c.do(t, http.MethodPost, examSessionsPath, noRoleA, idem(), c.sessionBody(k, nil)); x.status != http.StatusForbidden {
		t.Fatalf("no role: %d %s", x.status, x.raw)
	}
	if x := c.do(t, http.MethodPost, examSessionsPath, saAdmin, idem(), c.sessionBody(k, nil)); x.status != http.StatusBadRequest ||
		x.str("code") != "TENANT_HEADER_REQUIRED" {
		t.Fatalf("system admin without tenant: %d %s", x.status, x.raw)
	}
	sa := must(t, c.do(t, http.MethodPost, examSessionsPath, saAdmin, merge(idem(), hdr{"X-Tenant-Id": c.tenantA.String()}),
		c.sessionBody(k, map[string]any{"title": "Đề do System Admin tạo"})), http.StatusCreated, "system admin")
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.exam_sessions WHERE exam_session_id = $1 AND tenant_id = $2
		AND created_by_role = 'system_admin'`, sa.str("exam_session_id"), c.tenantA); n != 1 {
		t.Fatal("system admin session")
	}
	// AC-08 and AC-03: every invalid field is named; immediate feedback; a pool too small for the variants.
	now := c.clk.Now()
	bad := c.sessionBody(k, map[string]any{"title": strings.Repeat("đ", 256), "max_attempts": 6,
		"accommodations": []any{map[string]any{"student_id": c.users[hsAn].UserID, "extra_time_percent": 101}}})
	x := c.do(t, http.MethodPost, examSessionsPath, qlcmA, idem(), bad)
	if x.status != http.StatusUnprocessableEntity {
		t.Fatalf("invalid: %d %s", x.status, x.raw)
	}
	for _, want := range []string{"title:MAX_LENGTH", "max_attempts:INVALID", "accommodations.0.extra_time_percent:INVALID"} {
		if !slices.Contains(x.errorCodes(), want) {
			t.Errorf("missing %s in %v", want, x.errorCodes())
		}
	}
	window := c.sessionBody(k, map[string]any{"window_end": now.Add(-2 * time.Minute).Format(time.RFC3339)})
	if x := c.do(t, http.MethodPost, examSessionsPath, qlcmA, idem(), window); x.status != http.StatusUnprocessableEntity ||
		!slices.Contains(x.errorCodes(), "/window_end:INVALID") {
		t.Fatalf("window: %d %s", x.status, x.raw)
	}
	if x := c.do(t, http.MethodPost, examSessionsPath, qlcmA, idem(), c.sessionBody(k, map[string]any{"feedback_timing": "immediate"})); x.status !=
		http.StatusUnprocessableEntity || !slices.Contains(x.errorCodes(), "/feedback_timing:INVALID") {
		t.Fatalf("immediate: %d %s", x.status, x.raw)
	}
	outsider := map[string]any{"accommodations": []any{map[string]any{"student_id": c.users[hsDung].UserID, "extra_time_percent": 10}}}
	if x := c.do(t, http.MethodPost, examSessionsPath, qlcmA, idem(), c.sessionBody(k, outsider)); x.status != http.StatusUnprocessableEntity ||
		!slices.Contains(x.errorCodes(), "/accommodations/0/student_id:INVALID") {
		t.Fatalf("accommodation outside: %d %s", x.status, x.raw)
	}
	big := c.examSetupOn(ctx, t, k, 4, []any{k.loRule(2, 2)}, map[string]any{"strategy": "independent_draw", "count": 20})
	if x := c.do(t, http.MethodPost, examSessionsPath, qlcmA, idem(), c.sessionBody(big, nil)); x.status != http.StatusUnprocessableEntity ||
		!slices.Contains(x.errorCodes(), "/definition/sections/0/rules/0:INVALID") {
		t.Fatalf("pool × 20 variants: %d %s", x.status, x.raw)
	}
}
