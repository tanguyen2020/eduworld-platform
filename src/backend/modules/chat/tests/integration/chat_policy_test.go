//go:build integration

package integration

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// validation is the validation block every enabled rule of chat-policy.schema.json v5 carries.
var validation = map[string]any{"strictness": "strict", "profanity": "mask", "pii": "block", "unknown_link": "block", "spam": "block",
	"bullying_suspected": "hold_for_review"}

// directRule is a school-level direct rule that only tightens the platform default (hours narrowed to from–to).
func directRule(from, to string) map[string]any {
	return map[string]any{"channel_type": "direct", "enabled": true, "allowed_kinds": []string{"text", "emoji", "quick"}, "max_text_length": 200,
		"rate": map[string]any{"messages_per_minute": 10, "burst": 2}, "validation": validation,
		"allowed_hours": []map[string]any{{"days": "mon-sun", "from": from, "to": to}}}
}

func policyBody(reason string, rules ...map[string]any) map[string]any {
	return map[string]any{"rules": rules, "reason": reason}
}

func at(h, m int) time.Time { return time.Date(2026, 11, 23, h, m, 0, 0, ict) } // a Monday, ICT

// T021, T023, T055 — F120-AC-06 (TC-CHAT-001), F120-AC-10, F120-AC-26 (TC-CHAT-048): School Admin of the school saves the
// school level with If-Match (new version, audit before/after, chat_policy.updated of its schema); the change applies to
// the next message at once on every instance (cache generation, ≤ 60 s): a narrowed allowed_hours refuses 20:30 with
// CHAT_NOT_ALLOWED and the preset quick_message_emote_only leaves only quick + emoji, even in party; a stale If-Match → 412,
// none → 428; the tenant and platform levels are written by their own roles; getEffectiveChatPolicy shows the intersection.
func TestChatPolicyPutLevels(t *testing.T) {
	c := setupPersistent(t, chatOpts{})
	c.school(c.tenantA, c.schoolA, nil)
	an, binh := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	dm := c.open(channel.Direct, an, binh)
	party := c.open(channel.Party, an, binh)
	admin := c.staffAt(c.tenantA, "school_admin", c.schoolA)
	path := "/tenant/chat-policies/school/" + c.schoolA.String()
	if r := c.portal(admin, c.tenantA, http.MethodGet, path, nil, nil); r.status != http.StatusNotFound {
		t.Fatalf("no level yet: %d %s", r.status, r.raw)
	}
	// 20:30 ICT: the platform hours (06:30–21:30) allow it; the effective policy is now cached.
	c.pace = 0
	c.clock.Set(at(20, 30))
	if r := c.sendWS(an, "c1", dm, "chao ban"); !r.OK {
		t.Fatalf("before the school level: %+v", r.Problem)
	}
	if r := c.portal(admin, c.tenantA, http.MethodPut, path, nil, policyBody("Thu hep gio chat cua truong")); r.status != http.StatusPreconditionRequired {
		t.Fatalf("no If-Match: %d %s", r.status, r.raw)
	}
	r := c.portal(admin, c.tenantA, http.MethodPut, path, map[string]string{"If-Match": `"0"`},
		policyBody("Thu hep gio chat cua truong", directRule("07:00", "20:00")))
	if r.status != http.StatusOK || r.header.Get("ETag") != `"1"` || r.body["version"] != float64(1) {
		t.Fatalf("put school level: %d %s", r.status, r.raw)
	}
	c.clock.Add(time.Minute)
	// delta F112-6: details.reopen_at = 07:00 ICT the next day (the first minute of the school hours outside the curfew).
	if r := c.sendWS(an, "c1", dm, "con thuc khong"); replyCode(r) != "CHAT_NOT_ALLOWED" || r.Problem.Details["reopen_at"] != "2026-11-24T00:00:00Z" {
		t.Fatalf("narrowed hours 20:31: %+v", r)
	}
	if r := c.sendREST(an, dm, map[string]any{"client_message_id": uuid.New(), "kind": "text", "body": "qua REST"}); r.status != 403 ||
		r.code() != "CHAT_NOT_ALLOWED" || r.body["reopen_at"] != "2026-11-24T00:00:00Z" {
		t.Fatalf("narrowed hours REST: %d %s", r.status, r.raw)
	}
	c.clock.Set(at(10, 0))
	if r := c.sendWS(an, "c1", dm, "chao buoi sang"); !r.OK {
		t.Fatalf("inside the school hours: %+v", r.Problem)
	}
	// Preset quick_message_emote_only for the whole school (every channel, gameplay included).
	body := policyBody("Chi quick message va emote cho ca truong", directRule("07:00", "20:00"))
	body["chat_policy_preset"] = "quick_message_emote_only"
	if r := c.portal(admin, c.tenantA, http.MethodPut, path, map[string]string{"If-Match": `"1"`}, body); r.status != http.StatusOK ||
		r.body["chat_policy_preset"] != "quick_message_emote_only" {
		t.Fatalf("preset: %d %s", r.status, r.raw)
	}
	for _, ch := range []any{dm, party} {
		if r := c.ws(an, "c1", "chat.message.send", map[string]any{"channel_id": ch, "client_message_id": newKey(), "kind": "text",
			"body": "xin chao"}); replyCode(r) != "CHAT_KIND_NOT_ALLOWED" {
			t.Fatalf("text under the preset: %+v", r)
		}
	}
	if r := c.portal(admin, c.tenantA, http.MethodPut, path, map[string]string{"If-Match": `"1"`}, body); r.status != http.StatusPreconditionFailed ||
		r.code() != "VERSION_MISMATCH" {
		t.Fatalf("stale If-Match: %d %s", r.status, r.raw)
	}
	g := c.portal(admin, c.tenantA, http.MethodGet, path, nil, nil)
	if g.status != http.StatusOK || g.body["version"] != float64(2) || g.header.Get("ETag") != `"2"` {
		t.Fatalf("get school level: %d %s", g.status, g.raw)
	}
	eff := c.portal(admin, c.tenantA, http.MethodGet, "/tenant/chat-policies:effective?school_id="+c.schoolA.String()+"&channel_type=direct", nil, nil)
	if eff.status != http.StatusOK || eff.body["enabled"] != true || eff.body["preset"] != "quick_message_emote_only" ||
		len(eff.body["sources"].([]any)) != 2 || strings.Contains(eff.raw, "categories") {
		t.Fatalf("effective: %d %s", eff.status, eff.raw)
	}
	// Audit before/after and the event (ids, versions; no message content).
	if n := countOf(c.audits(), "chat_policy.updated"); n != 2 {
		t.Fatalf("audits: %v", c.audits())
	}
	if n := c.checkEvents("chat_policy.updated")["chat_policy.updated"]; n != 2 {
		t.Fatalf("events chat_policy.updated: %d", n)
	}
	// delta F112-7: each event tells what the level leaves of direct (enabled school rule → true) for module identity.
	for _, ev := range c.events("chat_policy.updated") {
		if ev["direct_message_enabled"] != true {
			t.Fatalf("direct_message_enabled: %v", ev)
		}
	}
	if n := c.count(`SELECT count(*) FROM chat.chat_policies WHERE tenant_id = $1 AND scope_type = 'school' AND scope_id = $2
		AND deleted_at IS NULL AND version = 2 AND reason IS NOT NULL`, c.tenantA, c.schoolA); n != 1 {
		t.Fatalf("stored level: %d", n)
	}
	// The tenant level (tenant.settings.write) and the platform level (System Admin, internal gateway).
	tenantAdmin := c.staff(c.tenantA, "tenant_admin")
	tr := c.portal(tenantAdmin, c.tenantA, http.MethodPut, "/tenant/chat-policies/tenant/"+c.tenantA.String(), map[string]string{"If-Match": "0"},
		policyBody("Tat world chat toan tenant", map[string]any{"channel_type": "world", "enabled": false, "allowed_kinds": []string{}}))
	if tr.status != http.StatusOK || tr.body["scope_type"] != "tenant" {
		t.Fatalf("tenant level: %d %s", tr.status, tr.raw)
	}
	sa := c.systemAdmin()
	pg := c.do(http.MethodGet, "/admin/chat/policies/platform", sa, nil, nil)
	if pg.status != http.StatusOK {
		t.Fatalf("get platform: %d %s", pg.status, pg.raw)
	}
	rules, _ := pg.body["rules"].([]any)
	pp := c.do(http.MethodPut, "/admin/chat/policies/platform", sa, map[string]string{"If-Match": pg.header.Get("ETag")},
		map[string]any{"rules": rules, "global_rate": map[string]any{"messages_per_minute": 30, "burst": 5}, "world_scope": "school",
			"reason": "Cap nhat chinh sach platform"})
	if pp.status != http.StatusOK || pp.body["version"] != pg.body["version"].(float64)+1 {
		t.Fatalf("put platform: %d %s", pp.status, pp.raw)
	}
}

