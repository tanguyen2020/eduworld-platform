//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/nats-io/nats.go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/config"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// T011 (F090-AC-05, AC-06, AC-07): the ticket is consumed atomically and the welcome carries every field; a reused,
// unknown or JWT-shaped ticket closes 4401 with eduworld_rt_ticket_rejected_total{reason}; no subprotocol, an Origin
// outside the allowlist or a credential in the URL refuse the handshake; the logs never contain the ticket, the
// resume token or a payload.
func TestHandshake(t *testing.T) {
	e := newEnv(t)
	n := e.node("gw-1", func(c *config.Config) { c.AllowedOrigins = []string{"https://portal.test.example"} })
	an := newStudent()
	tk := e.ticket(an)
	c := dial(t, n, tk)
	w := c.next("system.session.welcome")
	if w.Seq != 1 || w.V != 1 {
		t.Fatalf("welcome envelope: %+v", w)
	}
	// MONITORING §3: one ok handshake, the student online with the platform and grade band of the ticket.
	if v := n.counter("eduworld_ws_connect_total", map[string]string{"result": "ok"}); v != 1 {
		t.Fatalf("ws_connect_total{ok} = %v", v)
	}
	if v := n.counter("eduworld_players_online", map[string]string{"platform": "windows", "grade_band": "g4_5"}); v != 1 {
		t.Fatalf("players_online = %v", v)
	}
	for _, k := range []string{"conn_id", "server_time", "heartbeat_s", "timeout_s", "resume_token", "resume_window_s", "protocol_version", "limits"} {
		if w.field(k) == nil {
			t.Fatalf("welcome without %s: %s", k, w.Payload)
		}
	}
	if w.field("heartbeat_s") != float64(15) || w.field("timeout_s") != float64(45) || w.field("resume_window_s") != float64(120) ||
		w.field("protocol_version") != float64(1) || w.field("game_session_id") != an.session.String() {
		t.Fatalf("welcome values: %s", w.Payload)
	}
	limits := w.field("limits").(map[string]any)
	if limits["max_frame_bytes"] != float64(16384) {
		t.Fatalf("limits %v", limits)
	}
	if n, _ := e.vk.Do(context.Background(), e.vk.B().Exists().Key(rt.TicketKey(rt.HashTicket(tk))).Build()).AsInt64(); n != 0 {
		t.Fatal("ticket still stored after the handshake")
	}
	// The connection is bound to the ticket identity: a command carries it (TC-TEN-012 in single_connection_test).
	resumeToken := w.field("resume_token").(string)

	// Reused ticket and unknown ticket → 4401.
	for name, ticket := range map[string]string{"reused": tk, "unknown": "dW5rbm93bi10aWNrZXQtdmFsdWUtMTIzNDU2Nzg5MDEy",
		"jwt": "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ4In0.c2ln"} {
		ws, err := dialRaw(t, n, ticket, dialOpts{})
		if err != nil {
			t.Fatalf("%s: handshake refused instead of 4401: %v", name, err)
		}
		_, _, err = ws.Read(context.Background())
		if websocket.CloseStatus(err) != 4401 {
			t.Fatalf("%s: close %v", name, err)
		}
	}
	if v := n.counter("eduworld_rt_ticket_rejected_total", map[string]string{"reason": "unknown"}); v != 2 {
		t.Fatalf("rejected{unknown} = %v", v)
	}
	if v := n.counter("eduworld_rt_ticket_rejected_total", map[string]string{"reason": "malformed"}); v != 1 {
		t.Fatalf("rejected{malformed} = %v", v)
	}
	// Handshake refused: no subprotocol, Origin not allowed, JWT in the query string.
	if _, err := dialRaw(t, n, e.ticket(newStudent()), dialOpts{noProtocol: true}); err == nil {
		t.Fatal("handshake without eduworld.v1 accepted")
	}
	if _, err := dialRaw(t, n, e.ticket(newStudent()), dialOpts{origin: "https://evil.example"}); err == nil {
		t.Fatal("foreign Origin accepted")
	}
	if _, err := dialRaw(t, n, "", dialOpts{rawQuery: "access_token=eyJhbGciOiJSUzI1NiJ9.e30.x"}); err == nil {
		t.Fatal("JWT in query accepted")
	}
	// ws_connect_total: reused, unknown, JWT-shaped ticket and JWT in the URL are ticket_invalid; subprotocol and Origin rejected.
	if v := n.counter("eduworld_ws_connect_total", map[string]string{"result": "ticket_invalid"}); v != 4 {
		t.Fatalf("ws_connect_total{ticket_invalid} = %v", v)
	}
	if v := n.counter("eduworld_ws_connect_total", map[string]string{"result": "rejected"}); v != 2 {
		t.Fatalf("ws_connect_total{rejected} = %v", v)
	}
	web := newStudent()
	if ws, err := dialRaw(t, n, e.ticket(web), dialOpts{origin: "https://portal.test.example"}); err != nil {
		t.Fatalf("allowed Origin refused: %v", err)
	} else {
		_ = ws.CloseNow()
	}
	// A command with a recognizable payload, then scan the logs (R14).
	e.stub("chat", "chat.channel.join", func(*nats.Msg) rt.Reply { return rt.OK(nil) })
	marker := newID() // a payload value that must never reach the logs
	c.send("chat.channel.join", newID(), map[string]any{"channel_id": marker})
	c.next("system.command.acked")
	time.Sleep(100 * time.Millisecond)
	logs := e.logs.String()
	for _, secret := range []string{tk, resumeToken, marker, "eyJhbGciOiJSUzI1NiJ9"} {
		if strings.Contains(logs, secret) {
			t.Fatalf("log contains a secret or payload: %q", secret)
		}
	}
}
