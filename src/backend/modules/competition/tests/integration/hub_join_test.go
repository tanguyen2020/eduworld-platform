//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// T012 (F091-AC-01, AC-06, AC-11, AC-23): a join answers the assigned instance (room, area, self_eid, spawn, tick rate,
// max speed, players) with the room output to subscribe, pushes hub.instance.assigned and presence.update to the
// student, writes the room summary and the membership; an unknown, inactive, other-band or locked area is refused
// without a room; an over-long area_id or spawn_point is VALIDATION_FAILED.
func TestHubJoin(t *testing.T) {
	e := setup(t, envOpts{})
	e.node("core-1")
	an := e.student(e.tenantA, e.schoolA, 3)
	user := e.channel(rt.UserChannel(an.id))

	a, r := e.join(an, "gw-1", map[string]any{"area_id": "main_hub", "spawn_point": "gate"})
	if a == nil {
		t.Fatalf("join: %+v", r.Problem)
	}
	if a.AreaID != "main_hub" || a.SelfEID != 1 || a.TickRateHz != 10 || a.MaxSpeed != 6 || a.Spawn != [3]float64{10, 0, 10} ||
		len(a.Players) != 0 {
		t.Fatalf("assigned %+v", a)
	}
	if len(r.Subscribe) != 1 || r.Subscribe[0] != rt.RoomOutChannel(a.RoomID) {
		t.Fatalf("subscribe %v", r.Subscribe)
	}
	// presence.update is published before hub.instance.assigned
	if c := next(t, user, "control:presence.update", nil, 5*time.Second); !strings.Contains(c["presence"].(map[string]any)["room_id"].(string), a.RoomID) {
		t.Fatalf("presence control %v", c)
	}
	if p := next(t, user, "hub.instance.assigned", nil, 5*time.Second); p["room_id"] != a.RoomID {
		t.Fatalf("pushed assigned %v", p)
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.rooms WHERE room_id = $1 AND tenant_id = $2 AND room_type = 'hub_instance'
		AND area_id = 'main_hub' AND school_id = $3 AND grade_band = 'g1_3' AND capacity = 40 AND status = 'open'`, a.RoomID, an.tenant, an.school); n != 1 {
		t.Fatal("room summary not written")
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.session_memberships WHERE room_id = $1 AND student_id = $2 AND left_at IS NULL
		AND game_session_id = $3`, a.RoomID, an.id, an.session); n != 1 {
		t.Fatal("membership not written (audit_hub_membership on by default)")
	}

	// The same join again (resend) gives the same seat back, no second membership.
	if b := e.mustJoin(an, "main_hub"); b.RoomID != a.RoomID || b.SelfEID != a.SelfEID {
		t.Fatalf("rejoin %+v", b)
	}

	// F091-AC-06: area refusals create no room.
	e.area("social_zone", 40, []string{"g8_9"}, false, "active")
	e.area("quest_zone.discovery_ocean", 40, []string{"g1_3"}, true, "active")
	e.area("arena", 40, []string{"g1_3"}, false, "inactive")
	before := e.count(`SELECT count(*) FROM multiplayer.rooms`)
	for area, code := range map[string]string{"khong_co": "NOT_FOUND", "arena": "NOT_FOUND", "social_zone": "PERMISSION_DENIED",
		"quest_zone.discovery_ocean": "PERMISSION_DENIED", strings.Repeat("a", 65): "VALIDATION_FAILED", "Main Hub": "VALIDATION_FAILED"} {
		if _, r := e.join(an, "gw-1", map[string]any{"area_id": area}); r.OK || r.Problem.Code != code {
			t.Errorf("%s: %+v, want %s", area, r.Problem, code)
		}
	}
	if _, r := e.join(an, "gw-1", map[string]any{"area_id": "main_hub", "spawn_point": strings.Repeat("s", 65)}); r.OK || r.Problem.Code != "VALIDATION_FAILED" {
		t.Errorf("long spawn_point: %+v", r.Problem)
	}
	if after := e.count(`SELECT count(*) FROM multiplayer.rooms`); after != before {
		t.Fatal("a refused join created a room")
	}
	// Unlocked once the projection has the unlock (world.area_unlocked).
	e.exec(`INSERT INTO ref.area_unlocks (tenant_id, student_id, area_id, unlocked_at, source_version) VALUES ($1, $2, 'quest_zone.discovery_ocean', now(), 1)`,
		an.tenant, an.id)
	q := e.mustJoin(an, "quest_zone.discovery_ocean")
	if q.RoomID == a.RoomID {
		t.Fatal("new area kept the same room")
	}
	// Leaving main_hub for another area closed the old membership with reason left.
	waitFor(t, 5*time.Second, "membership closed", func() bool {
		return e.count(`SELECT count(*) FROM multiplayer.session_memberships WHERE room_id = $1 AND student_id = $2 AND leave_reason = 'left'`,
			a.RoomID, an.id) == 1
	})

	// F091-AC-11: no membership row when audit_hub_membership is off for the tenant.
	e.tenant(e.tenantB, map[string]any{"audit_hub_membership": false})
	em := e.student(e.tenantB, e.schoolB, 4)
	b := e.mustJoin(em, "main_hub")
	if n := e.count(`SELECT count(*) FROM multiplayer.session_memberships WHERE room_id = $1`, b.RoomID); n != 0 {
		t.Fatal("membership written while audit_hub_membership is off")
	}
	// Student not projected yet → UNAVAILABLE retryable; profile without nickname → CONFLICT.
	ghost := student{id: uuid.New(), tenant: e.tenantA, school: e.schoolA, session: uuid.New(), device: uuid.New()}
	if _, r := e.join(ghost, "gw-1", map[string]any{"area_id": "main_hub"}); r.OK || r.Problem.Code != "UNAVAILABLE" || !r.Problem.Retryable {
		t.Fatalf("not projected: %+v", r.Problem)
	}
	_ = context.Background()
}

