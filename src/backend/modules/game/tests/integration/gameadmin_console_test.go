//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/liveops"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// eventDraft creates an event draft (world festival) through the console, scheduled from start for days, decorating
// areas (nil = no decoration); it returns the config id.
func (e *env) eventDraft(t *testing.T, a adminActors, title string, start time.Time, days int, areas []string) string {
	t.Helper()
	it := e.festival(uuid.New(), func(m map[string]any) {
		m["schedule"] = map[string]any{"start_at": start.Format(time.RFC3339), "end_at": start.AddDate(0, 0, days).Format(time.RFC3339),
			"timezone": "Asia/Ho_Chi_Minh", "claim_grace_hours": 72}
		if areas != nil {
			m["components"].(map[string]any)["decoration"] = map[string]any{"areas": areas}
		}
	})
	var m map[string]any
	if err := json.Unmarshal(it.Payload, &m); err != nil {
		t.Fatal(err)
	}
	c := e.adminCall(t, http.MethodPost, "/game-admin/events", a.designer, "", map[string]any{"title": title, "payload": m})
	if c.status != http.StatusCreated {
		t.Fatalf("event %s: %d %s", title, c.status, c.raw)
	}
	return c.body["config_id"].(string)
}

// listed indexes the entries of a list response by config_id.
func listed(t *testing.T, r resp) map[string]map[string]any {
	t.Helper()
	if r.status != http.StatusOK {
		t.Fatalf("list: %d %s", r.status, r.raw)
	}
	out := map[string]map[string]any{}
	for _, d := range r.body["data"].([]any) {
		m := d.(map[string]any)
		out[m["config_id"].(string)] = m
	}
	return out
}

// warningsOf returns "other_config_id/area_id" of the calendar_warnings of an entry (nil when the field is absent).
func warningsOf(m map[string]any) []string {
	raw, ok := m["calendar_warnings"].([]any)
	if !ok {
		return nil
	}
	out := []string{}
	for _, w := range raw {
		x := w.(map[string]any)
		if x["code"] != "decoration_area_overlap" {
			return []string{"bad code"}
		}
		out = append(out, fmt.Sprintf("%s/%s", x["other_config_id"], x["area_id"]))
	}
	slices.Sort(out)
	return out
}

