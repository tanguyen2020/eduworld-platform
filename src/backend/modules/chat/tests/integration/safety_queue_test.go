//go:build integration

package integration

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// reportedCase makes reporter report a message of sender (seq 13 of 25 placeholder messages of a class channel) and
// returns the case, the channel and the message.
func (s *safetyEnv) reportedCase(sender, reporter student, reason string) (uuid.UUID, uuid.UUID, uuid.UUID) {
	s.t.Helper()
	ch := s.open(channel.Class, sender, reporter)
	var target uuid.UUID
	for seq := int64(1); seq <= 25; seq++ {
		who := reporter
		if seq%2 == 1 {
			who = sender
		}
		id := s.msg(ch, false, who, seq, "zqtin "+strconv.FormatInt(seq, 10))
		if seq == 13 {
			target = id
		}
	}
	r := s.report(reporter, newKey(), "vi", map[string]any{"target_type": "message", "target_id": target, "reason": reason})
	if r.status != http.StatusCreated {
		s.t.Fatalf("report: %d %s", r.status, r.raw)
	}
	return s.caseOf(target), ch, target
}

func bypassCount() float64 {
	return testutil.ToFloat64(tenancy.BypassCounter().WithLabelValues("chat", string(tenancy.ReasonModerationCase)))
}

func caseIDs(r resp) []string {
	var out []string
	for _, d := range r.data() {
		out = append(out, d["case_id"].(string))
	}
	return out
}

func contains(list []string, x string) bool {
	for _, v := range list {
		if v == x {
			return true
		}
	}
	return false
}

// auditMeta returns the metadata of the audit.recorded events of action.
func (s *safetyEnv) auditMeta(action string) []map[string]any {
	var out []map[string]any
	for _, p := range s.events("audit.recorded") {
		if p["action"] == action {
			m, _ := p["metadata"].(map[string]any)
			out = append(out, m)
		}
	}
	return out
}

// T017 / F121-AC-06 (TC-CHAT-042, TC-TEN-023): the Safety Team lists the cases of every tenant without X-Tenant-Id —
// filters tenant_ids, status, severity, assignee, category; keyset pages; each list is a tenancy.Bypass(moderation_case)
// (audit cross_tenant.accessed + eduworld_tenant_filter_bypass_total); metadata only, no message content.
func TestSafetyQueueCrossTenant(t *testing.T) {
	authztest.Cover(t, "listModerationCases", authztest.CrossTenant, authztest.RoleDenied)
	s := setupSafety(t)
	caseA, _, _ := s.reportedCase(s.student(s.tenantA, s.schoolA), s.student(s.tenantA, s.schoolA), "asked_personal_info")
	caseB, _, _ := s.reportedCase(s.student(s.tenantB, s.schoolB), s.student(s.tenantB, s.schoolB), "bullying")
	sm := s.moderator("safety_moderator", false)
	before := bypassCount()
	r := sm.get("/admin/moderation/cases")
	if r.status != http.StatusOK || !contains(caseIDs(r), caseA.String()) || !contains(caseIDs(r), caseB.String()) {
		t.Fatalf("queue of every tenant: %d %s", r.status, r.raw)
	}
	if strings.Contains(r.raw, "zqtin") || strings.Contains(r.raw, `"body"`) {
		t.Fatalf("queue leaks content: %s", r.raw)
	}
	if got := bypassCount() - before; got < 1 {
		t.Fatalf("bypass metric: %v", got)
	}
	found := false
	for _, m := range s.auditMeta("cross_tenant.accessed") {
		if m["reason"] == "moderation_case" {
			found = true
		}
	}
	if !found {
		t.Fatal("audit cross_tenant.accessed moderation_case")
	}
	for q, want := range map[string]string{"tenant_ids=" + s.tenantA.String(): caseA.String(), "severity=p2": caseB.String(),
		"tenant_ids=" + s.tenantB.String() + "&status=open": caseB.String(), "assignee=unassigned&category_code=asked_personal_info": caseA.String()} {
		got := caseIDs(sm.get("/admin/moderation/cases?" + q))
		if len(got) != 1 || got[0] != want {
			t.Errorf("%s: %v", q, got)
		}
	}
	if got := caseIDs(sm.get("/admin/moderation/cases?assignee=me")); len(got) != 0 {
		t.Errorf("assignee=me: %v", got)
	}
	p1 := sm.get("/admin/moderation/cases?limit=1")
	page, _ := p1.body["page"].(map[string]any)
	cursor, _ := page["next_cursor"].(string)
	if len(p1.data()) != 1 || cursor == "" || caseIDs(p1)[0] != caseA.String() {
		t.Fatalf("first page (p1 first): %s", p1.raw)
	}
	if p2 := sm.get("/admin/moderation/cases?limit=1&cursor=" + cursor); len(p2.data()) != 1 || caseIDs(p2)[0] != caseB.String() {
		t.Fatalf("second page: %s", p2.raw)
	}
	// System Admin reads the queue too (moderation.*).
	if r := s.moderator("system_admin", false).get("/admin/moderation/cases"); r.status != http.StatusOK || len(r.data()) != 2 {
		t.Fatalf("system_admin: %d %s", r.status, r.raw)
	}
}

