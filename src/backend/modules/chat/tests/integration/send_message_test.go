//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// T010 — F115-AC-01, AC-02, AC-14, AC-15 (TC-CHAT-025, TC-CHAT-004): An sends 20 texts over WS; each acked reply and
// chat.message.acked come after the JetStream ack with seq 1..20; Bình (member) receives chat.message.created in seq
// order with nickname/avatar only; held messages go to the sender only; chat-persister writes every message once,
// raises last_seq and writes chat.message_created (no content) to the outbox.
func TestSendAndReceive(t *testing.T) {
	c := setupChat(t, chatOpts{})
	an, binh := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA2) // a cross-school party (F115-AC-14)
	ch := c.open(channel.Party, an, binh)
	fan := channelInbox(t, c.vk, ch)
	anInbox := inbox(t, c.vk, an.id)
	time.Sleep(200 * time.Millisecond)

	for i := 1; i <= 20; i++ {
		if i == 11 {
			c.clock.Add(61 * time.Second) // party rate: 12 per minute (the window follows the clock of the use case)
		}
		r := c.sendWS(an, "conn-an", ch, "tin so "+uuid.NewString()[:4])
		if !r.OK {
			t.Fatalf("send %d: %+v", i, r.Problem)
		}
		res := map[string]any{}
		_ = jsonUnmarshal(r.Result, &res)
		if res["seq"] != float64(i) || res["message_id"] == nil || res["moderation_state"] != "allowed" || res["client_message_id"] == nil {
			t.Fatalf("acked result %d: %v", i, res)
		}
		if info, err := c.js.Stream(context.Background(), "CHAT_MESSAGES"); err != nil || info.CachedInfo().State.Msgs != uint64(i) {
			t.Fatalf("stream after %d sends: %v", i, err)
		}
	}
	for i := 1; i <= 20; i++ {
		env := next(fan, "chat.message.created", 3*time.Second)
		if env == nil {
			t.Fatalf("created %d not fanned out", i)
		}
		p := payloadOf(env)
		sender, _ := p["sender"].(map[string]any)
		if p["seq"] != float64(i) || sender["nickname"] != an.nickname || sender["user_id"] != an.id.String() {
			t.Fatalf("created %d: %v", i, p)
		}
		for _, k := range []string{"real_name", "school_id", "class_id", "full_name"} { // R16, F115-AC-14
			if _, ok := sender[k]; ok {
				t.Fatalf("sender card carries %s", k)
			}
		}
		if env.Target == nil || !containsID(env.Target.UserIDs, binh.id) || !containsID(env.Target.UserIDs, an.id) {
			t.Fatalf("recipients: %+v", env.Target)
		}
	}
	acked := 0
	for next(anInbox, "chat.message.acked", 500*time.Millisecond) != nil {
		acked++
	}
	if acked != 20 {
		t.Fatalf("chat.message.acked received %d, want 20", acked)
	}
	// hold_for_review: acked (Sent) to the sender, chat.message.held only to the sender, never fanned out (F115-AC-02).
	c.validator.set(message.HoldForReview, "")
	r := c.sendWS(an, "conn-an", ch, "can kiem tra")
	if !r.OK {
		t.Fatalf("held send: %+v", r.Problem)
	}
	if env := next(anInbox, "chat.message.held", 2*time.Second); env == nil || env.Target == nil || env.Target.ConnID != "conn-an" {
		t.Fatalf("held not pushed to the sending connection: %+v", env)
	}
	if env := next(fan, "chat.message.created", 400*time.Millisecond); env != nil {
		t.Fatalf("held message fanned out: %s", env.Message.Payload)
	}
	c.validator.set(message.Allow, "")

	// chat-persister: 21 rows (20 + held), last_seq 21, one chat.message_created per row without content.
	if n := c.waitCount(21, 10*time.Second, `SELECT count(*) FROM chat.gameplay_messages WHERE channel_id = $1`, ch); n != 21 {
		t.Fatalf("persisted %d rows", n)
	}
	if n := c.count(`SELECT count(*) FROM chat.gameplay_messages WHERE channel_id = $1 AND moderation_state = 'held'`, ch); n != 1 {
		t.Fatalf("held rows %d", n)
	}
	if n := c.count(`SELECT last_seq FROM chat.channels WHERE channel_id = $1`, ch); n != 21 {
		t.Fatalf("last_seq %d", n)
	}
	if n := c.count(`SELECT count(DISTINCT seq) FROM chat.gameplay_messages WHERE channel_id = $1`, ch); n != 21 {
		t.Fatalf("distinct seq %d", n)
	}
	if n := c.count(`SELECT count(*) FROM chat.gameplay_partition_channels WHERE channel_id = $1`, ch); n != 1 {
		t.Fatalf("partition channel rows %d", n)
	}
	evs := c.events("chat.message_created")
	if len(evs) != 21 {
		t.Fatalf("chat.message_created events %d", len(evs))
	}
	for _, p := range evs {
		if _, ok := p["body"]; ok || p["channel_id"] != ch.String() || p["kind"] != "text" {
			t.Fatalf("event payload %v", p)
		}
	}
	checkMessageCreatedSchema(t, c)
	// REST reads the same history (F115-AC-18): newest page, seq order, display content only.
	page := c.call(binh, http.MethodGet, "/chat/channels/"+ch.String()+"/messages?limit=5", "", nil)
	if page.status != http.StatusOK || len(page.data()) != 5 || page.data()[0]["seq"] != float64(16) || page.data()[4]["seq"] != float64(20) {
		t.Fatalf("history: %d %s", page.status, page.raw)
	}
	if more, _ := page.body["page"].(map[string]any)["has_more"].(bool); !more {
		t.Fatalf("has_more: %s", page.raw)
	}
	// Bình never sees An's held message; An does.
	if all := c.call(binh, http.MethodGet, "/chat/channels/"+ch.String()+"/messages?after_seq=19", "", nil); len(all.data()) != 1 {
		t.Fatalf("binh after 19: %s", all.raw)
	}
	if all := c.call(an, http.MethodGet, "/chat/channels/"+ch.String()+"/messages?after_seq=19", "", nil); len(all.data()) != 2 ||
		all.data()[1]["moderation_state"] != "held" {
		t.Fatalf("an after 19: %s", all.raw)
	}
}