// F079-AC-14 (events calendar): listGameConfigs events with from/to keeps the events whose schedule intersects the ICT
// window (filtered in the query, keyset pages), each with the calendar_warnings of the other events of the window that
// decorate one of its areas at an overlapping time; from/to on another type, alone or over 62 days is 422.
func TestGameAdminEventCalendar(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	a := e.actors()
	y, mo, d := e.clock.Now().In(liveops.ICT).Date()
	day := func(n int) time.Time { return time.Date(y, mo, d+n, 10, 0, 0, 0, liveops.ICT) } // day n at 10:00 ICT
	date := func(n int) string { return day(n).Format(time.DateOnly) }
	evA := e.eventDraft(t, a, "Lễ hội A", day(10), 8, []string{"main_hub", "plaza"})
	evB := e.eventDraft(t, a, "Lễ hội B", day(13), 8, []string{"main_hub"})
	evC := e.eventDraft(t, a, "Lễ hội C", day(30), 8, []string{"main_hub"}) // outside the window
	evD := e.eventDraft(t, a, "Lễ hội D", day(12), 8, []string{"forest"})   // in the window, another area
	evE := e.eventDraft(t, a, "Lễ hội E", day(1), 8, nil)                   // ends on day 9 at 10:00 ICT
	win := "/game-admin/events?from=" + date(10) + "&to=" + date(19)
	got := listed(t, e.adminCall(t, http.MethodGet, win, a.liveops, "", nil))
	if len(got) != 3 || got[evA] == nil || got[evB] == nil || got[evD] == nil {
		t.Fatalf("window: %v", got)
	}
	if w := warningsOf(got[evA]); !slices.Equal(w, []string{evB + "/main_hub"}) {
		t.Fatalf("A warnings: %v", w)
	}
	if w := warningsOf(got[evB]); !slices.Equal(w, []string{evA + "/main_hub"}) {
		t.Fatalf("B warnings: %v", w)
	}
	if w := warningsOf(got[evD]); w == nil || len(w) != 0 {
		t.Fatalf("D warnings (present and empty): %v", w)
	}
	if got[evA]["calendar_warnings"].([]any)[0].(map[string]any)["other_title"] != "Lễ hội B" {
		t.Fatalf("other_title: %v", got[evA]["calendar_warnings"])
	}
	// Both bounds inclusive as ICT days: the last day of E (day 9) is in a window ending on day 9; a window starting on
	// day 9 still holds E (10:00 ICT end), one starting on day 10 does not.
	if g := listed(t, e.adminCall(t, http.MethodGet, "/game-admin/events?from="+date(9)+"&to="+date(9), a.liveops, "", nil)); len(g) != 1 || g[evE] == nil {
		t.Fatalf("day 9: %v", g)
	}
	if g := listed(t, e.adminCall(t, http.MethodGet, "/game-admin/events?from="+date(29)+"&to="+date(29), a.liveops, "", nil)); len(g) != 0 {
		t.Fatalf("day 29 (C starts day 30): %v", g)
	}
	// Keyset pages inside the window: limit 1 walks the three events once.
	seen := []string{}
	cursor := ""
	for i := 0; i < 5; i++ {
		r := e.adminCall(t, http.MethodGet, win+"&limit=1"+cursor, a.liveops, "", nil)
		for id := range listed(t, r) {
			seen = append(seen, id)
		}
		pg := r.body["page"].(map[string]any)
		if pg["has_more"] != true {
			break
		}
		cursor = "&cursor=" + pg["next_cursor"].(string)
	}
	if slices.Sort(seen); len(seen) != 3 || slices.Contains(seen, evC) {
		t.Fatalf("pages: %v", seen)
	}
	// Without a window: every event, no calendar_warnings field.
	all := listed(t, e.adminCall(t, http.MethodGet, "/game-admin/events", a.liveops, "", nil))
	if len(all) != 5 || all[evA]["calendar_warnings"] != nil || all[evA]["render_status"] != "not_applicable" {
		t.Fatalf("no window: %v", all[evA])
	}
	for name, path := range map[string]string{
		"from only":   "/game-admin/events?from=" + date(10),
		"to only":     "/game-admin/events?to=" + date(10),
		"63 days":     "/game-admin/events?from=" + date(10) + "&to=" + date(72),
		"to < from":   "/game-admin/events?from=" + date(10) + "&to=" + date(9),
		"other type":  "/game-admin/quests?from=" + date(10) + "&to=" + date(19),
		"season type": "/game-admin/season-tracks?from=" + date(10) + "&to=" + date(19),
	} {
		if r := e.adminCall(t, http.MethodGet, path, a.liveops, "", nil); r.status != http.StatusUnprocessableEntity || r.code() != "VALIDATION_FAILED" {
			t.Errorf("%s: %d %s", name, r.status, r.raw)
		}
	}
	if r := e.adminCall(t, http.MethodGet, "/game-admin/events?from="+date(10)+"&to="+date(71), a.liveops, "", nil); r.status != http.StatusOK {
		t.Fatalf("62 days: %d %s", r.status, r.raw)
	}
}

// insertAsset stores a ready game media asset row (TEST fixture of the render pipeline, F078 covers the pipeline).
func (e *env) insertAsset(ctx context.Context, t *testing.T, kind string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := e.migrator.ExecContext(ctx, `INSERT INTO game.media_assets (asset_id, created_at, updated_at, kind, mime_type, original_filename,
		size_bytes, sha256, bucket, status) VALUES ($1, now(), now(), $2, 'image/png', 'x.png', 10, $3, 'ew-media-public', 'ready')`,
		id, kind, sha([]byte(id.String()))); err != nil {
		t.Fatal(err)
	}
	return id
}

