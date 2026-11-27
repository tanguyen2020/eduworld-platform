//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

func (p *pEnv) search(s student, q string) resp {
	return p.call(s, http.MethodGet, "/chat/search?q="+url.QueryEscape(q), "", nil)
}

// T044 — F120-AC-22, AC-23 (TC-CHAT-040): "nhan vat" finds "nhân vật" (with or without diacritics) in the club channel of
// the caller only; never another member's channel, another tenant, World, a recalled message, the original of a masked
// message or an anonymised message; q under 2 or over 100 characters → 422; pages by cursor.
func TestSearchMessages(t *testing.T) {
	c := setupPersistent(t, chatOpts{})
	c.pace = 0
	an, binh := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	outsider, foreign := c.student(c.tenantA, c.schoolA), c.student(c.tenantB, c.schoolB)
	clubID, ch := c.mustClub(an, "CLB Truyen")
	c.joinClub(an, binh, clubID)
	send := func(body string) uuid.UUID {
		c.clock.Add(6 * time.Second) // under the burst window of the team channel
		r := c.wsText(an, ch, body)
		if !r.OK {
			t.Fatalf("send %q: %+v", body, r.Problem)
		}
		var res struct {
			MessageID uuid.UUID `json:"message_id"`
		}
		_ = json.Unmarshal(r.Result, &res)
		return res.MessageID
	}
	send("Nhân vật chính rất dũng cảm")
	send("Một nhân vật phụ")
	recalled := send("nhân vật bị thu hồi")
	anonymized := send("nhân vật cũ đã ẩn danh")
	c.validator.set(message.Mask, "tu bi che ***")
	send("nhân vật gốc bị che")
	c.validator.set(message.Allow, "")
	world := channel.WorldID(c.tenantA, &c.schoolA)
	if r := c.wsText(an, world, "nhân vật ở world"); !r.OK {
		t.Fatalf("world: %+v", r.Problem)
	}
	c.waitCount(5, 5*time.Second, `SELECT count(*) FROM chat.messages WHERE channel_id = $1`, ch)
	c.exec(`UPDATE chat.messages SET recalled_at = now() WHERE message_id = $1`, recalled)
	c.exec(`UPDATE chat.messages SET anonymized_at = now(), body = NULL, body_display = NULL WHERE message_id = $1`, anonymized)
	r := c.search(binh, "nhan vat")
	if r.status != http.StatusOK || len(r.data()) != 2 {
		t.Fatalf("search: %d %s", r.status, r.raw)
	}
	for _, hit := range r.data() {
		if hit["channel_type"] != "team" || hit["channel_id"] != ch.String() {
			t.Fatalf("hit: %v", hit)
		}
	}
	if strings.Contains(r.raw, "gốc") || strings.Contains(r.raw, "world") || strings.Contains(r.raw, "thu hồi") {
		t.Fatalf("search leaks: %s", r.raw)
	}
	masked := c.search(binh, "tu bi che")
	if len(masked.data()) != 1 {
		t.Fatalf("display text of the masked message: %s", masked.raw)
	}
	// delta F112-8: highlight = the masked display text in plain text, ranges in code points of the search words only.
	if h := masked.data()[0]; h["highlight"] != "tu bi che ***" || fmt.Sprint(h["highlight_ranges"]) !=
		"[map[length:2 start:0] map[length:2 start:3] map[length:3 start:6]]" {
		t.Fatalf("highlight of the masked message: %v", h)
	}
	for _, h := range r.data() {
		ranges, _ := h["highlight_ranges"].([]any)
		if len(ranges) != 2 || strings.ContainsAny(fmt.Sprint(h["highlight"]), "<>[]") {
			t.Fatalf("nhan vat highlight: %v", h)
		}
	}
	if r := c.search(binh, "NHÂN VẬT CHÍNH"); len(r.data()) != 1 {
		t.Fatalf("upper case with diacritics: %s", r.raw)
	}
	for _, s := range []student{outsider, foreign} {
		if r := c.search(s, "nhan vat"); r.status != http.StatusOK || len(r.data()) != 0 {
			t.Fatalf("non-member search: %d %s", r.status, r.raw)
		}
	}
	p1 := c.call(binh, http.MethodGet, "/chat/search?q=nhan+vat&limit=1", "", nil)
	next, _ := p1.body["page"].(map[string]any)["next_cursor"].(string)
	if len(p1.data()) != 1 || next == "" {
		t.Fatalf("page 1: %s", p1.raw)
	}
	if p2 := c.call(binh, http.MethodGet, "/chat/search?q=nhan+vat&limit=1&cursor="+url.QueryEscape(next), "", nil); len(p2.data()) != 1 ||
		p2.data()[0]["message"].(map[string]any)["message_id"] == p1.data()[0]["message"].(map[string]any)["message_id"] {
		t.Fatalf("page 2: %s", p2.raw)
	}
	for _, q := range []string{"n", strings.Repeat("a", 101)} {
		if r := c.search(binh, q); r.status != 422 && r.status != 400 {
			t.Fatalf("q %d chars: %d %s", len(q), r.status, r.raw)
		}
	}
	authztest.Cover(t, "searchMessages", authztest.CrossTenant, authztest.RoleDenied)
	staff := c.staff(c.tenantA, "tenant_admin")
	if r := c.do(http.MethodGet, "/chat/search?q=nhan+vat", staff, map[string]string{"X-Tenant-Id": c.tenantA.String()}, nil); r.status != 403 {
		t.Fatalf("staff search: %d %s", r.status, r.raw)
	}
	if r := c.do(http.MethodGet, "/chat/search?q=nhan+vat", binh.token, map[string]string{"X-Tenant-Id": c.tenantB.String()}, nil); r.status != 403 {
		t.Fatalf("cross-tenant search: %d %s", r.status, r.raw)
	}
}

