//go:build integration

package integration

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
)

// Tests of the F122 module chat deltas T060–T066 (specs/122-portal-chat-safety, Clarifications 2026-11-23).

// platformUser registers a platform staff member of role and returns its portal token and user id.
func (e *env) platformUser(role string) (string, uuid.UUID) {
	d := authztest.Platform(role).PrincipalData
	sub := "kc-platform-" + uuid.NewString()
	e.res.Set(sub, d)
	return e.idp.Token(sub, httpxtest.PortalClient, time.Now()), d.UserID
}

// rejectSwitch calls rejectModerationSwitchChange with a new Idempotency-Key unless one is given in headers.
func (c *chatEnv) rejectSwitch(token, request string, body any, headers ...string) resp {
	c.t.Helper()
	return c.admin(token, http.MethodPost, "/admin/chat/moderation-switches/requests/"+request+":reject", body, headers...)
}

// T060 — F122-AC-27 (TC-CHAT-045): a second System Admin rejects a pending disabling request with a reason (≤ 500),
// Idempotency-Key and If-Match = the version of the request: rejected, nothing changes in force, no history row; audit
// with proposer, rejecter and reason; moderation_switch.change_rejected (schema-valid) in the same transaction. The
// proposer cannot reject (403 MODERATION_SWITCH_SELF_APPROVAL); no/stale If-Match → 428/412; no reason → 422; another
// level → 404; a decided request → 409; tenant staff and the Safety Moderator are refused (chat.policy.toggle).
func TestSwitchReject(t *testing.T) {
	authztest.Cover(t, "rejectModerationSwitchChange", authztest.CrossTenant, authztest.RoleDenied)
	c := setupChat(t, chatOpts{real: true})
	sa, proposer := c.platformUser("system_admin")
	sa2, rejecter := c.platformUser("system_admin")
	tB := c.tenantB
	hdrB := []string{"X-Tenant-Id", tB.String()}
	exp := c.clock.Now().Add(2 * time.Hour)
	r := c.setSwitches(sa, switchBody(&tB, true, []string{"spam"}, reasonOK, &exp))
	if r.status != http.StatusAccepted || r.body["version"] != float64(1) {
		t.Fatalf("pending request with its version: %d %s", r.status, r.raw)
	}
	req := idOf(r, "request_id")
	if l := c.admin(sa2, http.MethodGet, "/admin/chat/moderation-switches?tenant_ids="+tB.String(), nil); l.status != http.StatusOK ||
		!strings.Contains(l.raw, `"version":1`) {
		t.Fatalf("pending request lists its version: %d %s", l.status, l.raw)
	}
	reason := map[string]any{"reason": "Chưa có bằng chứng lọc nhầm, giữ kiểm duyệt"}
	match := append([]string{"If-Match", `W/"1"`}, hdrB...)

	// The proposer rejects → 403 MODERATION_SWITCH_SELF_APPROVAL; nothing changes.
	if x := c.rejectSwitch(sa, req, reason, match...); x.status != http.StatusForbidden || x.code() != "MODERATION_SWITCH_SELF_APPROVAL" {
		t.Fatalf("self reject: %d %s", x.status, x.raw)
	}
	// Preconditions and validation.
	if x := c.rejectSwitch(sa2, req, reason, hdrB...); x.status != http.StatusPreconditionRequired {
		t.Fatalf("no If-Match: %d %s", x.status, x.raw)
	}
	if x := c.rejectSwitch(sa2, req, reason, append([]string{"If-Match", `W/"7"`}, hdrB...)...); x.status != http.StatusPreconditionFailed ||
		x.code() != "VERSION_MISMATCH" {
		t.Fatalf("stale If-Match: %d %s", x.status, x.raw)
	}
	for name, body := range map[string]any{"missing": map[string]any{}, "blank": map[string]any{"reason": "   "},
		"501": map[string]any{"reason": strings.Repeat("a", 501)}} {
		if x := c.rejectSwitch(sa2, req, body, match...); x.status != http.StatusUnprocessableEntity {
			t.Fatalf("reason %s: %d %s", name, x.status, x.raw)
		}
	}
	// Another level: no X-Tenant-Id → platform requests only; another tenant → 404.
	if x := c.rejectSwitch(sa2, req, reason, "If-Match", `W/"1"`); x.status != http.StatusNotFound {
		t.Fatalf("reject without the tenant: %d %s", x.status, x.raw)
	}
	if x := c.rejectSwitch(sa2, req, reason, "If-Match", `W/"1"`, "X-Tenant-Id", c.tenantA.String()); x.status != http.StatusNotFound {
		t.Fatalf("reject in another tenant: %d %s", x.status, x.raw)
	}
	// Tenant staff and the Safety Moderator do not hold chat.policy.toggle.
	for name, token := range map[string]string{"tenant_admin": c.staff(tB, "tenant_admin"), "safety_moderator": c.platformStaff("safety_moderator")} {
		if x := c.rejectSwitch(token, req, reason, match...); x.status != http.StatusForbidden {
			t.Fatalf("%s rejects: %d %s", name, x.status, x.raw)
		}
	}
	if n := c.count(`SELECT count(*) FROM chat.moderation_switch_requests WHERE request_id = $1 AND status = 'pending_approval'`, req); n != 1 {
		t.Fatal("a refused call changed the request")
	}

	// A second System Admin rejects: 200 rejected, replayed with the same key; nothing in force, no history row.
	key := newKey()
	ok := c.rejectSwitch(sa2, req, reason, append([]string{"Idempotency-Key", key}, match...)...)
	if ok.status != http.StatusOK || ok.body["status"] != "rejected" || ok.body["decided_at"] == nil || ok.body["approved_by"] != nil ||
		ok.body["request_id"] != req {
		t.Fatalf("reject: %d %s", ok.status, ok.raw)
	}
	again := c.rejectSwitch(sa2, req, reason, append([]string{"Idempotency-Key", key}, match...)...)
	if again.status != http.StatusOK || again.header.Get("Idempotent-Replayed") != "true" || again.body["status"] != "rejected" {
		t.Fatalf("replay: %d %s", again.status, again.raw)
	}
	if n := c.count(`SELECT count(*) FROM chat.moderation_switch_requests WHERE request_id = $1 AND status = 'rejected'
		AND approved_by IS NULL AND decided_at IS NOT NULL AND updated_by = $2`, req, rejecter); n != 1 {
		t.Fatal("request row not rejected by the second System Admin")
	}
	if n := c.count(`SELECT count(*) FROM chat.moderation_switches WHERE tenant_id = $1`, tB); n != 0 {
		t.Fatal("a rejected request changed the switch")
	}
	if n := c.count(`SELECT count(*) FROM chat.moderation_switch_changes WHERE request_id = $1`, req); n != 0 {
		t.Fatal("a rejected request wrote history")
	}
	if e, _ := c.pipe.Switches.Effective(context.Background(), tB); len(e.Disabled()) != 0 {
		t.Fatalf("in force after rejection: %v", e.Disabled())
	}
	// Event (schema-valid) and audit: proposer, rejecter, reason.
	if n := c.checkEvents("moderation_switch.change_rejected")["moderation_switch.change_rejected"]; n != 1 {
		t.Fatalf("moderation_switch.change_rejected events: %d", n)
	}
	ev := c.events("moderation_switch.change_rejected")[0]
	if ev["request_id"] != req || ev["proposed_by"] != proposer.String() || ev["rejected_by"] != rejecter.String() ||
		ev["reason"] != reason["reason"] || ev["tenant_id"] != tB.String() || ev["scope"] != "tenant" {
		t.Fatalf("event payload: %v", ev)
	}
	audited := false
	for _, p := range c.events("audit.recorded") {
		meta, _ := p["metadata"].(map[string]any)
		if p["action"] == "chat.moderation_switch_request.rejected" && p["reason"] == reason["reason"] &&
			meta["proposed_by"] == proposer.String() && meta["rejected_by"] == rejecter.String() {
			audited = true
		}
	}
	if !audited {
		t.Fatal("rejection not audited with proposer, rejecter and reason")
	}
	// A decided request: rejecting or approving again → 409; it is no longer listed as pending.
	if x := c.rejectSwitch(sa2, req, reason, append([]string{"If-Match", `W/"2"`}, hdrB...)...); x.status != http.StatusConflict {
		t.Fatalf("reject twice: %d %s", x.status, x.raw)
	}
	if x := c.approveSwitch(sa2, req, hdrB...); x.status != http.StatusConflict {
		t.Fatalf("approve a rejected request: %d %s", x.status, x.raw)
	}
	if l := c.admin(sa2, http.MethodGet, "/admin/chat/moderation-switches?tenant_ids="+tB.String(), nil); strings.Contains(l.raw, req) {
		t.Fatalf("rejected request still pending: %s", l.raw)
	}
}

