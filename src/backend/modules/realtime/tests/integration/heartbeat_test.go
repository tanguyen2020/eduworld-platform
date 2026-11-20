//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/config"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// T017 (F090-AC-10, AC-11): system.heartbeat.ping → pong with client_time and server_time; WebSocket ping control
// frames every ws_ping; no frame for ws_timeout → close 4408.
func TestHeartbeat(t *testing.T) {
	e := newEnv(t)
	n := e.node("gw-1", func(c *config.Config) {
		c.Heartbeat, c.Timeout, c.Ping = 300*time.Millisecond, 1200*time.Millisecond, 200*time.Millisecond
	})
	e.stub(rt.SessionHeartbeatModule, rt.SessionHeartbeatType, func(*nats.Msg) rt.Reply { return rt.OK(nil) })
	c := dial(t, n, e.ticket(newStudent()))
	c.next("system.session.welcome")
	c.send("system.heartbeat.ping", newID(), map[string]any{"client_time": 1763190000123})
	p := c.next("system.heartbeat.pong")
	if p.field("client_time") != float64(1763190000123) || p.field("server_time") == nil {
		t.Fatalf("pong %s", p.Payload)
	}
	// Keep sending pings for a while: the connection stays open and the server pings the socket.
	for i := 0; i < 4; i++ {
		time.Sleep(300 * time.Millisecond)
		c.send("system.heartbeat.ping", newID(), map[string]any{"client_time": 1})
	}
	if n.counter("eduworld_rt_ws_pings_total", nil) < 2 {
		t.Fatalf("ws pings = %v", n.counter("eduworld_rt_ws_pings_total", nil))
	}
	// Silence longer than ws_timeout → 4408.
	if code := c.closed(5 * time.Second); code != 4408 {
		t.Fatalf("close %d", code)
	}
}

// T018 gateway side (F090-AC-12): ew.cmd.game.session.heartbeat every ws_heartbeat with the Ew-* headers; a reply
// CONFLICT (session ended or replaced) closes 4409.
func TestGameHeartbeat(t *testing.T) {
	e := newEnv(t)
	n := e.node("gw-1", func(c *config.Config) { c.Heartbeat, c.Timeout = 200*time.Millisecond, 5*time.Second })
	ended := make(chan struct{})
	st := e.stub(rt.SessionHeartbeatModule, rt.SessionHeartbeatType, func(*nats.Msg) rt.Reply {
		select {
		case <-ended:
			return rt.Fail("CONFLICT", false)
		default:
			return rt.OK(nil)
		}
	})
	an := newStudent()
	c := dial(t, n, e.ticket(an))
	c.next("system.session.welcome")
	time.Sleep(700 * time.Millisecond)
	if st.n() < 2 {
		t.Fatalf("heartbeats = %d", st.n())
	}
	if h := st.last().Header; h.Get(rt.HeaderGameSessionID) != an.session.String() || h.Get(rt.HeaderTenantID) != an.tenant.String() {
		t.Fatalf("headers %v", h)
	}
	if string(st.last().Data) != `{"game_session_id":"`+an.session.String()+`"}` {
		t.Fatalf("payload %s", st.last().Data)
	}
	close(ended)
	if code := c.closed(5 * time.Second); code != 4409 {
		t.Fatalf("close %d", code)
	}
}
