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

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

const reasonOK = "Sự kiện thi đấu cấp trường, tin lặp hợp lệ"

// switchBody is ModerationSwitchUpdate: the steps listed in off are false, the others true.
func switchBody(tenant *uuid.UUID, enabledAll bool, off []string, reason string, exp *time.Time) map[string]any {
	steps := map[string]any{"sensitive_terms": true, "spam": true, "bullying": true, "async_analysis": true}
	for _, s := range off {
		steps[s] = false
	}
	b := map[string]any{"scope": "platform", "enabled_all": enabledAll, "steps": steps}
	if tenant != nil {
		b["scope"], b["tenant_id"] = "tenant", *tenant
	}
	if reason != "" {
		b["reason"] = reason
	}
	if exp != nil {
		b["expires_at"] = exp.UTC().Format(time.RFC3339Nano)
	}
	return b
}

// setSwitches calls setModerationSwitches (PUT, new Idempotency-Key unless given in headers).
func (c *chatEnv) setSwitches(token string, body map[string]any, headers ...string) resp {
	c.t.Helper()
	return c.admin(token, http.MethodPut, "/admin/chat/moderation-switches", body, append([]string{"Idempotency-Key", newKey()}, headers...)...)
}

// approveSwitch calls approveModerationSwitchChange.
func (c *chatEnv) approveSwitch(token, request string, headers ...string) resp {
	c.t.Helper()
	return c.admin(token, http.MethodPost, "/admin/chat/moderation-switches/requests/"+request+":approve", nil, headers...)
}

// disable requests a disabling change as sa1 and has it approved by sa2 (X-Tenant-Id of the tenant level); it returns
// the approved state.
func (c *chatEnv) disable(sa1, sa2 string, tenant *uuid.UUID, enabledAll bool, off []string, d time.Duration) resp {
	c.t.Helper()
	exp := c.clock.Now().Add(d)
	r := c.setSwitches(sa1, switchBody(tenant, enabledAll, off, reasonOK, &exp))
	if r.status != http.StatusAccepted || r.body["status"] != "pending_approval" {
		c.t.Fatalf("disabling request: %d %s", r.status, r.raw)
	}
	var hdr []string
	if tenant != nil {
		hdr = []string{"X-Tenant-Id", tenant.String()}
	}
	a := c.approveSwitch(sa2, idOf(r, "request_id"), hdr...)
	if a.status != http.StatusOK {
		c.t.Fatalf("approve: %d %s", a.status, a.raw)
	}
	return a
}

func disabledSteps(r resp) []string {
	var out []string
	list, _ := r.body["disabled_steps"].([]any)
	for _, s := range list {
		out = append(out, s.(string))
	}
	return out
}