func containsID(ids []uuid.UUID, id uuid.UUID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// T012 — F115-AC-04, AC-05 (TC-CHAT-026): the same client_message_id (WS) or Idempotency-Key (REST) returns the same
// message_id and seq and creates no second message; REST without Idempotency-Key is 428; REST and WS share the use case
// (same seq sequence, same pipeline).
func TestSendIdempotency(t *testing.T) {
	c := setupChat(t, chatOpts{})
	an, binh := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	ch := c.open(channel.Party, an, binh)
	cmid := uuid.New()
	p := map[string]any{"channel_id": ch, "client_message_id": cmid, "kind": "text", "body": "mot lan"}
	r1, r2 := c.ws(an, "c1", "chat.message.send", p), c.ws(an, "c1", "chat.message.send", p)
	a, b := map[string]any{}, map[string]any{}
	_ = jsonUnmarshal(r1.Result, &a)
	_ = jsonUnmarshal(r2.Result, &b)
	if !r1.OK || !r2.OK || a["message_id"] != b["message_id"] || a["seq"] != b["seq"] || a["seq"] != float64(1) {
		t.Fatalf("WS retry: %v / %v", a, b)
	}
	// REST: same Idempotency-Key and body → same response (replayed); a second key with the same client_message_id →
	// same message as well (client_message_id dedupes across transports of the same sender).
	key := newKey()
	body := map[string]any{"client_message_id": uuid.New(), "kind": "text", "body": "rest"}
	x := c.call(an, http.MethodPost, "/chat/channels/"+ch.String()+"/messages", key, body)
	y := c.call(an, http.MethodPost, "/chat/channels/"+ch.String()+"/messages", key, body)
	if x.status != http.StatusCreated || y.status != http.StatusCreated || x.body["message_id"] != y.body["message_id"] ||
		x.body["seq"] != float64(2) || y.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("REST retry: %d %s / %d %s %v", x.status, x.raw, y.status, y.raw, y.header)
	}
	z := c.call(an, http.MethodPost, "/chat/channels/"+ch.String()+"/messages", newKey(), body)
	if z.status != http.StatusCreated || z.body["message_id"] != x.body["message_id"] {
		t.Fatalf("REST same client_message_id: %d %s", z.status, z.raw)
	}
	if r := c.call(an, http.MethodPost, "/chat/channels/"+ch.String()+"/messages", "", body); r.status != http.StatusPreconditionRequired ||
		r.code() != "IDEMPOTENCY_KEY_REQUIRED" {
		t.Fatalf("no key: %d %s", r.status, r.raw)
	}
	// Both transports went through the same validator: 2 pipeline calls (retries are answered from the record).
	if c.validator.count() != 2 {
		t.Fatalf("validator calls %d", c.validator.count())
	}
	if n := c.waitCount(2, 10*time.Second, `SELECT count(*) FROM chat.gameplay_messages WHERE channel_id = $1`, ch); n != 2 {
		t.Fatalf("rows %d", n)
	}
	if info, _ := c.js.Stream(context.Background(), "CHAT_MESSAGES"); info.CachedInfo().State.Msgs != 2 {
		t.Fatalf("stream holds %d messages", info.CachedInfo().State.Msgs)
	}
	// The REST body is a Message: PlayerCard sender, display body, no original content.
	if s, _ := x.body["sender"].(map[string]any); s["nickname"] != an.nickname || x.body["body"] != "rest" {
		t.Fatalf("REST message: %s", x.raw)
	}
}

