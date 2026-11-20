//go:build integration

package integration

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/config"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// T023 (F090-AC-17, AC-18; TC-SVC-012): commands go to ew.cmd.<module>.<type> with the Ew-* headers and traceparent
// and get exactly one ack/failed with ref = id; hub.player.move goes to ew.in.room.<room_id> without ack; a module
// that does not answer within nats_request_timeout gives UNAVAILABLE with retry_after_ms, events still flow.
func TestRouter(t *testing.T) {
	e := newEnv(t)
	n := e.node("gw-1", func(c *config.Config) { c.CmdTimeout = 400 * time.Millisecond })
	party := e.stub("competition", "party.member.ready_set", func(*nats.Msg) rt.Reply { return rt.OK(map[string]any{"ready": true}) })
	boss := e.stub("game", "boss.encounter.ready", func(*nats.Msg) rt.Reply {
		return rt.Reply{Problem: &rt.ReplyProblem{Code: "BOSS_ENCOUNTER_FULL", RetryAfterMs: 1500}}
	})
	moves := make(chan *nats.Msg, 4)
	sub, err := e.nc.ChanSubscribe(rt.RoomInputSubject("room-1"), moves)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Unsubscribe() }()
	_ = e.nc.Flush()
	an := newStudent()
	c := dial(t, n, e.ticket(an))
	c.next("system.session.welcome")

	id := newID()
	c.send("party.member.ready_set", id, map[string]any{"party_id": newID(), "ready": true})
	ack := c.next("system.command.acked")
	var res struct {
		Result map[string]any `json:"result"`
	}
	_ = json.Unmarshal(ack.Payload, &res)
	if ack.Ref != id || res.Result["ready"] != true || party.n() != 1 {
		t.Fatalf("ack %+v %s", ack, ack.Payload)
	}
	if h := party.last().Header; h.Get(rt.HeaderTraceparent) == "" || h.Get(rt.HeaderConnID) == "" || h.Get(rt.HeaderTenantID) != an.tenant.String() {
		t.Fatalf("headers %v", h)
	}
	if party.last().Subject != "ew.cmd.competition.party.member.ready_set" {
		t.Fatalf("subject %s", party.last().Subject)
	}
	bid := newID()
	c.send("boss.encounter.ready", bid, map[string]any{"encounter_id": newID(), "ready": true})
	f := c.next("system.command.failed")
	if f.Ref != bid || f.field("code") != "BOSS_ENCOUNTER_FULL" || f.field("retry_after_ms") != float64(1500) || boss.n() != 1 {
		t.Fatalf("failed %+v %s", f, f.Payload)
	}
	c.none("system.command.acked", 200*time.Millisecond) // exactly one answer per command

	// hub.player.move → NATS core ew.in.room.<room_id>, no ack.
	c.send("hub.player.move", newID(), map[string]any{"room_id": "room-1", "client_seq": 1, "p": []float64{1, 0, 2}, "r": 0.5})
	select {
	case m := <-moves:
		if m.Header.Get(rt.HeaderTenantID) != an.tenant.String() {
			t.Fatalf("move headers %v", m.Header)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("move not forwarded")
	}
	c.none("system.command.acked", 300*time.Millisecond)

	// No responder (module down) → UNAVAILABLE with retry_after_ms; Valkey delivery keeps working.
	uid := newID()
	start := time.Now()
	c.send("chat.channel.join", uid, map[string]any{"channel_id": newID()})
	u := c.next("system.command.failed")
	if u.Ref != uid || u.field("code") != "UNAVAILABLE" || u.field("retry_after_ms") == nil || time.Since(start) > 3*time.Second {
		t.Fatalf("unavailable %+v %s", u, u.Payload)
	}
	e.publish(rt.UserChannel(an.user), e.message("notification.item.created", map[string]any{"n": 1}))
	c.next("notification.item.created")
}
