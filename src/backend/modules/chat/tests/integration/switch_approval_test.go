//go:build integration

package integration

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
)

// T039 / F116-AC-31, F116-AC-32 (TC-CHAT-045, AC-MOD-031): a disabling change without expires_at, in the past or beyond
// now + 24 h is MODERATION_SWITCH_EXPIRY_REQUIRED; a valid one is only a pending_approval request (nothing in force);
// the proposer cannot approve it (403 MODERATION_SWITCH_SELF_APPROVAL); a second System Admin approves → in force well
// within 60 s with the proposer and the approver in the history and the audit; a request not approved within 24 hours
// expires and never applies; re-enabling needs no approval.
func TestSwitchApproval(t *testing.T) {
	c := setupChat(t, chatOpts{real: true})
	ctx := context.Background()
	sa, sa2 := c.platformStaff("system_admin"), c.platformStaff("system_admin")
	tA, tB := c.tenantA, c.tenantB
	hdrB := []string{"X-Tenant-Id", tB.String()}
	now := c.clock.Now()

	// F116-AC-31: expires_at missing, in the past, beyond 24 h → 422; nothing stored.
	for name, exp := range map[string]*time.Time{"missing": nil, "past": ptrTime(now.Add(-time.Minute)),
		"30 hours": ptrTime(now.Add(30 * time.Hour)), "24 h + 1 s": ptrTime(now.Add(24*time.Hour + time.Second))} {
		for _, b := range []map[string]any{switchBody(&tB, true, []string{"spam"}, reasonOK, exp), switchBody(nil, false, nil, reasonOK, exp)} {
			if r := c.setSwitches(sa, b); r.status != http.StatusUnprocessableEntity || r.code() != "MODERATION_SWITCH_EXPIRY_REQUIRED" {
				t.Fatalf("expires_at %s: %d %s", name, r.status, r.raw)
			}
		}
	}
	if n := c.count(`SELECT count(*) FROM chat.moderation_switch_requests`); n != 0 {
		t.Fatalf("requests stored: %d", n)
	}

	// F116-AC-32: spam off for tenant B, 5 minutes → 202 pending_approval; nothing in force yet.
	r := c.setSwitches(sa, switchBody(&tB, true, []string{"spam"}, reasonOK, ptrTime(now.Add(5*time.Minute))))
	if r.status != http.StatusAccepted || r.body["status"] != "pending_approval" || r.body["tenant_id"] != tB.String() ||
		r.body["approved_by"] != nil || r.body["steps"].(map[string]any)["spam"] != false {
		t.Fatalf("pending request: %d %s", r.status, r.raw)
	}
	req := idOf(r, "request_id")
	if exp, _ := time.Parse(time.RFC3339Nano, r.body["request_expires_at"].(string)); !exp.Equal(now.Add(5 * time.Minute)) {
		t.Fatalf("approval window bounded by expires_at: %v", r.body["request_expires_at"])
	}
	if n := c.count(`SELECT count(*) FROM chat.moderation_switches WHERE tenant_id = $1`, tB); n != 0 {
		t.Fatal("a pending request changed the switch")
	}
	dung, em := c.student(tB, c.schoolB), c.student(tB, c.schoolB)
	an, binh := c.student(tA, c.schoolA), c.student(tA, c.schoolA)
	chB, chA := c.open(channel.Party, dung, em), c.open(channel.Party, an, binh)
	repeat := func(s student, ch uuid.UUID, conn string) string { // three identical messages within 60 s; the third's outcome
		code := ""
		for i := 0; i < 3; i++ {
			c.clock.Add(5 * time.Second)
			r := c.sendWS(s, conn, ch, "lặp lại nha các bạn")
			code = replyCode(r)
			if r.OK {
				code = "OK"
			}
		}
		return code
	}
	if got := repeat(dung, chB, "conn-dung"); got != "CHAT_CONTENT_BLOCKED" {
		t.Fatalf("spam before approval: %s", got)
	}

	// The proposer approves → 403 MODERATION_SWITCH_SELF_APPROVAL (audited denial), nothing changes.
	sub := c.approveSwitch(sa, req, hdrB...)
	if sub.status != http.StatusForbidden || sub.code() != "MODERATION_SWITCH_SELF_APPROVAL" {
		t.Fatalf("self approval: %d %s", sub.status, sub.raw)
	}
	if n := c.count(`SELECT count(*) FROM chat.moderation_switch_requests WHERE request_id = $1 AND status = 'pending_approval'
		AND approved_by IS NULL`, req); n != 1 {
		t.Fatal("self approval changed the request")
	}
	// Without X-Tenant-Id only platform requests are found; another tenant → 404.
	if r := c.approveSwitch(sa2, req); r.status != http.StatusNotFound {
		t.Fatalf("approve without the tenant: %d %s", r.status, r.raw)
	}
	if r := c.approveSwitch(sa2, req, "X-Tenant-Id", tA.String()); r.status != http.StatusNotFound {
		t.Fatalf("approve in another tenant: %d %s", r.status, r.raw)
	}

	// A second System Admin approves → in force at once (≤ 60 s), only in tenant B; PII still blocked there.
	start := time.Now()
	ok := c.approveSwitch(sa2, req, hdrB...)
	if ok.status != http.StatusOK || !slices.Equal(disabledSteps(ok), []string{"spam"}) {
		t.Fatalf("approve: %d %s", ok.status, ok.raw)
	}
	c.clock.Add(time.Minute)
	if got := repeat(dung, chB, "conn-dung"); got != "OK" {
		t.Fatalf("spam after approval in tenant B: %s", got)
	}
	t.Logf("approved change in force after %v", time.Since(start))
	if got := repeat(an, chA, "conn-an"); got != "CHAT_CONTENT_BLOCKED" {
		t.Fatalf("spam in tenant A: %s", got)
	}
	c.clock.Add(11 * time.Second)
	if r := c.sendWS(em, "conn-em", chB, "gọi 0912345678 nhé"); replyCode(r) != "CHAT_CONTENT_BLOCKED" {
		t.Fatalf("PII in tenant B after approval: %+v", r)
	}
	// Two people in the history and the audit.
	if n := c.count(`SELECT count(*) FROM chat.moderation_switch_changes c JOIN chat.moderation_switch_requests r
		ON r.request_id = c.request_id WHERE c.tenant_id = $1 AND c.source = 'approved_request' AND c.actor_id = r.proposed_by
		AND c.approved_by = r.approved_by AND r.status = 'approved' AND r.decided_at IS NOT NULL AND r.approved_by <> r.proposed_by`,
		tB); n != 1 {
		t.Fatalf("two-person history rows %d", n)
	}
	twoPeople := false
	for _, p := range c.events("audit.recorded") {
		meta, _ := p["metadata"].(map[string]any)
		if p["action"] == "chat.moderation_switch.changed" && meta["source"] == "approved_request" && meta["proposed_by"] != nil &&
			meta["approved_by"] != nil && meta["proposed_by"] != meta["approved_by"] && meta["request_id"] == req {
			twoPeople = true
		}
	}
	if !twoPeople {
		t.Fatal("approval audit without proposer and approver")
	}
	ev := c.events("chat_policy.moderation_switch_changed")
	if len(ev) != 1 || ev[0]["source"] != "approved_request" || ev[0]["approved_by"] == nil || ev[0]["request_id"] != req {
		t.Fatalf("approval event: %v", ev)
	}
	// An approved request cannot be approved again.
	if r := c.approveSwitch(c.platformStaff("system_admin"), req, hdrB...); r.status != http.StatusConflict {
		t.Fatalf("approve twice: %d %s", r.status, r.raw)
	}

	// Re-enabling needs no approval: 200, in force at once.
	r = c.setSwitches(sa, switchBody(&tB, true, nil, "Kết thúc sự kiện, bật lại", nil))
	if r.status != http.StatusOK || len(disabledSteps(r)) != 0 {
		t.Fatalf("re-enable: %d %s", r.status, r.raw)
	}
	if e, _ := c.pipe.Switches.Effective(ctx, tB); len(e.Disabled()) != 0 {
		t.Fatalf("re-enable not in force: %v", e.Disabled())
	}

	// A request not approved within 24 hours expires (job) and can never be approved.
	r = c.setSwitches(sa, switchBody(nil, true, []string{"bullying"}, reasonOK, ptrTime(c.clock.Now().Add(24*time.Hour))))
	if r.status != http.StatusAccepted {
		t.Fatalf("platform request: %d %s", r.status, r.raw)
	}
	late := idOf(r, "request_id")
	c.clock.Add(24*time.Hour + time.Minute)
	if r := c.approveSwitch(sa2, late); r.status != http.StatusConflict {
		t.Fatalf("approve past 24 h: %d %s", r.status, r.raw)
	}
	if n, err := c.expiry.Run(ctx); err != nil || n != 1 {
		t.Fatalf("expiry of the pending request: %d %v", n, err)
	}
	if n := c.count(`SELECT count(*) FROM chat.moderation_switch_requests WHERE request_id = $1 AND status = 'expired'
		AND decided_at IS NOT NULL AND approved_by IS NULL`, late); n != 1 {
		t.Fatal("pending request not expired")
	}
	if n := c.count(`SELECT count(*) FROM chat.moderation_switches WHERE tenant_id IS NULL AND steps->>'bullying' = 'false'`); n != 0 {
		t.Fatal("an expired request changed the platform switch")
	}
	if !slices.Contains(c.audits(), "chat.moderation_switch_request.expired") {
		t.Fatal("request expiry not audited")
	}
	if r := c.admin(sa, http.MethodGet, "/admin/chat/moderation-switches", nil); strings.Contains(r.raw, late) {
		t.Fatalf("expired request still pending: %s", r.raw)
	}
}