// T065 — F122-AC-13 (TC-CHAT-016): listModerationSwitches without tenant_ids returns disabled_tenant_count — the tenants
// with a value off at the tenant level still in force — counted in SQL, never listed; with tenant_ids there is no count;
// a lapsed expires_at counts as on.
func TestSwitchDisabledTenantCount(t *testing.T) {
	c := setupChat(t, chatOpts{real: true})
	sa := c.platformStaff("system_admin")
	list := func(q string) resp {
		return c.admin(sa, http.MethodGet, "/admin/chat/moderation-switches"+q, nil)
	}
	if r := list(""); r.status != http.StatusOK || r.body["disabled_tenant_count"] != float64(0) {
		t.Fatalf("nothing off: %d %s", r.status, r.raw)
	}
	c.tenantSwitch(c.tenantA, true, map[string]bool{"spam": false})
	c.tenantSwitch(c.tenantB, false, map[string]bool{})
	c.tenantSwitch(c.tenantU, true, map[string]bool{"spam": true, "bullying": true}) // configured, every value on
	r := list("")
	if r.status != http.StatusOK || r.body["disabled_tenant_count"] != float64(2) || len(r.body["tenants"].([]any)) != 0 {
		t.Fatalf("two tenants off: %d %s", r.status, r.raw)
	}
	if strings.Contains(r.raw, c.tenantA.String()) || strings.Contains(r.raw, c.tenantB.String()) {
		t.Fatalf("the count lists tenants: %s", r.raw)
	}
	if r := list("?tenant_ids=" + c.tenantA.String()); r.status != http.StatusOK || r.body["disabled_tenant_count"] != nil {
		t.Fatalf("with tenant_ids: %d %s", r.status, r.raw)
	}
	c.clock.Add(2 * time.Hour) // expires_at of tenantSwitch = now + 1 h
	if r := list(""); r.body["disabled_tenant_count"] != float64(0) {
		t.Fatalf("lapsed switches still counted: %s", r.raw)
	}
	if r := c.admin(c.platformStaff("safety_moderator"), http.MethodGet, "/admin/chat/moderation-switches", nil); r.status != http.StatusForbidden {
		t.Fatalf("safety moderator: %d %s", r.status, r.raw)
	}
}

