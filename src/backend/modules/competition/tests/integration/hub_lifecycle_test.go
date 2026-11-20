//go:build integration

package integration

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats.go"

	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// presence publishes a presence signal like the gateway (ew.rt.presence.<tenant>).
func (e *env) presence(s student, status, reason string) {
	e.t.Helper()
	b, _ := json.Marshal(rt.PresenceChanged{V: 1, UserID: s.id, TenantID: s.tenant, Status: status, Reason: reason, Ts: time.Now().UnixMilli()})
	if err := e.nc.Publish(rt.PresenceSubject(s.tenant), b); err != nil {
		e.t.Fatal(err)
	}
	_ = e.nc.Flush()
}

// T015 (F091-AC-09, AC-10, AC-11): a disconnected member keeps the seat for disconnect_grace (avatar idle, no
// hub.player.left) and comes back to the same instance even when it is full; after the grace it leaves with
// hub.player.left and the membership closes (disconnected); an instance empty for hub_instance_idle_close closes
// (status closed, closed_at) and stops; the job closes open rooms whose room host vanished.
func TestHubLifecycle(t *testing.T) {
	e := setup(t, envOpts{grace: 1500 * time.Millisecond, idle: 1500 * time.Millisecond})
	n := e.node("core-1")
	e.area("learning_zone.math", 2, []string{"g1_3"}, false, "active")
	an := e.student(e.tenantA, e.schoolA, 3)
	binh := e.student(e.tenantA, e.schoolA, 3)
	ra := e.mustJoin(an, "learning_zone.math")
	out := e.channel(rt.RoomOutChannel(ra.RoomID))
	if rb := e.mustJoin(binh, "learning_zone.math"); rb.RoomID != ra.RoomID {
		t.Fatal("binh not seated with an")
	}
	// an loses the connection: seat held, an comes back within the grace to the full room.
	e.presence(an, rt.PresenceAway, rt.PresenceTimeout)
	time.Sleep(300 * time.Millisecond)
	chi := e.student(e.tenantA, e.schoolA, 3)
	if rc := e.mustJoin(chi, "learning_zone.math"); rc.RoomID == ra.RoomID {
		t.Fatal("held seat given to another student")
	}
	if back := e.mustJoin(an, "learning_zone.math"); back.RoomID != ra.RoomID || back.SelfEID != ra.SelfEID {
		t.Fatalf("not back to the held seat: %+v", back)
	}
	none(t, out, "hub.player.left", 300*time.Millisecond)

	// Grace over → hub.player.left and membership closed with disconnected.
	e.presence(binh, rt.PresenceOffline, rt.PresenceDisconnected)
	left := next(t, out, "hub.player.left", nil, 5*time.Second)
	if left["room_id"] != ra.RoomID || left["eid"] != float64(2) {
		t.Fatalf("left %v", left)
	}
	waitFor(t, 5*time.Second, "membership closed", func() bool {
		return e.count(`SELECT count(*) FROM multiplayer.session_memberships WHERE room_id = $1 AND student_id = $2 AND leave_reason = 'disconnected'
			AND left_at IS NOT NULL`, ra.RoomID, binh.id) == 1
	})
	// A held student whose input comes back (resume) is connected again without a join.
	e.presence(an, rt.PresenceOffline, rt.PresenceDisconnected)
	time.Sleep(200 * time.Millisecond)
	e.move(an, ra.RoomID, 1, [3]float64{0.5, 0, 0.5})
	none(t, out, "hub.player.left", 2*time.Second)

	// Everyone leaves (another area) → idle close after hub_instance_idle_close.
	e.mustJoin(an, "main_hub")
	waitFor(t, 8*time.Second, "room closed", func() bool {
		return e.count(`SELECT count(*) FROM multiplayer.rooms WHERE room_id = $1 AND status = 'closed' AND closed_at IS NOT NULL`, ra.RoomID) == 1
	})
	for _, id := range n.host.Hosted() {
		if id == ra.RoomID {
			t.Fatal("closed room still hosted")
		}
	}
	if live, _ := n.hub.Registry.Live(t.Context(), ra.RoomID); live {
		t.Fatal("closed room still in the registry")
	}
	// The job closes an open room whose room host and Valkey state are gone.
	e.exec(`INSERT INTO multiplayer.rooms (room_id, tenant_id, created_at, updated_at, room_type, area_id, school_id, grade_band, capacity, status)
		VALUES (gen_random_uuid(), $1, now() - interval '1 hour', now(), 'hub_instance', 'main_hub', $2, 'g1_3', 40, 'open')`, e.tenantA, e.schoolA)
	closed, err := n.hub.CloseOrphanRooms(t.Context(), 100)
	if err != nil || closed < 1 {
		t.Fatalf("orphan close: %d %v", closed, err)
	}
	if e.count(`SELECT count(*) FROM multiplayer.rooms WHERE status = 'open' AND created_at < now() - interval '30 minutes'`) != 0 {
		t.Fatal("orphan room still open")
	}
	_ = nats.ErrNoResponders
}
