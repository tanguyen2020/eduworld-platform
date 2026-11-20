//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/config"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// T025 (F090-AC-20): frame over ws_max_frame → VALIDATION_FAILED, unknown type → UNKNOWN_TYPE, a string over the
// maxLength of the AsyncAPI → VALIDATION_FAILED; nothing is forwarded; repeated protocol errors close 4400.
func TestProtocolLimits(t *testing.T) {
	e := newEnv(t)
	n := e.node("gw-1", func(c *config.Config) { c.MaxFrame = 1024; c.ProtocolErrors = 3 })
	st := e.stub("chat", "chat.message.send", func(*nats.Msg) rt.Reply { return rt.OK(nil) })
	c := dial(t, n, e.ticket(newStudent()))
	c.next("system.session.welcome")
	big := `{"v":1,"type":"chat.message.send","id":"` + newID() + `","payload":{"channel_id":"` + newID() + `","kind":"text","body":"` +
		strings.Repeat("x", 2000) + `"}}`
	c.sendRaw([]byte(big))
	if f := c.next("system.command.failed"); f.field("code") != "VALIDATION_FAILED" {
		t.Fatalf("big frame %s", f.Payload)
	}
	id := newID()
	c.send("foo.bar.baz", id, map[string]any{})
	if f := c.next("system.command.failed"); f.field("code") != "UNKNOWN_TYPE" || f.Ref != id {
		t.Fatalf("unknown %+v %s", f, f.Payload)
	}
	lid := newID()
	c.send("chat.message.send", lid, map[string]any{"channel_id": newID(), "kind": "text", "body": strings.Repeat("y", 301)})
	if f := c.next("system.command.failed"); f.field("code") != "VALIDATION_FAILED" || f.Ref != lid {
		t.Fatalf("max length %+v %s", f, f.Payload)
	}
	if st.n() != 0 {
		t.Fatal("invalid frames forwarded")
	}
	// The 4th protocol error within a minute closes 4400.
	c.sendRaw([]byte(`not json`))
	if code := c.closed(5 * time.Second); code != 4400 {
		t.Fatalf("close %d", code)
	}
}

// T025 (F090-AC-22): over ws_frame_rate → RATE_LIMITED with retry_after_ms; more than 3 violation episodes per minute
// → close 4429 and audit.recorded realtime.rate_limit_closed on JetStream EVT_REALTIME. Over ws_inflight_commands →
// RATE_LIMITED.
func TestRateLimit(t *testing.T) {
	e := newEnv(t)
	n := e.node("gw-1", func(c *config.Config) { c.FrameRate, c.FrameBurst, c.RateViolations, c.Inflight = 5, 5, 3, 1 })
	release := make(chan struct{})
	e.stub("chat", "chat.typing.start", func(*nats.Msg) rt.Reply { <-release; return rt.OK(nil) })
	an := newStudent()
	c := dial(t, n, e.ticket(an))
	c.next("system.session.welcome")

	// Inflight: the first command waits in the module, the second is refused.
	c.send("chat.typing.start", newID(), map[string]any{"channel_id": newID()})
	time.Sleep(100 * time.Millisecond)
	c.send("chat.typing.start", newID(), map[string]any{"channel_id": newID()})
	if f := c.next("system.command.failed"); f.field("code") != "RATE_LIMITED" {
		t.Fatalf("inflight %s", f.Payload)
	}
	close(release)
	c.next("system.command.acked")

	cons, err := e.js.CreateOrUpdateConsumer(context.Background(), "EVT_REALTIME", jetstream.ConsumerConfig{
		FilterSubject: "ew.evt.realtime.audit.recorded", DeliverPolicy: jetstream.DeliverAllPolicy, AckPolicy: jetstream.AckNonePolicy})
	if err != nil {
		t.Fatal(err)
	}
	// Bursts in 4 distinct seconds: RATE_LIMITED each time, then 4429.
	var limited int
	for burst := 0; burst < 4; burst++ {
		for i := 0; i < 8; i++ {
			c.send("system.heartbeat.ping", newID(), map[string]any{"client_time": 1})
		}
		time.Sleep(1100 * time.Millisecond)
	}
	for {
		select {
		case m := <-c.msgs:
			if m.Type == "system.command.failed" && m.field("code") == "RATE_LIMITED" {
				if m.field("retry_after_ms") == nil {
					t.Fatalf("no retry_after_ms: %s", m.Payload)
				}
				limited++
			}
			continue
		default:
		}
		break
	}
	if code := c.closed(5 * time.Second); code != 4429 {
		t.Fatalf("close %d after %d RATE_LIMITED", code, limited)
	}
	if limited == 0 {
		t.Fatal("no RATE_LIMITED before 4429")
	}
	msgs, err := cons.FetchNoWait(10)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for m := range msgs.Messages() {
		var env struct {
			EventType string         `json:"event_type"`
			TenantID  string         `json:"tenant_id"`
			Payload   map[string]any `json:"payload"`
		}
		_ = json.Unmarshal(m.Data(), &env)
		if env.EventType == "audit.recorded" && env.Payload["action"] == "realtime.rate_limit_closed" && env.TenantID == an.tenant.String() &&
			env.Payload["actor_id"] == an.user.String() && m.Headers().Get(jetstream.MsgIDHeader) != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("audit.recorded realtime.rate_limit_closed not on EVT_REALTIME")
	}
}

// T025 (F090-AC-23): a client that stops reading fills its send queue: droppable frames are dropped first
// (eduworld_rt_send_queue_dropped_total), and when non-droppable frames cannot be pushed for the send stall the
// gateway closes the connection with 4408 so the client resumes.
func TestSendQueueStall(t *testing.T) {
	e := newEnv(t)
	n := e.node("gw-1", func(c *config.Config) {
		c.SendQueue, c.SendStall, c.WriteTimeout = 8, 500*time.Millisecond, 2*time.Second
	})
	an := newStudent()
	c := dial(t, n, e.ticket(an))
	c.next("system.session.welcome")
	c.stopReading()
	big := strings.Repeat("x", 30000)
	snap := e.message("hub.snapshot.updated", map[string]any{"blob": big})
	snap.Droppable = true
	deadline := time.Now().Add(30 * time.Second)
	for n.counter("eduworld_rt_connections_closed_total", map[string]string{"code": "4408"}) == 0 {
		if time.Now().After(deadline) {
			t.Fatalf("no 4408; dropped=%v", n.counter("eduworld_rt_send_queue_dropped_total", nil))
		}
		for i := 0; i < 20; i++ {
			e.publish(rt.UserChannel(an.user), snap)
			e.publish(rt.UserChannel(an.user), e.message("notification.item.created", map[string]any{"blob": big}))
		}
	}
	if n.counter("eduworld_rt_send_queue_dropped_total", map[string]string{"type": "hub.snapshot.updated"}) == 0 {
		t.Fatal("no droppable frame dropped before closing")
	}
}
