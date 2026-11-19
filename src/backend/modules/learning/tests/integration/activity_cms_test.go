//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// quizConfig is the config of a quiz over fixed question versions.
func quizConfig(qvs ...string) map[string]any {
	qs := make([]any, len(qvs))
	for i, q := range qvs {
		qs[i] = map[string]any{"question_version_id": q}
	}
	return map[string]any{
		"content":         map[string]any{"question_source": map[string]any{"kind": "questions", "questions": qs}},
		"interaction":     map[string]any{"shuffle_items": false, "shuffle_options": false},
		"evaluation":      map[string]any{"strategies": []string{"auto"}, "max_score": 10, "pass_threshold": 0.5},
		"feedback":        map[string]any{"timing": "immediate"},
		"completion_rule": map[string]any{"mode": "submit"},
	}
}

// activityBody is an ActivityDefinitionInput of type typ (grade 4: no g1_3 audio rule) over the curriculum of qc.
func activityBody(qc qbCurriculum, typ string, config map[string]any) map[string]any {
	return map[string]any{"activity_type": typ, "title": map[string]string{"vi": "Luyện bảng nhân"}, "grade_levels": []int{4},
		"subject_id": qc.subject, "topic_id": qc.topic, "objective_ids": []string{qc.lo}, "difficulty": 2, "content_language": "vi",
		"description": map[string]string{"vi": "Ôn tập phép nhân"}, "instructions": map[string]string{"vi": "Chọn đáp án đúng"}, "config": config}
}

// createActivity creates an activity as sub and returns its id.
func (c *curEnv) createActivity(t *testing.T, sub string, body map[string]any) string {
	t.Helper()
	return must(t, c.do(t, http.MethodPost, "/cms/learning/activities", sub, idem(), body), http.StatusCreated, "create activity").str("activity_id")
}

// publishActivity has version 1 of an activity of qlcmA approved and published through the workflow and release endpoints.
func (c *curEnv) publishActivity(ctx context.Context, t *testing.T, id string, version int) {
	t.Helper()
	c.approve(ctx, t, qlcmA, "activity", id, version)
	must(t, c.publishV(ctx, t, qlcmA, "activity", id, version, nil, nil), http.StatusOK, "publish activity")
}

