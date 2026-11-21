//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
)

// T013 / F116-AC-09 (TC-CHAT-013): one decision row per message — stored messages through chat-persister, blocked ones
// through the decision writer — with steps, catalogue versions, switch snapshot and latency, and no content column.
func TestDecisionLog(t *testing.T) {
	c := setupChat(t, chatOpts{real: true})
	v := c.publishPlatform(defaultTerms()...)
	an, binh := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	ch := c.open(channel.Party, an, binh)
	if r := c.sendWS(an, "conn-an", ch, "chào cả nhà"); !r.OK {
		t.Fatalf("allow: %+v", r.Problem)
	}
	c.clock.Add(11 * time.Second)
	if r := c.sendWS(an, "conn-an", ch, "đồ zqơrốp"); replyCode(r) != "CHAT_CONTENT_BLOCKED" {
		t.Fatalf("block: %+v", r)
	}
	c.pipe.Decisions.Flush(context.Background())
	if n := c.waitCount(1, 5*time.Second, `SELECT count(*) FROM chat.moderation_decisions WHERE channel_id = $1 AND decision = 'allow'
		AND message_id IS NOT NULL AND message_id = decision_id`, ch); n != 1 {
		t.Fatalf("allow decision rows %d", n)
	}
	if n := c.count(`SELECT count(*) FROM chat.moderation_decisions WHERE channel_id = $1 AND decision = 'block' AND message_id IS NULL
		AND category_codes = '{insult}'`, ch); n != 1 {
		t.Fatalf("block decision rows %d", n)
	}
	if n := c.count(`SELECT count(*) FROM chat.moderation_decisions WHERE channel_id = $1 AND tenant_id = $2 AND sender_id = $3
		AND catalog_version_platform = $4 AND switch_snapshot_id IS NOT NULL AND latency_ms >= 0
		AND steps_run @> '{normalize,format,sensitive_terms,grooming,pii,spam,bullying,context}'::varchar[]`, ch, c.tenantA, an.id, v); n != 2 {
		t.Fatalf("complete decision rows %d", n)
	}
	// No content column in the decision log (R14).
	if n := c.count(`SELECT count(*) FROM information_schema.columns WHERE table_schema = 'chat' AND table_name = 'moderation_decisions'
		AND column_name ~ '(body|text|content|term|phrase)'`); n != 0 {
		t.Fatalf("content columns %d", n)
	}
	// The decision log is append-only.
	if _, err := c.su.ExecContext(context.Background(), `UPDATE chat.moderation_decisions SET latency_ms = 1`); err == nil {
		t.Fatal("decision log updated")
	}
}
