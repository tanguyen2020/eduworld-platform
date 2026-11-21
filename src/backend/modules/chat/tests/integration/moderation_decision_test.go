//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
)

func textBody(s string) map[string]any {
	return map[string]any{"client_message_id": uuid.New(), "kind": "text", "body": s}
}

func resultOf(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	m := map[string]any{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// T011 / F116-AC-04, F116-AC-05, F116-AC-06, F116-AC-29 (TC-CHAT-003, TC-CHAT-010, TC-CHAT-015, TC-I18N-009): the real
// pipeline on the send path — block (no seq, no row, category_code only), mask (only the span), held (stored held, no
// fan-out, chat.message_held), block_and_alert (held + system mute), English terms whatever the UI language.
func TestModerationDecisions(t *testing.T) {
	c := setupChat(t, chatOpts{real: true})
	c.publishPlatform(defaultTerms()...)
	an, binh := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	ch := c.open(channel.Party, an, binh)
	fan := channelInbox(t, c.vk, ch)
	anInbox := inbox(t, c.vk, an.id)
	time.Sleep(200 * time.Millisecond)
	step := func() { c.clock.Add(11 * time.Second) } // past the burst window of the party channel

	// block (PII) over WS: CHAT_CONTENT_BLOCKED with category_code only, nothing echoed.
	r := c.sendWS(an, "conn-an", ch, "số mình 0912345678 nha")
	if r.OK || replyCode(r) != "CHAT_CONTENT_BLOCKED" || r.Problem.Details["category_code"] != "pii" {
		t.Fatalf("PII over WS: %+v", r.Problem)
	}
	if b, _ := json.Marshal(r); strings.Contains(string(b), "0912345678") || strings.Contains(string(b), "start") {
		t.Fatalf("reply echoes content or position: %s", b)
	}
	// block (catalogue) over REST: 451 + category_code, no term, no position.
	step()
	rr := c.sendREST(an, ch, textBody("đồ zqơrốp"))
	if rr.status != http.StatusUnavailableForLegalReasons || rr.code() != "CHAT_CONTENT_BLOCKED" || rr.body["category_code"] != "insult" ||
		strings.Contains(rr.raw, "zq") {
		t.Fatalf("term over REST: %d %s", rr.status, rr.raw)
	}
	// English catalogue applies to a Vietnamese UI too (no translation, F116-AC-29).
	step()
	if r := c.sendWS(binh, "conn-binh", ch, "so zqenglish"); replyCode(r) != "CHAT_CONTENT_BLOCKED" {
		t.Fatalf("english term: %+v", r)
	}
	// mask: delivered with only the matched span hidden; the first stored message takes seq 1 (blocks got none).
	step()
	r = c.sendWS(an, "conn-an", ch, "xin chào zqlow nhé")
	if !r.OK || resultOf(t, r.Result)["moderation_state"] != "masked" || resultOf(t, r.Result)["seq"] != float64(1) {
		t.Fatalf("mask: %+v %s", r.Problem, r.Result)
	}
	env := next(fan, "chat.message.created", 3*time.Second)
	if env == nil || payloadOf(env)["body"] != "xin chào *** nhé" {
		t.Fatalf("masked fan-out: %+v", env)
	}
	// hold_for_review: acked to the sender as held, never fanned out, stored held, chat.message_held.
	step()
	r = c.sendWS(an, "conn-an", ch, "zqhigh")
	if !r.OK || resultOf(t, r.Result)["moderation_state"] != "held" {
		t.Fatalf("held: %+v %s", r.Problem, r.Result)
	}
	if next(anInbox, "chat.message.held", 3*time.Second) == nil {
		t.Fatal("sender did not get chat.message.held")
	}
	if e := next(fan, "chat.message.created", 500*time.Millisecond); e != nil {
		t.Fatalf("held message fanned out: %+v", e)
	}
	if n := c.waitCount(1, 5*time.Second, `SELECT count(*) FROM chat.gameplay_messages WHERE channel_id = $1 AND moderation_state = 'held'`, ch); n != 1 {
		t.Fatalf("held rows %d", n)
	}
	held := c.events("chat.message_held")
	if len(held) != 1 || !slices.Contains(toStrings(held[0]["reasons"]), "sexual") || held[0]["catalog_version"] == nil {
		t.Fatalf("chat.message_held: %v", held)
	}
	// block_and_alert: held + system mute until the Safety Team handles it; the next message is CHAT_MUTED.
	step()
	r = c.sendWS(an, "conn-an", ch, "zqcrit")
	if !r.OK || resultOf(t, r.Result)["moderation_state"] != "held" {
		t.Fatalf("block_and_alert: %+v", r)
	}
	if n := c.count(`SELECT count(*) FROM chat.sanctions WHERE user_id = $1 AND type = 'mute' AND scope = 'all'
		AND reason = 'block_and_alert_hold' AND ends_at IS NULL AND issued_by IS NULL`, an.id); n != 1 {
		t.Fatalf("system mute rows %d", n)
	}
	if ev := c.events("sanction.applied"); len(ev) != 1 || ev[0]["reason_code"] != "block_and_alert_hold" {
		t.Fatalf("sanction.applied: %v", ev)
	}
	if next(anInbox, "chat.sanction.applied", 3*time.Second) == nil {
		t.Fatal("chat.sanction.applied not pushed")
	}
	step()
	if r := c.sendWS(an, "conn-an", ch, "chào"); replyCode(r) != "CHAT_MUTED" {
		t.Fatalf("muted sender: %+v", r)
	}
	// blocked messages never reach PostgreSQL; the stored rows carry the decision, never the blocked text.
	time.Sleep(300 * time.Millisecond)
	if n := c.count(`SELECT count(*) FROM chat.gameplay_messages WHERE channel_id = $1 AND (body LIKE '%0912345678%' OR body LIKE '%zqơrốp%'
		OR body LIKE '%zqenglish%')`, ch); n != 0 {
		t.Fatalf("blocked content stored: %d", n)
	}
	if n := c.count(`SELECT count(*) FROM chat.gameplay_messages WHERE channel_id = $1`, ch); n != 3 {
		t.Fatalf("stored messages %d, want 3 (mask, held, held)", n)
	}
}

func toStrings(v any) []string {
	items, _ := v.([]any)
	out := make([]string, 0, len(items))
	for _, i := range items {
		if s, ok := i.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
