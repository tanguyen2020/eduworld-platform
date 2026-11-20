//go:build integration

package integration

import (
	"strconv"
	"testing"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/resume"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// T019 (F090-AC-13, AC-14; TC-AUTH-047): seq increases for every server message; after a network cut the client
// reconnects on another node with a new ticket, sends system.session.resume and receives, in order and without
// duplicates, every non-droppable message with seq > last_seq, its topics back, then system.session.resumed; seq
// continues on the new connection.
func TestResume(t *testing.T) {
	e := newEnv(t)
	gw1 := e.node("gw-1", nil)
	gw2 := e.node("gw-2", nil)
	ch := newID()
	e.stub("chat", "chat.channel.join", func(*nats.Msg) rt.Reply {
		return rt.Reply{OK: true, Subscribe: []string{rt.ChatFanoutChannel(ch)}}
	})
	an := newStudent()
	a := dial(t, gw1, e.ticket(an))
	w := a.next("system.session.welcome")
	token := w.field("resume_token").(string)
	a.send("chat.channel.join", newID(), map[string]any{"channel_id": ch})
	ack := a.next("system.command.acked")
	user := rt.UserChannel(an.user)
	e.publish(user, e.message("notification.item.created", map[string]any{"n": 1}))
	m1 := a.next("notification.item.created")
	if w.Seq >= ack.Seq || ack.Seq >= m1.Seq {
		t.Fatalf("seq not increasing: %d %d %d", w.Seq, ack.Seq, m1.Seq)
	}
	// Network cut: the client stops reading, the server keeps sending (and buffering) three messages and a droppable one.
	a.stopReading()
	snap := e.message("hub.snapshot.updated", map[string]any{"tick": 1})
	snap.Droppable = true
	for i := 2; i <= 4; i++ {
		e.publish(user, e.message("notification.item.created", map[string]any{"n": i}))
		if i == 3 {
			e.publish(user, snap)
		}
	}
	e.publish(rt.ChatFanoutChannel(ch), e.message("chat.message.created", map[string]any{"message_id": "m-5"}))
	time.Sleep(300 * time.Millisecond)

	b := dial(t, gw2, e.ticket(an))
	b.next("system.session.welcome")
	rid := newID()
	b.send("system.session.resume", rid, map[string]any{"resume_token": token, "last_seq": m1.Seq})
	var got []msg
	for len(got) < 4 {
		select {
		case m := <-b.msgs:
			if m.Type == "system.session.resumed" {
				t.Fatalf("resumed after %d replayed messages", len(got))
			}
			if m.Type == "notification.item.created" || m.Type == "chat.message.created" || m.Type == "hub.snapshot.updated" {
				got = append(got, m)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("replayed %d", len(got))
		}
	}
	for i, m := range got {
		if m.Type == "hub.snapshot.updated" {
			t.Fatal("droppable message replayed")
		}
		if i > 0 && m.Seq <= got[i-1].Seq || m.Seq <= m1.Seq {
			t.Fatalf("replay order: %v", got)
		}
	}
	if got[0].field("n") != float64(2) || got[3].Type != "chat.message.created" {
		t.Fatalf("replay content: %s %s", got[0].Payload, got[3].Payload)
	}
	r := b.next("system.session.resumed")
	if r.Ref != rid || r.field("replayed") != float64(4) || r.Seq <= got[3].Seq {
		t.Fatalf("resumed %+v %s", r, r.Payload)
	}
	if v := gw2.counter("eduworld_rt_resume_total", map[string]string{"result": "resumed"}); v != 1 {
		t.Fatalf("resume metric %v", v)
	}
	// Topics restored on the new node, seq continues.
	e.waitNumsub(rt.ChatFanoutChannel(ch), 1)
	e.publish(rt.ChatFanoutChannel(ch), e.message("chat.message.created", map[string]any{"message_id": "m-6"}))
	if m := b.next("chat.message.created"); m.Seq <= r.Seq || m.field("message_id") != "m-6" {
		t.Fatalf("after resume: %d %s", m.Seq, m.Payload)
	}
}

// T019 (F090-AC-15): an expired, unknown, other student's or trimmed buffer answers resume_failed with the reason and
// discloses nothing.
func TestResumeFailed(t *testing.T) {
	e := newEnv(t)
	n := e.node("gw-1", func(c *config.Config) { c.ResumeMax = 2 })
	an, binh := newStudent(), newStudent()
	a := dial(t, n, e.ticket(an))
	tokenA := a.next("system.session.welcome").field("resume_token").(string)
	for i := 0; i < 4; i++ {
		e.publish(rt.UserChannel(an.user), e.message("notification.item.created", map[string]any{"n": i}))
		a.next("notification.item.created")
	}
	time.Sleep(200 * time.Millisecond)
	b := dial(t, n, e.ticket(binh))
	b.next("system.session.welcome")
	expired := strconv.FormatInt(time.Now().Add(-10*time.Minute).UnixMilli(), 36) + ".AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	unknown, _ := resume.NewToken(time.Now())
	for token, want := range map[string]string{tokenA: "unknown_token", expired: "expired", unknown: "unknown_token"} {
		id := newID()
		b.send("system.session.resume", id, map[string]any{"resume_token": token, "last_seq": 0})
		f := b.next("system.session.resume_failed")
		if f.Ref != id || f.field("reason") != want {
			t.Fatalf("%s: %s", want, f.Payload)
		}
		b.none("notification.item.created", 100*time.Millisecond)
	}
	// Buffer of 2 messages: resuming from seq 1 misses trimmed messages → seq_gap.
	c := dial(t, n, e.ticket(an))
	c.next("system.session.welcome")
	c.send("system.session.resume", newID(), map[string]any{"resume_token": tokenA, "last_seq": 1})
	if f := c.next("system.session.resume_failed"); f.field("reason") != "seq_gap" {
		t.Fatalf("seq gap: %s", f.Payload)
	}
}

// T019 (F090-AC-16): a command sent again with the same id in resume_window gets the same answer and is not
// forwarded twice, also on another connection.
func TestCommandDedupe(t *testing.T) {
	e := newEnv(t)
	n := e.node("gw-1", nil)
	st := e.stub("chat", "chat.message.send", func(*nats.Msg) rt.Reply {
		return rt.OK(map[string]any{"message_id": "m-1"})
	})
	fail := e.stub("chat", "chat.pin.add", func(*nats.Msg) rt.Reply { return rt.Fail("CHAT_PIN_LIMIT_REACHED", false) })
	an := newStudent()
	c := dial(t, n, e.ticket(an))
	c.next("system.session.welcome")
	id := newID()
	ch := newID()
	body := map[string]any{"channel_id": ch, "client_msg_id": newID(), "kind": "text", "body": "xin chao"}
	c.send("chat.message.send", id, body)
	first := c.next("system.command.acked")
	c.send("chat.message.send", id, body)
	second := c.next("system.command.acked")
	if first.Ref != id || second.Ref != id || string(first.Payload) != string(second.Payload) || st.n() != 1 {
		t.Fatalf("dedupe: %s / %s, calls %d", first.Payload, second.Payload, st.n())
	}
	fid, mid := newID(), newID()
	c.send("chat.pin.add", fid, map[string]any{"channel_id": ch, "message_id": mid})
	f1 := c.next("system.command.failed")
	d := dial(t, n, e.ticket(an)) // same student, new connection
	d.next("system.session.welcome")
	d.send("chat.pin.add", fid, map[string]any{"channel_id": ch, "message_id": mid})
	f2 := d.next("system.command.failed")
	if f1.field("code") != "CHAT_PIN_LIMIT_REACHED" || string(f1.Payload) != string(f2.Payload) || fail.n() != 1 {
		t.Fatalf("failed dedupe: %s / %s, calls %d", f1.Payload, f2.Payload, fail.n())
	}
}
