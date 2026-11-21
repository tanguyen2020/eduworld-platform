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

var catGrooming = uuid.MustParse("01936a00-0000-7000-8000-000000000107")

// T038 / F116-AC-30 (TC-CHAT-044, AC-MOD-030): pii and grooming have no switch — a request turning them off at the
// platform or a tenant is 422 MODERATION_SWITCH_STEP_NOT_ALLOWED and stores nothing; with enabled_all off (approved) a
// phone number, an invitation to another app, a meeting proposal and a grooming term are still stopped; a legacy
// stored steps.pii = false is ignored when the switches are resolved.
func TestSwitchAlwaysOnSteps(t *testing.T) {
	c := setupChat(t, chatOpts{real: true})
	c.publishPlatform(append(defaultTerms(), placeholder(catGrooming, "zqgroom", "vi"))...)
	sa, sa2 := c.platformStaff("system_admin"), c.platformStaff("system_admin")
	tA, tB := c.tenantA, c.tenantB
	exp := c.clock.Now().Add(time.Hour)

	for _, tenant := range []*uuid.UUID{nil, &tA} {
		for _, step := range []string{"pii", "grooming"} {
			b := switchBody(tenant, true, nil, reasonOK, &exp)
			b["steps"].(map[string]any)[step] = false
			r := c.setSwitches(sa, b)
			if r.status != http.StatusUnprocessableEntity || r.code() != "MODERATION_SWITCH_STEP_NOT_ALLOWED" {
				t.Fatalf("%s off (tenant %v): %d %s", step, tenant, r.status, r.raw)
			}
			// with enabled_all off too: the always-on step is refused first.
			b["enabled_all"] = false
			if r = c.setSwitches(sa, b); r.code() != "MODERATION_SWITCH_STEP_NOT_ALLOWED" {
				t.Fatalf("%s off with enabled_all off: %d %s", step, r.status, r.raw)
			}
		}
	}
	if n := c.count(`SELECT count(*) FROM chat.moderation_switch_requests`) +
		c.count(`SELECT count(*) FROM chat.moderation_switches WHERE tenant_id IS NOT NULL`); n != 0 {
		t.Fatalf("a refused pii/grooming change was stored: %d", n)
	}

	// enabled_all off for tenant A (approved): content steps skipped, PII and grooming still run.
	a := c.disable(sa, sa2, &tA, false, nil, time.Hour)
	if slices.Contains(anyStrings(a.body["disabled_steps"]), "pii") || strings.Contains(a.raw, `"pii"`) {
		t.Fatalf("pii appears among the switches: %s", a.raw)
	}
	c.reload()
	an, binh := c.student(tA, c.schoolA), c.student(tA, c.schoolA)
	chi, dao := c.student(tA, c.schoolA), c.student(tA, c.schoolA)
	ch := c.open(channel.Party, an, binh, chi, dao)
	r := c.sendWS(an, "conn-an", ch, "đồ zqơrốp")
	if !r.OK || resultOf(t, r.Result)["moderation_state"] != "unchecked" {
		t.Fatalf("catalogue term with enabled_all off: %+v %s", r.Problem, r.Result)
	}
	// (two senders: three blocks of one sender within 10 minutes would mute them, F116-AC-07)
	for i, text := range []string{"số mình 0912345678", "không chín một hai ba bốn năm sáu bảy tám", "kết bạn zalo mình nhé"} {
		c.clock.Add(11 * time.Second)
		sender := binh
		if i == 2 {
			sender = chi
		}
		if r := c.sendWS(sender, "conn-pii", ch, text); replyCode(r) != "CHAT_CONTENT_BLOCKED" || r.Problem.Details["category_code"] != "pii" {
			t.Fatalf("PII with enabled_all off (%d chars): %+v", len(text), r)
		}
	}
	c.clock.Add(11 * time.Second)
	if r := c.sendWS(dao, "conn-dao", ch, "mai gặp ở công viên nhé"); !r.OK || resultOf(t, r.Result)["moderation_state"] != "held" {
		t.Fatalf("meeting proposal with enabled_all off: %+v %s", r.Problem, r.Result)
	}
	c.clock.Add(11 * time.Second)
	if r := c.sendWS(an, "conn-an", ch, "zqgroom"); !r.OK || resultOf(t, r.Result)["moderation_state"] != "held" {
		t.Fatalf("grooming term with enabled_all off: %+v %s", r.Problem, r.Result)
	}
	var held []map[string]any
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if held = c.events("chat.message_held"); len(held) == 2 {
			break
		}
	}
	if len(held) != 2 || !slices.Contains(toStrings(held[1]["reasons"]), "grooming_scam") {
		t.Fatalf("held events: %v", held)
	}

	// A legacy stored steps.pii = false (before ADR-0078) is ignored when resolved (tenant B).
	c.exec(`INSERT INTO chat.moderation_switches (switch_id, tenant_id, scope, enabled_all, steps)
		VALUES ($1, $2, 'tenant', true, '{"pii": false, "sensitive_terms": true, "spam": true, "bullying": true, "async_analysis": true}'::jsonb)`,
		uuid.New(), tB)
	c.reload()
	e, _ := c.pipe.Switches.Effective(context.Background(), tB)
	if !e.On("pii") || len(e.Disabled()) != 0 {
		t.Fatalf("legacy pii=false resolved: %v", e.Disabled())
	}
	dung, em := c.student(tB, c.schoolB), c.student(tB, c.schoolB)
	chB := c.open(channel.Party, dung, em)
	if r := c.sendWS(dung, "conn-dung", chB, "gọi 0987654321 nha"); replyCode(r) != "CHAT_CONTENT_BLOCKED" {
		t.Fatalf("PII with legacy pii=false: %+v", r)
	}
	c.pipe.Decisions.Flush(context.Background())
	if n := c.waitCount(1, 5*time.Second, `SELECT count(*) FROM chat.moderation_decisions WHERE sender_id = $1 AND decision = 'block'
		AND 'pii' = ANY(steps_run) AND NOT 'pii' = ANY(steps_skipped)`, dung.id); n != 1 {
		t.Fatalf("decision with legacy pii=false: %d", n)
	}
	r2 := c.admin(sa, http.MethodGet, "/admin/chat/moderation-switches?tenant_ids="+tB.String(), nil)
	if r2.status != http.StatusOK || strings.Contains(r2.raw, `"pii"`) {
		t.Fatalf("legacy pii exposed: %s", r2.raw)
	}
}