// T063 — F122-AC-07 (TC-CHAT-015): listSensitiveTerms filters locale, match_type and status in SQL; the keyset pages
// keep the filter; a value outside the enums → 422.
func TestSensitiveTermFilters(t *testing.T) {
	c := setupChat(t, chatOpts{real: true})
	sa := c.platformStaff("system_admin")
	mk := func(term, match, locale string) string {
		in := termInput(catInsult, term, match)
		in["locale"] = locale
		r := c.admin(sa, http.MethodPost, "/admin/chat/sensitive-terms", in)
		if r.status != http.StatusCreated {
			t.Fatalf("create %s: %d %s", term, r.status, r.raw)
		}
		return idOf(r, "term_id")
	}
	viWord, enWord, viPhrase, viWord2 := mk("zqfa", "word", "vi"), mk("zqfb", "word", "en"), mk("zqfc dd", "phrase", "vi"), mk("zqfd", "word", "vi")
	c.exec(`UPDATE chat.sensitive_terms SET status = 'published' WHERE term_id = ANY($1::uuid[])`, "{"+viWord+","+enWord+"}")
	ids := func(q string) []string {
		r := c.admin(sa, http.MethodGet, "/admin/chat/sensitive-terms?"+q, nil)
		if r.status != http.StatusOK {
			t.Fatalf("%s: %d %s", q, r.status, r.raw)
		}
		var out []string
		for _, d := range r.data() {
			if strings.HasPrefix(d["term"].(string), "zqf") {
				out = append(out, d["term_id"].(string))
			}
		}
		return out
	}
	same := func(got []string, want ...string) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}
	for q, want := range map[string][]string{
		"locale=en":                        {enWord},
		"match_type=phrase":                {viPhrase},
		"status=draft&q=zqf":               {viPhrase, viWord2},
		"status=published&locale=vi&q=zqf": {viWord},
		"locale=vi&match_type=word&q=zqf":  {viWord, viWord2},
		"locale=vi&match_type=word&status=draft&category_id=" + catInsult.String(): {viWord2},
	} {
		if got := ids(q); !same(got, want...) {
			t.Errorf("%s: %v want %v", q, got, want)
		}
	}
	// Keyset pages keep the filter (limit 1: two pages of the vi words, then nothing).
	p1 := c.admin(sa, http.MethodGet, "/admin/chat/sensitive-terms?locale=vi&match_type=word&q=zqf&limit=1", nil)
	page, _ := p1.body["page"].(map[string]any)
	cur, _ := page["next_cursor"].(string)
	if len(p1.data()) != 1 || cur == "" || idOf(resp{body: p1.data()[0]}, "term_id") != viWord {
		t.Fatalf("first filtered page: %s", p1.raw)
	}
	p2 := c.admin(sa, http.MethodGet, "/admin/chat/sensitive-terms?locale=vi&match_type=word&q=zqf&limit=1&cursor="+url.QueryEscape(cur), nil)
	if len(p2.data()) != 1 || idOf(resp{body: p2.data()[0]}, "term_id") != viWord2 || p2.body["page"].(map[string]any)["has_more"] != false {
		t.Fatalf("second filtered page: %s", p2.raw)
	}
	for _, q := range []string{"locale=fr", "match_type=regex", "status=archived"} {
		if r := c.admin(sa, http.MethodGet, "/admin/chat/sensitive-terms?"+q, nil); r.status != http.StatusUnprocessableEntity && r.status != http.StatusBadRequest {
			t.Errorf("%s: %d %s", q, r.status, r.raw)
		}
	}
}

