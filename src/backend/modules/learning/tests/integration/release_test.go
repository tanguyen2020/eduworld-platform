//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/snapshot"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

const tenantReviewer = "ad.donvi.a"

// approved creates a question of author (grade 4) and has it approved; it returns its id.
func (c *curEnv) approved(ctx context.Context, t *testing.T, author string, qc qbCurriculum, extra map[string]any) string {
	t.Helper()
	if extra == nil {
		extra = map[string]any{}
	}
	extra["grade_levels"] = []int{4}
	if author == cmNoidung {
		extra["owner_scope"] = "platform"
	}
	id, _ := c.question(t, author, qc, extra)
	c.approve(ctx, t, author, "question", id, 1)
	return id
}

// approve submits (author) and approves (another reviewer) a version.
func (c *curEnv) approve(ctx context.Context, t *testing.T, author, ctype, id string, version int) {
	t.Helper()
	must(t, c.act(ctx, t, author, ctype, id, version, "submit_review", ""), http.StatusOK, "submit")
	reviewer := cmDuyet
	if d := c.users[author]; d.TenantID != nil {
		reviewer = tenantReviewer
		if _, ok := c.users[reviewer]; !ok {
			c.user(reviewer, authztest.Staff(*d.TenantID, "tenant_admin").PrincipalData, true)
		}
	}
	must(t, c.act(ctx, t, reviewer, ctype, id, version, "approve", ""), http.StatusOK, "approve")
}

func (c *curEnv) publishV(ctx context.Context, t *testing.T, sub, ctype, id string, version int, body map[string]any, h hdr) resp {
	t.Helper()
	hs := merge(hdr{"If-Match": c.rowTag(ctx, t, ctype, id, version)}, idem(), h)
	path := fmt.Sprintf("/cms/learning/content/%s/%s/versions/%d:publish", ctype, id, version)
	if body == nil {
		return c.do(t, http.MethodPost, path, sub, hs, nil)
	}
	return c.do(t, http.MethodPost, path, sub, hs, body)
}

func (c *curEnv) pointer(ctx context.Context, t *testing.T, table, col, id string) string {
	t.Helper()
	var v *string
	// tenancyguard:ok TEST helper under learning_platform: reads the release pointer of one item
	if err := c.platform.QueryRowContext(ctx, `SELECT current_version_id::text FROM content.`+table+` WHERE `+col+` = $1`, id).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v == nil {
		return ""
	}
	return *v
}

