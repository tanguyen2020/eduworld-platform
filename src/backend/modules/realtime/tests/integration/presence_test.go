//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/presence"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// presenceSignals subscribes ew.rt.presence.<tenant> and returns the decoded signals.
func (e *env) presenceSignals(s student) chan rt.PresenceChanged {
	e.t.Helper()
	out := make(chan rt.PresenceChanged, 32)
	sub, err := e.nc.Subscribe(rt.PresenceSubject(s.tenant), func(m *nats.Msg) {
		var p rt.PresenceChanged
		if json.Unmarshal(m.Data, &p) == nil {
			out <- p
		}
	})
	if err != nil {
		e.t.Fatal(err)
	}
	_ = e.nc.Flush()
	e.t.Cleanup(func() { _ = sub.Unsubscribe() })
	return out
}

func nextSignal(t *testing.T, ch chan rt.PresenceChanged, status string, within time.Duration) rt.PresenceChanged {
	t.Helper()
	timeout := time.After(within)
	for {
		select {
		case p := <-ch:
			if p.Status == status {
				return p
			}
		case <-timeout:
			t.Fatalf("no presence signal %s within %v", status, within)
		}
	}
}

func (e *env) presenceHash(s student) map[string]string {
	e.t.Helper()
	m, err := e.vk.Do(context.Background(), e.vk.B().Hgetall().Key("ew:gateway:presence:user:"+s.user.String()).Build()).AsStrMap()
	if err != nil {
		e.t.Fatal(err)
	}
	return m
}