// T018–T019 / F121-AC-07, AC-08 (TC-CHAT-040, TC-CHAT-007): only the assignee or a team lead reads a case — details,
// the evidence and ±200 messages of context (original content), q searches inside the case only; every read is audited
// with the target tenant; a moderator claims an open case (in_review, moderation.case_assigned), another one cannot take
// it, the team lead reassigns; X-Tenant-Id of another tenant is TENANT_MISMATCH; a resolved case is 409.
func TestSafetyCaseReadAndAssign(t *testing.T) {
	authztest.Cover(t, "getModerationCase", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listModerationCaseMessages", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "assignModerationCase", authztest.CrossTenant, authztest.RoleDenied)
	s := setupSafety(t)
	an, binh := s.student(s.tenantA, s.schoolA), s.student(s.tenantA, s.schoolA)
	caseID, ch, target := s.reportedCase(an, binh, "asked_personal_info")
	for seq := int64(26); seq <= 60; seq++ { // context beyond the ±10 snapshot
		s.msg(ch, false, an, seq, "zqsau "+strconv.FormatInt(seq, 10))
	}
	sm1, sm2, lead := s.moderator("safety_moderator", false), s.moderator("safety_moderator", false), s.moderator("safety_moderator", true)
	path := "/admin/moderation/cases/" + caseID.String()
	if r := sm1.get(path); r.status != http.StatusForbidden || r.code() != "PERMISSION_DENIED" {
		t.Fatalf("unassigned read: %d %s", r.status, r.raw)
	}
	if r := sm1.get(path + "/messages"); r.status != http.StatusForbidden {
		t.Fatalf("unassigned messages: %d %s", r.status, r.raw)
	}
	if r := sm1.post(s.tenantB, path+":assign", map[string]any{"assignee_moderator_id": sm1.id}, 0); r.status != http.StatusForbidden ||
		r.code() != "TENANT_MISMATCH" {
		t.Fatalf("X-Tenant-Id of another tenant: %d %s", r.status, r.raw)
	}
	if r := sm1.post(s.tenantA, path+":assign", map[string]any{"assignee_moderator_id": sm2.id}, 0); r.status != http.StatusForbidden {
		t.Fatalf("assign another without lead: %d %s", r.status, r.raw)
	}
	r := sm1.post(s.tenantA, path+":assign", map[string]any{"assignee_moderator_id": sm1.id}, 0)
	if r.status != http.StatusOK || r.body["status"] != "in_review" || r.body["assignee_moderator_id"] != sm1.id.String() {
		t.Fatalf("claim: %d %s", r.status, r.raw)
	}
	if r := sm2.post(s.tenantA, path+":assign", map[string]any{"assignee_moderator_id": sm2.id}, 0); r.status != http.StatusForbidden {
		t.Fatalf("take a claimed case: %d %s", r.status, r.raw)
	}
	if n := s.checkEvents("moderation.case_assigned")["moderation.case_assigned"]; n != 1 {
		t.Fatalf("moderation.case_assigned: %d", n)
	}
	d := sm1.get(path)
	if d.status != http.StatusOK || d.header.Get("ETag") == "" || len(d.body["reports"].([]any)) != 1 || len(d.body["evidence"].([]any)) != 1 {
		t.Fatalf("assignee reads: %d %s", d.status, d.raw)
	}
	if acts := d.body["actions"].([]any); len(acts) < 3 {
		t.Fatalf("history report/hold/assign: %v", acts)
	}
	if _, ok := d.body["support_guide"]; ok {
		t.Fatal("no support guide outside self-harm")
	}
	if r := sm2.get(path); r.status != http.StatusForbidden {
		t.Fatalf("other moderator reads: %d", r.status)
	}
	m := sm1.get(path + "/messages?limit=100")
	if m.status != http.StatusOK || len(m.data()) != 60 {
		t.Fatalf("snapshot + context: %d %d %s", m.status, len(m.data()), m.raw[:min(len(m.raw), 400)])
	}
	inSnap := 0
	for _, it := range m.data() {
		if it["in_snapshot"] == true {
			inSnap++
		}
		if it["message_id"] == target.String() && it["body"] != "zqtin 13" {
			t.Fatalf("original body: %v", it)
		}
	}
	if inSnap != 21 {
		t.Fatalf("snapshot messages: %d", inSnap)
	}
	if q := sm1.get(path + "/messages?q=ZQTIN%2013"); q.status != http.StatusOK || len(q.data()) != 1 || q.data()[0]["message_id"] != target.String() {
		t.Fatalf("search inside the case: %s", q.raw)
	}
	for _, action := range []string{"moderation.case.viewed", "moderation.case.messages_viewed", "moderation.case.searched"} {
		metas := s.auditMeta(action)
		if len(metas) == 0 || metas[0]["tenant_id"] != s.tenantA.String() || metas[0]["case_id"] != caseID.String() {
			t.Errorf("audit %s: %v", action, metas)
		}
	}
	s.noContent("zqtin 13", "ZQTIN")
	if r := lead.get(path); r.status != http.StatusOK {
		t.Fatalf("team lead reads: %d", r.status)
	}
	if r := lead.post(s.tenantA, path+":assign", map[string]any{"assignee_moderator_id": sm2.id}, 0); r.status != http.StatusOK ||
		r.body["assignee_moderator_id"] != sm2.id.String() {
		t.Fatalf("lead reassigns: %d %s", r.status, r.raw)
	}
	if r := sm2.get(path); r.status != http.StatusOK {
		t.Fatalf("new assignee reads: %d", r.status)
	}
	if r := sm1.get("/admin/moderation/cases/" + uuid.NewString()); r.status != http.StatusNotFound {
		t.Fatalf("unknown case: %d", r.status)
	}
	v := int64(sm2.get(path).body["version"].(float64))
	if r := sm2.post(s.tenantA, path+":act", map[string]any{"action": "close", "note": "da xu ly"}, v); r.status != http.StatusOK {
		t.Fatalf("close: %d %s", r.status, r.raw)
	}
	if r := lead.post(s.tenantA, path+":assign", map[string]any{"assignee_moderator_id": lead.id}, 0); r.status != http.StatusConflict ||
		r.code() != "MODERATION_CASE_CLOSED" {
		t.Fatalf("assign resolved: %d %s", r.status, r.raw)
	}
}

// T020–T021 / F121-AC-09, AC-10 (TC-CHAT-042, TC-CMS-AUTHZ-02, TC-CMS-WS-02): tenant roles and students have no access
// to /admin/moderation/* (403, audited); a safety_moderator token is refused outside moderation (player routes, the
// switches of System Admin); without MFA → 403 MFA_REQUIRED (audited); a game token → 403.
func TestSafetyAuthzMatrix(t *testing.T) {
	authztest.Cover(t, "getModerationCase", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "actOnModerationCase", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listSanctions", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createSanction", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "revokeSanction", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "notifySchoolForCase", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listSchoolSafetyNotificationsPlatform", authztest.CrossTenant, authztest.RoleDenied)
	s := setupSafety(t)
	an, binh := s.student(s.tenantA, s.schoolA), s.student(s.tenantA, s.schoolA)
	caseID, _, _ := s.reportedCase(an, binh, "bullying")
	tenantHeaders := func() map[string]string {
		return map[string]string{"X-Tenant-Id": s.tenantA.String(), "Idempotency-Key": newKey(), "If-Match": `W/"1"`}
	}
	admin := []struct{ method, path string }{
		{http.MethodGet, "/admin/moderation/cases"}, {http.MethodGet, "/admin/moderation/cases/" + caseID.String()},
		{http.MethodGet, "/admin/moderation/cases/" + caseID.String() + "/messages"}, {http.MethodGet, "/admin/moderation/sanctions"},
		{http.MethodGet, "/admin/safety/school-notifications"},
		{http.MethodPost, "/admin/moderation/cases/" + caseID.String() + ":assign"},
		{http.MethodPost, "/admin/moderation/cases/" + caseID.String() + ":act"},
		{http.MethodPost, "/admin/moderation/sanctions"},
	}
	bodies := map[string]any{":assign": map[string]any{"assignee_moderator_id": uuid.New()}, ":act": map[string]any{"action": "warn", "note": "abc"},
		"sanctions": map[string]any{"user_id": an.id, "sanction_type": "mute", "reason": "abcd"}}
	bodyOf := func(p string) any {
		for k, v := range bodies {
			if strings.HasSuffix(p, k) {
				return v
			}
		}
		return nil
	}
	for role, token := range map[string]string{"school_admin": s.staff(s.tenantA, "school_admin"), "tenant_admin": s.staff(s.tenantA, "tenant_admin"),
		"student": an.token} {
		for _, rq := range admin {
			r := s.do(rq.method, rq.path, token, tenantHeaders(), bodyOf(rq.path))
			if r.status != http.StatusForbidden {
				t.Errorf("%s %s %s: %d %s", role, rq.method, rq.path, r.status, r.raw)
			}
		}
	}
	sm := s.moderator("safety_moderator", false)
	if r := s.do(http.MethodGet, "/admin/chat/moderation-switches", sm.token, nil, nil); r.status != http.StatusForbidden {
		t.Fatalf("safety_moderator on the switches: %d", r.status)
	}
	if r := s.do(http.MethodGet, "/social/friends", sm.in(s.tenantA), map[string]string{"X-Tenant-Id": s.tenantA.String()}, nil); r.status != http.StatusForbidden {
		t.Fatalf("safety_moderator on a player route: %d", r.status)
	}
	// No MFA: 403 MFA_REQUIRED, audited.
	sub := "kc-nomfa-" + uuid.NewString()
	s.res.Set(sub, sm.data)
	noMFA := s.idp.Token(sub, httpxtest.PortalClient, false, time.Now())
	if r := s.do(http.MethodGet, "/admin/moderation/cases", noMFA, nil, nil); r.status != http.StatusForbidden || r.code() != "MFA_REQUIRED" {
		t.Fatalf("no MFA: %d %s", r.status, r.raw)
	}
	game := s.idp.Token(sub, httpxtest.GameClient, true, time.Now())
	if r := s.do(http.MethodGet, "/admin/moderation/cases", game, nil, nil); r.status != http.StatusForbidden {
		t.Fatalf("game azp: %d %s", r.status, r.raw)
	}
	denied := 0
	for _, p := range s.events("audit.recorded") {
		if p["action"] == "authz.denied" {
			denied++
		}
	}
	mfa := false
	for _, m := range s.auditMeta("authz.denied") {
		if m["why"] == "mfa" {
			mfa = true
		}
	}
	if denied == 0 || !mfa {
		t.Fatalf("authz.denied audits: %d mfa=%v", denied, mfa)
	}
	if n := s.count(`SELECT count(*) FROM chat.moderation_actions WHERE case_id = $1 AND action NOT IN ('report','hold')`, caseID); n != 0 {
		t.Fatalf("a refused write changed the case: %d", n)
	}
}