// T064 — F122-AC-01 (TC-CHAT-042): the cases carry tenant_name projected into ref.tenants from tenant.created/updated
// (never a synchronous identity call; the code until a name is projected); listModerationCaseTenants lists the tenants
// that have cases (keyset in SQL) under tenancy.Bypass(moderation_case) — audited and counted; tenant staff is refused and
// the Safety Moderator does not get listTenants (R20).
func TestModerationCaseTenants(t *testing.T) {
	authztest.Cover(t, "listModerationCaseTenants", authztest.CrossTenant, authztest.RoleDenied)
	s := setupSafety(t)
	id := consumer.IdentityProjection{P: command.Projection{Refs: persistence.Refs{}, Social: persistence.Social{}, Now: s.clock.Now}}
	project := func(typ string, tenant uuid.UUID, code, name string, version int) {
		t.Helper()
		p := map[string]any{"tenant_id": tenant, "code": code, "tenant_type": "school", "status": "active", "changed_fields": []string{},
			"source_version": version}
		if name != "" {
			p["name"] = name
		}
		if err := s.handle(t, id.Handle, typ, tenant, p); err != nil {
			t.Fatal(err)
		}
	}
	project("tenant.updated", s.tenantA, "th-demo-a", "Trường Tiểu học Demo A", 2)
	project("tenant.updated", s.tenantB, "thcs-demo-b", "", 2) // no name yet → the code
	if n := s.count(`SELECT count(*) FROM ref.tenants WHERE tenant_id = $1 AND name = 'Trường Tiểu học Demo A'`, s.tenantA); n != 1 {
		t.Fatal("tenant name not projected")
	}
	caseA, _, _ := s.reportedCase(s.student(s.tenantA, s.schoolA), s.student(s.tenantA, s.schoolA), "bullying")
	s.reportedCase(s.student(s.tenantB, s.schoolB), s.student(s.tenantB, s.schoolB), "spam")
	sm := s.moderator("safety_moderator", true)

	// tenant_name on the queue and on the case.
	names := map[string]string{}
	for _, d := range sm.get("/admin/moderation/cases").data() {
		names[d["tenant_id"].(string)] = str(d, "tenant_name")
	}
	if names[s.tenantA.String()] != "Trường Tiểu học Demo A" || names[s.tenantB.String()] != "thcs-demo-b" {
		t.Fatalf("tenant_name of the queue: %v", names)
	}
	if r := sm.get("/admin/moderation/cases/" + caseA.String()); r.status != http.StatusOK || r.body["tenant_name"] != "Trường Tiểu học Demo A" ||
		r.body["tenant_type"] != "school" {
		t.Fatalf("tenant_name of the case: %d %s", r.status, r.raw)
	}
	// A renamed tenant (newer source_version) is seen at once; an older event is ignored.
	project("tenant.updated", s.tenantA, "th-demo-a", "TH Demo A (mới)", 3)
	project("tenant.updated", s.tenantA, "th-demo-a", "Tên cũ", 1)
	if r := sm.get("/admin/moderation/cases/" + caseA.String()); r.body["tenant_name"] != "TH Demo A (mới)" {
		t.Fatalf("renamed tenant: %s", r.raw)
	}

	// listModerationCaseTenants: tenants A and B (U has no case), keyset pages, bypass audited and counted.
	before := bypassCount()
	all := sm.get("/admin/moderation/case-tenants")
	got := map[string]string{}
	for _, d := range all.data() {
		got[d["tenant_id"].(string)] = str(d, "tenant_name")
	}
	if len(all.data()) == 0 || all.data()[0]["tenant_type"] != "school" {
		t.Fatalf("tenant_type of a case tenant: %s", all.raw)
	}
	if all.status != http.StatusOK || len(got) != 2 || got[s.tenantA.String()] != "TH Demo A (mới)" || got[s.tenantB.String()] != "thcs-demo-b" {
		t.Fatalf("case tenants: %d %s", all.status, all.raw)
	}
	if bypassCount()-before < 1 {
		t.Fatal("bypass metric of listModerationCaseTenants")
	}
	audited := false
	for _, m := range s.auditMeta("cross_tenant.accessed") {
		if m["reason"] == "moderation_case" {
			audited = true
		}
	}
	if !audited {
		t.Fatal("audit cross_tenant.accessed moderation_case")
	}
	p1 := sm.get("/admin/moderation/case-tenants?limit=1")
	page, _ := p1.body["page"].(map[string]any)
	cur, _ := page["next_cursor"].(string)
	p2 := sm.get("/admin/moderation/case-tenants?limit=1&cursor=" + url.QueryEscape(cur))
	if len(p1.data()) != 1 || cur == "" || len(p2.data()) != 1 || p1.data()[0]["tenant_id"] == p2.data()[0]["tenant_id"] ||
		p2.body["page"].(map[string]any)["has_more"] != false {
		t.Fatalf("keyset pages: %s / %s", p1.raw, p2.raw)
	}
	// Tenant staff (even with a tenant header) and a platform role without moderation.case.read are refused.
	if r := s.do(http.MethodGet, "/admin/moderation/case-tenants", s.staff(s.tenantA, "tenant_admin"), map[string]string{"X-Tenant-Id": s.tenantA.String()}, nil); r.status != http.StatusForbidden {
		t.Fatalf("tenant staff: %d %s", r.status, r.raw)
	}
	if r := s.do(http.MethodGet, "/admin/moderation/case-tenants", s.platformStaff("content_admin"), nil, nil); r.status != http.StatusForbidden {
		t.Fatalf("content_admin: %d %s", r.status, r.raw)
	}
}

