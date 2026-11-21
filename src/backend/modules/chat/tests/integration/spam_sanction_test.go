//go:build integration

package integration

import (
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
)

// T012 / F116-AC-07, F116-AC-08, F116-AC-24 (TC-CHAT-007, TC-CHAT-014): repeated content and the same content in
// several channels are blocked; 3 blocks in 10 minutes mute the sender (auto_mute_spam, audit); a mute refuses every
// message with CHAT_MUTED even with enabled_all off, and ends with its period.
func TestSpamAndSanctions(t *testing.T) {
	c := setupChat(t, chatOpts{real: true})
	c.publishPlatform(defaultTerms()...)
	an, binh := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	ch := c.open(channel.Party, an, binh)
	step := func() { c.clock.Add(11 * time.Second) }

	// ≥ 3 identical messages within 60 s: the third is blocked (spam).
	for i := 1; i <= 3; i++ {
		step()
		r := c.sendWS(an, "conn-an", ch, "Cùng một tin nè")
		if i < 3 && !r.OK {
			t.Fatalf("repeat %d: %+v", i, r.Problem)
		}
		if i == 3 && (replyCode(r) != "CHAT_CONTENT_BLOCKED" || r.Problem.Details["category_code"] != "spam") {
			t.Fatalf("third repeat: %+v", r.Problem)
		}
	}
	// the same content in ≥ 4 channels within 2 minutes: blocked in the fourth.
	chans := []uuid.UUID{c.open(channel.Party, binh, an), c.open(channel.Party, binh, an), c.open(channel.Party, binh, an),
		c.open(channel.Party, binh, an)}
	for i, id := range chans {
		c.clock.Add(time.Second)
		r := c.sendWS(binh, "conn-binh", id, "mời mọi người vào nhóm")
		if i < 3 && !r.OK {
			t.Fatalf("channel %d: %+v", i, r.Problem)
		}
		if i == 3 && (replyCode(r) != "CHAT_CONTENT_BLOCKED" || r.Problem.Details["category_code"] != "spam") {
			t.Fatalf("fourth channel: %+v", r.Problem)
		}
	}
	// two more blocks of An within 10 minutes (3 in total) → system mute auto_spam for 10 minutes, audited.
	step()
	if r := c.sendWS(an, "conn-an", ch, "gọi 0912345678"); replyCode(r) != "CHAT_CONTENT_BLOCKED" {
		t.Fatalf("block 2: %+v", r)
	}
	step()
	if r := c.sendWS(an, "conn-an", ch, "hoặc 0987654321"); replyCode(r) != "CHAT_CONTENT_BLOCKED" {
		t.Fatalf("block 3: %+v", r)
	}
	if n := c.count(`SELECT count(*) FROM chat.sanctions WHERE user_id = $1 AND reason = 'auto_spam' AND type = 'mute'
		AND ends_at = starts_at + interval '10 minutes'`, an.id); n != 1 {
		t.Fatalf("auto mute rows %d", n)
	}
	if !slices.Contains(c.audits(), "chat.sanction.applied") {
		t.Fatalf("audit: %v", c.audits())
	}
	step()
	if r := c.sendWS(an, "conn-an", ch, "xin lỗi"); replyCode(r) != "CHAT_MUTED" {
		t.Fatalf("muted: %+v", r)
	}
	// enabled_all off for the tenant: the mute still applies (technical gate).
	c.tenantSwitch(c.tenantA, false, map[string]bool{})
	if r := c.sendWS(an, "conn-an", ch, "xin lỗi mà"); replyCode(r) != "CHAT_MUTED" {
		t.Fatalf("muted with enabled_all off: %+v", r)
	}
	// once the mute ended the sender writes again (decision unchecked: every content step is off).
	c.clock.Add(10 * time.Minute)
	r := c.sendWS(an, "conn-an", ch, "mình quay lại rồi")
	if !r.OK || resultOf(t, r.Result)["moderation_state"] != "unchecked" {
		t.Fatalf("after the mute: %+v %s", r.Problem, r.Result)
	}
	// PII is still blocked with every switch off (ADR-0078).
	step()
	if r := c.sendWS(binh, "conn-binh", ch, "zalo mình nè"); replyCode(r) != "CHAT_CONTENT_BLOCKED" {
		t.Fatalf("PII with switches off: %+v", r)
	}
}

// F116 technical gate: burst (≤ burst messages per 10 s in a channel) and slow mode of the effective policy are
// always on (MODERATION §12.3).
func TestTechnicalGateBurstAndSlowMode(t *testing.T) {
	c := setupChat(t, chatOpts{real: true})
	an, binh := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	c.pace = 0                               // every send at the same instant
	party := c.open(channel.Party, an, binh) // rate 12/min, burst 3
	for i := 1; i <= 3; i++ {
		if r := c.sendWS(an, "conn-an", party, "tin thứ "+string(rune('0'+i))); !r.OK {
			t.Fatalf("burst %d: %+v", i, r.Problem)
		}
	}
	r := c.sendWS(an, "conn-an", party, "tin thứ tư")
	if replyCode(r) != "CHAT_RATE_LIMITED" || r.Problem.RetryAfterMs <= 0 {
		t.Fatalf("over burst: %+v", r.Problem)
	}
	c.clock.Add(10 * time.Second)
	if r := c.sendWS(an, "conn-an", party, "tin thứ năm"); !r.OK {
		t.Fatalf("after the burst window: %+v", r.Problem)
	}
	// Slow mode: a tenant policy tightens the party channels of tenant B to 5 s (always-on channels allow ≤ 10 s).
	c.exec(`INSERT INTO chat.chat_policies (policy_id, tenant_id, scope_type, scope_id, rules) VALUES ($1, $2, 'tenant', $2, $3::jsonb)`,
		uuid.New(), c.tenantB, `{"schema_version":5,"scope_type":"tenant","rules":[{"channel_type":"party","slow_mode_seconds":5}]}`)
	dung, em := c.student(c.tenantB, c.schoolB), c.student(c.tenantB, c.schoolB)
	slow := c.open(channel.Party, dung, em)
	if r := c.sendWS(dung, "conn-dung", slow, "chào cả nhóm"); !r.OK {
		t.Fatalf("slow 1: %+v", r.Problem)
	}
	c.clock.Add(2 * time.Second)
	if r := c.sendWS(dung, "conn-dung", slow, "chào lần hai"); replyCode(r) != "CHAT_RATE_LIMITED" || r.Problem.RetryAfterMs <= 0 {
		t.Fatalf("slow mode: %+v", r.Problem)
	}
	c.clock.Add(3 * time.Second)
	if r := c.sendWS(dung, "conn-dung", slow, "chào lần ba"); !r.OK {
		t.Fatalf("after slow mode: %+v", r.Problem)
	}
}