// F079-AC-16: render_status of every GameConfigVersion — not_applicable without a render, pending while the
// render_thumbnail job is queued or retrying, failed once dead, ready with the thumbnail (thumbnail_url) — on the list,
// the version read and the draft update.
func TestGameAdminRenderStatus(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	a := e.actors()
	model := e.insertAsset(ctx, t, "model3d")
	type q struct{ id, vid, etag string }
	quests := map[string]q{}
	for _, name := range []string{"none", "queued", "failed", "dead", "done"} {
		c := e.adminCall(t, http.MethodPost, "/game-admin/quests", a.designer, "", map[string]any{"payload": questDraft(t, "math.g3.render_"+name, nil)})
		if c.status != http.StatusCreated || c.body["render_status"] != "not_applicable" {
			t.Fatalf("create %s: %d %s", name, c.status, c.raw)
		}
		quests[name] = q{c.body["config_id"].(string), c.body["config_version_id"].(string), c.header.Get("ETag")}
	}
	job := func(name, status string, result *uuid.UUID) {
		if _, err := e.migrator.ExecContext(ctx, `INSERT INTO game.render_jobs (render_job_id, created_at, updated_at, config_type, item_type,
			config_version_id, source_asset_id, kind, status, attempts, result_asset_id) VALUES ($1, now(), now(), 'quests', 'quest', $2, $3,
			'render_thumbnail', $4, 1, $5)`, uuid.New(), quests[name].vid, model, status, result); err != nil {
			t.Fatal(err)
		}
	}
	job("queued", "queued", nil)
	job("failed", "failed", nil)
	job("dead", "dead", nil)
	thumb := e.insertAsset(ctx, t, "render_thumbnail")
	job("done", "done", &thumb)
	if _, err := e.migrator.ExecContext(ctx, `INSERT INTO game.media_usages (usage_id, created_at, updated_at, asset_id, item_type, config_id,
		config_version_id, usage) VALUES ($1, now(), now(), $2, 'quest', $3, $4, 'render_thumbnail')`, uuid.New(), thumb, quests["done"].id,
		quests["done"].vid); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"none": "not_applicable", "queued": "pending", "failed": "pending", "dead": "failed", "done": "ready"}
	got := listed(t, e.adminCall(t, http.MethodGet, "/game-admin/quests", a.reviewer, "", nil))
	for name, w := range want {
		if s := got[quests[name].id]["render_status"]; s != w {
			t.Errorf("list %s: %v, want %s", name, s, w)
		}
		g := e.adminCall(t, http.MethodGet, "/game-admin/quests/"+quests[name].id+"/versions/1", a.reviewer, "", nil)
		if g.body["render_status"] != w {
			t.Errorf("get %s: %s", name, g.raw)
		}
	}
	if !strings.HasPrefix(fmt.Sprint(got[quests["done"].id]["thumbnail_url"]), "https://cdn.test/ew-media-public/media/") ||
		got[quests["dead"].id]["thumbnail_url"] != nil {
		t.Fatalf("thumbnail_url: %v %v", got[quests["done"].id], got[quests["dead"].id])
	}
	// The command side (draft update) answers the same render_status.
	u := e.adminCall(t, http.MethodPatch, "/game-admin/quests/"+quests["dead"].id+"/versions/1", a.designer, quests["dead"].etag,
		map[string]any{"payload": questDraft(t, "math.g3.render_dead", nil)})
	if u.status != http.StatusOK || u.body["render_status"] != "failed" {
		t.Fatalf("update: %d %s", u.status, u.raw)
	}
}

