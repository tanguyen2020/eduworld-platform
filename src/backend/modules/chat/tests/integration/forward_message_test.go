//go:build integration

package integration

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
)

// T014 — F118-AC-07, AC-08 (TC-CHAT-031): a reply needs an original of the same channel and carries an excerpt (≤ 80) of
// its moderated content; a quote excerpt is cut by the server from the moderated (masked) content — never from the
// client — and goes through the pipeline.
func TestReplyQuote(t *testing.T) {
	f := setupFeatures(t, chatOpts{})
	startFeatures(t, f)
	an, binh := f.student(f.tenantA, f.schoolA), f.student(f.tenantA, f.schoolA)
	ch := f.party(an, binh)
	other := f.party(binh, an)
	f.validator.set(message.Mask, "toi o *** nhe")
	masked := f.sendText(an, ch, "toi o so 12 nhe", nil)
	f.validator.set(message.Allow, "")
	long := f.sendText(an, ch, strings.Repeat("a", 150), nil)
	calls := f.validator.count()
	q := f.sendREST(binh, ch, map[string]any{"client_message_id": uuid.New(), "kind": "text", "body": "that a",
		"quote": map[string]any{"message_id": masked, "excerpt_start": 0, "excerpt_end": 100}})
	if q.status != http.StatusCreated {
		t.Fatalf("quote: %d %s", q.status, q.raw)
	}
	qq, _ := q.body["quote"].(map[string]any)
	if qq == nil || qq["excerpt"] != "toi o *** nhe" || strings.Contains(q.raw, "so 12") {
		t.Fatalf("the excerpt must come from the masked content, not the client: %s", q.raw)
	}
	if f.validator.count() != calls+2 {
		t.Fatalf("body and excerpt must both go through the pipeline: %d calls", f.validator.count()-calls)
	}
	// Reply: excerpt ≤ 80 characters of the moderated original; another channel's message is refused.
	rid := f.sendText(binh, ch, "tra loi", map[string]any{"reply_to_message_id": long})
	f.persisted(rid)
	h := f.call(an, http.MethodGet, "/chat/channels/"+ch.String()+"/messages", "", nil)
	for _, m := range h.data() {
		if m["message_id"] == rid.String() {
			if ex, _ := m["reply_excerpt"].(string); len([]rune(ex)) != message.ReplyExcerptMax {
				t.Fatalf("reply excerpt: %q", ex)
			}
		}
	}
	if r := f.sendREST(binh, other, map[string]any{"client_message_id": uuid.New(), "kind": "text", "body": "x",
		"reply_to_message_id": long}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("reply to a message of another channel: %d %s", r.status, r.raw)
	}
}

