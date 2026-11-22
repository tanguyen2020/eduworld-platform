//go:build integration

package integration

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
)

// T009 — F118-AC-04, AC-05, AC-06 (TC-CHAT-030, TC-CHAT-031): a recall shows a placeholder to everyone, unpins, keeps the
// original and every revision in PostgreSQL as evidence; no user API (history, reply, quote, forward, pins) returns the
// recalled content again; past chat_recall_window it is refused.
func TestRecallMessage(t *testing.T) {
	f := setupFeatures(t, chatOpts{})
	startFeatures(t, f)
	an, binh := f.student(f.tenantA, f.schoolA), f.student(f.tenantA, f.schoolA)
	ch := f.open(channel.Match, an, binh)
	other := f.open(channel.Match, an, binh)
	inbox := channelInbox(t, f.vk, ch)
	id := f.sendText(an, ch, "hen gap o cong truong", nil)
	f.persisted(id)
	if r := f.edit(an, id, 1, "hen gap o san truong"); r.status != http.StatusOK {
		t.Fatalf("edit: %d %s", r.status, r.raw)
	}
	reply := f.sendText(binh, ch, "ok nhe", map[string]any{"reply_to_message_id": id})
	quote := f.sendText(binh, ch, "dong y", map[string]any{"quote": map[string]any{"message_id": id, "excerpt_start": 0, "excerpt_end": 6}})
	if r := f.call(an, http.MethodPut, "/chat/channels/"+ch.String()+"/pins/"+id.String(), "", nil); r.status != http.StatusNoContent {
		t.Fatalf("pin: %d %s", r.status, r.raw)
	}
	f.persisted(quote)
	if n := f.waitCount(1, 5*time.Second, `SELECT count(*) FROM chat.pinned_messages WHERE message_id = $1 AND deleted_at IS NULL`, id); n != 1 {
		t.Fatal("pin not persisted")
	}
	// Before the recall: the reply excerpt and the quote excerpt come from the moderated content.
	h := f.call(binh, http.MethodGet, "/chat/channels/"+ch.String()+"/messages", "", nil)
	byID := map[string]map[string]any{}
	for _, m := range h.data() {
		byID[m["message_id"].(string)] = m
	}
	if byID[reply.String()]["reply_excerpt"] != "hen gap o san truong" {
		t.Fatalf("reply excerpt before recall: %s", h.raw)
	}
	if q, _ := byID[quote.String()]["quote"].(map[string]any); q == nil || q["excerpt"] != "hen ga" {
		t.Fatalf("quote excerpt before recall: %s", h.raw)
	}

	f.clock.Add(time.Hour)
	if r := f.recall(an, id); r.status != http.StatusNoContent {
		t.Fatalf("recall: %d %s", r.status, r.raw)
	}
	env := next(inbox, "chat.message.recalled", 3*time.Second)
	if env == nil || strings.Contains(string(env.Message.Payload), "san truong") {
		t.Fatalf("chat.message.recalled: %+v", env)
	}
	if next(inbox, "chat.pin.updated", 3*time.Second) == nil {
		t.Fatal("a recalled pinned message must be unpinned (chat.pin.updated)")
	}
	// F118-AC-06: original + revisions kept; recalled_at set; the pin soft-deleted.
	if n := f.waitCount(1, 5*time.Second, `SELECT count(*) FROM chat.gameplay_messages WHERE message_id = $1 AND recalled_at IS NOT NULL
		AND recalled_by = $2 AND body = 'hen gap o san truong'`, id, an.id); n != 1 {
		t.Fatal("recall must keep the body as evidence")
	}
	if n := f.count(`SELECT count(*) FROM chat.message_revisions WHERE message_id = $1 AND body = 'hen gap o cong truong'`, id); n != 1 {
		t.Fatal("revision lost after recall")
	}
	if n := f.waitCount(1, 5*time.Second, `SELECT count(*) FROM chat.pinned_messages WHERE message_id = $1 AND deleted_at IS NOT NULL`, id); n != 1 {
		t.Fatal("pin not removed")
	}
	f.checkEvents("chat.message_recalled", "chat.message_pinned")
	if len(f.events("chat.message_recalled")) != 1 {
		t.Fatal("chat.message_recalled missing")
	}
	// F118-AC-05: no API returns the content again.
	h = f.call(binh, http.MethodGet, "/chat/channels/"+ch.String()+"/messages", "", nil)
	if strings.Contains(h.raw, "san truong") || strings.Contains(h.raw, "cong truong") || strings.Contains(h.raw, "hen ga") {
		t.Fatalf("history leaks recalled content: %s", h.raw)
	}
	for _, m := range h.data() {
		switch m["message_id"] {
		case id.String():
			if m["recalled"] != true || m["body"] != nil {
				t.Fatalf("placeholder expected: %v", m)
			}
		case reply.String():
			if _, ok := m["reply_excerpt"]; ok {
				t.Fatalf("reply excerpt of a recalled original: %v", m)
			}
		case quote.String():
			if q, _ := m["quote"].(map[string]any); q == nil || q["excerpt"] != nil {
				t.Fatalf("quote of a recalled original must lose its excerpt: %v", m)
			}
		}
	}
	if p := f.call(binh, http.MethodGet, "/chat/channels/"+ch.String()+"/pins", "", nil); p.status != http.StatusOK || len(p.data()) != 0 {
		t.Fatalf("pins after recall: %d %s", p.status, p.raw)
	}
	if r := f.call(binh, http.MethodPost, "/chat/messages/"+id.String()+":forward", newKey(),
		map[string]any{"target_channel_ids": []string{other.String()}}); r.status != http.StatusForbidden || r.code() != "CHAT_FORWARD_NOT_ALLOWED" {
		t.Fatalf("forward of a recalled message: %d %s", r.status, r.raw)
	}
	if r := f.call(binh, http.MethodPost, "/chat/messages/"+id.String()+"/reactions", newKey(), map[string]any{"reaction": "heart"}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("reaction on a recalled message: %d %s", r.status, r.raw)
	}
	if r := f.call(binh, http.MethodPost, "/chat/channels/"+ch.String()+"/messages", newKey(), map[string]any{"client_message_id": newKey(),
		"kind": "text", "body": "trich", "quote": map[string]any{"message_id": id}}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("quote of a recalled message: %d %s", r.status, r.raw)
	}
	// Past chat_recall_window (24 hours).
	late := f.sendText(an, ch, "tin moi", nil)
	f.clock.Add(25 * time.Hour)
	if r := f.recall(an, late); r.status != http.StatusConflict || r.code() != "CHAT_RECALL_WINDOW_EXPIRED" {
		t.Fatalf("late recall: %d %s", r.status, r.raw)
	}
}
