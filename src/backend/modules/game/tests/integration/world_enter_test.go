//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F071-AC-07, AC-08, AC-10 (T019, TC-WLD-002, TC-GAM-008): entering an open area returns the partition room, the spawn
// point and the level of the area, writes the presence and last_area_id (public areas), and world.area_entered once a
// minute; a locked area is 403 AREA_LOCKED with the missing requirements and no presence; a session that is not active
// or not the student's is 409; a portal of the previous area chooses the spawn point.
func TestEnterWorldArea(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	d, session := e.session(t, an)
	r := e.enter(t, an, d, session, "main_hub", "")
	if r.status != http.StatusOK || r.body["spawn_point_id"] != "main_hub.default" || r.body["room_type"] != "hub_instance" ||
		r.body["level_id"] != levelMainHub || r.body["room_id"] == nil {
		t.Fatalf("enter main_hub: %d %s", r.status, r.raw)
	}
	p, ok := e.presence.get(an.id)
	if !ok || p.AreaID != "main_hub" || p.GameSessionID.String() != session || p.TenantID != an.tenant {
		t.Fatalf("presence: %+v", p)
	}
	// Through the wind gate of the hub to the Math Kingdom, then back through its gate: the arrival spawn of the portal.
	if r := e.enter(t, an, d, session, "learning_zone.math", "portal.hub.to_math"); r.status != http.StatusOK ||
		r.body["spawn_point_id"] != "learning_zone.math.default" {
		t.Fatalf("enter math: %d %s", r.status, r.raw)
	}
	if r := e.enter(t, an, d, session, "main_hub", "portal.math.to_hub"); r.status != http.StatusOK || r.body["spawn_point_id"] != "main_hub.from_math" {
		t.Fatalf("back to hub: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_profiles WHERE student_id = $1 AND last_area_id = 'main_hub'`, an.id); n != 1 {
		t.Fatalf("last_area_id: %d", n)
	}
	// Re-entering within a minute does not repeat world.area_entered.
	_ = e.enter(t, an, d, session, "main_hub", "")
	if n := e.events(ctx, t, "world.area_entered"); n != 3 {
		t.Fatalf("world.area_entered: %d", n)
	}
	// AC-08: locked area → 403 AREA_LOCKED with the missing requirements (0/2 chapters), presence unchanged.
	locked := e.enter(t, an, d, session, "challenge_mountain", "")
	missing, _ := locked.body["missing_requirements"].([]any)
	if locked.status != http.StatusForbidden || locked.code() != string(problem.AreaLocked) || len(missing) != 1 {
		t.Fatalf("locked: %d %s", locked.status, locked.raw)
	}
	m := missing[0].(map[string]any)
	if m["kind"] != "chapters_completed" || m["current"] != float64(0) || m["target"] != float64(2) || m["met"] != false {
		t.Fatalf("missing requirement: %v", m)
	}
	if p, _ := e.presence.get(an.id); p.AreaID != "main_hub" {
		t.Fatalf("presence after a locked area: %+v", p)
	}
	// AC-10: an unknown session, another student's session, an ended session → 409.
	if r := e.enter(t, an, d, uuid.NewString(), "main_hub", ""); r.status != http.StatusConflict {
		t.Fatalf("unknown session: %d %s", r.status, r.raw)
	}
	chi := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	_, other := e.session(t, chi)
	if r := e.enter(t, an, d, other, "main_hub", ""); r.status != http.StatusConflict {
		t.Fatalf("session of another student: %d %s", r.status, r.raw)
	}
	if r := e.end(t, an, d, session); r.status != http.StatusNoContent {
		t.Fatalf("end session: %d", r.status)
	}
	if r := e.enter(t, an, d, session, "main_hub", ""); r.status != http.StatusConflict {
		t.Fatalf("ended session: %d %s", r.status, r.raw)
	}
	if r := e.enter(t, an, d, session, "nowhere", ""); r.status != http.StatusNotFound && r.status != http.StatusConflict {
		t.Fatalf("unknown area: %d %s", r.status, r.raw)
	}
}

// F071-AC-09 (T020): two students of different schools never share a partition room of an area; same school and grade
// band share it; another band of the same school does not.
func TestEnterAreaPartition(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	sessions := map[uuid.UUID][2]any{}
	room := func(s student) any {
		ds, ok := sessions[s.id]
		if !ok {
			d, session := e.session(t, s)
			ds = [2]any{d, session}
			sessions[s.id] = ds
		}
		r := e.enter(t, s, ds[0].(device), ds[1].(string), "main_hub", "")
		if r.status != http.StatusOK {
			t.Fatalf("enter: %d %s", r.status, r.raw)
		}
		return r.body["room_id"]
	}
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	binh := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	other := e.newStudent(ctx, t, e.tenantA, e.schoolA2, "active")
	older := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.setGrade(ctx, t, older, 7)
	if room(an) != room(binh) || room(an) == room(other) || room(an) == room(older) {
		t.Fatal("partition rooms")
	}
}