func countOf(list []string, s string) int {
	n := 0
	for _, x := range list {
		if x == s {
			n++
		}
	}
	return n
}

// T022, T025, T056 — F120-AC-07, AC-08 (TC-CHAT-010), AC-09 (TC-CHAT-018), AC-26, AC-27 (TC-CHAT-046), TC-TEN-024: a rule
// relaxing the platform → 422 CHAT_POLICY_RELAX_NOT_ALLOWED; a grade condition → 422 VALIDATION_FAILED; turning party off →
// 422 CHAT_POLICY_ALWAYS_ON; scope class → 422; allowed_hours into the curfew (06:00–22:00) → RELAX, 07:00–21:00 accepted;
// a preset by grade or at the tenant level → 422; reason over 500 characters or too many rules → 422. Nothing is stored
// for a refused write.
func TestChatPolicyValidation(t *testing.T) {
	c := setupPersistent(t, chatOpts{})
	c.school(c.tenantA, c.schoolA, nil)
	admin := c.staffAt(c.tenantA, "school_admin", c.schoolA)
	path := "/tenant/chat-policies/school/" + c.schoolA.String()
	put := func(body map[string]any) resp {
		return c.portal(admin, c.tenantA, http.MethodPut, path, map[string]string{"If-Match": "0"}, body)
	}
	relaxRate := directRule("07:00", "20:00")
	relaxRate["rate"] = map[string]any{"messages_per_minute": 60}
	grade := directRule("07:00", "20:00")
	grade["grade_band"] = "1-3"
	partyOff := map[string]any{"channel_type": "party", "enabled": false, "allowed_kinds": []string{}}
	for name, x := range map[string]struct {
		body   map[string]any
		status int
		code   string
	}{
		"relax rate":      {policyBody("Noi rong toc do chat", relaxRate), 422, "CHAT_POLICY_RELAX_NOT_ALLOWED"},
		"grade band":      {policyBody("Rieng cho khoi mot", grade), 422, "VALIDATION_FAILED"},
		"party off":       {policyBody("Tat chat party cua truong", partyOff), 422, "CHAT_POLICY_ALWAYS_ON"},
		"hours to 22:00":  {policyBody("Mo chat den muoi gio toi", directRule("06:00", "22:00")), 422, "CHAT_POLICY_RELAX_NOT_ALLOWED"},
		"preset by grade": {map[string]any{"rules": []any{directRule("07:00", "20:00")}, "reason": "Preset cho khoi ba", "chat_policy_preset": "quick_message_emote_only", "grade_level": 3}, 422, "VALIDATION_FAILED"},
		"reason too long": {policyBody(strings.Repeat("a", 501), directRule("07:00", "20:00")), 422, "VALIDATION_FAILED"},
		"too many rules": {policyBody("Qua nhieu rule trong mot cap", directRule("07:00", "20:00"), directRule("07:00", "20:00"),
			directRule("07:00", "20:00"), directRule("07:00", "20:00"), directRule("07:00", "20:00"), directRule("07:00", "20:00"),
			directRule("07:00", "20:00")), 422, "VALIDATION_FAILED"},
	} {
		if r := put(x.body); r.status != x.status || r.code() != x.code {
			t.Errorf("%s: %d %s", name, r.status, r.raw)
		}
	}
	if r := c.portal(admin, c.tenantA, http.MethodPut, "/tenant/chat-policies/class/"+c.schoolA.String(), map[string]string{"If-Match": "0"},
		policyBody("Chinh sach cap lop cu", directRule("07:00", "20:00"))); r.status != 422 || r.code() != "VALIDATION_FAILED" {
		t.Errorf("scope class: %d %s", r.status, r.raw)
	}
	tenantAdmin := c.staff(c.tenantA, "tenant_admin")
	tp := policyBody("Preset cap tenant khong co", directRule("07:00", "20:00"))
	tp["chat_policy_preset"] = "quick_message_emote_only"
	if r := c.portal(tenantAdmin, c.tenantA, http.MethodPut, "/tenant/chat-policies/tenant/"+c.tenantA.String(), map[string]string{"If-Match": "0"},
		tp); r.status != 422 || r.code() != "VALIDATION_FAILED" {
		t.Errorf("tenant preset: %d %s", r.status, r.raw)
	}
	if n := c.count(`SELECT count(*) FROM chat.chat_policies WHERE tenant_id = $1 AND deleted_at IS NULL`, c.tenantA); n != 0 {
		t.Fatalf("a refused write was stored: %d", n)
	}
	if r := put(policyBody("Gio chat bay gio den chin gio", directRule("07:00", "21:00"))); r.status != http.StatusOK {
		t.Fatalf("07:00-21:00: %d %s", r.status, r.raw)
	}
}