// T066 — F122-AC-18 (TC-CHAT-043): listSchoolSafetyNotifications filters created_from in SQL (the portal sends now − 30
// days); newest first, cursor pages keep the filter; an invalid date-time → 422.
func TestSchoolNotificationsCreatedFrom(t *testing.T) {
	s := setupSafety(t)
	caseID, an, _, chi := s.multiCase()
	sm := s.moderator("safety_moderator", false)
	path := "/admin/moderation/cases/" + caseID.String() + ":notify-school"
	summary := "Hoc sinh bi xin thong tin ca nhan trong chat, da tam han che nguoi gui."
	old := sm.post(s.tenantA, path, map[string]any{"school_id": s.schoolA, "student_ids": []uuid.UUID{an.id}, "summary": summary}, 0)
	if old.status != http.StatusCreated {
		t.Fatalf("notify A: %d %s", old.status, old.raw)
	}
	s.clock.Add(40 * 24 * time.Hour)
	recent := sm.post(s.tenantA, path, map[string]any{"school_id": s.schoolA2, "student_ids": []uuid.UUID{chi.id}, "summary": summary}, 0)
	if recent.status != http.StatusCreated {
		t.Fatalf("notify A2: %d %s", recent.status, recent.raw)
	}
	admin := s.schoolStaff(s.tenantA, "tenant_admin", nil)
	list := "/tenant/safety/school-notifications"
	ids := func(r resp) []string {
		var out []string
		for _, d := range r.data() {
			out = append(out, d["notification_id"].(string))
		}
		return out
	}
	if got := ids(s.tenantGet(admin, s.tenantA, list)); len(got) != 2 {
		t.Fatalf("without created_from: %v", got)
	}
	from := url.QueryEscape(s.clock.Now().Add(-30 * 24 * time.Hour).UTC().Format(time.RFC3339))
	r := s.tenantGet(admin, s.tenantA, list+"?created_from="+from)
	if got := ids(r); r.status != http.StatusOK || len(got) != 1 || got[0] != recent.body["notification_id"] {
		t.Fatalf("created_from now − 30 days: %d %s", r.status, r.raw)
	}
	if got := ids(s.tenantGet(admin, s.tenantA, list+"?acknowledged=false&limit=1&created_from="+from)); len(got) != 1 {
		t.Fatalf("created_from with acknowledged and limit: %v", got)
	}
	if r := s.tenantGet(admin, s.tenantA, list+"?created_from=hier"); r.status != http.StatusUnprocessableEntity && r.status != http.StatusBadRequest {
		t.Fatalf("invalid created_from: %d %s", r.status, r.raw)
	}
}