// T015 — F118-AC-09, AC-10, AC-11 (TC-CHAT-032): a forward creates a new message in each target channel of the same
// tenant, each through the pipeline and policy of its channel; another tenant, a channel without the right to send, a
// recalled/masked/held source or too many targets is refused; a retry with the same Idempotency-Key replays.
func TestForwardMessage(t *testing.T) {
	f := setupFeatures(t, chatOpts{})
	startFeatures(t, f)
	an, binh, chi := f.student(f.tenantA, f.schoolA), f.student(f.tenantA, f.schoolA), f.student(f.tenantA, f.schoolA)
	dung := f.student(f.tenantB, f.schoolB)
	src := f.party(an, binh)
	t1 := f.open(channel.Match, an, chi)
	t2 := f.open(channel.Direct, an, binh)
	notMine := f.open(channel.Match, binh, chi)
	foreign := f.party(dung)
	id := f.sendText(binh, src, "di lam nhiem vu nao", nil)
	f.persisted(id)
	forward := func(key string, targets ...uuid.UUID) resp {
		ids := make([]string, len(targets))
		for i, x := range targets {
			ids[i] = x.String()
		}
		f.clock.Add(f.pace)
		return f.call(an, http.MethodPost, "/chat/messages/"+id.String()+":forward", key, map[string]any{"target_channel_ids": ids})
	}
	calls := f.validator.count()
	key := newKey()
	r := forward(key, t1, t2)
	if r.status != http.StatusCreated || len(r.data()) != 2 {
		t.Fatalf("forward: %d %s", r.status, r.raw)
	}
	for _, m := range r.data() {
		ff, _ := m["forwarded_from"].(map[string]any)
		if ff == nil || ff["message_id"] != id.String() || ff["channel_type"] != "party" || ff["sender_nickname"] != binh.nickname {
			t.Fatalf("forwarded_from: %v", m)
		}
	}
	if f.validator.count() != calls+2 {
		t.Fatalf("each forwarded copy must run the pipeline of its channel: %d", f.validator.count()-calls)
	}
	// F118-AC-11: same key → same messages, nothing new.
	again := forward(key, t1, t2)
	if again.status != http.StatusCreated || again.data()[0]["message_id"] != r.data()[0]["message_id"] {
		t.Fatalf("replay: %d %s", again.status, again.raw)
	}
	if n := f.waitCount(2, 5*time.Second, `SELECT count(*) FROM (SELECT 1 FROM chat.gameplay_messages WHERE forwarded_from IS NOT NULL
		UNION ALL SELECT 1 FROM chat.messages WHERE forwarded_from IS NOT NULL) x`); n != 2 {
		t.Fatalf("forwarded rows: %d", n)
	}
	if n := f.checkEvents("chat.message_forwarded")["chat.message_forwarded"]; n != 2 {
		t.Fatalf("chat.message_forwarded events: %d", n)
	}
	if r := f.call(an, http.MethodPost, "/chat/messages/"+id.String()+":forward", "", map[string]any{"target_channel_ids": []string{t1.String()}}); r.status != http.StatusPreconditionRequired || r.code() != "IDEMPOTENCY_KEY_REQUIRED" {
		t.Fatalf("no key: %d %s", r.status, r.raw)
	}
	// F118-AC-10.
	for name, targets := range map[string][]uuid.UUID{"other tenant": {foreign}, "not a member": {notMine}, "mixed": {t1, foreign}} {
		if r := forward(newKey(), targets...); r.status != http.StatusForbidden || r.code() != "CHAT_FORWARD_NOT_ALLOWED" {
			t.Errorf("%s: %d %s", name, r.status, r.raw)
		}
	}
	six := []uuid.UUID{t1, t2, notMine, foreign, src, uuid.New()}
	if r := forward(newKey(), six...); r.status != http.StatusUnprocessableEntity {
		t.Errorf("6 targets: %d %s", r.status, r.raw)
	}
	f.validator.set(message.Mask, "bi ***")
	maskedID := f.sendText(binh, src, "bi che", nil)
	f.validator.set(message.Allow, "")
	if r := f.call(an, http.MethodPost, "/chat/messages/"+maskedID.String()+":forward", newKey(),
		map[string]any{"target_channel_ids": []string{t1.String()}}); r.status != http.StatusForbidden || r.code() != "CHAT_FORWARD_NOT_ALLOWED" {
		t.Errorf("masked source: %d %s", r.status, r.raw)
	}
	// A blocked copy in the target channel: the pipeline of the target refuses.
	f.validator.set(message.Block, "")
	if r := forward(newKey(), t1); r.code() != "CHAT_CONTENT_BLOCKED" {
		t.Errorf("blocked in the target: %d %s", r.status, r.raw)
	}
	f.validator.set(message.Allow, "")
	// WS chat.message.forward takes the same path.
	f.clock.Add(f.pace)
	if rep := f.ws(an, "c1", "chat.message.forward", map[string]any{"message_id": id, "target_channel_ids": []uuid.UUID{t1}}); rep.Problem != nil {
		t.Fatalf("ws forward: %+v", rep.Problem)
	}
}
