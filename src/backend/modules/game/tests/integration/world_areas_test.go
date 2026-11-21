//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	gamehttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

var allAreas = []string{"main_hub", "practice_zone", "social_zone", "learning_zone.math", "learning_zone.vietnamese", "learning_zone.english",
	"learning_zone.science", "learning_zone.history", "quest_zone.discovery_ocean", "exam_zone", "challenge_mountain", "arena"}

func areasOf(t *testing.T, r resp) map[string]map[string]any {
	t.Helper()
	if r.status != http.StatusOK {
		t.Fatalf("listWorldAreas: %d %s", r.status, r.raw)
	}
	out := map[string]map[string]any{}
	data, _ := r.body["data"].([]any)
	for _, d := range data {
		m := d.(map[string]any)
		out[m["area_id"].(string)] = m
	}
	return out
}

func (e *env) listAreas(t *testing.T, s student) map[string]map[string]any {
	t.Helper()
	return areasOf(t, e.do(t, http.MethodGet, "/game/world/areas", e.gtoken(s), th(s), nil))
}

// F071-AC-01, AC-02 (T012, TC-WLD-001, TC-GAM-009, SC-001): the 12 areas of the 8 regions with stable ids and names vi/en
// covering the functional zones A–G; at level 1 with no coins the learning areas of the grade are open for every grade
// 1–9; the adventure areas stay locked with their conditions.
func TestListWorldAreas(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active") // grade 3
	areas := e.listAreas(t, an)
	if len(areas) != 12 {
		t.Fatalf("areas: %d", len(areas))
	}
	regions := map[string]bool{}
	classes := map[string]bool{}
	for _, id := range allAreas {
		a, ok := areas[id]
		if !ok {
			t.Fatalf("missing area %s", id)
		}
		name, _ := a["name"].(map[string]any)
		if name["vi"] == "" || name["en"] == "" || name["vi"] == nil || name["en"] == nil {
			t.Fatalf("%s: name vi/en: %v", id, a["name"])
		}
		regions[a["region_code"].(string)] = true
		classes[a["area_class"].(string)] = true
	}
	if len(regions) != 8 {
		t.Fatalf("regions: %v", regions)
	}
	for _, c := range []string{"hub", "learning", "practice", "exam", "arena", "social", "quest"} { // zones A–G
		if !classes[c] {
			t.Fatalf("zone class %s missing: %v", c, classes)
		}
	}
	if areas["learning_zone.math"]["accessible"] != true {
		t.Fatal("learning_zone.math must be open")
	}
	ocean := areas["quest_zone.discovery_ocean"]
	reqs, _ := ocean["unlock_requirements"].([]any)
	if ocean["accessible"] != false || ocean["lock_reason"] != "requirements_not_met" || len(reqs) != 1 ||
		reqs[0].(map[string]any)["kind"] != "chapters_completed" {
		t.Fatalf("discovery ocean: %v", ocean)
	}
	if h := areas["learning_zone.history"]; h["accessible"] != false || h["lock_reason"] != "grade_not_allowed" {
		t.Fatalf("history at grade 3: %v", h)
	}
	// SC-001: every learning area of the grade is open at level 1, 0 coins, for grades 1–9.
	for g := 1; g <= 9; g++ {
		e.setGrade(ctx, t, an, g)
		areas := e.listAreas(t, an)
		for _, id := range []string{"learning_zone.math", "learning_zone.vietnamese", "learning_zone.english", "learning_zone.science"} {
			if areas[id]["accessible"] != true {
				t.Fatalf("grade %d: %s closed", g, id)
			}
		}
		if g >= 4 && areas["learning_zone.history"]["accessible"] != true {
			t.Fatalf("grade %d: history closed", g)
		}
	}
}

// F071-AC-03 (T013): a subject the school turned off is hidden from listWorldAreas, getWorldManifest and
// getWorldExploration (its chapter too); another school of the tenant still sees it.
func TestSubjectDisabledHidden(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	other := e.newStudent(ctx, t, e.tenantA, e.schoolA2, "active")
	e.setGrade(ctx, t, an, 5)
	e.setGrade(ctx, t, other, 5)
	e.enableOnly(ctx, t, e.schoolA, "math", "vietnamese", "english", "science")
	if _, ok := e.listAreas(t, an)["learning_zone.history"]; ok {
		t.Fatal("history listed")
	}
	m := e.do(t, http.MethodGet, "/game/world/manifest", e.gtoken(an), th(an), nil)
	if m.status != http.StatusOK || strings.Contains(m.raw, `"area_id":"learning_zone.history"`) || strings.Contains(m.raw, "portal.hub.to_history") {
		t.Fatalf("manifest: %d %s", m.status, m.raw)
	}
	x := e.do(t, http.MethodGet, "/game/world/exploration", e.gtoken(an), th(an), nil)
	if x.status != http.StatusOK || strings.Contains(x.raw, "learning_zone.history") || strings.Contains(x.raw, "chapter.4.history_kingdom") {
		t.Fatalf("exploration: %d %s", x.status, x.raw)
	}
	d, session := e.session(t, an)
	if r := e.enter(t, an, d, session, "learning_zone.history", ""); r.status != http.StatusNotFound {
		t.Fatalf("enter a disabled subject: %d %s", r.status, r.raw)
	}
	if _, ok := e.listAreas(t, other)["learning_zone.history"]; !ok {
		t.Fatal("the other school still has history")
	}
}