// T027 (F091-AC-21, AC-22, AC-24, AC-25): a connection marks the student online with TTL presence_ttl and one
// signal on ew.rt.presence.<tenant> (ids and status only); presence.status.set changes status/activity/hidden and is
// acked, a value outside the enum (or offline) is VALIDATION_FAILED; a 4408 close goes away and then offline when no
// connection comes back within ws_timeout; a normal close goes offline at once; the control presence.update of the
// room host sets area/room and is never delivered to the client.
func TestPresence(t *testing.T) {
	e := newEnv(t)
	n := e.node("gw-1", func(c *config.Config) { c.Heartbeat, c.Timeout = 300*time.Millisecond, 1500*time.Millisecond })
	an := newStudent()
	signals := e.presenceSignals(an)
	c := dial(t, n, e.ticket(an))
	c.next("system.session.welcome")
	stopPing := keepAlive(c)

	on := nextSignal(t, signals, rt.PresenceOnline, 5*time.Second)
	if on.UserID != an.user || on.TenantID != an.tenant || on.SchoolID == nil || *on.SchoolID != an.school || on.Reason != rt.PresenceConnected {
		t.Fatalf("online signal %+v", on)
	}
	if h := e.presenceHash(an); h["status"] != "online" {
		t.Fatalf("presence %v", h)
	}
	ttl, _ := e.vk.Do(context.Background(), e.vk.B().Ttl().Key("ew:gateway:presence:user:"+an.user.String()).Build()).AsInt64()
	if ttl <= 0 || ttl > 90 {
		t.Fatalf("presence ttl %d", ttl)
	}
	if n, _ := e.vk.Do(context.Background(), e.vk.B().Zscore().Key("ew:gateway:presence:tenant:"+an.tenant.String()+":online").Member(an.user.String()).Build()).AsFloat64(); n <= 0 {
		t.Fatal("not in the tenant online index")
	}

	// presence.status.set (F091-AC-22).
	id := newID()
	c.send("presence.status.set", id, map[string]any{"status": "in_activity", "activity": "learning", "hidden": true})
	if a := c.next("system.command.acked"); a.Ref != id {
		t.Fatalf("ack %+v", a)
	}
	set := nextSignal(t, signals, rt.PresenceInActivity, 5*time.Second)
	if set.Activity != "learning" || !set.Hidden || set.Reason != rt.PresenceStatusSet {
		t.Fatalf("status set signal %+v", set)
	}
	for _, bad := range []map[string]any{{"status": "offline"}, {"status": "busy"}, {"status": "online", "activity": "sleeping"}} {
		bid := newID()
		c.send("presence.status.set", bid, bad)
		if f := c.next("system.command.failed"); f.Ref != bid || f.field("code") != "VALIDATION_FAILED" {
			t.Fatalf("%v: %+v %s", bad, f, f.Payload)
		}
	}
	if h := e.presenceHash(an); h["status"] != "in_activity" || h["hidden"] != "1" {
		t.Fatalf("presence after refused values %v", h)
	}

	// Control presence.update of the room host (F091-AC-23 gateway side): recorded, not delivered (F091-AC-25).
	upd, err := rt.PresenceUpdate("main_hub", "room-1")
	if err != nil {
		t.Fatal(err)
	}
	e.publish(rt.UserChannel(an.user), upd)
	area := nextSignal(t, signals, rt.PresenceInActivity, 5*time.Second)
	if area.AreaID != "main_hub" || area.RoomID != "room-1" || area.Reason != rt.PresenceAreaChanged {
		t.Fatalf("area signal %+v", area)
	}
	c.none("presence.friend.updated", 300*time.Millisecond)

	// 4408 (no frame within ws_timeout) → away, then offline after ws_timeout without a new connection (F091-AC-24).
	stopPing()
	c.stopReading()
	away := nextSignal(t, signals, rt.PresenceAway, 10*time.Second)
	if away.Reason != rt.PresenceTimeout {
		t.Fatalf("away signal %+v", away)
	}
	off := nextSignal(t, signals, rt.PresenceOffline, 10*time.Second)
	if off.Reason != rt.PresenceTimeout {
		t.Fatalf("offline signal %+v", off)
	}
	if h := e.presenceHash(an); h["status"] != "offline" {
		t.Fatalf("presence %v", h)
	}

	// A new connection within ws_timeout keeps the student online (no offline after the away).
	binh := newStudent()
	bs := e.presenceSignals(binh)
	b1 := dial(t, n, e.ticket(binh))
	b1.next("system.session.welcome")
	nextSignal(t, bs, rt.PresenceOnline, 5*time.Second)
	b1.stopReading()
	nextSignal(t, bs, rt.PresenceAway, 10*time.Second)
	b2 := dial(t, n, e.ticket(binh))
	b2.next("system.session.welcome")
	nextSignal(t, bs, rt.PresenceOnline, 5*time.Second)
	defer keepAlive(b2)()
	select {
	case p := <-bs:
		if p.Status == rt.PresenceOffline {
			t.Fatalf("offline although reconnected: %+v", p)
		}
	case <-time.After(2500 * time.Millisecond):
	}
	if h := e.presenceHash(binh); h["status"] != "online" {
		t.Fatalf("reconnected presence %v", h)
	}

	// A normal close by the client goes offline at once (F091-AC-21).
	chi := newStudent()
	cs := e.presenceSignals(chi)
	cc := dial(t, n, e.ticket(chi))
	cc.next("system.session.welcome")
	nextSignal(t, cs, rt.PresenceOnline, 5*time.Second)
	_ = cc.ws.Close(1000, "bye")
	if p := nextSignal(t, cs, rt.PresenceOffline, 5*time.Second); p.Reason != rt.PresenceDisconnected {
		t.Fatalf("offline signal %+v", p)
	}
}

// keepAlive sends a heartbeat ping every 200 ms until the returned stop is called.
func keepAlive(c *client) func() {
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(200 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				b, _ := json.Marshal(map[string]any{"v": 1, "type": "system.heartbeat.ping", "id": newID(), "ts": time.Now().UnixMilli(),
					"payload": map[string]any{"client_time": time.Now().UnixMilli()}})
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				_ = c.ws.Write(ctx, 1, b)
				cancel()
			}
		}
	}()
	stopped := false
	return func() {
		if !stopped {
			stopped = true
			close(done)
		}
	}
}