// T046, T047 — F120-AC-24 (TC-CHAT-024), F120-AC-25 (TC-CHAT-022): persistent-chat-retention anonymises, in batches, the
// club/direct messages older than their retention (body, body_display, quote and revisions overwritten, anonymized_at
// set), never DELETE, never a gameplay message; a message of a moderation case is copied to moderation_evidence first;
// one run row and one aggregate audit per tenant (no content); the API never returns an anonymised message.
func TestPersistentRetention(t *testing.T) {
	c := setupPersistent(t, chatOpts{})
	c.pace = 0
	an, binh := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	clubID, club := c.mustClub(an, "CLB Cu")
	c.joinClub(an, binh, clubID)
	dm := c.open(channel.Direct, an, binh)
	party := c.open(channel.Party, an, binh)
	old := c.clock.Now().Add(-200 * 24 * time.Hour)
	insert := func(ch uuid.UUID, seq int, body string, created time.Time, table string) uuid.UUID {
		id := uuid.New()
		extra, val := "", ""
		if table == "gameplay_messages" {
			extra, val = ", created_on", ", $8::date"
		}
		c.exec(`INSERT INTO chat.`+table+` (message_id, channel_id, seq, tenant_id, school_id, sender_id, kind, body, body_display, created_at`+extra+`)
			VALUES ($1, $2, $3, $4, $5, $6, 'text', $7, $7, $8::timestamptz`+val+`)`, id, ch, seq, c.tenantA, c.schoolA, an.id, body, created)
		return id
	}
	reported := insert(club, 1, "tin cu bi bao cao", old, "messages")
	insert(club, 2, "tin cu thu hai", old.Add(time.Minute), "messages")
	insert(dm, 1, "tin rieng cu", old.Add(2*time.Minute), "messages")
	fresh := insert(club, 3, "tin moi", c.clock.Now().Add(time.Minute), "messages")
	gameplay := insert(party, 1, "tin tran cu", old, "gameplay_messages")
	c.exec(`UPDATE chat.channels SET last_seq = 3 WHERE channel_id = $1`, club)
	c.exec(`INSERT INTO chat.message_revisions (tenant_id, channel_id, message_id, revision, body, body_display, decision, edited_at)
		VALUES ($1, $2, $3, 1, 'ban truoc', 'ban truoc', 'allow', now())`, c.tenantA, club, reported)
	caseID := uuid.New()
	c.exec(`INSERT INTO chat.moderation_cases (case_id, tenant_id, created_at, updated_at, school_ids, severity, source, target_type, target_id,
		subject_user_ids, first_response_due_at, sla_due_at) VALUES ($1, $2, now(), now(), '[]', 'p2', 'report', 'message', $3, '[]', now(), now())`,
		caseID, c.tenantA, reported)
	before := c.count(`SELECT count(*) FROM chat.messages`)
	n, err := c.retention.Run(context.Background())
	if err != nil || n != 3 {
		t.Fatalf("retention: %d %v", n, err)
	}
	if after := c.count(`SELECT count(*) FROM chat.messages`); after != before {
		t.Fatalf("rows deleted: %d → %d", before, after)
	}
	if k := c.count(`SELECT count(*) FROM chat.messages WHERE anonymized_at IS NOT NULL AND body IS NULL AND body_display IS NULL`); k != 3 {
		t.Fatalf("anonymised rows: %d", k)
	}
	if k := c.count(`SELECT count(*) FROM chat.messages WHERE message_id = $1 AND anonymized_at IS NULL AND body = 'tin moi'`, fresh); k != 1 {
		t.Fatal("a fresh message was anonymised")
	}
	if k := c.count(`SELECT count(*) FROM chat.gameplay_messages WHERE message_id = $1 AND body = 'tin tran cu'`, gameplay); k != 1 {
		t.Fatal("a gameplay message was touched (purge of F117 only)")
	}
	if k := c.count(`SELECT count(*) FROM chat.message_revisions WHERE message_id = $1 AND body IS NULL AND body_display IS NULL`, reported); k != 1 {
		t.Fatal("revision not anonymised")
	}
	if k := c.count(`SELECT count(*) FROM chat.moderation_evidence WHERE case_id = $1 AND snapshot->>'target_id' = $2
		AND snapshot::text LIKE '%tin cu bi bao cao%'`, caseID, reported.String()); k != 1 {
		t.Fatal("evidence not copied before anonymisation")
	}
	if k := c.count(`SELECT count(*) FROM chat.persistent_retention_runs WHERE tenant_id = $1 AND status = 'succeeded'
		AND messages_anonymized = 3 AND evidence_copied = 1 AND channels_scanned = 2`, c.tenantA); k != 1 {
		t.Fatal("run row")
	}
	if countOf(c.audits(), "chat.retention_applied") != 1 {
		t.Fatalf("audit: %v", c.audits())
	}
	for _, e := range c.events("audit.recorded") {
		if e["action"] == "chat.retention_applied" && strings.Contains(stringify(e), "tin cu") {
			t.Fatal("audit carries content")
		}
	}
	h := c.call(binh, http.MethodGet, "/chat/channels/"+club.String()+"/messages", "", nil)
	if h.status != http.StatusOK || len(h.data()) != 1 || h.data()[0]["body"] != "tin moi" {
		t.Fatalf("history after retention: %d %s", h.status, h.raw)
	}
	if again, err := c.retention.Run(context.Background()); err != nil || again != 0 {
		t.Fatalf("second run: %d %v", again, err)
	}
}

func stringify(m map[string]any) string {
	var b strings.Builder
	for k, v := range m {
		b.WriteString(k)
		if s, ok := v.(string); ok {
			b.WriteString(s)
		}
		if mm, ok := v.(map[string]any); ok {
			b.WriteString(stringify(mm))
		}
	}
	return b.String()
}
