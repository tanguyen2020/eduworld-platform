//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"

	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// T024 (F090-AC-19, AC-21; TC-CHAT-008, TC-TEN-012): the gateway subscribes only what a module reply grants, before
// the ack, with one Valkey subscription per node and channel (reference count); a message on a channel reaches only
// the local connections registered on it — a student of tenant B never gets the channel of tenant A; a user or node
// channel in a reply is refused; ew:rt:broadcast reaches every connection.
func TestSubscriptions(t *testing.T) {
	e := newEnv(t)
	n := e.node("gw-1", nil)
	chA := uuid.NewString()
	foreign := newStudent()
	e.stub("chat", "chat.channel.join", func(m *nats.Msg) rt.Reply {
		c, _ := rt.CallerFrom(m.Header)
		if c.TenantID == foreign.tenant { // the module refuses the channel of another tenant (it decides, not the gateway)
			return rt.Fail("CHAT_NOT_ALLOWED", false)
		}
		return rt.Reply{OK: true, Subscribe: []string{rt.ChatFanoutChannel(chA), rt.UserChannel(foreign.user), rt.NodeChannel("gw-9"),
			rt.BroadcastChannel}}
	})
	e.stub("chat", "chat.channel.leave", func(*nats.Msg) rt.Reply {
		return rt.Reply{OK: true, Unsubscribe: []string{rt.ChatFanoutChannel(chA)}}
	})
	an, binh := newStudent(), newStudent()
	binh.tenant = an.tenant
	a := dial(t, n, e.ticket(an))
	a.next("system.session.welcome")
	b := dial(t, n, e.ticket(binh))
	b.next("system.session.welcome")
	f := dial(t, n, e.ticket(foreign))
	f.next("system.session.welcome")

	ch := rt.ChatFanoutChannel(chA)
	if e.numsub(ch) != 0 {
		t.Fatal("subscribed before any reply")
	}
	a.send("chat.channel.join", newID(), map[string]any{"channel_id": chA})
	a.next("system.command.acked")
	if e.numsub(ch) != 1 { // subscribed before the ack was sent
		t.Fatalf("numsub after ack = %d", e.numsub(ch))
	}
	if e.numsub(rt.UserChannel(foreign.user)) != 1 || e.numsub(rt.NodeChannel("gw-9")) != 0 {
		t.Fatal("refused topics must not be subscribed for the connection")
	}
	if n.counter("eduworld_rt_subscribe_refused_total", nil) != 3 {
		t.Fatalf("refused = %v", n.counter("eduworld_rt_subscribe_refused_total", nil))
	}
	f.send("chat.channel.join", newID(), map[string]any{"channel_id": chA})
	if m := f.next("system.command.failed"); m.field("code") != "CHAT_NOT_ALLOWED" {
		t.Fatalf("foreign join %s", m.Payload)
	}
	e.publish(ch, e.message("chat.message.created", map[string]any{"message_id": "m-1"}))
	if m := a.next("chat.message.created"); m.field("message_id") != "m-1" {
		t.Fatalf("a: %s", m.Payload)
	}
	b.none("chat.message.created", 300*time.Millisecond)
	f.none("chat.message.created", 100*time.Millisecond)

	// Reference count: b joins (still one Valkey subscription), a leaves (still subscribed), b leaves (unsubscribed).
	b.send("chat.channel.join", newID(), map[string]any{"channel_id": chA})
	b.next("system.command.acked")
	if e.numsub(ch) != 1 || n.m.Count() != 3 {
		t.Fatalf("numsub with two receivers = %d", e.numsub(ch))
	}
	a.send("chat.channel.leave", newID(), map[string]any{"channel_id": chA})
	a.next("system.command.acked")
	if e.numsub(ch) != 1 {
		t.Fatal("unsubscribed while b still listens")
	}
	e.publish(ch, e.message("chat.message.created", map[string]any{"message_id": "m-2"}))
	b.next("chat.message.created")
	a.none("chat.message.created", 300*time.Millisecond)
	b.send("chat.channel.leave", newID(), map[string]any{"channel_id": chA})
	b.next("system.command.acked")
	e.waitNumsub(ch, 0)

	// Broadcast reaches every connection of the node.
	e.publish(rt.BroadcastChannel, e.message("system.notice.broadcasted", map[string]any{"level": "maintenance", "message": map[string]any{"vi": "x"}}))
	for _, c := range []*client{a, b, f} {
		c.next("system.notice.broadcasted")
	}
}