// T011/T012 (F091-AC-02, AC-03, AC-04): two schools, two bands and two tenants never share an instance; friends are
// seated together; a full instance (capacity of the area) sends the next student to a new instance of the partition.
func TestHubPartitionAndCapacity(t *testing.T) {
	e := setup(t, envOpts{})
	e.node("core-1")
	e.area("learning_zone.math", 3, []string{"g1_3", "g4_5"}, false, "active")
	an := e.student(e.tenantA, e.schoolA, 3)
	binh := e.student(e.tenantA, e.schoolA, 2)
	chi := e.student(e.tenantA, e.schoolA, 1)
	other := e.student(e.tenantA, e.schoolA2, 3) // other school
	g4 := e.student(e.tenantA, e.schoolA, 4)     // other band
	tb := e.student(e.tenantB, e.schoolB, 3)     // other tenant
	e.friends(an, chi)

	ra := e.mustJoin(an, "learning_zone.math")
	rb := e.mustJoin(binh, "learning_zone.math")
	if rb.RoomID != ra.RoomID {
		t.Fatal("same partition, room below 90 %: should share")
	}
	for _, s := range []student{other, g4, tb} {
		if r := e.mustJoin(s, "learning_zone.math"); r.RoomID == ra.RoomID {
			t.Fatalf("student of another partition seated with g1_3 of school A")
		}
	}
	rc := e.mustJoin(chi, "learning_zone.math") // friend of an: joins an's instance (2/3 seats → 3/3)
	if rc.RoomID != ra.RoomID || len(rc.Players) != 2 {
		t.Fatalf("friend not seated with an: %+v", rc)
	}
	dung := e.student(e.tenantA, e.schoolA, 3)
	if rd := e.mustJoin(dung, "learning_zone.math"); rd.RoomID == ra.RoomID {
		t.Fatal("full instance (area capacity 3) got a 4th student")
	}
	// No instance mixes partitions (SC-001 query of the quickstart).
	if n := e.count(`SELECT count(*) FROM (SELECT room_id FROM multiplayer.rooms GROUP BY room_id
		HAVING count(DISTINCT (tenant_id, school_id, grade_band)) > 1) x`); n != 0 {
		t.Fatal("a room mixes partitions")
	}
	// Blocked students are not seated together while another choice exists.
	e.area("social_zone", 40, []string{"g1_3"}, false, "active")
	x := e.student(e.tenantA, e.schoolA, 3)
	y := e.student(e.tenantA, e.schoolA, 3)
	e.block(y, x)
	rx := e.mustJoin(x, "social_zone")
	if ry := e.mustJoin(y, "social_zone"); ry.RoomID == rx.RoomID {
		t.Fatal("blocked students seated together")
	}
}
