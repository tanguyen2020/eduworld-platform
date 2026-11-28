//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
)

// adminActors are the console actors of a test: a game designer (author), a content reviewer, a live-ops publisher.
type adminActors struct {
	designer, designerSub, reviewer, liveops string
}

func (e *env) actors() adminActors {
	sub := e.platformStaff("game_designer")
	return adminActors{designer: e.staffToken(sub), designerSub: sub, reviewer: e.platformToken("content_reviewer"),
		liveops: e.platformToken("liveops")}
}

// approved creates a quest draft and takes it to approved (designer authors, reviewer approves).
func (e *env) approvedQuest(t *testing.T, a adminActors, code string) (string, string) {
	t.Helper()
	c := e.adminCall(t, http.MethodPost, "/game-admin/quests", a.designer, "", map[string]any{"title": "Quest " + code, "payload": questDraft(t, code, nil)})
	if c.status != http.StatusCreated {
		t.Fatalf("create: %d %s", c.status, c.raw)
	}
	id := c.body["config_id"].(string)
	base := "/game-admin/quests/" + id + "/versions/1"
	s := e.adminCall(t, http.MethodPost, base+":transition", a.designer, c.header.Get("ETag"), map[string]any{"action": "submit_review"})
	if s.status != http.StatusOK {
		t.Fatalf("submit: %d %s", s.status, s.raw)
	}
	g := e.adminCall(t, http.MethodGet, base, a.reviewer, "", nil)
	ap := e.adminCall(t, http.MethodPost, base+":transition", a.reviewer, g.header.Get("ETag"), map[string]any{"action": "approve"})
	if ap.status != http.StatusOK || ap.body["status"] != "approved" {
		t.Fatalf("approve: %d %s", ap.status, ap.raw)
	}
	g = e.adminCall(t, http.MethodGet, base, a.liveops, "", nil)
	return id, g.header.Get("ETag")
}

