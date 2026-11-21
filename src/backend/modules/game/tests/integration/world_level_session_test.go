//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

func (e *env) startLevel(t *testing.T, s student, d device, level, session, key string) resp {
	t.Helper()
	h := th(s)
	h["Idempotency-Key"] = key
	return e.do(t, http.MethodPost, "/game/levels/"+level+":start", d.token, h, map[string]any{"game_session_id": session, "manifest_version": 1})
}

func (e *env) endLevel(t *testing.T, s student, d device, id, outcome, key string) resp {
	t.Helper()
	h := th(s)
	h["Idempotency-Key"] = key
	return e.do(t, http.MethodPost, "/game/level-sessions/"+id+":end", d.token, h, map[string]any{"outcome": outcome})
}

func pinned(r resp, itemType string) map[string]any {
	pins, _ := r.body["content_versions"].([]any)
	for _, p := range pins {
		m := p.(map[string]any)
		if m["item_type"] == itemType {
			return m
		}
	}
	return nil
}

// F071-AC-11 and AC-05 (T021, T009 part, TC-CNT-001/005): startLevel pins the current versions of the level, its area,
// the chapters and the NPC dialogues of the area with the manifest version; the same Idempotency-Key replays the same
// level session; one active level session per student (an older one expires); a new area version published while the
// level runs leaves the pins unchanged (a discovery is checked against the pinned layout); endLevelSession ends it,
// twice is 409, and the next start pins the new version.
func TestLevelSessionPins(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	e.rebuildManifests(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	d, session := e.session(t, an)
	if r := e.enter(t, an, d, session, "main_hub", ""); r.status != http.StatusOK {
		t.Fatalf("enter: %d %s", r.status, r.raw)
	}
	key := newKey()
	r := e.startLevel(t, an, d, levelMainHub, session, key)
	area := pinned(r, write.ItemWorldArea)
	if r.status != http.StatusCreated || r.body["status"] != "active" || r.body["manifest_version"] != float64(1) || area == nil ||
		area["item_code"] != "main_hub" || area["version_no"] != float64(1) || pinned(r, write.ItemNPCDialogue) == nil || pinned(r, write.ItemStoryChapter) == nil {
		t.Fatalf("start level: %d %s", r.status, r.raw)
	}
	id := r.body["level_session_id"].(string)
	if again := e.startLevel(t, an, d, levelMainHub, session, key); again.body["level_session_id"] != id || again.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %s %v", again.raw, again.header)
	}
	if p, _ := e.presence.get(an.id); p.LevelSessionID == nil || p.LevelSessionID.String() != id {
		t.Fatalf("presence level session: %+v", p)
	}
	// v2 of main_hub drops the spark tree discovery and adds another; the running level keeps v1.
	v2 := seedItem(t, write.ItemWorldArea, "area_id", "main_hub", func(m map[string]any) {
		layout := m["layout"].(map[string]any)
		var kept []any
		for _, it := range layout["interactables"].([]any) {
			if it.(map[string]any)["interactable_id"] != "discovery.hub.spark_tree" {
				kept = append(kept, it)
			}
		}
		layout["interactables"] = append(kept, map[string]any{"interactable_id": "discovery.hub.new_statue", "kind": "discovery",
			"position": []any{3, 0, -3}})
	})
	if res, err := e.publish(ctx, t, []command.ContentItem{v2}, "v2"); err != nil || res.Published != 1 {
		t.Fatalf("v2: %+v %v", res, err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.level_sessions WHERE level_session_id = $1
		AND content_versions @> '[{"item_type":"world_area","version_no":1}]'`, id); n != 1 {
		t.Fatalf("pins changed: %d", n)
	}
	if r := e.discover(t, an, d, "main_hub", "discovery.hub.spark_tree", newKey()); r.status != http.StatusOK {
		t.Fatalf("discovery of the pinned layout: %d %s", r.status, r.raw)
	}
	if r := e.discover(t, an, d, "main_hub", "discovery.hub.new_statue", newKey()); r.code() != string(problem.StepPreconditionFailed) {
		t.Fatalf("discovery of the new layout during the pinned level: %d %s", r.status, r.raw)
	}
	// A second start expires the first (one active level session per student).
	second := e.startLevel(t, an, d, levelMainHub, session, newKey())
	if second.status != http.StatusCreated || pinned(second, write.ItemWorldArea)["version_no"] != float64(2) {
		t.Fatalf("second start: %d %s", second.status, second.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.level_sessions WHERE level_session_id = $1 AND status = 'expired'`, id); n != 1 {
		t.Fatalf("first level session not expired: %d", n)
	}
	id2 := second.body["level_session_id"].(string)
	if r := e.endLevel(t, an, d, id2, "completed", newKey()); r.status != http.StatusOK || r.body["status"] != "completed" {
		t.Fatalf("end: %d %s", r.status, r.raw)
	}
	if r := e.endLevel(t, an, d, id2, "completed", newKey()); r.status != http.StatusConflict {
		t.Fatalf("end twice: %d %s", r.status, r.raw)
	}
	if p, _ := e.presence.get(an.id); p.LevelSessionID != nil {
		t.Fatalf("presence still pinned: %+v", p)
	}
	// AC-10: startLevel with an inactive session → 409; an unknown level → 404.
	if r := e.startLevel(t, an, d, levelMainHub, newKey(), newKey()); r.status != http.StatusConflict {
		t.Fatalf("unknown session: %d %s", r.status, r.raw)
	}
	if r := e.startLevel(t, an, d, newKey(), session, newKey()); r.status != http.StatusNotFound {
		t.Fatalf("unknown level: %d %s", r.status, r.raw)
	}
	// A level of a locked area is 403 AREA_LOCKED.
	if r := e.startLevel(t, an, d, seedID(0x71200, 11), session, newKey()); r.status != http.StatusForbidden || r.code() != "AREA_LOCKED" {
		t.Fatalf("level of challenge_mountain: %d %s", r.status, r.raw)
	}
}
