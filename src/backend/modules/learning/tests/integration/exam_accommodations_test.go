//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// F055-AC-18 (delta D2, FR-EXM-009, EXAM_ENGINE §5): putExamAccommodations replaces the personal extra times of an open
// session — If-Match required (428) and checked (412), reason and students of the audience validated (422), the
// effective duration of each participation recomputed (a removed student goes back to the session duration), audit
// with the reason, Idempotency-Key replay; another school / tenant 404, a student or a role without
// lms.assignment.write 403.
func TestExamAccommodations(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.examSetup(ctx, t, 5, nil)
	authztest.Cover(t, "putExamAccommodations", authztest.CrossTenant, authztest.RoleDenied)
	created := must(t, c.do(t, http.MethodPost, examSessionsPath, qlcmA, idem(), c.sessionBody(k, map[string]any{"type": "chapter",
		"accommodations": []any{map[string]any{"student_id": c.users[hsAn].UserID, "extra_time_percent": 50}}})), http.StatusCreated, "create")
	id, etag := created.str("exam_session_id"), created.header.Get("ETag")
	path := examSessionsPath + "/" + id + "/accommodations"
	body := map[string]any{"reason": "Học sinh cần hỗ trợ đọc", "accommodations": []any{map[string]any{"student_id": c.users[hsBinh].UserID,
		"extra_time_percent": 25}}}
	if x := c.do(t, http.MethodPut, path, qlcmA, idem(), body); x.status != http.StatusPreconditionRequired {
		t.Fatalf("no If-Match: %d %s", x.status, x.raw)
	}
	if x := c.do(t, http.MethodPut, path, qlcmA, merge(idem(), ifm(`W/"9"`)), body); x.status != http.StatusPreconditionFailed {
		t.Fatalf("stale If-Match: %d %s", x.status, x.raw)
	}
	if x := c.do(t, http.MethodPut, path, qlcmA, merge(idem(), ifm(etag)), map[string]any{"reason": "x", "accommodations": []any{}}); x.status != http.StatusUnprocessableEntity {
		t.Fatalf("short reason: %d %s", x.status, x.raw)
	}
	outsider := map[string]any{"reason": "Hỗ trợ", "accommodations": []any{map[string]any{"student_id": ids.New().String(), "extra_time_percent": 10}}}
	if x := c.do(t, http.MethodPut, path, qlcmA, merge(idem(), ifm(etag)), outsider); x.status != http.StatusUnprocessableEntity ||
		x.str("code") != "VALIDATION_FAILED" {
		t.Fatalf("student outside the audience: %d %s", x.status, x.raw)
	}
	// AC-18 (F042-AC-05 formula): Bình 900 s + 25 %; An removed → back to 900 s; one audit with the reason; replay.
	key := merge(idem(), ifm(etag))
	r := must(t, c.do(t, http.MethodPut, path, qlcmA, key, body), http.StatusOK, "put")
	again := must(t, c.do(t, http.MethodPut, path, qlcmA, key, body), http.StatusOK, "replay")
	acc, _ := r.body["accommodations"].([]any)
	if len(acc) != 1 || acc[0].(map[string]any)["extra_time_percent"] != float64(25) || r.header.Get("ETag") == etag ||
		again.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("put: %s / %s", r.raw, again.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.exam_participations WHERE exam_session_id = $1
		AND ((student_id = $2 AND effective_duration_sec = 1125 AND extra_time_percent = 25) OR
		     (student_id = $3 AND effective_duration_sec = 900 AND extra_time_percent = 0))`, id, c.users[hsBinh].UserID, c.users[hsAn].UserID); n != 2 {
		t.Fatalf("participations: %d", n)
	}
	if n := c.audits(ctx, t, "exam_session.accommodations_updated", &c.tenantA); n != 1 {
		t.Fatalf("audits: %d", n)
	}
	if got := must(t, c.do(t, http.MethodGet, examSessionsPath+"/"+id, hsBinh, nil, nil), http.StatusOK, "student").num("effective_duration_seconds"); got != 1125 {
		t.Fatalf("student effective duration: %v", got)
	}
	// Scope: another school of the tenant and another tenant → 404; a student and a role without the permission → 403.
	fresh := merge(idem(), ifm(r.header.Get("ETag")))
	for _, sub := range []string{adA2, adB} {
		if x := c.do(t, http.MethodPut, path, sub, fresh, body); x.status != http.StatusNotFound {
			t.Fatalf("%s: %d %s", sub, x.status, x.raw)
		}
	}
	for _, sub := range []string{hsAn, noRoleA} {
		if x := c.do(t, http.MethodPut, path, sub, fresh, body); x.status != http.StatusForbidden {
			t.Fatalf("%s: %d %s", sub, x.status, x.raw)
		}
	}
}