func (c *curEnv) versionID(ctx context.Context, t *testing.T, id string, no int) string {
	t.Helper()
	var v string
	if err := c.platform.QueryRowContext(ctx, `SELECT question_version_id::text FROM content.question_versions WHERE question_id = $1 AND version_no = $2`,
		id, no).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

// F026-AC-01…AC-07, AC-12 (US1, quickstart): publish of an approved version in one transaction, replay, stale tag,
// immutability, new version based on the release, previous release kept, tenant isolation and authorization.
func TestReleasePublish(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	authztest.Cover(t, "publishContentVersion", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "rollbackContent", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "unpublishContent", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listContentVersions", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getContentVersion", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "diffContentVersions", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listContentReleases", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getContentUsages", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "cancelScheduledPublication", authztest.CrossTenant, authztest.RoleDenied)
	id := c.approved(ctx, t, qlcmA, qc, nil)
	v1 := c.versionID(ctx, t, id, 1)
	key := idem()
	tag := c.rowTag(ctx, t, "question", id, 1)
	path := "/cms/learning/content/question/" + id + "/versions/1:publish"
	// AC-07 first: another tenant, no role, an editor without publish.
	if r := c.do(t, http.MethodPost, path, qlcmB, merge(idem(), ifm(tag)), nil); r.status != http.StatusNotFound {
		t.Fatalf("tenant B: %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodPost, path, edA, merge(idem(), ifm(tag)), nil); r.status != http.StatusForbidden {
		t.Fatalf("editor: %d %s", r.status, r.raw)
	}
	r := must(t, c.do(t, http.MethodPost, path, qlcmA, merge(key, ifm(tag)), nil), http.StatusOK, "publish")
	if r.str("action") != "publish" || r.str("to_version") != "1" || r.str("release_id") == "" {
		t.Fatalf("publish: %s", r.raw)
	}
	again := must(t, c.do(t, http.MethodPost, path, qlcmA, merge(key, ifm(tag)), nil), http.StatusOK, "replay")
	if again.header.Get("Idempotent-Replayed") != "true" || again.str("release_id") != r.str("release_id") {
		t.Fatalf("replay: %s", again.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.content_releases WHERE item_id = $1 AND action = 'publish'`, id); n != 1 {
		t.Fatalf("%d releases", n)
	}
	if c.pointer(ctx, t, "questions", "question_id", id) != v1 {
		t.Fatal("pointer")
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.question_versions WHERE question_version_id = $1 AND status = 'published'
		AND published_at IS NOT NULL AND published_by IS NOT NULL AND snapshot_sha256 IS NOT NULL AND snapshot_key LIKE 'ew:learning:cv:question:%'`, v1); n != 1 {
		t.Fatal("published version row")
	}
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'content.published'
		AND envelope->'payload'->>'item_id' = $1 AND envelope->'payload'->'tenant_ids'->>0 = $2`, id, c.tenantA.String()); n != 1 {
		t.Fatal("content.published with tenant_ids")
	}
	if c.audits(ctx, t, "content.published", &c.tenantA) != 1 {
		t.Fatal("audit content.published")
	}
	// AC-03: a stale tag on the next version is 409 CONTENT_VERSION_CONFLICT.
	// AC-04: the published version is immutable (use case and trigger).
	if r := c.do(t, http.MethodPatch, "/cms/learning/questions/"+id+"/versions/1", qlcmA, ifm(c.rowTag(ctx, t, "question", id, 1)),
		map[string]any{"difficulty": 3}); r.status != http.StatusConflict || r.str("code") != "CONTENT_NOT_EDITABLE" {
		t.Fatalf("edit published: %d %s", r.status, r.raw)
	}
	// tenancyguard:ok TEST: the immutability trigger refuses a direct update of a published version
	if _, err := c.platform.ExecContext(ctx, `UPDATE content.question_versions SET difficulty = 5 WHERE question_version_id = $1`, v1); err == nil ||
		!strings.Contains(err.Error(), "CONTENT_NOT_EDITABLE") {
		t.Fatalf("trigger: %v", err)
	}
	// AC-02: publish of a draft.
	// AC-05: a new version is based on the release; a second open draft is refused.
	r = must(t, c.do(t, http.MethodPost, "/cms/learning/questions/"+id+"/versions", qlcmA, idem(), map[string]any{"from_version": 1}),
		http.StatusCreated, "new version")
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.question_versions WHERE question_id = $1 AND version_no = 2 AND based_on_version_id = $2`,
		id, v1); n != 1 {
		t.Fatal("based_on")
	}
	if r := c.do(t, http.MethodPost, "/cms/learning/questions/"+id+"/versions", qlcmA, idem(), map[string]any{"from_version": 1}); r.status != http.StatusConflict {
		t.Fatalf("second draft: %d", r.status)
	}
	if r := c.publishV(ctx, t, qlcmA, "question", id, 2, nil, nil); r.status != http.StatusUnprocessableEntity || r.str("code") != "INVALID_STATE_TRANSITION" {
		t.Fatalf("publish draft: %d %s", r.status, r.raw)
	}
	// AC-06: publishing v2 keeps v1 published.
	c.approve(ctx, t, qlcmA, "question", id, 2)
	if r := c.publishV(ctx, t, qlcmA, "question", id, 2, nil, ifm(`W/"99"`)); r.status != http.StatusConflict || r.str("code") != "CONTENT_VERSION_CONFLICT" {
		t.Fatalf("stale tag: %d %s", r.status, r.raw)
	}
	must(t, c.publishV(ctx, t, qlcmA, "question", id, 2, nil, nil), http.StatusOK, "publish v2")
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.question_versions WHERE question_id = $1 AND status = 'published'`, id); n != 2 {
		t.Fatal("v1 not kept published")
	}
	if c.pointer(ctx, t, "questions", "question_id", id) != c.versionID(ctx, t, id, 2) {
		t.Fatal("pointer v2")
	}
	// AC-07: assigned platform content is read-only for a tenant.
	pid := c.approved(ctx, t, cmNoidung, qc, nil)
	c.assignResource(ctx, t, "question", pid, c.tenantA)
	if r := c.publishV(ctx, t, qlcmA, "question", pid, 1, nil, nil); r.status != http.StatusForbidden {
		t.Fatalf("assigned platform: %d %s", r.status, r.raw)
	}
	must(t, c.publishV(ctx, t, cmNoidung, "question", pid, 1, nil, nil), http.StatusOK, "platform publish")
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'content.published'
		AND envelope->'payload'->>'item_id' = $1 AND envelope->'payload'->'tenant_ids' ? $2`, pid, c.tenantA.String()); n != 1 {
		t.Fatal("platform release without the assigned tenant")
	}
	// AC-02: a question set referencing an unpublished question version.
	qd, qdv := c.question(t, qlcmA, qc, map[string]any{"grade_levels": []int{4}})
	_ = qd
	set := must(t, c.do(t, http.MethodPost, "/cms/learning/question-sets", qlcmA, idem(), setBody(qc, staticDef(qdv))), http.StatusCreated, "set")
	sid := set.str("content_id")
	c.approve(ctx, t, qlcmA, "question_set", sid, 1)
	if r := c.publishV(ctx, t, qlcmA, "question_set", sid, 1, nil, nil); r.status != http.StatusConflict || r.str("code") != "CONTENT_NOT_PUBLISHED" ||
		!strings.Contains(r.raw, qdv) {
		t.Fatalf("children: %d %s", r.status, r.raw)
	}
}

// readerCtx is a read session context in the scope of tenant (nil = platform).
func (c *curEnv) inScope(ctx context.Context, t *testing.T, tenant *uuid.UUID, fn func(ctx context.Context) error) {
	t.Helper()
	var (
		sc  tenancy.Scope
		err error
	)
	if tenant != nil {
		sc, err = authz.EventScope(*tenant)
	} else {
		sc, err = authz.SystemPlatformScope("test")
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := c.read.Read(tenancy.WithScope(ctx, sc), fn); err != nil {
		t.Fatal(err)
	}
}

// F026-AC-08…AC-11, AC-15 (US2, US3; SC-003, SC-004): rollback moves the pointer only; invalid targets; unpublish;
// reasons; the pinned version reads the same through every release; new units follow the pointer.
func TestReleaseRollbackUnpublish(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	id := c.approved(ctx, t, cmNoidung, qc, nil)
	must(t, c.publishV(ctx, t, cmNoidung, "question", id, 1, nil, nil), http.StatusOK, "v1")
	must(t, c.do(t, http.MethodPost, "/cms/learning/questions/"+id+"/versions", cmNoidung, idem(), map[string]any{"from_version": 1}), http.StatusCreated, "v2")
	c.approve(ctx, t, cmNoidung, "question", id, 2)
	must(t, c.publishV(ctx, t, cmNoidung, "question", id, 2, nil, nil), http.StatusOK, "v2 publish")
	v1, v2 := c.versionID(ctx, t, id, 1), c.versionID(ctx, t, id, 2)
	reader := c.reader()
	var pinned []byte
	c.inScope(ctx, t, nil, func(ctx context.Context) error {
		var err error
		pinned, err = reader.Snapshot(ctx, contentmodel.TypeQuestion, uuid.MustParse(id), 1)
		return err
	})
	if len(snapshot.Leaks(pinned)) != 0 {
		t.Fatalf("snapshot leaks %v", snapshot.Leaks(pinned))
	}
	path := "/cms/learning/content/question/" + id
	// AC-11: permission and reason.
	if r := c.do(t, http.MethodPost, path+":rollback", cmNoidung, idem(), map[string]any{"to_version": 1, "reason": "lỗi"}); r.status != http.StatusForbidden {
		t.Fatalf("content admin rollback: %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodPost, path+":rollback", saAdmin, idem(), map[string]any{"to_version": 1}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("no reason: %d", r.status)
	}
	if r := c.do(t, http.MethodPost, path+":rollback", saAdmin, idem(), map[string]any{"to_version": 1, "reason": strings.Repeat("a", 501)}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("long reason: %d", r.status)
	}
	// AC-09: invalid targets.
	for _, to := range []int{2, 9} {
		if r := c.do(t, http.MethodPost, path+":rollback", saAdmin, idem(), map[string]any{"to_version": to, "reason": "x"}); r.str("code") != "ROLLBACK_TARGET_INVALID" {
			t.Fatalf("target %d: %s", to, r.raw)
		}
	}
	versions := count(ctx, t, c.platform, `SELECT count(*) FROM content.question_versions WHERE question_id = $1`, id)
	// AC-08.
	r := must(t, c.do(t, http.MethodPost, path+":rollback", saAdmin, idem(), map[string]any{"to_version": 1, "reason": "Đề v2 sai"}), http.StatusOK, "rollback")
	if r.str("action") != "rollback" || r.str("to_version") != "1" || r.str("from_version") != "2" {
		t.Fatalf("rollback: %s", r.raw)
	}
	if c.pointer(ctx, t, "questions", "question_id", id) != v1 || count(ctx, t, c.platform, `SELECT count(*) FROM content.question_versions WHERE question_id = $1`, id) != versions {
		t.Fatal("rollback created a version or did not move the pointer")
	}
	if count(ctx, t, c.platform, `SELECT count(*) FROM content.content_releases WHERE item_id = $1 AND action = 'rollback' AND reason = 'Đề v2 sai'`, id) != 1 ||
		count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'content.rolled_back' AND envelope->'payload'->>'item_id' = $1`, id) != 1 ||
		c.audits(ctx, t, "content.rolled_back", nil) != 1 {
		t.Fatal("rollback release / event / audit")
	}
	// The trigger refuses a pointer to a version that is not published.
	// tenancyguard:ok TEST: release-target trigger
	if _, err := c.platform.ExecContext(ctx, `UPDATE content.questions SET current_version_id = $2 WHERE question_id = $1`, id, uuid.New()); err == nil {
		t.Fatal("trigger accepted a foreign target")
	}
	// AC-10: unpublish.
	r = must(t, c.do(t, http.MethodPost, path+":unpublish", cmNoidung, idem(), map[string]any{"reason": "Gỡ để sửa"}), http.StatusOK, "unpublish")
	if r.str("action") != "unpublish" || c.pointer(ctx, t, "questions", "question_id", id) != "" {
		t.Fatalf("unpublish: %s", r.raw)
	}
	if r := c.do(t, http.MethodPost, path+":unpublish", cmNoidung, idem(), map[string]any{"reason": "lần hai"}); r.str("code") != "CONTENT_NOT_PUBLISHED" {
		t.Fatalf("unpublish twice: %s", r.raw)
	}
	// AC-15: the pinned v1 reads the same; a new unit gets CONTENT_NOT_PUBLISHED; after a new publish it gets v2.
	c.inScope(ctx, t, nil, func(ctx context.Context) error {
		b, err := reader.Snapshot(ctx, contentmodel.TypeQuestion, uuid.MustParse(id), 1)
		if err != nil || string(b) != string(pinned) {
			t.Fatalf("pinned snapshot changed: %v", err)
		}
		if _, _, err := reader.Current(ctx, contentmodel.TypeQuestion, uuid.MustParse(id)); code(err) != "CONTENT_NOT_PUBLISHED" {
			t.Fatalf("current after unpublish: %v", err)
		}
		return nil
	})
	must(t, c.do(t, http.MethodPost, path+":rollback", saAdmin, idem(), map[string]any{"to_version": 2, "reason": "sửa xong"}), http.StatusOK, "roll forward")
	c.inScope(ctx, t, nil, func(ctx context.Context) error {
		vid, no, err := reader.Current(ctx, contentmodel.TypeQuestion, uuid.MustParse(id))
		if err != nil || vid.String() != v2 || no != 2 {
			t.Fatalf("current %v %d %v", vid, no, err)
		}
		return nil
	})
	// SC-003: every rollback has a reason.
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.content_releases WHERE action IN ('rollback','unpublish') AND reason IS NULL`); n != 0 {
		t.Fatal("release without reason")
	}
}

// F026-AC-17…AC-19 (US4): schedule ≥ min lead (202), cancel before it runs, the job applies due schedules once and
// marks a schedule failed when the guard refuses it.
func TestReleaseSchedule(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	id := c.approved(ctx, t, qlcmA, qc, nil)
	now := time.Now().UTC()
	if r := c.publishV(ctx, t, qlcmA, "question", id, 1, map[string]any{"publish_at": now.Add(time.Minute)}, nil); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("too soon: %d %s", r.status, r.raw)
	}
	r := must(t, c.publishV(ctx, t, qlcmA, "question", id, 1, map[string]any{"publish_at": now.Add(10 * time.Minute)}, nil), http.StatusAccepted, "schedule")
	sid := r.str("release_id")
	if r.str("status") != "scheduled" || c.audits(ctx, t, "content.publish_scheduled", &c.tenantA) != 1 ||
		count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'content.publish_scheduled'`) != 1 {
		t.Fatalf("schedule: %s", r.raw)
	}
	cancel := "/cms/learning/content/question/" + id + "/releases/" + sid + ":cancel"
	if r := c.do(t, http.MethodPost, cancel, qlcmB, idem(), nil); r.status != http.StatusNotFound {
		t.Fatalf("cancel by tenant B: %d", r.status)
	}
	must(t, c.do(t, http.MethodPost, cancel, qlcmA, idem(), nil), http.StatusOK, "cancel")
	if r := c.do(t, http.MethodPost, cancel, qlcmA, idem(), nil); r.str("code") != "INVALID_STATE_TRANSITION" {
		t.Fatalf("cancel twice: %s", r.raw)
	}
	must(t, c.publishV(ctx, t, qlcmA, "question", id, 1, map[string]any{"publish_at": now.Add(10 * time.Minute)}, nil), http.StatusAccepted, "schedule again")
	// A second item scheduled then revised back to draft: the job marks it failed.
	id2 := c.approved(ctx, t, qlcmA, qc, nil)
	must(t, c.publishV(ctx, t, qlcmA, "question", id2, 1, map[string]any{"publish_at": now.Add(10 * time.Minute)}, nil), http.StatusAccepted, "schedule 2")
	must(t, c.act(ctx, t, qlcmA, "question", id2, 1, "revise", ""), http.StatusOK, "revise")
	rel := c.releases()
	rel.Clock = func() time.Time { return now.Add(11 * time.Minute) }
	n, err := rel.RunDueSchedules(ctx, 10)
	if err != nil || n != 1 {
		t.Fatalf("job: %d %v", n, err)
	}
	if n, err := rel.RunDueSchedules(ctx, 10); err != nil || n != 0 {
		t.Fatalf("second worker applied %d (%v)", n, err)
	}
	if c.pointer(ctx, t, "questions", "question_id", id) != c.versionID(ctx, t, id, 1) || c.pointer(ctx, t, "questions", "question_id", id2) != "" {
		t.Fatal("pointers after the job")
	}
	if count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'content.published' AND envelope->'payload'->>'scheduled' = 'true'`) != 1 ||
		count(ctx, t, c.platform, `SELECT count(*) FROM content.scheduled_publications WHERE item_id = $1 AND status = 'failed'
			AND failure_code = 'INVALID_STATE_TRANSITION'`, id2) != 1 ||
		count(ctx, t, c.platform, `SELECT count(*) FROM content.content_releases WHERE item_id = $1 AND scheduled_for IS NOT NULL`, id) != 1 {
		t.Fatal("applied / failed schedules")
	}
}