// F079-AC-07: getGameClientManifest is the intersection of the newest manifest of each registered platform (the values
// submit_review accepts) with the names of the published areas and NPCs; registered=false and empty lists before any
// build; only console readers (write, review, publish) see it.
func TestGameAdminClientManifest(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	a := e.actors()
	r := e.adminCall(t, http.MethodGet, "/game-admin/client-manifests/current", a.reviewer, "", nil)
	if r.status != http.StatusOK || r.body["registered"] != false || len(r.body["platforms"].([]any)) != 0 || len(r.body["areas"].([]any)) != 0 ||
		len(r.body["npcs"].([]any)) != 0 || len(r.body["scene_refs"].([]any)) != 0 || len(r.body["interactables"].([]any)) != 0 {
		t.Fatalf("none registered: %d %s", r.status, r.raw)
	}
	manifest := func(version, platform, doc string, age time.Duration) {
		if _, err := e.migrator.ExecContext(ctx, `INSERT INTO game.client_manifests (client_manifest_id, created_at, updated_at, client_version,
			platform, manifest, published_at) VALUES ($1, now(), now(), $2, $3, $4::jsonb, now() - $5::interval)`, uuid.New(), version, platform, doc,
			fmt.Sprintf("%d seconds", int(age.Seconds()))); err != nil {
			t.Fatal(err)
		}
	}
	manifest("1.4.0", "windows", `{"area_ids":["main_hub","old_area"],"npc_ids":["npc.old"],"scene_refs":["s.old"],"interactable_ids":[]}`, 2*time.Hour)
	manifest("1.5.0", "windows", `{"area_ids":["main_hub","learning_zone.math"],"npc_ids":["npc.mr_numbers","npc.x"],"scene_refs":["s1","s2"],
		"interactable_ids":["i1","i2"]}`, time.Hour)
	manifest("1.5.0", "web", `{"area_ids":["main_hub","learning_zone.math","web_only"],"npc_ids":["npc.mr_numbers"],"scene_refs":["s1"],
		"interactable_ids":["i2"]}`, time.Minute)
	// Published names: the world area main_hub and a dialogue of npc.mr_numbers (current release of the console tables).
	published := func(itemType, key, payload string) {
		cid, vid := uuid.New(), uuid.New()
		if _, err := e.migrator.ExecContext(ctx, `INSERT INTO game.config_items (config_id, created_at, updated_at, item_type, item_key,
			latest_version_no) VALUES ($1, now(), now(), $2, $3, 1)`, cid, itemType, key); err != nil {
			t.Fatal(err)
		}
		if _, err := e.migrator.ExecContext(ctx, `INSERT INTO game.config_versions (config_version_id, created_at, updated_at, config_id, item_type,
			version_no, status, payload, payload_sha256, published_at) VALUES ($1, now(), now(), $2, $3, 1, 'published', $4::jsonb, $5, now())`,
			vid, cid, itemType, payload, sha([]byte(payload))); err != nil {
			t.Fatal(err)
		}
		if _, err := e.migrator.ExecContext(ctx, `UPDATE game.config_items SET current_version_id = $1 WHERE config_id = $2`, vid, cid); err != nil {
			t.Fatal(err)
		}
	}
	published("world_area", "main_hub", `{"area_id":"main_hub","name":{"vi":"Quảng trường","en":"Main hub"}}`)
	published("npc_dialogue", uuid.NewString(), `{"npc_id":"npc.mr_numbers","npc_name":{"vi":"Thầy Số","en":"Mr Numbers"}}`)
	r = e.adminCall(t, http.MethodGet, "/game-admin/client-manifests/current", a.designer, "", nil)
	if r.status != http.StatusOK || r.body["registered"] != true || len(r.body["platforms"].([]any)) != 2 {
		t.Fatalf("registered: %d %s", r.status, r.raw)
	}
	var m struct {
		Platforms []struct{ Platform, ClientVersion string } `json:"platforms"`
		Areas     []struct {
			AreaID string            `json:"area_id"`
			Name   map[string]string `json:"name"`
		} `json:"areas"`
		NPCs []struct {
			NPCID string            `json:"npc_id"`
			Name  map[string]string `json:"name"`
		} `json:"npcs"`
		Scenes        []string `json:"scene_refs"`
		Interactables []string `json:"interactables"`
	}
	if err := json.Unmarshal([]byte(r.raw), &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Areas) != 2 || m.Areas[0].AreaID != "learning_zone.math" || m.Areas[0].Name != nil || m.Areas[1].AreaID != "main_hub" ||
		m.Areas[1].Name["en"] != "Main hub" {
		t.Fatalf("areas: %+v", m.Areas)
	}
	if len(m.NPCs) != 1 || m.NPCs[0].NPCID != "npc.mr_numbers" || m.NPCs[0].Name["vi"] != "Thầy Số" {
		t.Fatalf("npcs: %+v", m.NPCs)
	}
	if !slices.Equal(m.Scenes, []string{"s1"}) || !slices.Equal(m.Interactables, []string{"i2"}) {
		t.Fatalf("scenes/interactables: %v %v", m.Scenes, m.Interactables)
	}
	for _, tok := range []string{a.liveops, e.platformToken("content_admin"), e.staffToken(e.staff(e.tenantA, "tenant_admin"))} {
		if tok == a.liveops {
			// liveops holds game.event.write and game.content.publish: a console reader.
			if r := e.adminCall(t, http.MethodGet, "/game-admin/client-manifests/current", tok, "", nil); r.status != http.StatusOK {
				t.Fatalf("liveops: %d %s", r.status, r.raw)
			}
			continue
		}
		if r := e.adminCall(t, http.MethodGet, "/game-admin/client-manifests/current", tok, "", nil); r.status != http.StatusForbidden {
			t.Errorf("denied: %d %s", r.status, r.raw)
		}
	}
	authztest.Cover(t, "getGameClientManifest", authztest.RoleDenied)
}

