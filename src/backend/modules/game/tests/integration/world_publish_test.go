//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F071-AC-05 (T009, TC-CNT-001/005): the release core publishes the world seed (12 areas, 12 levels, 7 chapters,
// 14 dialogues of 13 NPCs) with release rows, content.published and one audit record; the manifest builder rebuilds
// the manifest of every tenant; a new version of an area raises manifest_version and changes the ETag; an identical
// seed publishes nothing; published versions are immutable and a pointer only points at a published version.
func TestPublishGameContent(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	dung := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	res := e.publishWorld(ctx, t)
	if res.Published != 45 || res.Unchanged != 0 {
		t.Fatalf("seed v1: %+v", res)
	}
	if n := e.events(ctx, t, "content.published"); n != 45 {
		t.Fatalf("content.published: %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.content_releases WHERE action = 'publish'`); n != 45 {
		t.Fatalf("releases: %d", n)
	}
	if n := e.audits(ctx, t, "game_content.published"); n != 1 {
		t.Fatalf("audit: %d", n)
	}
	e.rebuildManifests(ctx, t)
	get := func(s student, inm string) resp {
		h := map[string]string{"X-Tenant-Id": s.tenant.String()}
		if inm != "" {
			h["If-None-Match"] = inm
		}
		return e.do(t, http.MethodGet, "/game/content-manifest", e.device(s).token, h, nil)
	}
	ra, rb := get(an, ""), get(dung, "")
	areas, _ := ra.body["areas"].([]any)
	if ra.status != http.StatusOK || ra.body["manifest_version"] != float64(1) || len(areas) != 12 || rb.body["manifest_version"] != float64(1) {
		t.Fatalf("manifests after seed: %d %s / %s", ra.status, ra.raw, rb.raw)
	}
	etag := ra.header.Get("ETag")
	// Identical seed: nothing new (seed can run again).
	if res, err := e.publish(ctx, t, worldSeed(t), "seed v1 again"); err != nil || res.Published != 0 || res.Unchanged != 45 {
		t.Fatalf("seed again: %+v %v", res, err)
	}
	// Rename main_hub through data: version 2, new manifest version and ETag for every tenant.
	renamed := seedItem(t, write.ItemWorldArea, "area_id", "main_hub", func(m map[string]any) {
		m["name"] = map[string]any{"vi": "Quảng trường Tia Sáng Mới", "en": "New Spark Square"}
	})
	if res, err := e.publish(ctx, t, []command.ContentItem{renamed}, "rename"); err != nil || res.Published != 1 || res.Releases[0].VersionNo != 2 {
		t.Fatalf("rename: %+v %v", res, err)
	}
	e.rebuildManifests(ctx, t)
	ra2, rb2 := get(an, etag), get(dung, "")
	if ra2.status != http.StatusOK || ra2.body["manifest_version"] != float64(2) || ra2.header.Get("ETag") == etag || rb2.body["manifest_version"] != float64(2) {
		t.Fatalf("manifest after rename: %d %v %s", ra2.status, ra2.body["manifest_version"], ra2.header.Get("ETag"))
	}
	if !strings.Contains(ra2.raw, `"item_code":"main_hub"`) {
		t.Fatalf("world_area entry: %s", ra2.raw)
	}
	// Published versions are immutable (R11) and a pointer only points at a published version.
	if _, err := e.platform.ExecContext(ctx, `UPDATE game.world_area_versions SET payload = '{}'::jsonb WHERE area_id = 'main_hub'`); err == nil ||
		!strings.Contains(err.Error(), "CONTENT_VERSION_IMMUTABLE") {
		t.Fatalf("immutable version: %v", err)
	}
	// F078: publishing keeps the older version published (rollback target); archive it explicitly first.
	if _, err := e.platform.ExecContext(ctx, `UPDATE game.world_area_versions SET status = 'archived'
		WHERE area_id = 'main_hub' AND version_no = 1`); err != nil {
		t.Fatalf("archive v1: %v", err)
	}
	if _, err := e.platform.ExecContext(ctx, `UPDATE game.world_areas SET current_version_id = (SELECT area_version_id FROM game.world_area_versions
		WHERE area_id = 'main_hub' AND status = 'archived') WHERE area_id = 'main_hub'`); err == nil || !strings.Contains(err.Error(), "CONTENT_POINTER_NOT_PUBLISHED") {
		t.Fatalf("pointer to an archived version: %v", err)
	}
	// The student role never writes platform content.
	if _, err := e.app.ExecContext(ctx, `INSERT INTO game.world_areas (area_id, created_at, updated_at) VALUES ('x', now(), now())`); err == nil {
		t.Fatal("game_app must not write platform content")
	}
	// Outside a platform scope the release core refuses.
	ctxA := ctxOf(t, e, an)
	if _, err := e.publisher().Publish(ctxA, []command.ContentItem{renamed}, "tenant", ""); codeOf(err) != problem.PermissionDenied {
		t.Fatalf("tenant scope publish: %v", err)
	}
	if got := e.checkEventSchemas(ctx, t); got["content.published"] != 46 {
		t.Fatalf("content.published checked: %v", got)
	}
}
