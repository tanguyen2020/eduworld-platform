//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/natsstream"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/persistence"
)

func jetstreamOrdered() jetstream.OrderedConsumerConfig {
	return jetstream.OrderedConsumerConfig{FilterSubjects: []string{"ew.chat.*.msg.*"}}
}

// Set moves the test clock.
func (c *clock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

// T011 — F115-AC-03 (TC-CHAT-026): when the durable stream cannot ack (stream gone, then NATS stopped) every send — REST
// and WS — answers CHAT_TEMPORARILY_UNAVAILABLE (REST 503 + Retry-After, within chat_publish_ack_timeout), nothing is
// fanned out nor written; history keeps working; once the stream is back sending works again.
func TestNATSDown(t *testing.T) {
	// The persister recreates a deleted stream after its backoff: 3 s keeps the outage observable under a loaded CI machine.
	c := setupChat(t, chatOpts{backoff: 3 * time.Second})
	an, binh := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	ch := c.open(channel.Party, an, binh)
	if r := c.sendREST(an, ch, map[string]any{"client_message_id": uuid.New(), "kind": "text", "body": "truoc"}); r.status != http.StatusCreated {
		t.Fatalf("before: %d %s", r.status, r.raw)
	}
	if n := c.waitCount(1, 10*time.Second, `SELECT count(*) FROM chat.gameplay_messages WHERE channel_id = $1`, ch); n != 1 {
		t.Fatalf("first message not persisted: %d", n)
	}
	fan := channelInbox(t, c.vk, ch)
	time.Sleep(100 * time.Millisecond)
	ctx := context.Background()
	unavailable := func(phase string, history int) {
		start := time.Now()
		r := c.sendREST(an, ch, map[string]any{"client_message_id": uuid.New(), "kind": "text", "body": "khong gui duoc"})
		if r.status != http.StatusServiceUnavailable || r.code() != "CHAT_TEMPORARILY_UNAVAILABLE" || r.header.Get("Retry-After") == "" {
			t.Fatalf("%s REST: %d %s %v", phase, r.status, r.raw, r.header)
		}
		if time.Since(start) > 3*time.Second {
			t.Fatalf("%s: send waited %v (chat_publish_ack_timeout 2 s)", phase, time.Since(start))
		}
		if phase == "stream deleted" { // NATS core still answers the gateway: the WS command gets the same code
			if w := c.sendWS(an, "c1", ch, "khong gui duoc"); w.OK || replyCode(w) != "CHAT_TEMPORARILY_UNAVAILABLE" || !w.Problem.Retryable {
				t.Fatalf("%s WS: %+v", phase, w)
			}
		}
		if env := next(fan, "chat.message.created", 500*time.Millisecond); env != nil {
			t.Fatalf("%s: fanned out without ack: %s", phase, env.Message.Payload)
		}
		if h := c.call(binh, http.MethodGet, "/chat/channels/"+ch.String()+"/messages", "", nil); h.status != http.StatusOK || len(h.data()) != history {
			t.Fatalf("%s: history: %d %s", phase, h.status, h.raw)
		}
	}
	// 1. The stream is gone (no ack possible).
	if err := c.js.DeleteStream(ctx, "CHAT_MESSAGES"); err != nil {
		t.Fatal(err)
	}
	unavailable("stream deleted", 1)
	if err := natsstream.EnsureStream(ctx, c.js, natsstream.StreamOptions{MaxAge: 72 * time.Hour}); err != nil {
		t.Fatal(err)
	}
	if r := c.sendREST(an, ch, map[string]any{"client_message_id": uuid.New(), "kind": "text", "body": "sau"}); r.status != http.StatusCreated {
		t.Fatalf("after the stream is back: %d %s", r.status, r.raw)
	}
	if env := next(fan, "chat.message.created", 2*time.Second); env == nil {
		t.Fatal("no fan-out after recovery")
	}
	if n := c.waitCount(2, 15*time.Second, `SELECT count(*) FROM chat.gameplay_messages WHERE channel_id = $1`, ch); n != 2 {
		t.Fatalf("rows %d (refused sends must not be stored)", n)
	}
	// 2. NATS itself stopped.
	if err := c.natsC.Stop(ctx, nil); err != nil {
		t.Fatal(err)
	}
	unavailable("NATS stopped", 2)
	if n := c.count(`SELECT count(*) FROM chat.gameplay_messages WHERE channel_id = $1`, ch); n != 2 {
		t.Fatalf("rows %d after NATS stopped", n)
	}
}

// T029 — F115-AC-16 (TC-CHAT-027): while PostgreSQL is unavailable (TEST DOUBLE outage of the chat path) sending and
// receiving keep working from the cached facts; when it is back chat-persister writes every message once, in seq.
func TestPGDown(t *testing.T) {
	c := setupChat(t, chatOpts{factsFresh: 200 * time.Millisecond})
	an, binh := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	ch := c.open(channel.Class, an, binh)
	send := func(n int) {
		for i := 0; i < n; i++ {
			s := an
			if i%2 == 1 {
				s = binh
			}
			if r := c.sendWS(s, "c", ch, "tin nhan"); !r.OK {
				t.Fatalf("send: %+v", r.Problem)
			}
		}
	}
	send(4) // warms the facts
	if n := c.waitCount(4, 10*time.Second, `SELECT count(*) FROM chat.messages WHERE channel_id = $1`, ch); n != 4 {
		t.Fatalf("before outage %d", n)
	}
	c.outage.set(true)
	time.Sleep(300 * time.Millisecond) // facts are now stale: reloads fail, stale values serve
	fan := channelInbox(t, c.vk, ch)
	time.Sleep(100 * time.Millisecond)
	send(16) // 10/min/channel per sender (class) → 8 each
	if env := next(fan, "chat.message.created", 2*time.Second); env == nil {
		t.Fatal("no realtime delivery during the outage")
	}
	time.Sleep(time.Second)
	if n := c.count(`SELECT count(*) FROM chat.messages WHERE channel_id = $1`, ch); n != 4 {
		t.Fatalf("written during the outage: %d", n)
	}
	// History during the outage: the recent buffer.
	if h := c.call(binh, http.MethodGet, "/chat/channels/"+ch.String()+"/messages?after_seq=4", "", nil); len(h.data()) != 16 {
		t.Fatalf("history during the outage: %s", h.raw)
	}
	c.outage.set(false)
	if n := c.waitCount(20, 20*time.Second, `SELECT count(*) FROM chat.messages WHERE channel_id = $1`, ch); n != 20 {
		t.Fatalf("after the outage %d rows", n)
	}
	if n := c.count(`SELECT count(DISTINCT seq) FROM chat.messages WHERE channel_id = $1 AND seq BETWEEN 1 AND 20`, ch); n != 20 {
		t.Fatalf("distinct seq %d", n)
	}
	if n := c.count(`SELECT count(*) FROM infra.dead_letters WHERE consumer = 'chat.persister'`); n != 0 {
		t.Fatalf("dead letters during an outage: %d", n)
	}
}

// failingStore is a TEST DOUBLE message store failing for one message id (a poison message).
type failingStore struct {
	inner  write.MessageStore
	poison uuid.UUID
}

func (s failingStore) Insert(ctx context.Context, tenant uuid.UUID, es []message.Envelope) (write.PersistResult, error) {
	for _, e := range es {
		if e.MessageID == s.poison {
			return write.PersistResult{}, errPoison
		}
	}
	return s.inner.Insert(ctx, tenant, es)
}

type poisonErr struct{}

func (poisonErr) Error() string { return "TEST DOUBLE: poison message" }

var errPoison = poisonErr{}

// T028 — F115-AC-15, AC-17: a redelivered envelope is written once (ON CONFLICT, last_seq never moves back); a message
// failing more than max_deliver times goes to DLQ_CHAT + infra.dead_letters (no content) without blocking the others;
// an envelope whose channel is not in its tenant is dead-lettered (never written to another tenant).
func TestPersisterIdempotentAndDLQ(t *testing.T) {
	poisonID := uuid.Must(uuid.NewV7())
	c := setupChat(t, chatOpts{maxDeliver: 2, store: failingStore{inner: persistence.Messages{}, poison: poisonID}})
	an, binh := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	ch := c.open(channel.Class, an, binh)
	for i := 0; i < 3; i++ {
		if r := c.sendWS(an, "c", ch, "ok"); !r.OK {
			t.Fatalf("send: %+v", r.Problem)
		}
	}
	if n := c.waitCount(3, 10*time.Second, `SELECT count(*) FROM chat.messages WHERE channel_id = $1`, ch); n != 3 {
		t.Fatalf("rows %d", n)
	}
	// Redelivery of an old envelope (seq 1) and of a copy with an old seq: written once, last_seq stays 3.
	rows := c.call(an, http.MethodGet, "/chat/channels/"+ch.String()+"/messages?after_seq=0&limit=1", "", nil)
	first := rows.data()[0]
	env := message.Envelope{V: 1, Kind: "msg", MessageID: uuid.MustParse(first["message_id"].(string)), TenantID: c.tenantA,
		ChannelID: ch, ChannelType: "class", Seq: 1, Revision: 1, SenderID: an.id, MsgKind: message.KindText,
		Moderation: message.Moderation{Decision: message.Allow, State: message.StateAllowed}, CreatedAt: c.clock.Now().UTC()}
	body := "ok"
	env.BodyDisplay = &body
	if err := c.send.Stream.Publish(context.Background(), env); err != nil { // within duplicate_window: deduped by NATS
		t.Fatal(err)
	}
	env.MessageID = uuid.Must(uuid.NewV7()) // a new message with a taken seq (lost Valkey counter): refused by uniqueness
	if err := c.send.Stream.Publish(context.Background(), env); err != nil {
		t.Fatal(err)
	}
	// Poison message + a good one after it.
	env.MessageID, env.Seq = poisonID, 50
	if err := c.send.Stream.Publish(context.Background(), env); err != nil {
		t.Fatal(err)
	}
	if r := c.sendWS(an, "c", ch, "sau poison"); !r.OK {
		t.Fatalf("send after poison: %+v", r.Problem)
	}
	// Another tenant's envelope pointing at this channel: dead-lettered, never written.
	foreign := env
	foreign.MessageID, foreign.TenantID, foreign.Seq = uuid.Must(uuid.NewV7()), c.tenantB, 60
	if err := c.send.Stream.Publish(context.Background(), foreign); err != nil {
		t.Fatal(err)
	}
	if n := c.waitCount(4, 15*time.Second, `SELECT count(*) FROM chat.messages WHERE channel_id = $1`, ch); n != 4 {
		t.Fatalf("rows %d (the good message after the poison must be written)", n)
	}
	if n := c.waitCount(3, 20*time.Second, `SELECT count(*) FROM infra.dead_letters WHERE consumer = 'chat.persister'`); n != 3 {
		t.Fatalf("dead letters %d (taken seq, poison, foreign tenant)", n)
	}
	if n := c.count(`SELECT count(*) FROM infra.dead_letters WHERE consumer = 'chat.persister' AND payload::text LIKE '%body%'`); n != 0 {
		t.Fatal("a dead letter keeps message content")
	}
	if n := c.count(`SELECT last_seq FROM chat.channels WHERE channel_id = $1`, ch); n != 4 {
		t.Fatalf("last_seq %d", n)
	}
	if n := c.count(`SELECT count(*) FROM chat.messages WHERE tenant_id = $1`, c.tenantB); n != 0 {
		t.Fatal("foreign envelope written")
	}
	dlq, err := c.js.Stream(context.Background(), "DLQ_CHAT")
	if err != nil || dlq.CachedInfo().State.Msgs != 3 {
		t.Fatalf("DLQ_CHAT: %v", err)
	}
	if !strings.Contains(strings.Join(c.audits(), ","), "chat.channel.opened") {
		t.Fatalf("audit of OpenChannel: %v", c.audits())
	}
}

// T031 — F115-AC-20 (TC-CHAT-028): when ew:chat:seq:<channel_id> is lost the counter restarts from
// GREATEST(channels.last_seq, max seq of the recent buffer): no seq is reused.
func TestSeqRecovery(t *testing.T) {
	c := setupChat(t, chatOpts{})
	an, binh := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	ch := c.open(channel.Party, an, binh)
	for i := 0; i < 3; i++ {
		c.sendWS(an, "c", ch, "mot")
	}
	if n := c.waitCount(3, 10*time.Second, `SELECT last_seq FROM chat.channels WHERE channel_id = $1`, ch); n != 3 {
		t.Fatalf("last_seq %d", n)
	}
	ctx := context.Background()
	key := "ew:chat:seq:" + ch.String()
	if err := c.vk.Do(ctx, c.vk.B().Del().Key(key).Build()).Error(); err != nil {
		t.Fatal(err)
	}
	r := c.sendWS(an, "c", ch, "sau khi mat key")
	res := map[string]any{}
	_ = jsonUnmarshal(r.Result, &res)
	if !r.OK || res["seq"] != float64(4) {
		t.Fatalf("seq after loss (PostgreSQL floor): %v %+v", res, r.Problem)
	}
	// Persister lagging: the recent buffer holds a higher seq than channels.last_seq.
	c.exec(`UPDATE chat.channels SET last_seq = 1 WHERE channel_id = $1`, ch)
	if err := c.vk.Do(ctx, c.vk.B().Del().Key(key).Build()).Error(); err != nil {
		t.Fatal(err)
	}
	r = c.sendWS(binh, "c", ch, "van tiep noi")
	res = map[string]any{}
	_ = jsonUnmarshal(r.Result, &res)
	if !r.OK || res["seq"] != float64(5) {
		t.Fatalf("seq after loss (recent buffer floor): %v", res)
	}
	if n := c.waitCount(5, 10*time.Second, `SELECT count(DISTINCT seq) FROM chat.gameplay_messages WHERE channel_id = $1`, ch); n != 5 {
		t.Fatalf("distinct seq %d", n)
	}
}