// T030 — F115-AC-18, AC-19 (TC-CHAT-028): history merges PostgreSQL with the recent buffer (messages not persisted yet),
// starts at joined_at; chat.channel.sync pushes the missing messages (≤ chat_recent_buffer) as chat.history.loaded to
// the asking connection; chat.channel.join with since_seq does the same and subscribes the fan-out channel.
func TestHistoryAndSync(t *testing.T) {
	c := setupChat(t, chatOpts{noPersister: true}) // nothing reaches PostgreSQL: the recent buffer serves
	an, binh := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	ch := c.open(channel.Party, an, binh)
	for i := 0; i < 5; i++ {
		if r := c.sendWS(an, "c1", ch, "hello"); !r.OK {
			t.Fatalf("send: %+v", r.Problem)
		}
	}
	page := c.call(binh, http.MethodGet, "/chat/channels/"+ch.String()+"/messages?after_seq=2", "", nil)
	if len(page.data()) != 3 || page.data()[0]["seq"] != float64(3) {
		t.Fatalf("recent-only history: %s", page.raw)
	}
	bInbox := inbox(t, c.vk, binh.id)
	time.Sleep(100 * time.Millisecond)
	r := c.ws(binh, "conn-b", "chat.channel.sync", map[string]any{"channels": map[string]int{ch.String(): 2}})
	if !r.OK {
		t.Fatalf("sync: %+v", r.Problem)
	}
	env := next(bInbox, "chat.history.loaded", 2*time.Second)
	if env == nil || env.Target == nil || env.Target.ConnID != "conn-b" {
		t.Fatalf("history.loaded: %+v", env)
	}
	p := payloadOf(env)
	msgs, _ := p["messages"].([]any)
	if len(msgs) != 3 || p["has_more"] != false || msgs[0].(map[string]any)["seq"] != float64(3) {
		t.Fatalf("sync payload: %v", p)
	}
	// join with since_seq: subscribe + missing messages.
	r = c.ws(binh, "conn-b", "chat.channel.join", map[string]any{"channel_id": ch, "since_seq": 4})
	if !r.OK || len(r.Subscribe) != 1 || r.Subscribe[0] != rt.ChatFanoutChannel(ch.String()) {
		t.Fatalf("join: %+v", r)
	}
	if env := next(bInbox, "chat.history.loaded", 2*time.Second); env == nil || len(payloadOf(env)["messages"].([]any)) != 1 {
		t.Fatalf("join history: %+v", env)
	}
	// A member who joins later does not read messages before joined_at.
	c.clock.Add(time.Minute)
	chi := c.student(c.tenantA, c.schoolA)
	if err := c.channels.AddMember(context.Background(), c.tenantA, ch, chi.id); err != nil {
		t.Fatal(err)
	}
	if page := c.call(chi, http.MethodGet, "/chat/channels/"+ch.String()+"/messages", "", nil); page.status != http.StatusOK ||
		len(page.data()) != 0 {
		t.Fatalf("late member history: %d %s", page.status, page.raw)
	}
	// More than chat_recent_buffer → has_more (the client loads the rest over REST).
	small := c.reads
	small.Config.RecentBuffer = 2
	res, err := small.Sync(context.Background(), query.Viewer{TenantID: binh.tenant, UserID: binh.id, ConnID: "conn-b"},
		map[uuid.UUID]int64{ch: 0})
	if err != nil || len(res) != 1 || !res[0].HasMore || res[0].Count != 2 {
		t.Fatalf("sync over the buffer: %+v %v", res, err)
	}
	if env := next(bInbox, "chat.history.loaded", 2*time.Second); env == nil || payloadOf(env)["has_more"] != true {
		t.Fatalf("has_more: %+v", env)
	}
	// A channel the caller is not a member of is reported NOT_FOUND without any message (F115-AC-10).
	r = c.ws(chi, "conn-c", "chat.channel.sync", map[string]any{"channels": map[string]int{uuid.NewString(): 0}})
	if !r.OK || !strings.Contains(string(r.Result), "NOT_FOUND") {
		t.Fatalf("sync of an unknown channel: %+v %s", r.Problem, r.Result)
	}
}