// F079-AC-10, AC-15: simulateGameConfig computes on the payload of the version (read only): XP per play day =
// min(minutes × 14, 1000) on the curve, points.daily_cap on the season track; 422 without minutes_per_day for a curve; 403
// without the read permission of the type; 404 for an unknown version.
func TestGameAdminSimulate(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	a := e.actors()
	curve := progressionItem(t, seedCurve, func(m map[string]any) {
		m["formula"] = map[string]any{"kind": "power_linear", "base": 50, "exponent": 1, "linear": 0}
		m["level_overrides"] = []any{map[string]any{"level": 3, "xp_to_next": 200}}
		m["code"] = "progression.f079_sim"
		delete(m, "curve_id") // a new item (the seed curve id is taken)
	})
	var cm map[string]any
	_ = json.Unmarshal(curve.Payload, &cm)
	c := e.adminCall(t, http.MethodPost, "/game-admin/progression-curves", a.designer, "", map[string]any{"payload": cm})
	if c.status != http.StatusCreated {
		t.Fatalf("curve: %d %s", c.status, c.raw)
	}
	base := "/game-admin/progression-curves/" + c.body["config_id"].(string) + "/versions/1:simulate"
	before := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events`)
	r := e.adminCall(t, http.MethodGet, base+"?minutes_per_day=25&days_per_week=5&weeks=2", a.reviewer, "", nil)
	if r.status != http.StatusOK {
		t.Fatalf("simulate curve: %d %s", r.status, r.raw)
	}
	as := r.body["assumptions"].(map[string]any)
	if as["per_play_day"] != float64(350) || as["minute_rate"] != float64(14) || as["daily_cap"] != float64(1000) || as["minutes_per_day"] != float64(25) {
		t.Fatalf("assumptions: %v", as)
	}
	// Thresholds: 50 × L except level 3 (override 200): cumulative 0, 50, 150, 350, 550, 800, …
	th := r.body["thresholds"].([]any)
	maxLevel := int(cm["max_level"].(float64))
	if len(th) != maxLevel || th[2].(map[string]any)["to_next"] != float64(200) || th[3].(map[string]any)["total"] != float64(350) ||
		th[len(th)-1].(map[string]any)["to_next"] != float64(0) {
		t.Fatalf("thresholds: %v", th[:5])
	}
	// Week 1: 1750 XP; week 2: 3500 XP — levels from the cumulative thresholds.
	pts := r.body["points"].([]any)
	levelFor := func(xp float64) float64 {
		l := 0.0
		for _, x := range th {
			if x.(map[string]any)["total"].(float64) <= xp {
				l = x.(map[string]any)["level"].(float64)
			}
		}
		return l
	}
	for i, total := range []float64{1750, 3500} {
		p := pts[i].(map[string]any)
		if p["week"] != float64(i+1) || p["total"] != total || p["level"] != levelFor(total) || p["level"].(float64) < 2 {
			t.Fatalf("point %d: %v", i, p)
		}
	}
	// The daily cap bounds a long play day.
	if r := e.adminCall(t, http.MethodGet, base+"?minutes_per_day=240&days_per_week=1&weeks=1", a.reviewer, "", nil); r.body["assumptions"].(map[string]any)["per_play_day"] != float64(1000) {
		t.Fatalf("capped: %s", r.raw)
	}
	if r := e.adminCall(t, http.MethodGet, base+"?days_per_week=5&weeks=2", a.reviewer, "", nil); r.status != http.StatusUnprocessableEntity ||
		!strings.Contains(r.raw, "minutes_per_day") {
		t.Fatalf("no minutes_per_day: %d %s", r.status, r.raw)
	}
	if r := e.adminCall(t, http.MethodGet, strings.Replace(base, "/versions/1:", "/versions/9:", 1)+"?minutes_per_day=25&days_per_week=5&weeks=2",
		a.reviewer, "", nil); r.status != http.StatusNotFound {
		t.Fatalf("unknown version: %d %s", r.status, r.raw)
	}
	if after := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events`); after != before {
		t.Fatalf("curve simulations wrote %d outbox rows", after-before)
	}
	// Season track: 5 tiers × 100 points, 100 points/day.
	tr := trackItem(t, uuid.New(), nil)
	var tm map[string]any
	_ = json.Unmarshal(tr.Payload, &tm)
	s := e.adminCall(t, http.MethodPost, "/game-admin/season-tracks", a.liveops, "", map[string]any{"payload": tm})
	if s.status != http.StatusCreated {
		t.Fatalf("track: %d %s", s.status, s.raw)
	}
	before = count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events`) // the track draft wrote its own events
	sb := "/game-admin/season-tracks/" + s.body["config_id"].(string) + "/versions/1:simulate"
	sr := e.adminCall(t, http.MethodGet, sb+"?days_per_week=2&weeks=4", a.liveops, "", nil)
	if sr.status != http.StatusOK || sr.body["assumptions"].(map[string]any)["per_play_day"] != float64(100) ||
		sr.body["assumptions"].(map[string]any)["minute_rate"] != nil {
		t.Fatalf("simulate track: %d %s", sr.status, sr.raw)
	}
	var levels []float64
	for _, p := range sr.body["points"].([]any) {
		levels = append(levels, p.(map[string]any)["level"].(float64))
	}
	if !slices.Equal(levels, []float64{2, 4, 5, 5}) {
		t.Fatalf("track tiers by week: %v", levels)
	}
	sth := sr.body["thresholds"].([]any)
	if len(sth) != 5 || sth[0].(map[string]any)["total"] != float64(100) || sth[4].(map[string]any)["to_next"] != float64(0) {
		t.Fatalf("track thresholds: %v", sth)
	}
	// Read only: nothing written to the outbox (no event, no audit).
	if after := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events`); after != before {
		t.Fatalf("simulate wrote %d outbox rows", after-before)
	}
	// Authorization: the read permission of the type (liveops reads curves through game.content.publish; content_admin
	// and a tenant token cannot); the curve is simulated by its readers only.
	for name, tok := range map[string]string{"content admin": e.platformToken("content_admin"), "competition admin": e.platformToken("competition_admin"),
		"tenant admin": e.staffToken(e.staff(e.tenantA, "tenant_admin"))} {
		if r := e.adminCall(t, http.MethodGet, base+"?minutes_per_day=25&days_per_week=5&weeks=2", tok, "", nil); r.status != http.StatusForbidden {
			t.Errorf("%s: %d %s", name, r.status, r.raw)
		}
	}
	h := map[string]string{"X-Tenant-Id": e.tenantB.String()}
	if r := e.do(t, http.MethodGet, sb+"?days_per_week=2&weeks=4", a.liveops, h, nil); r.status != http.StatusOK {
		t.Fatalf("X-Tenant-Id ignored (platform content): %d %s", r.status, r.raw)
	}
	authztest.Cover(t, "simulateGameConfig", authztest.CrossTenant, authztest.RoleDenied)
}