// T061 — F122-AC-23 (TC-SOC-008): staff of the school of a club (social.club.approve) lists its members (keyset in SQL,
// nicknames only) and removes one with a reason: membership ended (removed), member_count, the fixed team channel left in
// the same transaction, audit with reason and staff, club.member_left (removed); replay with the same key; the owner
// cannot be removed; another school of the tenant or another tenant → 404; a student or staff without the permission → 403.
func TestSchoolClubMembersStaff(t *testing.T) {
	authztest.Cover(t, "listSchoolClubMembers", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "removeSchoolClubMember", authztest.CrossTenant, authztest.RoleDenied)
	c := setupPersistent(t, chatOpts{})
	owner, binh, chi := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	clubID, ch := c.mustClub(owner, "CLB Co vua")
	c.clock.Add(time.Second)
	c.joinClub(owner, binh, clubID)
	c.clock.Add(time.Second)
	c.joinClub(owner, chi, clubID)
	admin := c.staffAt(c.tenantA, "school_admin", c.schoolA)
	members := "/tenant/social/clubs/" + clubID.String() + "/members"
	remove := func(token string, tenant uuid.UUID, user uuid.UUID, key string, body any) resp {
		return c.portal(token, tenant, http.MethodPost, members+"/"+user.String()+":remove", map[string]string{"Idempotency-Key": key}, body)
	}

	// Keyset pages (limit 2): owner, binh | chi.
	p1 := c.portal(admin, c.tenantA, http.MethodGet, members+"?limit=2", nil, nil)
	page, _ := p1.body["page"].(map[string]any)
	cur, _ := page["next_cursor"].(string)
	if p1.status != http.StatusOK || len(p1.data()) != 2 || cur == "" || p1.data()[0]["user_id"] != owner.id.String() ||
		p1.data()[0]["role"] != "owner" || p1.data()[1]["nickname"] != binh.nickname {
		t.Fatalf("first page: %d %s", p1.status, p1.raw)
	}
	p2 := c.portal(admin, c.tenantA, http.MethodGet, members+"?limit=2&cursor="+url.QueryEscape(cur), nil, nil)
	if len(p2.data()) != 1 || p2.data()[0]["user_id"] != chi.id.String() || p2.body["page"].(map[string]any)["has_more"] != false {
		t.Fatalf("second page: %s", p2.raw)
	}
	pii(t, p1)

	// Scope: another school of the tenant, another tenant → 404; a student → 403; staff without the permission → 403.
	other := c.staffAt(c.tenantA, "school_admin", c.schoolA2)
	if r := c.portal(other, c.tenantA, http.MethodGet, members, nil, nil); r.status != http.StatusNotFound {
		t.Fatalf("other school list: %d %s", r.status, r.raw)
	}
	if r := remove(other, c.tenantA, binh.id, newKey(), map[string]any{"reason": "Vi pham noi quy club"}); r.status != http.StatusNotFound {
		t.Fatalf("other school remove: %d %s", r.status, r.raw)
	}
	foreign := c.staffAt(c.tenantB, "school_admin", c.schoolB)
	if r := c.portal(foreign, c.tenantB, http.MethodGet, members, nil, nil); r.status != http.StatusNotFound {
		t.Fatalf("other tenant list: %d %s", r.status, r.raw)
	}
	if r := c.call(binh, http.MethodGet, members, "", nil); r.status != http.StatusForbidden && r.status != http.StatusUnauthorized {
		t.Fatalf("student list: %d %s", r.status, r.raw)
	}
	if r := c.portal(c.noRole(c.tenantA), c.tenantA, http.MethodGet, members, nil, nil); r.status != http.StatusForbidden {
		t.Fatalf("no permission: %d %s", r.status, r.raw)
	}
	tenantAdmin := c.staff(c.tenantA, "tenant_admin") // tenant_admin does not hold social.club.approve
	if r := remove(tenantAdmin, c.tenantA, binh.id, newKey(), map[string]any{"reason": "Vi pham noi quy club"}); r.status != http.StatusForbidden {
		t.Fatalf("tenant admin remove: %d %s", r.status, r.raw)
	}
	// Reason required; the owner is never removed here.
	for name, body := range map[string]any{"missing": map[string]any{}, "blank": map[string]any{"reason": "  "},
		"501": map[string]any{"reason": strings.Repeat("a", 501)}} {
		if r := remove(admin, c.tenantA, binh.id, newKey(), body); r.status != http.StatusUnprocessableEntity {
			t.Fatalf("reason %s: %d %s", name, r.status, r.raw)
		}
	}
	if r := remove(admin, c.tenantA, owner.id, newKey(), map[string]any{"reason": "Thu go chu club"}); r.status != http.StatusNotFound {
		t.Fatalf("remove the owner: %d %s", r.status, r.raw)
	}

	// Remove binh: membership, member_count, team channel — one transaction; audit, event; replay.
	staffReason := "Vi pham noi quy club nhieu lan"
	key := newKey()
	if r := remove(admin, c.tenantA, binh.id, key, map[string]any{"reason": staffReason}); r.status != http.StatusNoContent {
		t.Fatalf("remove: %d %s", r.status, r.raw)
	}
	if r := remove(admin, c.tenantA, binh.id, key, map[string]any{"reason": staffReason}); r.status != http.StatusNoContent ||
		r.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %d %s", r.status, r.raw)
	}
	if n := c.count(`SELECT count(*) FROM social.club_members WHERE club_id = $1 AND user_id = $2 AND left_at IS NOT NULL
		AND left_reason = 'removed'`, clubID, binh.id); n != 1 {
		t.Fatal("membership not ended as removed")
	}
	if n := c.count(`SELECT count(*) FROM social.clubs WHERE club_id = $1 AND member_count = 2`, clubID); n != 1 {
		t.Fatal("member_count not decremented")
	}
	if n := c.count(`SELECT count(*) FROM chat.channel_members WHERE channel_id = $1 AND user_id = $2 AND left_at IS NOT NULL`, ch, binh.id); n != 1 {
		t.Fatal("removed member still in the team channel")
	}
	if r := c.call(binh, http.MethodGet, "/chat/channels/"+ch.String()+"/messages", "", nil); r.status != http.StatusNotFound {
		t.Fatalf("removed member reads the club channel: %d %s", r.status, r.raw)
	}
	left := c.events("club.member_left")
	if len(left) != 1 || left[0]["reason"] != "removed" || left[0]["user_id"] != binh.id.String() {
		t.Fatalf("club.member_left: %v", left)
	}
	audited := false
	for _, p := range c.events("audit.recorded") {
		meta, _ := p["metadata"].(map[string]any)
		if p["action"] == "club.member_removed" && p["reason"] == staffReason && meta["user_id"] == binh.id.String() && meta["by"] == "staff" {
			audited = true
		}
	}
	if !audited {
		t.Fatal("staff removal not audited with the reason")
	}
	if n := c.checkEvents(clubEvents...)["club.member_left"]; n != 1 {
		t.Fatalf("club.member_left schema: %d", n)
	}
	if r := remove(admin, c.tenantA, binh.id, newKey(), map[string]any{"reason": staffReason}); r.status != http.StatusNotFound {
		t.Fatalf("remove again: %d %s", r.status, r.raw)
	}
	if r := c.portal(admin, c.tenantA, http.MethodGet, members, nil, nil); len(r.data()) != 2 {
		t.Fatalf("members after removal: %s", r.raw)
	}
}

