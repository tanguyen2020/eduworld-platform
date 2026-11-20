//go:build integration

package integration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/config"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// T032 (F090-AC-27): on shutdown the node reports not ready, refuses new connections, sends system.server.draining
// with reconnect_after_ms, closes with 1012 within the drain window; the client resumes on another node and gets the
// non-droppable messages it had not read.
func TestGracefulShutdown(t *testing.T) {
	e := newEnv(t)
	gw1 := e.node("gw-1", func(c *config.Config) { c.DrainWindow = time.Second })
	gw2 := e.node("gw-2", nil)
	an := newStudent()
	a := dial(t, gw1, e.ticket(an))
	token := a.next("system.session.welcome").field("resume_token").(string)
	e.publish(rt.UserChannel(an.user), e.message("notification.item.created", map[string]any{"n": 1}))
	m1 := a.next("notification.item.created")
	a.stopReading()
	e.publish(rt.UserChannel(an.user), e.message("notification.item.created", map[string]any{"n": 2}))
	e.publish(rt.UserChannel(an.user), e.message("notification.item.created", map[string]any{"n": 3}))
	time.Sleep(200 * time.Millisecond)

	done := make(chan struct{})
	start := time.Now()
	go func() { gw1.shutdown(); close(done) }()
	deadline := time.Now().Add(2 * time.Second)
	for gw1.m.Ready(context.Background()) == nil {
		if time.Now().After(deadline) {
			t.Fatal("readiness still ok while draining")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := dialRaw(t, gw1, e.ticket(newStudent()), dialOpts{}); err == nil {
		t.Fatal("new connection accepted while draining")
	}
	a.mu.Lock()
	close(a.pause)
	a.pause = nil
	a.mu.Unlock()
	d := a.next("system.server.draining")
	if ms, ok := d.field("reconnect_after_ms").(float64); !ok || ms < 0 || ms > 120000 {
		t.Fatalf("draining %s", d.Payload)
	}
	if code := a.closed(5 * time.Second); code != websocket.StatusServiceRestart {
		t.Fatalf("close %d", code)
	}
	<-done
	if time.Since(start) > 30*time.Second {
		t.Fatalf("drain took %v", time.Since(start))
	}
	b := dial(t, gw2, e.ticket(an))
	b.next("system.session.welcome")
	b.send("system.session.resume", newID(), map[string]any{"resume_token": token, "last_seq": m1.Seq})
	for want := 2; want <= 3; want++ {
		if m := b.next("notification.item.created"); m.field("n") != float64(want) {
			t.Fatalf("replay %d: %s", want, m.Payload)
		}
	}
	if r := b.next("system.session.resumed"); r.field("replayed") != float64(2) {
		t.Fatalf("resumed %s", r.Payload)
	}
}

// T033 (F090-AC-28, AC-29; TC-PERF-014): over the accept rate or ws_per_gateway_node a new connection is closed with
// 4429 and a retry time, before its ticket is consumed.
func TestAcceptLimits(t *testing.T) {
	e := newEnv(t)
	n := e.node("gw-1", func(c *config.Config) { c.AcceptRate, c.MaxConnections = 1, 1 })
	first := dial(t, n, e.ticket(newStudent()))
	first.next("system.session.welcome")
	tk := e.ticket(newStudent())
	ws, err := dialRaw(t, n, tk, dialOpts{})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = ws.Read(context.Background())
	var ce websocket.CloseError
	if websocket.CloseStatus(err) != 4429 || !asCloseError(err, &ce) || !strings.HasPrefix(ce.Reason, "retry_after_ms=") {
		t.Fatalf("close %v", err)
	}
	if n, _ := e.vk.Do(context.Background(), e.vk.B().Exists().Key(rt.TicketKey(rt.HashTicket(tk))).Build()).AsInt64(); n != 1 {
		t.Fatal("ticket consumed by a refused connection")
	}
}

func asCloseError(err error, ce *websocket.CloseError) bool { return errors.As(err, ce) }

// T034 (F090-AC-30; TC-PERF-008): when the Valkey Pub/Sub connection is lost the connections get
// system.service.degraded; the gateway reconnects and subscribes every channel again; with Valkey down the node is not
// ready.
func TestValkeyDegraded(t *testing.T) {
	e := newEnv(t)
	n := e.node("gw-1", nil)
	an := newStudent()
	c := dial(t, n, e.ticket(an))
	c.next("system.session.welcome")
	if err := e.vk.Do(context.Background(), e.vk.B().ClientKill().TypePubsub().Build()).Error(); err != nil {
		t.Fatal(err)
	}
	d := c.next("system.service.degraded")
	if d.field("poll_interval_s") == nil {
		t.Fatalf("degraded %s", d.Payload)
	}
	deadline := time.Now().Add(10 * time.Second)
	for n.m.Ready(context.Background()) != nil || e.numsub(rt.UserChannel(an.user)) != 1 {
		if time.Now().After(deadline) {
			t.Fatal("not resubscribed")
		}
		time.Sleep(50 * time.Millisecond)
	}
	e.publish(rt.UserChannel(an.user), e.message("notification.item.created", map[string]any{"n": 1}))
	c.next("notification.item.created")
	// Valkey gone: readiness fails.
	if err := e.vkCtr.Stop(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(10 * time.Second)
	for n.m.Ready(context.Background()) == nil {
		if time.Now().After(deadline) {
			t.Fatal("ready while Valkey is down")
		}
		time.Sleep(50 * time.Millisecond)
	}
}