// F030-AC-20, AC-21, AC-22 (US4, quickstart 16–17): the registry refuses types without plugin, the config is checked
// against the schema of the type on draft save and again on publish, listActivityTypes lists the plugins.
func TestActivityRegistryAndConfig(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	authztest.Cover(t, "listActivityTypes", authztest.RoleDenied)
	q1 := c.published(ctx, t, qlcmA, qc, nil)

	r := must(t, c.do(t, http.MethodGet, "/cms/learning/activity-types", qlcmA, nil, nil), http.StatusOK, "types")
	types := r.items("data")
	if len(types) != 2 || types[0]["activity_type"] != "interactive_exercise" || types[1]["activity_type"] != "quiz" {
		t.Fatalf("types: %s", r.raw)
	}
	for _, ty := range types {
		if ty["min_client_version"] == "" || !strings.HasPrefix(ty["schema_url"].(string), "https://schemas.eduworld.vn/content/activities/") ||
			ty["status"] != "enabled" || ty["schema_version"] != float64(1) {
			t.Fatalf("type info: %v", ty)
		}
	}
	if r := c.do(t, http.MethodGet, "/cms/learning/activity-types", hsAn, nil, nil); r.status != http.StatusForbidden {
		t.Fatalf("student reads the registry: %d", r.status)
	}

	// AC-20: a type of the release without plugin, and a type outside the release (refused by the contract enum).
	if r := c.do(t, http.MethodPost, "/cms/learning/activities", qlcmA, idem(), activityBody(qc, "video_lesson", quizConfig(q1))); r.status != 422 ||
		r.str("code") != "ACTIVITY_TYPE_NOT_SUPPORTED" {
		t.Fatalf("video_lesson: %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodPost, "/cms/learning/activities", qlcmA, idem(), activityBody(qc, "speaking", quizConfig(q1))); r.status != 422 {
		t.Fatalf("speaking: %d %s", r.status, r.raw)
	}
	// AC-21 on draft save: missing evaluation, a question type outside the quiz, a quiz type list with a non-quiz type.
	bad := quizConfig(q1)
	delete(bad, "evaluation")
	r = c.do(t, http.MethodPost, "/cms/learning/activities", qlcmA, idem(), activityBody(qc, "quiz", bad))
	if r.status != 422 || r.str("code") != "ACTIVITY_CONFIG_INVALID" || !strings.Contains(strings.Join(r.errorCodes(), ","), "/config") {
		t.Fatalf("missing evaluation: %d %s", r.status, r.raw)
	}
	bad = quizConfig(q1)
	bad["content"].(map[string]any)["allowed_question_types"] = []string{"numeric"}
	if r := c.do(t, http.MethodPost, "/cms/learning/activities", qlcmA, idem(), activityBody(qc, "quiz", bad)); r.status != 422 ||
		r.str("code") != "ACTIVITY_CONFIG_INVALID" {
		t.Fatalf("numeric in a quiz: %d %s", r.status, r.raw)
	}
	bad = quizConfig(q1)
	bad["content"].(map[string]any)["allowed_question_types"] = []string{"true_false"}
	if r := c.do(t, http.MethodPost, "/cms/learning/activities", qlcmA, idem(), activityBody(qc, "quiz", bad)); r.status != 422 ||
		r.str("code") != "ACTIVITY_CONFIG_INVALID" || !strings.Contains(r.raw, "/config/content/question_source/questions/0") {
		t.Fatalf("single_choice question in a true_false quiz: %d %s", r.status, r.raw)
	}

	// A valid quiz: created as draft with the curriculum version of its objectives.
	id := c.createActivity(t, qlcmA, activityBody(qc, "quiz", quizConfig(q1)))
	got := must(t, c.do(t, http.MethodGet, "/cms/learning/activities/"+id, qlcmA, nil, nil), http.StatusOK, "get")
	if got.str("activity_type") != "quiz" || got.str("status") != "draft" || got.str("version") != "1" || got.str("difficulty") != "2" ||
		got.header.Get("ETag") == "" || got.body["current_published_version"] != nil {
		t.Fatalf("activity: %s", got.raw)
	}
	// AC-21 on publish: a draft whose stored config lost its feedback is refused at publish.
	// tenancyguard:ok TEST helper under learning_platform: corrupts the config of one draft
	if _, err := c.platform.ExecContext(ctx, `UPDATE content.activity_versions SET body = body #- '{config,feedback}' WHERE activity_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	c.approve(ctx, t, qlcmA, "activity", id, 1)
	if r := c.publishV(ctx, t, qlcmA, "activity", id, 1, nil, nil); r.status != 422 || r.str("code") != "ACTIVITY_CONFIG_INVALID" {
		t.Fatalf("publish with an invalid config: %d %s", r.status, r.raw)
	}

	// A valid activity goes through review and publish; content.published carries activity_type and min_client_version.
	id2 := c.createActivity(t, qlcmA, activityBody(qc, "quiz", quizConfig(q1)))
	c.publishActivity(ctx, t, id2, 1)
	got = must(t, c.do(t, http.MethodGet, "/cms/learning/activities/"+id2, qlcmA, nil, nil), http.StatusOK, "get published")
	if got.str("status") != "published" || got.str("current_published_version") != "1" {
		t.Fatalf("published activity: %s", got.raw)
	}
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'content.published'
		AND envelope->'payload'->>'item_id' = $1 AND envelope->'payload'->>'activity_type' = 'quiz'
		AND envelope->'payload'->>'min_client_version' = '0.1.0'`, id2); n != 1 {
		t.Fatalf("content.published of the activity: %d", n)
	}
	list := must(t, c.do(t, http.MethodGet, "/cms/learning/activities?activity_type=quiz&status=published", qlcmA, nil, nil), http.StatusOK, "list")
	if ds := list.items("data"); len(ds) != 1 || ds[0]["activity_id"] != id2 {
		t.Fatalf("list: %s", list.raw)
	}
	if list := must(t, c.do(t, http.MethodGet, "/cms/learning/activities?activity_type=interactive_exercise", qlcmA, nil, nil), http.StatusOK,
		"list ie"); len(list.items("data")) != 0 {
		t.Fatalf("list interactive: %s", list.raw)
	}
}

// F030-AC-24 (US4, quickstart 19): CMS activities are tenant-scoped, need learning_content.activity.write, If-Match and
// bounded strings.
func TestActivityCmsAuthzTenant(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	authztest.Cover(t, "listCmsActivities", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createCmsActivity", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getCmsActivity", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateCmsActivityDraft", authztest.CrossTenant, authztest.RoleDenied)
	q1 := c.published(ctx, t, qlcmA, qc, nil)
	body := activityBody(qc, "quiz", quizConfig(q1))
	// school_admin holds learning_content.write but not learning_content.activity.write.
	if r := c.do(t, http.MethodPost, "/cms/learning/activities", adA, idem(), body); r.status != http.StatusForbidden {
		t.Fatalf("school_admin creates an activity: %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodPost, "/cms/learning/activities", noRoleA, idem(), body); r.status != http.StatusForbidden {
		t.Fatalf("no role: %d", r.status)
	}
	long := activityBody(qc, "quiz", quizConfig(q1))
	long["title"] = map[string]string{"vi": strings.Repeat("a", 256)}
	if r := c.do(t, http.MethodPost, "/cms/learning/activities", qlcmA, idem(), long); r.status != 422 || r.str("code") != "VALIDATION_FAILED" {
		t.Fatalf("title > 255: %d %s", r.status, r.raw)
	}
	id := c.createActivity(t, qlcmA, body)
	path := "/cms/learning/activities/" + id
	// Tenant B sees nothing of tenant A (404 on read and write, empty list).
	if r := c.do(t, http.MethodGet, path, qlcmB, nil, nil); r.status != http.StatusNotFound {
		t.Fatalf("tenant B get: %d", r.status)
	}
	if r := c.do(t, http.MethodGet, "/cms/learning/activities", qlcmB, nil, nil); r.status != 200 || len(r.items("data")) != 0 {
		t.Fatalf("tenant B list: %d %s", r.status, r.raw)
	}
	tag := must(t, c.do(t, http.MethodGet, path, qlcmA, nil, nil), http.StatusOK, "get").header.Get("ETag")
	// updateCmsActivityDraft takes a whole ActivityDefinitionInput as application/json (the contract body).
	patch := activityBody(qc, "quiz", quizConfig(q1))
	patch["title"] = map[string]string{"vi": "Luyện bảng nhân 8"}
	js := hdr{"Content-Type": "application/json"}
	if r := c.do(t, http.MethodPatch, path, qlcmB, merge(js, ifm(tag)), patch); r.status != http.StatusNotFound {
		t.Fatalf("tenant B patch: %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodGet, "/cms/learning/activities", edA, nil, nil); r.status != 200 || len(r.items("data")) != 1 {
		t.Fatalf("tenant editor reads: %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodPatch, path, adA, merge(js, ifm(tag)), patch); r.status != http.StatusForbidden {
		t.Fatalf("school_admin patch: %d", r.status)
	}
	// If-Match is required (428), stale (412), then the draft changes; activity_type never changes.
	if r := c.do(t, http.MethodPatch, path, qlcmA, js, patch); r.status != http.StatusPreconditionRequired {
		t.Fatalf("no If-Match: %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodPatch, path, qlcmA, merge(js, ifm(`W/"99"`)), patch); r.status != http.StatusPreconditionFailed {
		t.Fatalf("stale If-Match: %d %s", r.status, r.raw)
	}
	r := must(t, c.do(t, http.MethodPatch, path, qlcmA, merge(js, ifm(tag)), patch), http.StatusOK, "patch")
	if r.str("title", "vi") != "Luyện bảng nhân 8" {
		t.Fatalf("patched: %s", r.raw)
	}
	ie := activityBody(qc, "interactive_exercise", quizConfig(q1))
	if r := c.do(t, http.MethodPatch, path, qlcmA, merge(js, ifm(r.header.Get("ETag"))), ie); r.status != 422 {
		t.Fatalf("type change: %d %s", r.status, r.raw)
	}
	// A published version is not editable.
	c.publishActivity(ctx, t, id, 1)
	tag = must(t, c.do(t, http.MethodGet, path, qlcmA, nil, nil), http.StatusOK, "get").header.Get("ETag")
	if r := c.do(t, http.MethodPatch, path, qlcmA, merge(js, ifm(tag)), patch); r.status != http.StatusConflict || r.str("code") != "CONTENT_NOT_EDITABLE" {
		t.Fatalf("published patch: %d %s", r.status, r.raw)
	}
	if n := c.audits(ctx, t, "content.version_created", &c.tenantA); n < 1 {
		t.Fatal("audit content.version_created")
	}
}
