//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats.go"

	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// T014 (F091-AC-07, AC-08, TC-TEN-012): a command or input on a room the sender is not a member of — or a room of
// another tenant — answers ROOM_NOT_MEMBER and leaves the room untouched; identity and tenant come from the Ew-* headers
// only: a request without them is UNAUTHENTICATED and ids in the payload are ignored. (The NATS account permissions
// that keep any other NATS user off ew.cmd.> / ew.in.> are deployment config, OQ F090-2.)
func TestHubAuthorization(t *testing.T) {
	e := setup(t, envOpts{})
	e.node("core-1")
	an := e.student(e.tenantA, e.schoolA, 3)
	em := e.student(e.tenantB, e.schoolB, 4)   // other tenant
	binh := e.student(e.tenantA, e.schoolA, 5) // same tenant, other band
	ra := e.mustJoin(an, "main_hub")
	out := e.channel(rt.RoomOutChannel(ra.RoomID))

	// emote on a room of another tenant / not a member
	for _, s := range []student{em, binh} {
		c := s.caller("gw-1")
		if r := e.request("hub.player.emote", &c, map[string]any{"room_id": ra.RoomID, "emote_id": "emote_wave"}); r.OK || r.Problem.Code != "ROOM_NOT_MEMBER" {
			t.Fatalf("emote of a non member: %+v", r.Problem)
		}
	}
	// move on another tenant's room: refused, the room unchanged, the sender told ROOM_NOT_MEMBER
	emUser := e.channel(rt.UserChannel(em.id))
	e.move(em, ra.RoomID, 1, [3]float64{1, 0, 1})
	if p := next(t, emUser, "system.command.failed", nil, 5*time.Second); p["code"] != "ROOM_NOT_MEMBER" {
		t.Fatalf("move of a non member: %v", p)
	}
	none(t, out, "hub.player.joined", 300*time.Millisecond)

	// No Ew-* headers → UNAUTHENTICATED; payload ids are never identity.
	if r := e.request("hub.instance.join", nil, map[string]any{"area_id": "main_hub", "user_id": an.id, "tenant_id": an.tenant}); r.OK ||
		r.Problem.Code != "UNAUTHENTICATED" {
		t.Fatalf("no headers: %+v", r.Problem)
	}
	m := nats.NewMsg(rt.RoomInputSubject(ra.RoomID)) // input without headers: dropped silently
	m.Data, _ = json.Marshal(map[string]any{"room_id": ra.RoomID, "client_seq": 1, "p": []float64{50, 0, 50}})
	_ = e.nc.PublishMsg(m)
	// headers of em with an's ids in the payload: em is not seated in an's room
	c := em.caller("gw-1")
	r := e.request("hub.instance.join", &c, map[string]any{"area_id": "main_hub", "user_id": an.id.String()})
	if !r.OK {
		t.Fatalf("em join: %+v", r.Problem)
	}
	var a assigned
	_ = json.Unmarshal(r.Result, &a)
	if a.RoomID == ra.RoomID {
		t.Fatal("payload user_id changed the identity")
	}
	_ = context.Background()
}