// T024 — F120-AC-11 (TC-TEN-003), TC-TEN-001: another school of the tenant → 403 PERMISSION_DENIED + authz.denied; a school
// of another tenant → 404; the tenant level without tenant.settings.write → 403; X-Tenant-Id of another tenant → 403
// TENANT_MISMATCH; a student → 403; the platform level and the common-tenant override refuse every tenant role (403, internal
// gateway + platform role).
func TestChatPolicyAuthz(t *testing.T) {
	c := setupPersistent(t, chatOpts{})
	c.school(c.tenantA, c.schoolA, nil)
	c.school(c.tenantA, c.schoolA2, nil)
	c.school(c.tenantB, c.schoolB, nil)
	admin := c.staffAt(c.tenantA, "school_admin", c.schoolA)
	body := policyBody("Thu hep gio chat cua truong", directRule("07:00", "20:00"))
	im := map[string]string{"If-Match": "0"}
	before := countOf(c.audits(), "authz.denied")
	if r := c.portal(admin, c.tenantA, http.MethodPut, "/tenant/chat-policies/school/"+c.schoolA2.String(), im, body); r.status != 403 ||
		r.code() != "PERMISSION_DENIED" {
		t.Fatalf("other school of the tenant: %d %s", r.status, r.raw)
	}
	if countOf(c.audits(), "authz.denied") <= before {
		t.Fatal("no authz.denied audit")
	}
	if r := c.portal(admin, c.tenantA, http.MethodGet, "/tenant/chat-policies/school/"+c.schoolA2.String(), nil, nil); r.status != 403 {
		t.Fatalf("read other school: %d %s", r.status, r.raw)
	}
	for _, m := range []string{http.MethodPut, http.MethodGet} {
		if r := c.portal(admin, c.tenantA, m, "/tenant/chat-policies/school/"+c.schoolB.String(), im, body); r.status != 404 {
			t.Fatalf("%s school of another tenant: %d %s", m, r.status, r.raw)
		}
	}
	if r := c.portal(admin, c.tenantA, http.MethodGet, "/tenant/chat-policies:effective?school_id="+c.schoolB.String()+"&channel_type=direct",
		nil, nil); r.status != 404 {
		t.Fatalf("effective of another tenant: %d %s", r.status, r.raw)
	}
	if r := c.portal(admin, c.tenantA, http.MethodPut, "/tenant/chat-policies/tenant/"+c.tenantA.String(), im, body); r.status != 403 {
		t.Fatalf("school admin on the tenant level: %d %s", r.status, r.raw)
	}
	tenantAdmin := c.staff(c.tenantA, "tenant_admin")
	if r := c.portal(tenantAdmin, c.tenantB, http.MethodPut, "/tenant/chat-policies/tenant/"+c.tenantB.String(), im, body); r.status != 403 ||
		r.code() != "TENANT_MISMATCH" {
		t.Fatalf("other tenant header: %d %s", r.status, r.raw)
	}
	if r := c.portal(tenantAdmin, c.tenantA, http.MethodPut, "/tenant/chat-policies/tenant/"+c.tenantB.String(), im, body); r.status != 404 {
		t.Fatalf("tenant level of another tenant: %d %s", r.status, r.raw)
	}
	an := c.student(c.tenantA, c.schoolA)
	if r := c.do(http.MethodPut, "/tenant/chat-policies/school/"+c.schoolA.String(), an.token, map[string]string{"X-Tenant-Id": c.tenantA.String(),
		"If-Match": "0"}, body); r.status != 403 || r.code() != "PORTAL_ACCESS_STAFF_ONLY" {
		t.Fatalf("student: %d %s", r.status, r.raw)
	}
	for _, tok := range []string{admin, tenantAdmin} {
		if r := c.do(http.MethodGet, "/admin/chat/policies/platform", tok, nil, nil); r.status != 403 {
			t.Fatalf("tenant role on the platform level: %d %s", r.status, r.raw)
		}
		if r := c.do(http.MethodPut, "/admin/chat/policies/platform", tok, map[string]string{"If-Match": "2"}, body); r.status != 403 {
			t.Fatalf("tenant role writes the platform level: %d %s", r.status, r.raw)
		}
		if r := c.portal(tok, c.tenantA, http.MethodPut, "/admin/chat/common-tenant-social", map[string]string{"Idempotency-Key": newKey()},
			map[string]any{"direct_enabled": true, "world_enabled": true, "area_free_text_enabled": true, "reason": "Bat lai cho tenant"}); r.status != 403 ||
			r.code() != "PERMISSION_DENIED" {
			t.Fatalf("tenant role re-enables common: %d %s", r.status, r.raw)
		}
	}
	authztest.Cover(t, "getChatPolicy", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "putChatPolicy", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getEffectiveChatPolicy", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getPlatformChatPolicy", authztest.RoleDenied)
	authztest.Cover(t, "putPlatformChatPolicy", authztest.RoleDenied)
	authztest.Cover(t, "getCommonTenantSocialOverride", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "putCommonTenantSocialOverride", authztest.CrossTenant, authztest.RoleDenied)
}
