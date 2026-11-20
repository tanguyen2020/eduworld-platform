//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// F030 T054 (OQ owner 2026-11-20): createCmsActivityVersion opens version n+1 draft from a version of the activity
// (same pattern as createQuestionVersion): the published version is unchanged (R11), Idempotency-Key replays the same
// draft, an open draft → 409, learning_content.activity.write and the tenant scope are required.
func TestActivityNewVersion(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	authztest.Cover(t, "createCmsActivityVersion", authztest.CrossTenant, authztest.RoleDenied)
	q1 := c.published(ctx, t, qlcmA, qc, nil)
	// The replay of createCmsActivity is ActivityDefinition too (the command stores the body of the endpoint).
	ck := idem()
	created := must(t, c.do(t, http.MethodPost, "/cms/learning/activities", qlcmA, ck, activityBody(qc, "quiz", quizConfig(q1))), http.StatusCreated, "create")
	replayed := must(t, c.do(t, http.MethodPost, "/cms/learning/activities", qlcmA, ck, activityBody(qc, "quiz", quizConfig(q1))), http.StatusCreated,
		"create replay")
	if replayed.header.Get("Idempotent-Replayed") != "true" || replayed.str("activity_id") != created.str("activity_id") ||
		replayed.str("activity_version_id") != created.str("activity_version_id") {
		t.Fatalf("create replay: %s", replayed.raw)
	}
	id := created.str("activity_id")
	path := "/cms/learning/activities/" + id + "/versions"
	from1 := map[string]any{"from_version": 1, "note": "sửa lỗi chính tả"}
	// Version 1 is still the open draft.
	if r := c.do(t, http.MethodPost, path, qlcmA, idem(), from1); r.status != http.StatusConflict {
		t.Fatalf("open draft: %d %s", r.status, r.raw)
	}
	c.publishActivity(ctx, t, id, 1)
	if r := c.do(t, http.MethodPost, path, qlcmA, nil, from1); r.status != http.StatusPreconditionRequired || r.str("code") != "IDEMPOTENCY_KEY_REQUIRED" {
		t.Fatalf("no key: %d %s", r.status, r.raw)
	}
	// Authorization: school_admin lacks learning_content.activity.write, a staff without role too; tenant B sees nothing.
	for _, sub := range []string{adA, noRoleA} {
		if r := c.do(t, http.MethodPost, path, sub, idem(), from1); r.status != http.StatusForbidden {
			t.Fatalf("%s: %d %s", sub, r.status, r.raw)
		}
	}
	if r := c.do(t, http.MethodPost, path, qlcmB, idem(), from1); r.status != http.StatusNotFound {
		t.Fatalf("tenant B: %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodPost, path, qlcmA, idem(), map[string]any{"from_version": 7}); r.status != http.StatusNotFound {
		t.Fatalf("unknown source version: %d %s", r.status, r.raw)
	}
	key := idem()
	v2 := must(t, c.do(t, http.MethodPost, path, qlcmA, key, from1), http.StatusCreated, "new version")
	if v2.str("status") != "draft" || v2.body["version"] != float64(2) || v2.body["current_published_version"] != float64(1) ||
		v2.str("activity_type") != "quiz" || v2.header.Get("ETag") == "" {
		t.Fatalf("version 2: %s", v2.raw)
	}
	again := must(t, c.do(t, http.MethodPost, path, qlcmA, key, from1), http.StatusCreated, "replay")
	if again.header.Get("Idempotent-Replayed") != "true" || again.str("activity_version_id") != v2.str("activity_version_id") {
		t.Fatalf("replay: %s", again.raw)
	}
	if r := c.do(t, http.MethodPost, path, qlcmA, idem(), from1); r.status != http.StatusConflict {
		t.Fatalf("second draft: %d %s", r.status, r.raw)
	}
	// The published version is unchanged; the draft is version 2 based on version 1 and is the one getCmsActivity edits.
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.activity_versions WHERE activity_id = $1 AND version_no = 1 AND status = 'published'`,
		id); n != 1 {
		t.Fatal("version 1 is no longer published")
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.activity_versions v2 JOIN content.activity_versions v1
		ON v2.based_on_version_id = v1.activity_version_id WHERE v2.activity_id = $1 AND v2.version_no = 2 AND v1.version_no = 1`, id); n != 1 {
		t.Fatal("version 2 is not based on version 1")
	}
	g := must(t, c.do(t, http.MethodGet, "/cms/learning/activities/"+id, qlcmA, nil, nil), http.StatusOK, "get")
	if g.body["version"] != float64(2) || g.str("status") != "draft" {
		t.Fatalf("get after new version: %s", g.raw)
	}
}