// F091-AC-14 (gateway side): a snapshot bundle published on ew:rt:room:<id>:out gives each local connection only the
// payload of its own user, as a droppable message of the bundle type; a connection without an item gets nothing.
func TestRoomSnapshotBundle(t *testing.T) {
	e := newEnv(t)
	n := e.node("gw-1", nil)
	room := "0193a5f4-6c1e-7b2a-9d1f-00000000abcd"
	e.stub("competition", "hub.instance.join", func(*nats.Msg) rt.Reply {
		r := rt.OK(map[string]any{"room_id": room})
		r.Subscribe = []string{rt.RoomOutChannel(room)}
		return r
	})
	an, binh, chi := newStudent(), newStudent(), newStudent()
	clients := map[string]*client{}
	for name, s := range map[string]student{"an": an, "binh": binh, "chi": chi} {
		c := dial(t, n, e.ticket(s))
		c.next("system.session.welcome")
		c.send("hub.instance.join", newID(), map[string]any{"area_id": "main_hub"})
		c.next("system.command.acked")
		clients[name] = c
	}
	e.waitNumsub(rt.RoomOutChannel(room), 1) // one Valkey subscription for the node, three local receivers
	b := rt.Envelope{V: 1, Kind: rt.KindBundle, Droppable: true, Bundle: &rt.Bundle{Type: "hub.snapshot.updated", Ts: time.Now().UnixMilli(),
		Items: []rt.BundleItem{
			{UserID: an.user, Payload: []byte(`{"room_id":"` + room + `","tick":7,"entities":[{"eid":2,"p":[1,0,1]}]}`)},
			{UserID: binh.user, Payload: []byte(`{"room_id":"` + room + `","tick":7,"entities":[{"eid":1,"p":[0,0,0]}]}`)},
		}}}
	e.publish(rt.RoomOutChannel(room), b)
	for name, want := range map[string]float64{"an": 2, "binh": 1} {
		m := clients[name].next("hub.snapshot.updated")
		var p struct {
			Tick     int `json:"tick"`
			Entities []struct {
				EID float64 `json:"eid"`
			} `json:"entities"`
		}
		_ = json.Unmarshal(m.Payload, &p)
		if p.Tick != 7 || len(p.Entities) != 1 || p.Entities[0].EID != want {
			t.Fatalf("%s got %s", name, m.Payload)
		}
	}
	clients["chi"].none("hub.snapshot.updated", 300*time.Millisecond)
}

// F095-AC-22 (gateway side): while the connection lives, the heartbeat republishes the unchanged presence with reason
// refresh at most every presence_ttl/3, so module chat keeps its copy (TTL presence_ttl) without expiring a student
// who stays connected; an offline student gets no refresh signal.
func TestPresenceRefreshSignal(t *testing.T) {
	e := newEnv(t)
	an := newStudent()
	signals := e.presenceSignals(an)
	svc := presence.New(e.vk, e.nc, 3*time.Second, 45*time.Second, nil, nil)
	c := presence.Caller{UserID: an.user, TenantID: an.tenant, SchoolID: an.school}
	ctx := context.Background()
	if err := svc.Connected(ctx, c); err != nil {
		t.Fatal(err)
	}
	nextSignal(t, signals, rt.PresenceOnline, 2*time.Second)
	refreshes := func(within time.Duration) int {
		n, timeout := 0, time.After(within)
		for {
			select {
			case p := <-signals:
				if p.Reason == rt.PresenceRefresh {
					if p.Status != rt.PresenceOnline || p.UserID != an.user {
						t.Fatalf("refresh signal %+v", p)
					}
					n++
				}
			case <-timeout:
				return n
			}
		}
	}
	for i := 0; i < 3; i++ { // three heartbeats within presence_ttl/3: one refresh
		if err := svc.Refresh(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	if n := refreshes(300 * time.Millisecond); n != 1 {
		t.Fatalf("refresh signals within ttl/3: %d", n)
	}
	time.Sleep(time.Second + 50*time.Millisecond)
	if err := svc.Refresh(ctx, c); err != nil {
		t.Fatal(err)
	}
	if n := refreshes(300 * time.Millisecond); n != 1 {
		t.Fatalf("refresh signals after ttl/3: %d", n)
	}
	if err := svc.Disconnected(ctx, c, 1000); err != nil {
		t.Fatal(err)
	}
	nextSignal(t, signals, rt.PresenceOffline, 2*time.Second)
	if err := svc.Refresh(ctx, c); err != nil {
		t.Fatal(err)
	}
	if n := refreshes(300 * time.Millisecond); n != 0 {
		t.Fatalf("refresh of an offline student: %d", n)
	}
}