// F071-AC-04 (T014): the world manifest gives each area its scene and the bundle of its level (url, sha256, size_bytes);
// renaming an area through data shows on the next read without a client build.
func TestWorldManifest(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	e.rebuildManifests(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	r := e.do(t, http.MethodGet, "/game/world/manifest", e.gtoken(an), th(an), nil)
	areas, _ := r.body["areas"].([]any)
	bundles, _ := r.body["bundles"].([]any)
	if r.status != http.StatusOK || r.body["manifest_version"] != "1" || len(areas) != 12 || len(bundles) != 12 {
		t.Fatalf("manifest: %d %s", r.status, r.raw)
	}
	b := bundles[0].(map[string]any)
	if !strings.HasPrefix(b["url"].(string), "https://") || len(b["sha256"].(string)) != 64 || b["size_bytes"] == nil {
		t.Fatalf("bundle: %v", b)
	}
	if _, err := e.publish(ctx, t, []command.ContentItem{seedItem(t, write.ItemWorldArea, "area_id", "learning_zone.math", func(m map[string]any) {
		m["name"] = map[string]any{"vi": "Vương quốc Số Mới", "en": "New Math Kingdom"}
	})}, "rename"); err != nil {
		t.Fatal(err)
	}
	if a := e.listAreas(t, an)["learning_zone.math"]; a["name"].(map[string]any)["vi"] != "Vương quốc Số Mới" || a["version"] != float64(2) {
		t.Fatalf("renamed area: %v", a)
	}
}

// F071-AC-06 (T015, TC-AUTH-014, TC-TEN-001): every F071 player route × 5 staff roles → 403 GAME_ACCESS_STUDENT_ONLY
// (TestPlayerRoutesStaffDenied iterates gamehttp.PlayerOperations, which holds them); a student of tenant A with the
// header of tenant B → 403 TENANT_MISMATCH; unlocks, exploration and level sessions of tenant B are never read in A.
func TestWorldTenantIsolation(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	for _, id := range gamehttp.WorldOperations {
		found := false
		for _, p := range gamehttp.PlayerOperations {
			found = found || p == id
		}
		if !found || gamehttp.Operations[id].Guard == 0 {
			t.Fatalf("%s is not a guarded player operation", id)
		}
	}
	authztest.Cover(t, "getWorldManifest", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "listWorldAreas", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "enterWorldArea", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "startLevel", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "endLevelSession", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "getNpcDialogue", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "getWorldExploration", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "reportDiscovery", authztest.RoleDenied, authztest.CrossTenant)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	dung := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	// Tenant B: arena unlocked, a discovery found, a level session.
	e.progress(ctx, t, "quest.completed", e.tenantB, map[string]any{"student_id": dung.id, "quest_id": questCh0Last})
	d, session := e.session(t, dung)
	if r := e.enter(t, dung, d, session, "main_hub", ""); r.status != http.StatusOK {
		t.Fatalf("enter: %d %s", r.status, r.raw)
	}
	if r := e.discover(t, dung, d, "main_hub", "discovery.hub.spark_tree", newKey()); r.status != http.StatusOK {
		t.Fatalf("discover: %d %s", r.status, r.raw)
	}
	h := th(dung)
	h["Idempotency-Key"] = newKey()
	if r := e.do(t, http.MethodPost, "/game/levels/"+levelMainHub+":start", d.token, h, map[string]any{"game_session_id": session, "manifest_version": 0}); r.status != http.StatusCreated {
		t.Fatalf("start level: %d %s", r.status, r.raw)
	}
	// The header of another tenant is refused.
	wrong := map[string]string{"X-Tenant-Id": e.tenantB.String()}
	if r := e.do(t, http.MethodGet, "/game/world/areas", e.gtoken(an), wrong, nil); r.status != http.StatusForbidden || r.code() != string(problem.TenantMismatch) {
		t.Fatalf("tenant mismatch: %d %s", r.status, r.raw)
	}
	// Tenant A sees none of B's progress (TenantScope + RLS).
	if a := e.listAreas(t, an)["arena"]; a["accessible"] != false {
		t.Fatalf("arena of tenant A: %v", a)
	}
	x := e.do(t, http.MethodGet, "/game/world/exploration", e.gtoken(an), th(an), nil)
	if strings.Contains(x.raw, `"discoveries_found":1`) {
		t.Fatalf("exploration of tenant A: %s", x.raw)
	}
	for _, table := range []string{"world_area_unlocks", "exploration_progress", "level_sessions", "world_unlock_facts", "story_progress"} {
		var n int
		err := e.uow.InTenantTx(ctx, scopeOf(t, e.tenantA), func(ctx context.Context) error {
			return txOf(ctx).QueryRowContext(ctx, `SELECT count(*) FROM game.`+table).Scan(&n)
		})
		if err != nil || n != 0 {
			t.Fatalf("%s visible in tenant A: %d %v", table, n, err)
		}
	}
}