// F079 T045: getGameConfigCounts counts the configs of module game by type and the status of their newest version
// (GROUP BY in the query); readers of the dashboard only.
func TestGameAdminConfigCounts(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	a := e.actors()
	counts := func(tok string) map[string]float64 {
		r := e.adminCall(t, http.MethodGet, "/game-admin/stats/config-counts", tok, "", nil)
		if r.status != http.StatusOK || r.body["generated_at"] == nil {
			t.Fatalf("counts: %d %s", r.status, r.raw)
		}
		out := map[string]float64{}
		for _, d := range r.body["data"].([]any) {
			m := d.(map[string]any)
			out[m["config_type"].(string)+"/"+m["status"].(string)] += m["count"].(float64)
		}
		return out
	}
	start := counts(a.liveops)
	e.approvedQuest(t, a, "math.g3.count_a")
	c := e.adminCall(t, http.MethodPost, "/game-admin/quests", a.designer, "", map[string]any{"payload": questDraft(t, "math.g3.count_b", nil)})
	if c.status != http.StatusCreated {
		t.Fatalf("draft: %d %s", c.status, c.raw)
	}
	// An item with two versions counts once, by its newest version (draft).
	idx := e.adminCall(t, http.MethodPost, "/game-admin/quests", a.designer, "", map[string]any{"payload": questDraft(t, "math.g3.count_c", nil)})
	if v := e.adminCall(t, http.MethodPost, "/game-admin/quests/"+idx.body["config_id"].(string)+"/versions", a.designer, "", map[string]any{}); v.status != http.StatusCreated {
		t.Fatalf("version 2: %d %s", v.status, v.raw)
	}
	// A soft-deleted item is not counted.
	gone := e.adminCall(t, http.MethodPost, "/game-admin/quests", a.designer, "", map[string]any{"payload": questDraft(t, "math.g3.count_d", nil)})
	if _, err := e.migrator.ExecContext(ctx, `UPDATE game.config_items SET deleted_at = now() WHERE config_id = $1`, gone.body["config_id"]); err != nil {
		t.Fatal(err)
	}
	got := counts(e.platformToken("game_designer"))
	if got["quests/approved"]-start["quests/approved"] != 1 || got["quests/draft"]-start["quests/draft"] != 2 {
		t.Fatalf("counts: %v (start %v)", got, start)
	}
	for k := range got {
		if strings.HasPrefix(k, "gifs/") || strings.HasPrefix(k, "matchmaking") {
			t.Fatalf("another module's type: %s", k)
		}
	}
	_ = counts(a.reviewer)
	for name, tok := range map[string]string{"content admin": e.platformToken("content_admin"), "competition admin": e.platformToken("competition_admin"),
		"tenant admin": e.staffToken(e.staff(e.tenantA, "tenant_admin"))} {
		if r := e.adminCall(t, http.MethodGet, "/game-admin/stats/config-counts", tok, "", nil); r.status != http.StatusForbidden {
			t.Errorf("%s: %d %s", name, r.status, r.raw)
		}
	}
	authztest.Cover(t, "getGameConfigCounts", authztest.RoleDenied)
}
