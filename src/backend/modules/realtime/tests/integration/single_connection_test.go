//go:build integration

package integration

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats.go"

	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// T012 (F090-AC-08, AC-09, AC-28; TC-TEN-012): a new connection of the same student replaces the old one on the
// same node and across nodes (ew:rt:node:<id>:in) with close 4409 and no session.session.replaced; every forwarded
// command carries Ew-Tenant-Id of the ticket, even when the payload names another tenant.
func TestSingleConnection(t *testing.T) {
	e := newEnv(t)
	gw1 := e.node("gw-1", nil)
	gw2 := e.node("gw-2", nil)
	an := newStudent()

	a := dial(t, gw1, e.ticket(an))
	a.next("system.session.welcome")
	b := dial(t, gw1, e.ticket(an)) // same node
	b.next("system.session.welcome")
	if code := a.closed(5 * time.Second); code != 4409 {
		t.Fatalf("same node: close %d", code)
	}
	select {
	case m := <-a.msgs:
		if m.Type == "session.session.replaced" {
			t.Fatal("4409 of a newer connection must not carry session.session.replaced")
		}
	default:
	}
	c := dial(t, gw2, e.ticket(an)) // other node: the later registration wins
	c.next("system.session.welcome")
	if code := b.closed(5 * time.Second); code != 4409 {
		t.Fatalf("cross node: close %d", code)
	}
	for deadline := time.Now().Add(5 * time.Second); gw1.m.Count() != 0 && time.Now().Before(deadline); {
		time.Sleep(20 * time.Millisecond)
	}
	if gw1.m.Count() != 0 || gw2.m.Count() != 1 {
		t.Fatalf("connections gw1=%d gw2=%d", gw1.m.Count(), gw2.m.Count())
	}
	// Messages for the student reach the node holding the connection (no sticky session).
	e.publish(rt.UserChannel(an.user), e.message("notification.item.created", map[string]any{"notification_id": "n1"}))
	if m := c.next("notification.item.created"); m.field("notification_id") != "n1" {
		t.Fatalf("delivery: %s", m.Payload)
	}

	// Tenant fixed by the ticket (FR-AUTH-019, ADR-0065).
	st := e.stub("chat", "chat.channel.join", func(*nats.Msg) rt.Reply { return rt.OK(map[string]any{"joined": true}) })
	other := newStudent().tenant
	id := newID()
	ch := newID()
	c.send("chat.channel.join", id, map[string]any{"channel_id": ch, "tenant_id": other.String()})
	if ack := c.next("system.command.acked"); ack.Ref != id {
		t.Fatalf("ack ref %s", ack.Ref)
	}
	h := st.last().Header
	if h.Get(rt.HeaderTenantID) != an.tenant.String() || h.Get(rt.HeaderUserID) != an.user.String() ||
		h.Get(rt.HeaderGameSessionID) != an.session.String() || h.Get(rt.HeaderDeviceID) != an.device.String() ||
		h.Get(rt.HeaderNodeID) != "gw-2" || h.Get(rt.HeaderConnID) == "" {
		t.Fatalf("headers %v", h)
	}
	var body map[string]any
	_ = json.Unmarshal(st.last().Data, &body)
	if body["channel_id"] != ch {
		t.Fatalf("payload forwarded: %v", body)
	}
}