// F026-AC-20…AC-22, AC-27, AC-28 (US5, US7): history with the current release, read-only versions (answers only with
// review), diff, releases newest first, usages, archive.
func TestReleaseHistory(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	id := c.approved(ctx, t, qlcmA, qc, nil)
	must(t, c.publishV(ctx, t, qlcmA, "question", id, 1, nil, nil), http.StatusOK, "v1")
	must(t, c.do(t, http.MethodPost, "/cms/learning/questions/"+id+"/versions", qlcmA, idem(), map[string]any{"from_version": 1}), http.StatusCreated, "v2")
	must(t, c.do(t, http.MethodPatch, "/cms/learning/questions/"+id+"/versions/2", qlcmA, ifm(c.rowTag(ctx, t, "question", id, 2)),
		map[string]any{"difficulty": 4}), http.StatusOK, "edit v2")
	c.approve(ctx, t, qlcmA, "question", id, 2)
	must(t, c.publishV(ctx, t, qlcmA, "question", id, 2, nil, nil), http.StatusOK, "v2")
	base := "/cms/learning/content/question/" + id
	h := must(t, c.do(t, http.MethodGet, base+"/versions", qlcmA, nil, nil), http.StatusOK, "history")
	data := h.items("data")
	if h.str("current_version") != "2" || len(data) != 2 || data[0]["version"] != float64(2) || data[0]["is_current"] != true || data[1]["is_current"] != false {
		t.Fatalf("history: %s", h.raw)
	}
	if r := c.do(t, http.MethodGet, base+"/versions", qlcmB, nil, nil); r.status != http.StatusNotFound {
		t.Fatalf("history tenant B: %d", r.status)
	}
	// AC-28: archive the current release is refused; the old one is archived.
	if r := c.act(ctx, t, qlcmA, "question", id, 2, "archive", ""); r.str("code") != "INVALID_STATE_TRANSITION" {
		t.Fatalf("archive current: %s", r.raw)
	}
	r := must(t, c.act(ctx, t, qlcmA, "question", id, 1, "archive", "thay bằng v2"), http.StatusOK, "archive v1")
	if r.str("status") != "archived" || count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'content.version_archived'`) != 1 ||
		c.audits(ctx, t, "content.archived", &c.tenantA) != 1 {
		t.Fatalf("archive: %s", r.raw)
	}
	// AC-20: archived version read-only, answers only with review.
	g := must(t, c.do(t, http.MethodGet, base+"/versions/1", qlcmA, nil, nil), http.StatusOK, "get v1")
	if g.str("status") != "archived" || !strings.Contains(g.raw, "answer_key") || g.header.Get("ETag") == "" {
		t.Fatalf("get v1: %s", g.raw)
	}
	c.user("doc.a", authztest.Staff(c.tenantA, "tenant_content_editor").PrincipalData, true)
	g = must(t, c.do(t, http.MethodGet, base+"/versions/1", "doc.a", nil, nil), http.StatusOK, "get v1 editor")
	if strings.Contains(g.raw, "answer_key") || strings.Contains(g.raw, "scoring_rule") {
		t.Fatalf("answers without review: %s", g.raw)
	}
	// AC-21.
	d := must(t, c.do(t, http.MethodGet, base+"/versions/2:diff?with=1", qlcmA, nil, nil), http.StatusOK, "diff")
	if d.str("from_version") != "1" || d.str("to_version") != "2" || len(d.items("changes")) == 0 {
		t.Fatalf("diff: %s", d.raw)
	}
	// AC-22.
	rl := must(t, c.do(t, http.MethodGet, base+"/releases", qlcmA, nil, nil), http.StatusOK, "releases")
	rs := rl.items("data")
	if len(rs) != 2 || rs[0]["to_version"] != float64(2) || rs[0]["from_version"] != float64(1) || rs[1]["to_version"] != float64(1) {
		t.Fatalf("releases: %s", rl.raw)
	}
	// AC-27: a question set holding v2.
	set := must(t, c.do(t, http.MethodPost, "/cms/learning/question-sets", qlcmA, idem(), setBody(qc, staticDef(c.versionID(ctx, t, id, 2)))), http.StatusCreated, "set")
	u := must(t, c.do(t, http.MethodGet, base+"/usages", qlcmA, nil, nil), http.StatusOK, "usages")
	us := u.items("data")
	if len(us) != 1 || us[0]["ref_type"] != "question_set" || us[0]["ref_id"] != set.str("content_id") || us[0]["used_version"] != float64(2) {
		t.Fatalf("usages: %s", u.raw)
	}
	// API_DESIGN §5: usages are cursor paginated — a second set, limit 1 → two pages.
	set2 := must(t, c.do(t, http.MethodPost, "/cms/learning/question-sets", qlcmA, idem(), setBody(qc, staticDef(c.versionID(ctx, t, id, 2)))), http.StatusCreated, "set 2")
	p1 := must(t, c.do(t, http.MethodGet, base+"/usages?limit=1", qlcmA, nil, nil), http.StatusOK, "usages page 1")
	if len(p1.items("data")) != 1 || p1.obj("page")["has_more"] != true || p1.str("page", "next_cursor") == "" {
		t.Fatalf("usages page 1: %s", p1.raw)
	}
	p2 := must(t, c.do(t, http.MethodGet, base+"/usages?limit=1&cursor="+url.QueryEscape(p1.str("page", "next_cursor")), qlcmA, nil, nil), http.StatusOK, "usages page 2")
	if us2 := p2.items("data"); len(us2) != 1 || p2.obj("page")["has_more"] != false || us2[0]["ref_id"] == p1.items("data")[0]["ref_id"] {
		t.Fatalf("usages page 2: %s (set2 %s)", p2.raw, set2.str("content_id"))
	}
	if r := c.do(t, http.MethodGet, base+"/usages", qlcmB, nil, nil); r.status != http.StatusNotFound {
		t.Fatalf("usages tenant B: %d", r.status)
	}
}

// F026-AC-12…AC-14, AC-16 (US3, quickstart): snapshots without answers; platform content on the public CDN, tenant and
// exam content never; Valkey cache with PostgreSQL fallback; the pointer cache consumer per target tenant.
func TestReleaseSnapshotsCache(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t, withMedia)
	qc := c.qbSetup(ctx, t)
	pid := c.approved(ctx, t, cmNoidung, qc, nil)
	c.assignResource(ctx, t, "question", pid, c.tenantA)
	exam := c.approved(ctx, t, cmNoidung, qc, map[string]any{"exam_only": true})
	tid := c.approved(ctx, t, qlcmA, qc, nil)
	for _, x := range []struct{ sub, id string }{{cmNoidung, pid}, {cmNoidung, exam}, {qlcmA, tid}} {
		must(t, c.publishV(ctx, t, x.sub, "question", x.id, 1, nil, nil), http.StatusOK, "publish "+x.id)
	}
	objects := func(id string) int {
		return count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'content.published' AND envelope->'payload'->>'item_id' = $1
			AND envelope->'payload'->>'cdn_eligible' = 'true' AND envelope->'payload'->>'snapshot_url' IS NOT NULL`, id)
	}
	if objects(pid) != 1 || objects(exam) != 0 || objects(tid) != 0 {
		t.Fatal("cdn eligibility")
	}
	var sha string
	if err := c.platform.QueryRowContext(ctx, `SELECT snapshot_sha256 FROM content.question_versions WHERE question_id = $1`, pid).Scan(&sha); err != nil {
		t.Fatal(err)
	}
	b := c.object(ctx, t, "ew-media-public", "content/question/"+pid+"/1/"+sha+".json")
	if len(snapshot.Leaks(b)) != 0 || !strings.Contains(string(b), "render_payload") {
		t.Fatalf("cdn snapshot: %s", b)
	}
	// AC-14: the cache holds the snapshot; a dead cache falls back to PostgreSQL with the same bytes.
	reader := c.reader()
	broken := c.reader()
	broken.Cache = deadCache{}
	c.inScope(ctx, t, &c.tenantA, func(ctx context.Context) error {
		a, err := reader.Snapshot(ctx, contentmodel.TypeQuestion, uuid.MustParse(tid), 1)
		if err != nil {
			return err
		}
		b, err := broken.Snapshot(ctx, contentmodel.TypeQuestion, uuid.MustParse(tid), 1)
		if err != nil || string(a) != string(b) || len(snapshot.Leaks(a)) != 0 {
			t.Fatalf("fallback: %v", err)
		}
		return nil
	})
	if _, err := c.media.cache.Get(ctx, command.SnapshotKey(contentmodel.TypeQuestion, uuid.MustParse(tid), 1)); err != nil {
		t.Fatalf("snapshot not cached: %v", err)
	}
	// AC-16: the consumer sets the pointer of every target tenant (idempotent).
	c.runPointerConsumer(ctx, t)
	c.runPointerConsumer(ctx, t)
	for _, x := range []struct {
		tenant *uuid.UUID
		id     string
	}{{&c.tenantA, pid}, {&c.tenantA, tid}} {
		v, err := c.media.cache.Current(ctx, x.tenant, "question", uuid.MustParse(x.id))
		if err != nil || v.String() != c.versionID(ctx, t, x.id, 1) {
			t.Fatalf("pointer %s: %v %v", x.id, v, err)
		}
	}
	if _, err := c.media.cache.Current(ctx, nil, "question", uuid.MustParse(exam)); err != nil {
		t.Fatalf("_platform pointer of the unassigned exam question: %v", err)
	}
	if _, err := c.media.cache.Current(ctx, &c.tenantB, "question", uuid.MustParse(pid)); err == nil {
		t.Fatal("pointer for a tenant without assignment")
	}
}