// T026 / F116-AC-20, F116-AC-21 (TC-CHAT-015, TC-CHAT-016): setModerationSwitches stores enabled_all, the steps, the
// reason and expires_at of a level, writes the before/after history, the audit and chat_policy.moderation_switch_changed,
// and is in force on the instances well within 60 s (re-enabling at once, disabling once approved); a missing or short
// reason is MODERATION_SWITCH_REASON_REQUIRED; without chat.policy.toggle (tenant roles, Safety Team, game token) every
// switch operation is refused and changes nothing.
func TestModerationSwitches(t *testing.T) {
	authztest.Cover(t, "listModerationSwitches", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "setModerationSwitches", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listModerationSwitchHistory", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "approveModerationSwitchChange", authztest.CrossTenant, authztest.RoleDenied)

	c := setupChat(t, chatOpts{real: true})
	ctx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)
	go func() { _ = c.pipe.Run(ctx) }() // a second instance would receive the same ew:chat:switches:changed notification
	time.Sleep(300 * time.Millisecond)
	sa, sa2 := c.platformStaff("system_admin"), c.platformStaff("system_admin")
	tA := c.tenantA
	// the instance serves tenants A and B (their catalogue views are loaded).
	view(t, c.pipe, c.tenantA)
	view(t, c.pipe, c.tenantB)
	effective := func(tenant uuid.UUID) []string {
		e, _ := c.pipe.Switches.Effective(context.Background(), tenant)
		return e.Disabled()
	}

	// F116-AC-21: reason missing or shorter than 10 characters → 422 MODERATION_SWITCH_REASON_REQUIRED (enable and disable).
	exp := c.clock.Now().Add(time.Hour)
	for name, body := range map[string]map[string]any{
		"disable without reason": switchBody(&tA, true, []string{"spam"}, "", &exp),
		"disable short reason":   switchBody(&tA, true, []string{"spam"}, "ngắn quá", &exp),
		"enable without reason":  switchBody(&tA, true, nil, "", nil),
	} {
		if r := c.setSwitches(sa, body); r.status != http.StatusUnprocessableEntity || r.code() != "MODERATION_SWITCH_REASON_REQUIRED" {
			t.Fatalf("%s: %d %s", name, r.status, r.raw)
		}
	}
	// scope tenant without tenant_id, unknown step, X-Tenant-Id of another tenant.
	noTenant := switchBody(nil, true, nil, reasonOK, nil)
	noTenant["scope"] = "tenant"
	if r := c.setSwitches(sa, noTenant); r.status != http.StatusUnprocessableEntity || r.code() != "VALIDATION_FAILED" {
		t.Fatalf("tenant scope without tenant_id: %d %s", r.status, r.raw)
	}
	unknown := switchBody(&tA, true, nil, reasonOK, nil)
	unknown["steps"].(map[string]any)["zqstep"] = false
	if r := c.setSwitches(sa, unknown); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown step: %d %s", r.status, r.raw)
	}
	if r := c.setSwitches(c.inTenant("system_admin", c.tenantB), switchBody(&tA, true, nil, reasonOK, nil),
		"X-Tenant-Id", c.tenantB.String()); r.status != http.StatusNotFound {
		t.Fatalf("X-Tenant-Id of another tenant: %d %s", r.status, r.raw)
	}
	if n := c.count(`SELECT count(*) FROM chat.moderation_switches WHERE tenant_id IS NOT NULL`) +
		c.count(`SELECT count(*) FROM chat.moderation_switch_requests`); n != 0 {
		t.Fatalf("a refused change was stored: %d", n)
	}

	// F116-AC-20: tenant A, spam off for 1 h (two System Admins) → stored, history, audit, event, in force at once for A only.
	a := c.disable(sa, sa2, &tA, true, []string{"spam"}, time.Hour)
	if a.body["scope"] != "tenant" || a.body["tenant_id"] != tA.String() || !slices.Equal(disabledSteps(a), []string{"spam"}) {
		t.Fatalf("approved state: %s", a.raw)
	}
	var enabledAll bool
	var stepsJSON, reason string
	var expires time.Time
	if err := c.su.QueryRowContext(ctx, `SELECT enabled_all, steps::text, reason, expires_at FROM chat.moderation_switches
		WHERE tenant_id = $1 AND deleted_at IS NULL`, tA).Scan(&enabledAll, &stepsJSON, &reason, &expires); err != nil {
		t.Fatal(err)
	}
	if !enabledAll || !strings.Contains(stepsJSON, `"spam": false`) || strings.Contains(stepsJSON, "pii") || reason != reasonOK ||
		!expires.Equal(c.clock.Now().Add(time.Hour)) {
		t.Fatalf("stored switch: %v %s %q %v", enabledAll, stepsJSON, reason, expires)
	}
	if got := effective(c.tenantA); !slices.Equal(got, []string{"spam"}) {
		t.Fatalf("tenant A effective: %v", got)
	}
	if got := effective(c.tenantB); len(got) != 0 {
		t.Fatalf("tenant B affected: %v", got)
	}
	// Re-enable (only enables): 200 at once, manual history with before/after.
	r := c.setSwitches(sa, switchBody(&tA, true, nil, "Hết sự kiện, bật lại kiểm tra spam", nil))
	if r.status != http.StatusOK || len(disabledSteps(r)) != 0 || r.body["configured"].(map[string]any)["expires_at"] != nil {
		t.Fatalf("re-enable: %d %s", r.status, r.raw)
	}
	if got := effective(c.tenantA); len(got) != 0 {
		t.Fatalf("tenant A after re-enable: %v", got)
	}
	if n := c.count(`SELECT count(*) FROM chat.moderation_switch_changes WHERE tenant_id = $1 AND source = 'manual'
		AND before->>'spam' = 'false' AND after->>'spam' = 'true' AND actor_id IS NOT NULL AND reason IS NOT NULL`, tA); n != 1 {
		t.Fatalf("manual history rows %d", n)
	}
	if n := c.count(`SELECT count(*) FROM chat.moderation_switch_changes WHERE tenant_id = $1 AND source = 'approved_request'
		AND before->>'spam' = 'true' AND after->>'spam' = 'false' AND request_id IS NOT NULL AND approved_by <> actor_id`, tA); n != 1 {
		t.Fatalf("approved history rows %d", n)
	}
	if seen := c.checkEvents("chat_policy.moderation_switch_changed"); seen["chat_policy.moderation_switch_changed"] != 2 {
		t.Fatalf("switch events: %v", seen)
	}
	if aud := c.audits(); !slices.Contains(aud, "chat.moderation_switch.requested") || !slices.Contains(aud, "chat.moderation_switch.changed") {
		t.Fatalf("audits: %v", aud)
	}

	// Platform level: bullying off for every tenant once approved; each instance drops the cached switches of the tenants
	// it serves on ew:chat:switches:changed — in force for A and B well within 60 s.
	c.disable(sa, sa2, nil, true, []string{"bullying"}, 2*time.Hour)
	start := time.Now()
	for !slices.Equal(effective(c.tenantA), []string{"bullying"}) || !slices.Equal(effective(c.tenantB), []string{"bullying"}) {
		if time.Since(start) > 60*time.Second {
			t.Fatalf("platform change not in force after 60 s: A %v B %v", effective(c.tenantA), effective(c.tenantB))
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Logf("platform change in force on the instance after %v", time.Since(start))

	// listModerationSwitches: platform + the tenants of tenant_ids (configured / effective / disabled / pending requests).
	pending := c.setSwitches(sa, switchBody(&tA, false, nil, reasonOK, ptrTime(c.clock.Now().Add(time.Hour))))
	r = c.admin(sa, http.MethodGet, "/admin/chat/moderation-switches?tenant_ids="+c.tenantA.String()+","+c.tenantB.String(), nil)
	if r.status != http.StatusOK {
		t.Fatalf("list: %d %s", r.status, r.raw)
	}
	plat := r.body["platform"].(map[string]any)
	tenants, _ := r.body["tenants"].([]any)
	if plat["scope"] != "platform" || len(tenants) != 2 || !slices.Contains(anyStrings(plat["disabled_steps"]), "bullying") {
		t.Fatalf("list: %s", r.raw)
	}
	for _, it := range tenants {
		st := it.(map[string]any)
		reqs, _ := st["pending_requests"].([]any)
		switch st["tenant_id"] {
		case tA.String():
			if st["configured"] == nil || len(reqs) != 1 || reqs[0].(map[string]any)["request_id"] != idOf(pending, "request_id") {
				t.Fatalf("tenant A state: %v", st)
			}
		case c.tenantB.String():
			if st["configured"] != nil || len(reqs) != 0 || !slices.Equal(anyStrings(st["disabled_steps"]), []string{"bullying"}) {
				t.Fatalf("tenant B state (inherits the platform): %v", st)
			}
		}
	}
	if r = c.admin(sa, http.MethodGet, "/admin/chat/moderation-switches", nil); len(r.body["tenants"].([]any)) != 0 {
		t.Fatalf("list without tenant_ids: %s", r.raw)
	}
	// listModerationSwitchHistory: platform changes, or the changes of tenant_ids (newest first, paged).
	if r = c.admin(sa, http.MethodGet, "/admin/chat/moderation-switches/history", nil); len(r.data()) != 1 ||
		r.data()[0]["source"] != "approved_request" || r.data()[0]["scope"] != "platform" {
		t.Fatalf("platform history: %s", r.raw)
	}
	r = c.admin(sa, http.MethodGet, "/admin/chat/moderation-switches/history?limit=1&tenant_ids="+c.tenantA.String(), nil)
	next, _ := r.body["page"].(map[string]any)["next_cursor"].(string)
	if len(r.data()) != 1 || r.data()[0]["source"] != "manual" || next == "" {
		t.Fatalf("tenant history page 1: %s", r.raw)
	}
	if r = c.admin(sa, http.MethodGet, "/admin/chat/moderation-switches/history?limit=1&cursor="+next+"&tenant_ids="+c.tenantA.String(),
		nil); len(r.data()) != 1 || r.data()[0]["source"] != "approved_request" || r.data()[0]["approved_by"] == nil {
		t.Fatalf("tenant history page 2: %s", r.raw)
	}

	// Idempotency: the same key replays the pending request (one row).
	key := newKey()
	b := switchBody(&c.tenantB, true, []string{"sensitive_terms"}, reasonOK, ptrTime(c.clock.Now().Add(time.Hour)))
	r1 := c.setSwitches(sa, b, "Idempotency-Key", key)
	r2 := c.setSwitches(sa, b, "Idempotency-Key", key)
	if r1.status != http.StatusAccepted || r2.status != http.StatusAccepted || idOf(r1, "request_id") != idOf(r2, "request_id") ||
		r2.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("idempotent replay: %d %s / %d %s", r1.status, r1.raw, r2.status, r2.raw)
	}
	if n := c.count(`SELECT count(*) FROM chat.moderation_switch_requests WHERE tenant_id = $1`, c.tenantB); n != 1 {
		t.Fatalf("requests of tenant B: %d", n)
	}

	// F116-AC-21, AC-18: without chat.policy.toggle every switch operation is refused and nothing changes.
	before := c.count(`SELECT count(*) FROM chat.moderation_switches`) + c.count(`SELECT count(*) FROM chat.moderation_switch_requests`) +
		c.count(`SELECT count(*) FROM chat.moderation_switch_changes`)
	student := c.student(c.tenantA, c.schoolA)
	tokens := map[string]string{"tenant_admin": c.staff(tA, "tenant_admin"), "school_admin": c.staff(tA, "school_admin"),
		"academic_manager": c.staff(tA, "academic_manager"), "safety_moderator": c.platformStaff("safety_moderator"),
		"content_admin": c.platformStaff("content_admin"), "game": student.token}
	ops := []struct{ method, path string }{{http.MethodGet, "/admin/chat/moderation-switches"},
		{http.MethodPut, "/admin/chat/moderation-switches"}, {http.MethodGet, "/admin/chat/moderation-switches/history"},
		{http.MethodPost, "/admin/chat/moderation-switches/requests/" + idOf(pending, "request_id") + ":approve"}}
	for who, tok := range tokens {
		for _, op := range ops {
			hdr := []string{"X-Tenant-Id", tA.String(), "Idempotency-Key", newKey()}
			if who == "safety_moderator" || who == "content_admin" {
				hdr = []string{"Idempotency-Key", newKey()}
			}
			r := c.admin(tok, op.method, op.path, switchBody(&tA, true, nil, reasonOK, nil), hdr...)
			want := "PERMISSION_DENIED"
			if who == "game" {
				want = "PORTAL_ACCESS_STAFF_ONLY"
			}
			if r.status != http.StatusForbidden || r.code() != want {
				t.Errorf("%s %s %s: %d %s", who, op.method, op.path, r.status, r.raw)
			}
		}
	}
	after := c.count(`SELECT count(*) FROM chat.moderation_switches`) + c.count(`SELECT count(*) FROM chat.moderation_switch_requests`) +
		c.count(`SELECT count(*) FROM chat.moderation_switch_changes`)
	if after != before {
		t.Fatal("a refused switch call changed the switches")
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

func anyStrings(v any) []string {
	var out []string
	list, _ := v.([]any)
	for _, s := range list {
		out = append(out, s.(string))
	}
	return out
}
