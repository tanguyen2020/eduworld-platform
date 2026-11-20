package unit

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/hub"
)

// T021 (F091-AC-14): cells of 16 m, the viewer's cell and its 8 neighbours; the snapshot of a viewer only carries
// entities in that area that are new or changed, and removed[] for those that left it.
func TestInterestGrid(t *testing.T) {
	if hub.CellOf(hub.Vec3{15.99, 0, -0.01}) != (hub.Cell{X: 0, Z: -1}) || hub.CellOf(hub.Vec3{16, 0, 32}) != (hub.Cell{X: 1, Z: 2}) {
		t.Fatal("cell")
	}
	c := hub.Cell{}
	for _, n := range []hub.Cell{{X: -1, Z: -1}, {X: 1, Z: 1}, {X: 0, Z: 1}, {X: 0, Z: 0}} {
		if !c.Near(n) {
			t.Fatalf("%v not near", n)
		}
	}
	for _, f := range []hub.Cell{{X: 2, Z: 0}, {X: 0, Z: -2}, {X: 2, Z: 2}} {
		if c.Near(f) {
			t.Fatalf("%v near", f)
		}
	}
	viewer := hub.Entity{EID: 1, P: hub.Vec3{8, 0, 8}}
	near := hub.Entity{EID: 2, P: hub.Vec3{20, 0, 20}, A: "walk"}
	far := hub.Entity{EID: 3, P: hub.Vec3{40, 0, 8}}
	changed, removed, view := hub.Delta(viewer, []hub.Entity{viewer, near, far}, nil)
	if len(changed) != 1 || changed[0].EID != 2 || len(removed) != 0 {
		t.Fatalf("first: %v %v", changed, removed)
	}
	if changed, removed, _ := hub.Delta(viewer, []hub.Entity{viewer, near, far}, view); len(changed) != 0 || len(removed) != 0 {
		t.Fatalf("unchanged: %v %v", changed, removed)
	}
	near.P = hub.Vec3{40, 0, 40}
	far.P = hub.Vec3{-5, 0, 0}
	changed, removed, _ = hub.Delta(viewer, []hub.Entity{viewer, near, far}, view)
	if len(changed) != 1 || changed[0].EID != 3 || len(removed) != 1 || removed[0] != 2 {
		t.Fatalf("moved: %v %v", changed, removed)
	}
}

// T021 (F091-AC-14, AC-15): Tick gives one snapshot per connected viewer that has news, with its own ack_client_seq;
// a held member gets none and stands idle; the JSON of snapshots and of the public player has no real name, school,
// class or username — only eid, position, yaw, animation and PublicPlayer (nickname, avatar_config, level, user_id).
func TestTickAndPayloadPrivacy(t *testing.T) {
	now := time.Now()
	p := hub.Partition{TenantID: uuid.New(), SchoolID: uuid.New(), GradeBand: "g1_3"}
	in := hub.NewInstance("r1", testArea(), p, 40, now)
	users := make([]uuid.UUID, 3)
	for i := range users {
		users[i] = uuid.New()
		s := hub.Seat{Player: hub.PublicPlayer{UserID: users[i], Nickname: "Meo", Level: 2, AvatarConfig: map[string]any{"hair": "hair_01"}},
			TenantID: p.TenantID, SchoolID: p.SchoolID, GradeBand: "g1_3", Node: "gw-" + string(rune('a'+i))}
		if _, _, err := in.Join(s, hub.Vec3{float64(i), 0, 0}, now); err != nil {
			t.Fatal(err)
		}
	}
	snaps := in.Tick()
	if len(snaps) != 3 {
		t.Fatalf("first tick: %d snapshots", len(snaps))
	}
	for _, s := range snaps {
		if len(s.Entities) != 2 || s.Tick != 1 {
			t.Fatalf("snapshot %+v", s)
		}
	}
	if snaps := in.Tick(); len(snaps) != 0 {
		t.Fatalf("idle tick sent %d snapshots", len(snaps))
	}
	now = now.Add(time.Second)
	if _, err := in.Move(users[0], hub.Input{ClientSeq: 5, P: hub.Vec3{1, 0, 1}, R: 0.5, A: "walk"}, now); err != nil {
		t.Fatal(err)
	}
	in.Hold(users[2], now, 30*time.Second)
	snaps = in.Tick()
	got := map[uuid.UUID]hub.ViewerSnapshot{}
	for _, s := range snaps {
		got[s.UserID] = s
	}
	if _, ok := got[users[2]]; ok {
		t.Fatal("held member got a snapshot")
	}
	if s := got[users[0]]; s.AckSeq != 5 || len(s.Entities) != 0 {
		t.Fatalf("mover's own snapshot (ack only): %+v", s)
	}
	if s := got[users[1]]; len(s.Entities) != 1 || s.Entities[0].A != "walk" || s.Entities[0].P != (hub.Vec3{1, 0, 1}) {
		t.Fatalf("other viewer: %+v", s)
	}
	b, _ := json.Marshal(snaps)
	pb, _ := json.Marshal(hub.PublicPlayer{UserID: users[0], Nickname: "Meo", Level: 2, AvatarConfig: map[string]any{"hair": "hair_01"}})
	var keys map[string]any
	_ = json.Unmarshal(pb, &keys)
	for k := range keys {
		if k != "user_id" && k != "nickname" && k != "avatar_config" && k != "level" {
			t.Fatalf("PublicPlayer field %s", k)
		}
	}
	var ent []map[string]any
	e, _ := json.Marshal(got[users[1]].Entities)
	_ = json.Unmarshal(e, &ent)
	for _, m := range ent {
		for k := range m {
			if k != "eid" && k != "p" && k != "r" && k != "a" {
				t.Fatalf("entity field %s", k)
			}
		}
	}
	for _, banned := range []string{"full_name", "school", "class", "username", "email"} {
		if strings.Contains(string(b), banned) || strings.Contains(string(pb), banned) {
			t.Fatalf("payload contains %s", banned)
		}
	}
}