// deadCache is a TEST DOUBLE of port.SnapshotCache (R17): Valkey down, every call fails.
type deadCache struct{}

func (deadCache) Get(context.Context, string) ([]byte, error) { return nil, errors.New("valkey down") }
func (deadCache) Put(context.Context, string, []byte) error   { return errors.New("valkey down") }
func (deadCache) SetCurrent(context.Context, *uuid.UUID, string, uuid.UUID, *uuid.UUID) error {
	return errors.New("valkey down")
}
func (deadCache) Current(context.Context, *uuid.UUID, string, uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, errors.New("valkey down")
}

// reader is the ContentReader of the environment (Valkey cache with withMedia).
func (c *curEnv) reader() query.ContentReader {
	r := query.ContentReader{Questions: read.QuestionModel{}, Docs: read.DocModel{}}
	if c.media != nil {
		r.Cache = c.media.cache
	}
	return r
}

// releases is the release command of the environment (job tests change its clock).
func (c *curEnv) releases() command.Releases { return c.rel }

// runPointerConsumer feeds every release event of the outbox to learning.release_pointer_cache (as the relay would).
func (c *curEnv) runPointerConsumer(ctx context.Context, t *testing.T) {
	t.Helper()
	rows, err := c.relay.QueryContext(ctx, `SELECT envelope FROM infra.outbox_events
		WHERE event_type IN ('content.published', 'content.rolled_back', 'content.unpublished') ORDER BY created_at`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	h := consumer.ReleasePointerCache{Cache: c.media.cache}
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			t.Fatal(err)
		}
		var e outbox.Envelope
		if err := json.Unmarshal(b, &e); err != nil {
			t.Fatal(err)
		}
		if err := h.Handle(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
}