// F078-AC-01, AC-02 (direct call with a tenant token), AC-03, SC-001.
func TestGameAdminAuthz(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	a := e.actors()
	payload := map[string]any{"payload": questDraft(t, "math.g3.authz", nil)}
	denied := map[string]string{
		"tenant admin":       e.staffToken(e.staff(e.tenantA, "tenant_admin")),
		"content admin":      e.platformToken("content_admin"),
		"liveops for quests": a.liveops,
		"game token":         e.gtoken(e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")),
		"platform game azp":  e.idp.Token(e.platformStaff("system_admin"), httpxtest.GameClient, nowWall()),
	}
	before := e.audits(ctx, t, "authz.denied")
	for name, tok := range denied {
		if r := e.adminCall(t, http.MethodPost, "/game-admin/quests", tok, "", payload); r.status != http.StatusForbidden {
			t.Errorf("%s create: %d %s", name, r.status, r.raw)
		}
		if r := e.adminCall(t, http.MethodGet, "/game-admin/quests", tok, "", nil); r.status != http.StatusForbidden && name != "liveops for quests" {
			t.Errorf("%s list: %d %s", name, r.status, r.raw)
		}
		if r := e.adminCall(t, http.MethodPost, "/game-media:presign-upload", tok, "", map[string]any{"kind": "image", "filename": "a.png",
			"mime_type": "image/png", "size_bytes": 10, "sha256": strings.Repeat("a", 64), "license": "CC0"}); r.status != http.StatusForbidden &&
			name != "liveops for quests" {
			t.Errorf("%s presign: %d %s", name, r.status, r.raw)
		}
	}
	if after := e.audits(ctx, t, "authz.denied"); after <= before {
		t.Fatalf("authz.denied not recorded (%d → %d)", before, after)
	}
	// Live-ops reads quests (game.content.publish) but cannot write them.
	if r := e.adminCall(t, http.MethodGet, "/game-admin/quests", a.liveops, "", nil); r.status != http.StatusOK {
		t.Fatalf("liveops list: %d %s", r.status, r.raw)
	}
	// AC-03: X-Tenant-Id is ignored, the content is platform (no tenant); a type of another module is 404.
	h := map[string]string{"Idempotency-Key": newKey(), "X-Tenant-Id": e.tenantA.String()}
	c := e.do(t, http.MethodPost, "/game-admin/quests", a.designer, h, payload)
	if c.status != http.StatusCreated || c.body["status"] != "draft" || c.body["version"] != float64(1) {
		t.Fatalf("create with X-Tenant-Id: %d %s", c.status, c.raw)
	}
	// bosses has a type module since F103 (spec Clarifications): no longer an unknown type.
	for _, typ := range []string{"gifs", "matchmaking-configs", "currencies"} {
		if r := e.adminCall(t, http.MethodGet, "/game-admin/"+typ, a.designer, "", nil); r.status != http.StatusNotFound {
			t.Errorf("%s: %d %s", typ, r.status, r.raw)
		}
	}
	authztest.Cover(t, "listGameConfigs", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createGameConfig", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "presignGameMediaUpload", authztest.RoleDenied)
	e.checkEventSchemas(ctx, t)
}

// F078-AC-04 … AC-10: drafts, If-Match, submission checks, review by someone else, reads, limits.
func TestGameAdminDraftWorkflow(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	a := e.actors()
	// AC-04: schema violation names the field; a valid draft is version 1, author = designer, ETag.
	bad := questDraft(t, "math.g3.flow", func(m map[string]any) { delete(m, "steps") })
	if r := e.adminCall(t, http.MethodPost, "/game-admin/quests", a.designer, "", map[string]any{"payload": bad}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("schema: %d %s", r.status, r.raw)
	}
	// AC-10: title over 200 characters, payload over 64 KiB.
	if r := e.adminCall(t, http.MethodPost, "/game-admin/quests", a.designer, "", map[string]any{"title": strings.Repeat("a", 201),
		"payload": questDraft(t, "math.g3.flow", nil)}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("title: %d %s", r.status, r.raw)
	}
	big := questDraft(t, "math.g3.flow", func(m map[string]any) { m["tags"] = []string{strings.Repeat("x", 70000)} })
	if r := e.adminCall(t, http.MethodPost, "/game-admin/quests", a.designer, "", map[string]any{"payload": big}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("payload size: %d %s", r.status, r.raw)
	}
	c := e.adminCall(t, http.MethodPost, "/game-admin/quests", a.designer, "", map[string]any{"title": "Chợ phiên", "payload": questDraft(t, "math.g3.flow", nil)})
	if c.status != http.StatusCreated || c.header.Get("ETag") == "" || c.body["status"] != "draft" || c.body["author_id"] == nil {
		t.Fatalf("create: %d %s", c.status, c.raw)
	}
	id := c.body["config_id"].(string)
	if c.body["payload"].(map[string]any)["quest_id"] != id {
		t.Fatalf("server key: %s", c.raw)
	}
	base := "/game-admin/quests/" + id + "/versions/1"
	// AC-05: If-Match required, stale ETag 412, a draft changes.
	edit := map[string]any{"title": "Chợ phiên 2", "payload": questDraft(t, "math.g3.flow", nil)}
	if r := e.adminCall(t, http.MethodPatch, base, a.designer, "", edit); r.status != http.StatusPreconditionRequired {
		t.Fatalf("no If-Match: %d %s", r.status, r.raw)
	}
	if r := e.adminCall(t, http.MethodPatch, base, a.designer, `"99"`, edit); r.status != http.StatusPreconditionFailed {
		t.Fatalf("stale: %d %s", r.status, r.raw)
	}
	u := e.adminCall(t, http.MethodPatch, base, a.designer, c.header.Get("ETag"), edit)
	if u.status != http.StatusOK || u.body["title"] != "Chợ phiên 2" || u.header.Get("ETag") == c.header.Get("ETag") {
		t.Fatalf("update: %d %s", u.status, u.raw)
	}
	etag := u.header.Get("ETag")
	// AC-07: the client manifest of the build must hold the areas/NPCs/interactables; the learning activity must be released.
	if _, err := e.migrator.ExecContext(ctx, `INSERT INTO game.client_manifests (client_manifest_id, created_at, updated_at, client_version, platform,
		manifest, published_at) VALUES ($1, now(), now(), '1.4.0', 'windows', $2::jsonb, now() - interval '1 hour')`, uuid.New(),
		`{"area_ids":["main_hub"],"npc_ids":["npc.mr_numbers"],"scene_refs":[],"interactable_ids":[]}`); err != nil {
		t.Fatal(err)
	}
	if r := e.adminCall(t, http.MethodPost, base+":transition", a.designer, etag, map[string]any{"action": "submit_review"}); r.status != http.StatusUnprocessableEntity ||
		r.code() != "ITEM_REF_INVALID" || !strings.Contains(r.raw, "learning_zone.math") {
		t.Fatalf("client manifest: %d %s", r.status, r.raw)
	}
	if _, err := e.migrator.ExecContext(ctx, `INSERT INTO game.client_manifests (client_manifest_id, created_at, updated_at, client_version, platform,
		manifest, published_at) VALUES ($1, now(), now(), '1.5.0', 'windows', $2::jsonb, now())`, uuid.New(),
		`{"area_ids":["main_hub","learning_zone.math"],"npc_ids":["npc.mr_numbers"],"scene_refs":[],"interactable_ids":["quest.math.apple_basket_1","quest.math.apple_basket_2"]}`); err != nil {
		t.Fatal(err)
	}
	e.released = map[string]bool{}
	if r := e.adminCall(t, http.MethodPost, base+":transition", a.designer, etag, map[string]any{"action": "submit_review"}); r.status != http.StatusUnprocessableEntity ||
		r.code() != "ITEM_REF_INVALID" || !strings.Contains(r.raw, testContent) {
		t.Fatalf("learning content not released: %d %s", r.status, r.raw)
	}
	e.released = nil
	s := e.adminCall(t, http.MethodPost, base+":transition", a.designer, etag, map[string]any{"action": "submit_review"})
	if s.status != http.StatusOK || s.body["status"] != "in_review" {
		t.Fatalf("submit: %d %s", s.status, s.raw)
	}
	if e.events(ctx, t, "content.version_submitted") != 1 {
		t.Fatal("content.version_submitted")
	}
	// AC-05: a version in review is not editable.
	if r := e.adminCall(t, http.MethodPatch, base, a.designer, s.header.Get("ETag"), edit); r.status != http.StatusConflict || r.code() != "CONTENT_NOT_EDITABLE" {
		t.Fatalf("edit in review: %d %s", r.status, r.raw)
	}
	g := e.adminCall(t, http.MethodGet, base, a.reviewer, "", nil)
	// AC-08: the author never reviews his own version (also holding game.*); a reject needs a note.
	if r := e.adminCall(t, http.MethodPost, base+":transition", a.designer, g.header.Get("ETag"), map[string]any{"action": "approve"}); r.status != http.StatusForbidden {
		t.Fatalf("self approve: %d %s", r.status, r.raw)
	}
	if r := e.adminCall(t, http.MethodPost, base+":transition", a.reviewer, g.header.Get("ETag"), map[string]any{"action": "reject"}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("reject without note: %d %s", r.status, r.raw)
	}
	rj := e.adminCall(t, http.MethodPost, base+":transition", a.reviewer, g.header.Get("ETag"), map[string]any{"action": "reject", "note": "Bước 3 thiếu gợi ý"})
	if rj.status != http.StatusOK || rj.body["status"] != "rejected" || rj.body["review_note"] != "Bước 3 thiếu gợi ý" {
		t.Fatalf("reject: %d %s", rj.status, rj.raw)
	}
	// A rejected version is reworked (back to draft), submitted again and approved by the reviewer.
	u2 := e.adminCall(t, http.MethodPatch, base, a.designer, rj.header.Get("ETag"), edit)
	if u2.status != http.StatusOK || u2.body["status"] != "draft" {
		t.Fatalf("rework: %d %s", u2.status, u2.raw)
	}
	s2 := e.adminCall(t, http.MethodPost, base+":transition", a.designer, u2.header.Get("ETag"), map[string]any{"action": "submit_review"})
	if r := e.adminCall(t, http.MethodPost, base+":transition", a.designer, s2.header.Get("ETag"), map[string]any{"action": "archive"}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("archive in review: %d %s", r.status, r.raw)
	}
	ap := e.adminCall(t, http.MethodPost, base+":transition", a.reviewer, s2.header.Get("ETag"), map[string]any{"action": "approve"})
	if ap.status != http.StatusOK || ap.body["status"] != "approved" || ap.body["reviewer_id"] == nil {
		t.Fatalf("approve: %d %s", ap.status, ap.raw)
	}
	if r := e.adminCall(t, http.MethodPatch, base, a.designer, ap.header.Get("ETag"), edit); r.status != http.StatusConflict {
		t.Fatalf("edit approved: %d %s", r.status, r.raw)
	}
	for _, n := range []string{"content.version_approved", "content.version_rejected"} {
		if e.events(ctx, t, n) != 1 {
			t.Fatalf("%s: %d", n, e.events(ctx, t, n))
		}
	}
	// AC-09: reads for the reviewer and live-ops; versions, diff by field path.
	l := e.adminCall(t, http.MethodGet, "/game-admin/quests?status=approved&limit=1", a.reviewer, "", nil)
	if l.status != http.StatusOK || len(l.body["data"].([]any)) != 1 {
		t.Fatalf("list: %d %s", l.status, l.raw)
	}
	v := e.adminCall(t, http.MethodGet, "/game-admin/quests/"+id+"/versions", a.liveops, "", nil)
	if v.status != http.StatusOK || len(v.body["data"].([]any)) != 1 {
		t.Fatalf("versions: %d %s", v.status, v.raw)
	}
	for _, n := range []string{"content.version_created", "content.submitted", "content.approved", "content.rejected", "game_config.changed"} {
		if e.audits(ctx, t, n) == 0 {
			t.Errorf("audit %s missing", n)
		}
	}
	authztest.Cover(t, "getGameConfigVersion", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateGameConfigDraft", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "transitionGameConfigVersion", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listGameConfigVersions", authztest.CrossTenant, authztest.RoleDenied)
	e.checkEventSchemas(ctx, t)
}

// F078-AC-06, AC-11 … AC-16, AC-21 … AC-23: publish now and at a time, new version from a published one, rollback,
// unpublish with usages, pinned plays, release timeline, audit; tenant assignments and the school catalog.
func TestGameAdminReleases(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	a := e.actors()
	id, etag := e.approvedQuest(t, a, "math.g3.release")
	base := "/game-admin/quests/" + id
	// AC-21: a resource not published cannot be assigned.
	fa := facade.GameAdmin{Assignments: e.assignUC(), Catalog: e.adminQueries()}
	sa := authztest.Ctx(ctx, authztest.Platform("system_admin"))
	assign := func(tenants ...uuid.UUID) (api.ApplyTenantAssignmentsResponseObject, error) {
		return fa.ApplyTenantAssignments(sa, api.ApplyTenantAssignmentsRequestObject{Body: &api.ApplyTenantAssignmentsJSONRequestBody{
			ResourceType: "quest", ResourceId: uuid.MustParse(id), TenantIds: tenants}})
	}
	if _, err := assign(e.tenantA); !codeIs(err, "CONTENT_NOT_PUBLISHED") {
		t.Fatalf("assign draft: %v", err)
	}
	// AC-11: only publish rights; If-Match required; publish now moves the pointer and emits content.published + catalog.
	if r := e.adminCall(t, http.MethodPost, base+"/versions/1:publish", a.reviewer, etag, map[string]any{}); r.status != http.StatusForbidden {
		t.Fatalf("reviewer publish: %d", r.status)
	}
	if r := e.adminCall(t, http.MethodPost, base+"/versions/1:publish", a.liveops, "", map[string]any{}); r.status != http.StatusPreconditionRequired {
		t.Fatalf("publish without If-Match: %d %s", r.status, r.raw)
	}
	p := e.adminCall(t, http.MethodPost, base+"/versions/1:publish", a.liveops, etag, map[string]any{"change_note": "Phát hành lần đầu"})
	if p.status != http.StatusOK || p.body["action"] != "publish" || p.body["status"] != "applied" || p.body["to_version"] != float64(1) {
		t.Fatalf("publish: %d %s", p.status, p.raw)
	}
	v1 := pointerOf(ctx, t, e, id)
	pub := e.payloads(ctx, t, "content.published")
	last := pub[len(pub)-1]
	cat, _ := last["catalog"].(map[string]any)
	if last["config_id"] != id || cat == nil || cat["title"].(map[string]any)["vi"] != "Bài toán phiên chợ" || len(last["content_refs"].([]any)) != 1 {
		t.Fatalf("content.published payload: %+v", last)
	}
	// AC-21/AC-22/AC-23: assign to tenant A; the manifest and the catalog of A hold the quest, B never.
	// F045 Clarifications 2026-11-28: created = true on the first apply, false when the tenant already has it.
	if res, err := assign(e.tenantA); err != nil || len(res.(api.ApplyTenantAssignments200JSONResponse).Data) != 1 ||
		!createdFlag(res.(api.ApplyTenantAssignments200JSONResponse).Data[0].Created, true) {
		t.Fatalf("assign: %v %+v", err, res)
	}
	if res, err := assign(e.tenantA); err != nil || len(res.(api.ApplyTenantAssignments200JSONResponse).Data) != 1 ||
		!createdFlag(res.(api.ApplyTenantAssignments200JSONResponse).Data[0].Created, false) ||
		count(ctx, t, e.platform, `SELECT count(*) FROM infra.tenant_resource_assignments WHERE resource_id = $1 AND deleted_at IS NULL`, id) != 1 {
		t.Fatalf("assign again: %v", err)
	}
	if e.events(ctx, t, "tenant_data.assigned") != 0 {
		t.Fatal("module game must not publish tenant_data.assigned")
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM game.content_manifests WHERE tenant_id = $1 AND entries::text LIKE '%'||$2||'%'`, e.tenantA, id) == 0 {
		t.Fatal("manifest of tenant A lacks the quest")
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM game.content_manifests WHERE tenant_id = $1 AND entries::text LIKE '%'||$2||'%'`, e.tenantB, id) != 0 {
		t.Fatal("manifest of tenant B holds the quest")
	}
	catalogOf := func(tenant uuid.UUID) []api.GameActivityCatalogEntry {
		res, err := fa.ListAssignedGameActivities(ctx, api.ListAssignedGameActivitiesRequestObject{Params: api.ListAssignedGameActivitiesParams{TenantId: tenant}})
		if err != nil {
			t.Fatal(err)
		}
		return res.(api.ListAssignedGameActivities200JSONResponse).Data
	}
	if c := catalogOf(e.tenantA); len(c) != 1 || c[0].ContentId.String() != id || c[0].Title.Vi != "Bài toán phiên chợ" || len(c[0].GradeLevels) != 1 {
		t.Fatalf("catalog A: %+v", c)
	}
	if c := catalogOf(e.tenantB); len(c) != 0 {
		t.Fatalf("catalog B: %+v", c)
	}
	// AC-15: a student of A accepts the quest on v1 (pinned).
	s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	if r := e.accept(t, s, e.gtoken(s), id, ""); r.status != http.StatusOK {
		t.Fatalf("accept: %d %s", r.status, r.raw)
	}
	// AC-06: a new draft copied from the published version; v1 stays published and unchanged.
	nv := e.adminCall(t, http.MethodPost, base+"/versions", a.designer, "", map[string]any{"based_on_version": 1, "change_note": "Thêm gợi ý"})
	if nv.status != http.StatusCreated || nv.body["version"] != float64(2) || nv.body["based_on_version"] != float64(1) || nv.body["status"] != "draft" {
		t.Fatalf("new version: %d %s", nv.status, nv.raw)
	}
	g1 := e.adminCall(t, http.MethodGet, base+"/versions/1", a.designer, "", nil)
	if g1.body["status"] != "published" || g1.body["is_current"] != true {
		t.Fatalf("v1: %s", g1.raw)
	}
	if r := e.adminCall(t, http.MethodPatch, base+"/versions/1", a.designer, g1.header.Get("ETag"), map[string]any{"payload": questDraft(t, "x", nil)}); r.status != http.StatusConflict {
		t.Fatalf("edit published: %d %s", r.status, r.raw)
	}
	edit := g1.body["payload"].(map[string]any)
	edit["estimated_minutes"] = 20
	u := e.adminCall(t, http.MethodPatch, base+"/versions/2", a.designer, nv.header.Get("ETag"), map[string]any{"payload": edit})
	sb := e.adminCall(t, http.MethodPost, base+"/versions/2:transition", a.designer, u.header.Get("ETag"), map[string]any{"action": "submit_review"})
	ap := e.adminCall(t, http.MethodPost, base+"/versions/2:transition", a.reviewer, sb.header.Get("ETag"), map[string]any{"action": "approve"})
	if ap.status != http.StatusOK {
		t.Fatalf("approve v2: %d %s", ap.status, ap.raw)
	}
	diff := e.adminCall(t, http.MethodGet, base+"/versions/2:diff?with=1", a.reviewer, "", nil)
	if diff.status != http.StatusOK || !strings.Contains(diff.raw, `"/estimated_minutes"`) || strings.Contains(diff.raw, `"/quest_version_id"`) {
		t.Fatalf("diff: %d %s", diff.status, diff.raw)
	}
	// AC-12: a time sooner than scheduled_publish_min_lead is 422; a valid one is 202 + content.publish_scheduled; cancel.
	soon := e.clock.Now().Add(2 * time.Minute)
	if r := e.adminCall(t, http.MethodPost, base+"/versions/2:publish", a.liveops, ap.header.Get("ETag"), map[string]any{"publish_at": soon}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("lead: %d %s", r.status, r.raw)
	}
	at := e.clock.Now().Add(10 * time.Minute)
	sch := e.adminCall(t, http.MethodPost, base+"/versions/2:publish", a.liveops, ap.header.Get("ETag"), map[string]any{"publish_at": at})
	if sch.status != http.StatusAccepted || sch.body["status"] != "scheduled" {
		t.Fatalf("schedule: %d %s", sch.status, sch.raw)
	}
	if r := e.adminCall(t, http.MethodPost, base+"/versions/2:publish", a.liveops, ap.header.Get("ETag"), map[string]any{"publish_at": at}); r.status != http.StatusConflict {
		t.Fatalf("second schedule: %d %s", r.status, r.raw)
	}
	cn := e.adminCall(t, http.MethodPost, base+"/releases/"+sch.body["release_id"].(string)+":cancel", a.liveops, "", map[string]any{"reason": "Dời lịch"})
	if cn.status != http.StatusOK || cn.body["status"] != "cancelled" {
		t.Fatalf("cancel: %d %s", cn.status, cn.raw)
	}
	sch2 := e.adminCall(t, http.MethodPost, base+"/versions/2:publish", a.liveops, ap.header.Get("ETag"), map[string]any{"publish_at": at})
	if sch2.status != http.StatusAccepted {
		t.Fatalf("schedule again: %d %s", sch2.status, sch2.raw)
	}
	if n, err := e.adminUC().ApplyDueSchedules(ctx); err != nil || n != 0 {
		t.Fatalf("not due yet: %d %v", n, err)
	}
	e.clock.Add(11 * time.Minute)
	if n, err := e.adminUC().ApplyDueSchedules(ctx); err != nil || n != 1 {
		t.Fatalf("due: %d %v", n, err)
	}
	if n, _ := e.adminUC().ApplyDueSchedules(ctx); n != 0 {
		t.Fatal("applied twice")
	}
	v2 := pointerOf(ctx, t, e, id)
	if v2 == v1 {
		t.Fatal("pointer did not move")
	}
	// AC-15: the pinned play keeps v1; the pointer is v2.
	var pinned string
	if err := e.platform.QueryRowContext(ctx, `SELECT quest_version_id::text FROM game.quest_progress WHERE student_id = $1`, s.id).Scan(&pinned); err != nil || pinned != v1 {
		t.Fatalf("pinned %s want %s: %v", pinned, v1, err)
	}
	// AC-13: rollback moves the pointer back without a version; the current version or a draft is not a target.
	versions := count(ctx, t, e.platform, `SELECT count(*) FROM game.config_versions WHERE config_id = $1`, id)
	if r := e.adminCall(t, http.MethodPost, base+":rollback", a.liveops, "", map[string]any{"to_version": 2, "reason": "x"}); r.status != http.StatusUnprocessableEntity ||
		r.code() != "ROLLBACK_TARGET_INVALID" {
		t.Fatalf("rollback to current: %d %s", r.status, r.raw)
	}
	rb := e.adminCall(t, http.MethodPost, base+":rollback", a.liveops, "", map[string]any{"to_version": 1, "reason": "Bước 2 sai"})
	if rb.status != http.StatusOK || rb.body["action"] != "rollback" || rb.body["from_version"] != float64(2) || rb.body["to_version"] != float64(1) {
		t.Fatalf("rollback: %d %s", rb.status, rb.raw)
	}
	if pointerOf(ctx, t, e, id) != v1 || count(ctx, t, e.platform, `SELECT count(*) FROM game.config_versions WHERE config_id = $1`, id) != versions {
		t.Fatal("rollback changed versions or pointer")
	}
	if e.events(ctx, t, "content.rolled_back") != 1 {
		t.Fatal("content.rolled_back")
	}
	// AC-14: unpublish empties the pointer and lists the usages (tenant A assignment).
	un := e.adminCall(t, http.MethodPost, base+":unpublish", a.liveops, "", map[string]any{"reason": "Gỡ để sửa"})
	if un.status != http.StatusOK || un.body["action"] != "unpublish" || !strings.Contains(un.raw, `"tenant_assignment"`) {
		t.Fatalf("unpublish: %d %s", un.status, un.raw)
	}
	if pointerOf(ctx, t, e, id) != "" || e.events(ctx, t, "content.unpublished") != 1 {
		t.Fatal("unpublish pointer/event")
	}
	if c := catalogOf(e.tenantA); len(c) != 0 {
		t.Fatalf("catalog after unpublish: %+v", c)
	}
	if r := e.adminCall(t, http.MethodPost, base+":unpublish", a.liveops, "", map[string]any{"reason": "again"}); r.status != http.StatusConflict {
		t.Fatalf("unpublish again: %d %s", r.status, r.raw)
	}
	// AC-08: archive a published version that is not the pointer.
	g2 := e.adminCall(t, http.MethodGet, base+"/versions/2", a.designer, "", nil)
	if r := e.adminCall(t, http.MethodPost, base+"/versions/2:transition", a.designer, g2.header.Get("ETag"), map[string]any{"action": "archive"}); r.status != http.StatusOK ||
		r.body["status"] != "archived" {
		t.Fatalf("archive: %d %s", r.status, r.raw)
	}
	// AC-16: the release timeline and the audit.
	rl := e.adminCall(t, http.MethodGet, base+"/releases?limit=2", a.reviewer, "", nil)
	if rl.status != http.StatusOK || len(rl.body["data"].([]any)) != 2 || rl.body["page"].(map[string]any)["has_more"] != true {
		t.Fatalf("releases page: %d %s", rl.status, rl.raw)
	}
	all := e.adminCall(t, http.MethodGet, base+"/releases?limit=20", a.reviewer, "", nil)
	if n := len(all.body["data"].([]any)); n != 5 || !strings.Contains(all.raw, `"cancelled"`) || !strings.Contains(all.raw, `"scheduled_for"`) {
		t.Fatalf("timeline (%d): %s", n, all.raw)
	}
	for _, n := range []string{"content.published", "content.publish_scheduled", "content.rolled_back", "content.unpublished", "content.archived",
		"content.publish_schedule_cancelled", "game_config.changed", "tenant_data.assigned"} {
		if e.audits(ctx, t, n) == 0 {
			t.Errorf("audit %s missing", n)
		}
	}
	// Revoke: soft delete, the manifest of A loses the quest.
	rv, err := fa.RevokeTenantAssignments(sa, api.RevokeTenantAssignmentsRequestObject{Body: &api.RevokeTenantAssignmentsJSONRequestBody{
		ResourceType: "quest", ResourceId: uuid.MustParse(id), TenantIds: []uuid.UUID{e.tenantA}, Reason: "Hết học kỳ"}})
	if err != nil || rv.(api.RevokeTenantAssignments200JSONResponse).Data[0].Status != "revoked" ||
		count(ctx, t, e.platform, `SELECT count(*) FROM infra.tenant_resource_assignments WHERE resource_id = $1 AND deleted_at IS NOT NULL`, id) != 1 {
		t.Fatalf("revoke: %v", err)
	}
	authztest.Cover(t, "publishGameConfigVersion", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "rollbackGameConfig", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "unpublishGameConfig", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listGameConfigReleases", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createGameConfigVersion", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "cancelScheduledGameConfigPublish", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "diffGameConfigVersions", authztest.CrossTenant, authztest.RoleDenied)
	e.checkEventSchemas(ctx, t)
}

// pointerOf is the current version id of a quest in its type table ("" = none).
func pointerOf(ctx context.Context, t *testing.T, e *env, id string) string {
	t.Helper()
	var cur *string
	if err := e.platform.QueryRowContext(ctx, `SELECT current_version_id::text FROM game.quest_definitions WHERE quest_id = $1`, id).Scan(&cur); err != nil {
		t.Fatal(err)
	}
	if cur == nil {
		return ""
	}
	return *cur
}

// F078-AC-17: the preview token reads its version only, for its creator, 30 minutes; nothing is played.
func TestGameAdminPreview(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	a := e.actors()
	c := e.adminCall(t, http.MethodPost, "/game-admin/quests", a.designer, "", map[string]any{"payload": questDraft(t, "math.g3.preview", nil)})
	id := c.body["config_id"].(string)
	pv := e.adminCall(t, http.MethodPost, "/game-admin/preview/sessions", a.designer, "", map[string]any{"config_type": "quests", "config_id": id, "version": 1})
	if pv.status != http.StatusCreated || pv.body["preview_token"] == nil {
		t.Fatalf("preview: %d %s", pv.status, pv.raw)
	}
	tok := pv.body["preview_token"].(string)
	if count(ctx, t, e.platform, `SELECT count(*) FROM game.preview_sessions WHERE token_hash = $1`, tok) != 0 {
		t.Fatal("raw token stored")
	}
	read := func(token, version string) resp {
		return e.do(t, http.MethodGet, "/game-admin/preview/snapshot?config_type=quests&config_id="+id+"&version="+version, token,
			map[string]string{"X-Preview-Token": tok}, nil)
	}
	if r := read(a.designer, "1"); r.status != http.StatusOK || r.body["payload"] == nil {
		t.Fatalf("snapshot: %d %s", r.status, r.raw)
	}
	if r := read(a.designer, "2"); r.status != http.StatusForbidden {
		t.Fatalf("other version: %d %s", r.status, r.raw)
	}
	if r := read(a.reviewer, "1"); r.status != http.StatusForbidden {
		t.Fatalf("other principal: %d %s", r.status, r.raw)
	}
	e.clock.Add(31 * time.Minute)
	if r := read(a.designer, "1"); r.status != http.StatusForbidden {
		t.Fatalf("expired: %d %s", r.status, r.raw)
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM game.game_sessions`) != 0 || count(ctx, t, e.platform, `SELECT count(*) FROM gamification.xp_ledger`) != 0 {
		t.Fatal("preview created a game session or XP")
	}
	authztest.Cover(t, "createGamePreviewSession", authztest.RoleDenied)
	authztest.Cover(t, "getGamePreviewSnapshot", authztest.RoleDenied)
}

// F078-AC-18 … AC-20: game media upload pipeline, MEDIA_NOT_READY, renders of a published 3D item, retries and DLQ.
func TestGameMediaAndRenders(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	a := e.actors()
	model := glb(2048)
	presign := func(kind, mime string, size int, s string) resp {
		return e.adminCall(t, http.MethodPost, "/game-media:presign-upload", a.designer, "", map[string]any{"kind": kind, "filename": "bear.glb",
			"mime_type": mime, "size_bytes": size, "sha256": s, "license": "CC0"})
	}
	if r := presign("model3d", "model/gltf-binary", 60<<20, sha(model)); r.status != http.StatusRequestEntityTooLarge {
		t.Fatalf("too large: %d %s", r.status, r.raw)
	}
	if r := presign("video", "video/mp4", 100, sha(model)); r.status != http.StatusUnsupportedMediaType {
		t.Fatalf("video: %d %s", r.status, r.raw)
	}
	if r := presign("image", "image/gif", 100, sha(model)); r.status != http.StatusUnsupportedMediaType {
		t.Fatalf("gif: %d %s", r.status, r.raw)
	}
	ok := presign("model3d", "model/gltf-binary", len(model), sha(model))
	if ok.status != http.StatusCreated || ok.body["status"] != "quarantined" || ok.body["upload_url"] == nil {
		t.Fatalf("presign: %d %s", ok.status, ok.raw)
	}
	asset := ok.body["asset_id"].(string)
	if r := presign("model3d", "model/gltf-binary", len(model), sha(model)); r.body["duplicate_of"] != asset {
		t.Fatalf("duplicate: %s", r.raw)
	}
	// A version referencing the asset is not submitted while it is not ready (MEDIA_NOT_READY).
	item := map[string]any{"schema_version": 1, "item_id": "bear_hat_01", "kind": "accessory", "rarity": "rare", "equip_slot": "head", "stackable": false,
		"max_stack": 1, "tradable": false, "name": map[string]any{"vi": "Mũ gấu", "en": "Bear hat"}, "sources": []string{"shop"},
		"asset_refs": map[string]any{"model": "asset:" + asset}}
	c := e.adminCall(t, http.MethodPost, "/game-admin/items", a.designer, "", map[string]any{"payload": item})
	if c.status != http.StatusCreated {
		t.Fatalf("item: %d %s", c.status, c.raw)
	}
	id := c.body["config_id"].(string)
	base := "/game-admin/items/" + id + "/versions/1"
	if r := e.adminCall(t, http.MethodPost, base+":transition", a.designer, c.header.Get("ETag"), map[string]any{"action": "submit_review"}); r.code() != "MEDIA_NOT_READY" {
		t.Fatalf("media not ready: %d %s", r.status, r.raw)
	}
	// The pipeline checks the upload and publishes it content-addressed; a bad file is rejected.
	m := e.mediaUC()
	e.storage.put("ew-media-quarantine", "quarantine/"+asset, model)
	other := glb(10)
	bad := presign("model3d", "model/gltf-binary", len(other), sha(other))
	e.storage.put("ew-media-quarantine", "quarantine/"+bad.body["asset_id"].(string), append(other, 'x'))
	if n, err := m.ProcessUploads(ctx); err != nil || n != 2 {
		t.Fatalf("pipeline: %d %v", n, err)
	}
	ga := e.adminCall(t, http.MethodGet, "/game-media/"+asset, a.reviewer, "", nil)
	if ga.status != http.StatusOK || ga.body["status"] != "ready" || len(ga.body["variants"].([]any)) != 1 {
		t.Fatalf("asset: %d %s", ga.status, ga.raw)
	}
	if !e.storage.has("ew-media-public", "media/"+sha(model)+"/original.glb") {
		t.Fatal("public object missing")
	}
	if gb := e.adminCall(t, http.MethodGet, "/game-media/"+bad.body["asset_id"].(string), a.reviewer, "", nil); gb.body["status"] != "rejected" {
		t.Fatalf("bad upload: %s", gb.raw)
	}
	if e.events(ctx, t, "media.processed") != 1 || e.events(ctx, t, "media.rejected") != 1 || e.events(ctx, t, "media.uploaded") != 2 {
		t.Fatal("media events")
	}
	// Submit, approve, publish: the item is rendered (thumbnail + turntable), once.
	g := e.adminCall(t, http.MethodGet, base, a.designer, "", nil)
	s := e.adminCall(t, http.MethodPost, base+":transition", a.designer, g.header.Get("ETag"), map[string]any{"action": "submit_review"})
	ap := e.adminCall(t, http.MethodPost, base+":transition", a.reviewer, s.header.Get("ETag"), map[string]any{"action": "approve"})
	pb := e.adminCall(t, http.MethodPost, base+":publish", a.liveops, ap.header.Get("ETag"), map[string]any{})
	if pb.status != http.StatusOK {
		t.Fatalf("publish item: %d %s", pb.status, pb.raw)
	}
	vid := uuid.MustParse(ap.body["config_version_id"].(string))
	sc, _ := authz.SystemPlatformScope("game-render-thumbnails")
	run := func() {
		if err := e.uow.InTenantTx(ctx, sc, func(ctx context.Context) error { return m.ScheduleRenders(ctx, "item", vid) }); err != nil {
			t.Fatal(err)
		}
	}
	run()
	run()
	jobs := e.renders.all()
	if len(jobs) != 2 || jobs[0].Key != "media/"+sha(model)+"/original.glb" {
		t.Fatalf("render jobs: %+v", jobs)
	}
	var thumb, turn port.RenderRequest
	for _, j := range jobs {
		if j.Kind == "render_thumbnail" {
			thumb = j
		} else {
			turn = j
		}
	}
	if thumb.Frames != 1 || turn.Frames != 24 {
		t.Fatalf("frames: %+v %+v", thumb, turn)
	}
	png := []byte("\x89PNG\r\n\x1a\nthumbnail")
	key := "media/" + sha(png) + "/render_thumbnail.png"
	if err := m.RecordRenderResult(ctx, port.RenderOutcome{RenderJobID: thumb.RenderJobID, Attempt: 1, Done: true, Bucket: "ew-media-public", Key: key,
		MIME: "image/png", SHA256: sha(png), SizeBytes: int64(len(png)), Width: 512, Height: 512, Frames: 1}); err != nil {
		t.Fatal(err)
	}
	// Replay of the same result: nothing new.
	if err := m.RecordRenderResult(ctx, port.RenderOutcome{RenderJobID: thumb.RenderJobID, Attempt: 1, Done: true, Key: key, SHA256: sha(png)}); err != nil {
		t.Fatal(err)
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM game.media_assets WHERE kind = 'render_thumbnail'`) != 1 {
		t.Fatal("render asset duplicated")
	}
	l := e.adminCall(t, http.MethodGet, "/game-admin/items", a.reviewer, "", nil)
	if !strings.Contains(l.raw, "https://cdn.test/ew-media-public/"+key) || strings.Contains(l.raw, "turntable_url") {
		t.Fatalf("thumbnail_url: %s", l.raw)
	}
	// AC-20: the turntable fails three times: retried with backoff, then dead-lettered; the publication is untouched.
	for i := 1; i <= 3; i++ {
		if err := m.RecordRenderResult(ctx, port.RenderOutcome{RenderJobID: turn.RenderJobID, Attempt: i, Error: fmt.Sprintf("godot crash %d", i)}); err != nil {
			t.Fatal(err)
		}
		if i < 3 {
			e.clock.Add(time.Duration(1<<(i-1)) * time.Minute)
			if n, err := m.RetryRenders(ctx); err != nil || n != 1 {
				t.Fatalf("retry %d: %d %v", i, n, err)
			}
		}
	}
	var st string
	if err := e.platform.QueryRowContext(ctx, `SELECT status FROM game.render_jobs WHERE render_job_id = $1`, turn.RenderJobID).Scan(&st); err != nil || st != "dead" {
		t.Fatalf("turntable status %s %v", st, err)
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM infra.dead_letters WHERE consumer = 'game.render_results' AND event_id = $1`, turn.RenderJobID) != 1 {
		t.Fatal("dead letter missing")
	}
	var cur *string
	_ = e.platform.QueryRowContext(ctx, `SELECT current_version_id::text FROM game.item_definitions WHERE item_id = 'bear_hat_01'`).Scan(&cur)
	if cur == nil || *cur != vid.String() {
		t.Fatal("publication changed by a failed render")
	}
	authztest.Cover(t, "getGameMediaAsset", authztest.RoleDenied, authztest.CrossTenant)
	e.checkEventSchemas(ctx, t)
}

// createdFlag reports whether the created flag of an applied assignment is present and equal to want.
func createdFlag(c *bool, want bool) bool { return c != nil && *c == want }