// Owner rule 2026-11-23 (API_DESIGN §5, CODE_PLAN §4.3): listSensitiveCategories and listTenantSensitiveCategories are
// keyset pages (code, category_id) read in SQL — every page walked gives each live category once, in order; the tenant
// list carries moderation_switches on every page; a forged cursor is refused.
func TestSensitiveCategoryPages(t *testing.T) {
	c := setupChat(t, chatOpts{real: true})
	sa := c.platformStaff("system_admin")
	total := c.count(`SELECT count(*) FROM chat.sensitive_term_categories WHERE tenant_id IS NULL AND deleted_at IS NULL`)
	if total < 3 {
		t.Fatalf("seeded categories: %d", total)
	}
	walk := func(get func(cursor string) resp) []string {
		t.Helper()
		var codes []string
		cursor := ""
		for i := 0; i < total+1; i++ {
			r := get(cursor)
			if r.status != http.StatusOK || len(r.data()) > 2 {
				t.Fatalf("page: %d %s", r.status, r.raw)
			}
			for _, d := range r.data() {
				codes = append(codes, str(d, "code"))
			}
			page, _ := r.body["page"].(map[string]any)
			next, _ := page["next_cursor"].(string)
			if page["has_more"] != true {
				return codes
			}
			cursor = next
		}
		t.Fatal("pages never end")
		return nil
	}
	platform := walk(func(cursor string) resp {
		return c.admin(sa, http.MethodGet, "/admin/chat/sensitive-categories?limit=2&cursor="+url.QueryEscape(cursor), nil)
	})
	admin := c.staff(c.tenantA, "tenant_admin")
	tenant := walk(func(cursor string) resp {
		r := c.do(http.MethodGet, "/tenant/chat/categories?limit=2&cursor="+url.QueryEscape(cursor), admin, map[string]string{"X-Tenant-Id": c.tenantA.String()}, nil)
		if r.body["moderation_switches"] == nil {
			t.Fatalf("moderation_switches missing on a page: %s", r.raw)
		}
		return r
	})
	for name, codes := range map[string][]string{"platform": platform, "tenant": tenant} {
		if len(codes) != total {
			t.Fatalf("%s: %d categories of %d", name, len(codes), total)
		}
		for i := 1; i < len(codes); i++ {
			if codes[i-1] >= codes[i] {
				t.Fatalf("%s: not in code order: %v", name, codes)
			}
		}
	}
	if r := c.admin(sa, http.MethodGet, "/admin/chat/sensitive-categories?cursor=forged", nil); r.status != http.StatusBadRequest {
		t.Fatalf("forged cursor: %d %s", r.status, r.raw)
	}
}
