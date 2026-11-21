//go:build integration

package integration

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// partyCase stores 5 placeholder messages of sender in a party channel (gameplay) and makes reporter report the third.
func (s *safetyEnv) partyCase(sender, reporter student, reason string) (uuid.UUID, uuid.UUID, uuid.UUID) {
	s.t.Helper()
	ch := s.open(channel.Party, sender, reporter)
	var target uuid.UUID
	for seq := int64(1); seq <= 5; seq++ {
		id := s.msg(ch, true, sender, seq, "zqparty "+strconv.FormatInt(seq, 10))
		if seq == 3 {
			target = id
		}
	}
	s.exec(`UPDATE chat.channels SET last_seq = 5 WHERE channel_id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, ch, sender.tenant)
	r := s.report(reporter, newKey(), "vi", map[string]any{"target_type": "message", "target_id": target, "reason": reason})
	if r.status != http.StatusCreated {
		s.t.Fatalf("report: %d %s", r.status, r.raw)
	}
	return s.caseOf(target), ch, target
}

func version(r resp) int64 {
	v, _ := r.body["version"].(float64)
	return int64(v)
}

// T024–T025, T028 (AC-16) / F121-AC-11, AC-12, AC-16 (TC-CHAT-007): the assignee acts with If-Match — warn and restrict
// apply sanctions, remove_message turns the message into a placeholder for players (original kept, chat.message_removed,
// realtime chat.message.removed), escalate, close resolves (reporters get report.resolved without any detail), a resolved
// case accepts only reopen (409 MODERATION_CASE_CLOSED); a stale version is 412; only the assignee or the lead acts.
func TestSafetyActOnCase(t *testing.T) {
	authztest.Cover(t, "actOnModerationCase", authztest.CrossTenant, authztest.RoleDenied)
	s := setupSafety(t)
	an, binh := s.student(s.tenantA, s.schoolA), s.student(s.tenantA, s.schoolA)
	caseID, ch, target := s.partyCase(an, binh, "bullying")
	sm, other := s.moderator("safety_moderator", false), s.moderator("safety_moderator", false)
	path := "/admin/moderation/cases/" + caseID.String()
	claim := sm.post(s.tenantA, path+":assign", map[string]any{"assignee_moderator_id": sm.id}, 0)
	v := version(claim)
	if claim.status != http.StatusOK || v < 1 {
		t.Fatalf("claim: %d %s", claim.status, claim.raw)
	}
	if r := other.post(s.tenantA, path+":act", map[string]any{"action": "warn", "note": "nhac nho"}, v); r.status != http.StatusForbidden {
		t.Fatalf("not the assignee: %d %s", r.status, r.raw)
	}
	if r := sm.post(s.tenantA, path+":act", map[string]any{"action": "warn", "note": "nhac nho"}, v-1); r.status != http.StatusPreconditionFailed {
		t.Fatalf("stale If-Match: %d %s", r.status, r.raw)
	}
	if r := sm.post(s.tenantA, path+":act", map[string]any{"action": "warn", "note": "ab"}, v); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("note < 3: %d %s", r.status, r.raw)
	}
	w := sm.post(s.tenantA, path+":act", map[string]any{"action": "warn", "note": "nhac nho than thien"}, v)
	if w.status != http.StatusOK || w.body["status"] != "in_review" || version(w) != v+1 {
		t.Fatalf("warn: %d %s", w.status, w.raw)
	}
	v = version(w)
	if n := s.count(`SELECT count(*) FROM chat.sanctions WHERE user_id = $1 AND type = 'warning' AND case_id = $2`, an.id, caseID); n != 1 {
		t.Fatalf("warning sanction: %d", n)
	}
	fan := channelInbox(t, s.vk, ch)
	time.Sleep(200 * time.Millisecond)
	rm := sm.post(s.tenantA, path+":act", map[string]any{"action": "remove_message", "message_id": target, "note": "go tin vi pham"}, v)
	if rm.status != http.StatusOK {
		t.Fatalf("remove_message: %d %s", rm.status, rm.raw)
	}
	v = version(rm)
	var state string
	var body *string
	var deleted *time.Time
	if err := s.su.QueryRowContext(context.Background(), `SELECT moderation_state, body, deleted_at FROM chat.gameplay_messages
		WHERE message_id = $1`, target).Scan(&state, &body, &deleted); err != nil {
		t.Fatal(err)
	}
	if state != "removed" || deleted == nil || body == nil || *body != "zqparty 3" {
		t.Fatalf("removed message keeps its evidence: %s %v %v", state, deleted, body)
	}
	if e := next(fan, "chat.message.removed", 3*time.Second); e == nil || payloadOf(e)["message_id"] != target.String() {
		t.Fatalf("realtime chat.message.removed: %v", e)
	}
	h := s.call(binh, http.MethodGet, "/chat/channels/"+ch.String()+"/messages", "", nil)
	if h.status != http.StatusOK || strings.Contains(h.raw, "zqparty 3") || strings.Contains(h.raw, target.String()) {
		t.Fatalf("player history shows the removed message: %d %s", h.status, h.raw)
	}
	if n := s.checkEvents("chat.message_removed")["chat.message_removed"]; n != 1 {
		t.Fatalf("chat.message_removed: %d", n)
	}
	if r := sm.post(s.tenantA, path+":act", map[string]any{"action": "restrict", "note": "han che"}, v); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("restrict without sanction_type: %d %s", r.status, r.raw)
	}
	rs := sm.post(s.tenantA, path+":act", map[string]any{"action": "restrict", "sanction_type": "restrict_text", "note": "chi quick message"}, v)
	if rs.status != http.StatusOK {
		t.Fatalf("restrict_text: %d %s", rs.status, rs.raw)
	}
	v = version(rs)
	var ends time.Time
	if err := s.su.QueryRowContext(context.Background(), `SELECT ends_at FROM chat.sanctions WHERE user_id = $1 AND type = 'restrict_text'`,
		an.id).Scan(&ends); err != nil || !ends.Equal(s.clock.Now().Add(7*24*time.Hour)) {
		t.Fatalf("restrict_text default 7 days: %v %v", ends, err)
	}
	esc := sm.post(s.tenantA, path+":act", map[string]any{"action": "escalate", "note": "can truong ca"}, v)
	if esc.status != http.StatusOK || esc.body["status"] != "escalated" {
		t.Fatalf("escalate: %d %s", esc.status, esc.raw)
	}
	v = version(esc)
	cl := sm.post(s.tenantA, path+":act", map[string]any{"action": "close", "note": "da xu ly xong"}, v)
	if cl.status != http.StatusOK || cl.body["status"] != "resolved" || cl.body["resolved_at"] == nil {
		t.Fatalf("close: %d %s", cl.status, cl.raw)
	}
	v = version(cl)
	resolved := s.events("report.resolved")
	if len(resolved) != 1 || resolved[0]["reporter_id"] != binh.id.String() || resolved[0]["kind"] != "moderation_notice" ||
		resolved[0]["message_code"] != "moderation.report_handled" {
		t.Fatalf("report.resolved: %v", resolved)
	}
	for k := range resolved[0] {
		if strings.Contains(k, "sanction") || k == "user_id" || k == "target_id" {
			t.Fatalf("report.resolved leaks %s", k)
		}
	}
	if n := s.checkEvents("report.resolved")["report.resolved"]; n != 1 {
		t.Fatalf("report.resolved schema: %d", n)
	}
	if r := sm.post(s.tenantA, path+":act", map[string]any{"action": "warn", "note": "them nua"}, v); r.status != http.StatusConflict ||
		r.code() != "MODERATION_CASE_CLOSED" {
		t.Fatalf("act on resolved: %d %s", r.status, r.raw)
	}
	if r := sm.post(s.tenantA, path+":act", map[string]any{"action": "reopen", "note": "khang nghi moi"}, v); r.status != http.StatusOK ||
		r.body["status"] != "reopened" {
		t.Fatalf("reopen: %d %s", r.status, r.raw)
	}
	if !hasAudit(s.audits(), "moderation.case.acted") || !hasAudit(s.audits(), "moderation.message.removed") {
		t.Fatal("audits of the actions")
	}
	if n := s.count(`SELECT count(*) FROM chat.moderation_actions WHERE case_id = $1`, caseID); n < 7 {
		t.Fatalf("append-only history: %d", n)
	}
	s.noContent("zqparty 3", "nhac nho than thien", "go tin vi pham")
}

// T026–T028 / F121-AC-13, AC-14, AC-15 (TC-CHAT-014, TC-CHAT-019): sanctions of the Safety Team — mute refuses every
// message (CHAT_MUTED), restrict_text leaves quick messages only (CHAT_KIND_NOT_ALLOWED), even with every moderation step
// switched off and in an always-on match channel; restrict_multiplayer > 7 days and account_lock need the team lead or a
// System Admin; sanction.applied + chat.sanction.applied (reason_code, until) to the student only; revokeSanction and the
// end of ends_at publish sanction.lifted and the student sends again at once.
func TestSafetySanctions(t *testing.T) {
	authztest.Cover(t, "createSanction", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "revokeSanction", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listSanctions", authztest.CrossTenant, authztest.RoleDenied)
	s := setupSafety(t)
	an, binh, chi := s.student(s.tenantA, s.schoolA), s.student(s.tenantA, s.schoolA), s.student(s.tenantA, s.schoolA)
	party := s.open(channel.Party, an, binh, chi)
	match := s.open(channel.Match, an, binh, chi)
	sm, lead, sa := s.moderator("safety_moderator", false), s.moderator("safety_moderator", true), s.moderator("system_admin", false)
	anInbox := inbox(t, s.vk, an.id)
	time.Sleep(200 * time.Millisecond)
	mute := sm.post(s.tenantA, "/admin/moderation/sanctions", map[string]any{"user_id": an.id, "sanction_type": "mute", "duration_minutes": 60,
		"reason": "spam nhieu lan"}, 0)
	if mute.status != http.StatusCreated || mute.body["reason_code"] != "moderation" || mute.body["issued_by_kind"] != "moderator" {
		t.Fatalf("mute: %d %s", mute.status, mute.raw)
	}
	e := next(anInbox, "chat.sanction.applied", 3*time.Second)
	if e == nil || payloadOf(e)["sanction_type"] != "mute" || payloadOf(e)["reason_code"] != "moderation" || payloadOf(e)["until"] == nil {
		t.Fatalf("chat.sanction.applied: %v", e)
	}
	if strings.Contains(string(e.Message.Payload), "spam nhieu lan") {
		t.Fatal("the internal reason reached the student")
	}
	if r := s.sendWS(an, "c-an", party, "zqxin"); replyCode(r) != "CHAT_MUTED" {
		t.Fatalf("muted send: %+v", r)
	}
	// restrict_text: quick messages only, whatever the switches, also in a match channel.
	rt := sm.post(s.tenantA, "/admin/moderation/sanctions", map[string]any{"user_id": binh.id, "sanction_type": "restrict_text",
		"reason": "ngon ngu tho"}, 0)
	if rt.status != http.StatusCreated {
		t.Fatalf("restrict_text: %d %s", rt.status, rt.raw)
	}
	s.tenantSwitch(s.tenantA, false, map[string]bool{"sensitive_terms": false, "spam": false, "bullying": false, "async_analysis": false})
	for _, ch := range []uuid.UUID{party, match} {
		if r := s.sendWS(binh, "c-binh", ch, "zqtext"); replyCode(r) != "CHAT_KIND_NOT_ALLOWED" {
			t.Fatalf("restrict_text text in %s: %+v", ch, r)
		}
	}
	quick := s.ws(binh, "c-binh", "chat.message.send", map[string]any{"channel_id": party, "client_message_id": uuid.New(), "kind": "quick",
		"quick_message_id": "qm_hello"})
	if quick.Problem != nil {
		t.Fatalf("quick message allowed under restrict_text: %+v", quick.Problem)
	}
	// Team-lead rule (MODERATION §7).
	long := map[string]any{"user_id": chi.id, "sanction_type": "restrict_multiplayer", "duration_minutes": 14 * 24 * 60, "reason": "pha tran nhieu lan"}
	if r := sm.post(s.tenantA, "/admin/moderation/sanctions", long, 0); r.status != http.StatusForbidden || r.code() != "PERMISSION_DENIED" {
		t.Fatalf("restrict_multiplayer 14 days without lead: %d %s", r.status, r.raw)
	}
	if r := lead.post(s.tenantA, "/admin/moderation/sanctions", long, 0); r.status != http.StatusCreated {
		t.Fatalf("lead restrict_multiplayer 14 days: %d %s", r.status, r.raw)
	}
	lock := map[string]any{"user_id": chi.id, "sanction_type": "account_lock", "reason": "yeu cau phap ly"}
	if r := sm.post(s.tenantA, "/admin/moderation/sanctions", lock, 0); r.status != http.StatusForbidden {
		t.Fatalf("account_lock without lead: %d %s", r.status, r.raw)
	}
	if r := sa.post(s.tenantA, "/admin/moderation/sanctions", lock, 0); r.status != http.StatusCreated {
		t.Fatalf("system_admin account_lock: %d %s", r.status, r.raw)
	}
	if r := sm.post(s.tenantA, "/admin/moderation/sanctions", map[string]any{"user_id": uuid.New(), "sanction_type": "mute", "reason": "khong co"}, 0); r.status != http.StatusNotFound {
		t.Fatalf("unknown student: %d %s", r.status, r.raw)
	}
	bStudent := s.student(s.tenantB, s.schoolB)
	if r := sm.post(s.tenantA, "/admin/moderation/sanctions", map[string]any{"user_id": bStudent.id, "sanction_type": "mute", "reason": "tenant khac"}, 0); r.status != http.StatusNotFound {
		t.Fatalf("student of another tenant: %d %s", r.status, r.raw)
	}
	if n := s.checkEvents("sanction.applied")["sanction.applied"]; n != 4 {
		t.Fatalf("sanction.applied: %d", n)
	}
	// listSanctions (cross-tenant read through the bypass).
	if l := sm.get("/admin/moderation/sanctions?active=true&user_id=" + chi.id.String()); l.status != http.StatusOK || len(l.data()) != 2 {
		t.Fatalf("listSanctions: %d %s", l.status, l.raw)
	}
	// Revoke: sanction.lifted, the student sends again at once (cache cleared).
	muteID := mute.body["sanction_id"].(string)
	rv := sm.post(s.tenantA, "/admin/moderation/sanctions/"+muteID+":revoke", map[string]any{"reason": "da hieu loi"}, 0)
	if rv.status != http.StatusOK || rv.body["revoked_at"] == nil {
		t.Fatalf("revoke: %d %s", rv.status, rv.raw)
	}
	if r := s.sendWS(an, "c-an", party, "zqxin lai"); r.Problem != nil {
		t.Fatalf("send after revoke: %+v", r.Problem)
	}
	if r := sm.post(s.tenantA, "/admin/moderation/sanctions/"+muteID+":revoke", map[string]any{"reason": "lan hai"}, 0); r.status != http.StatusConflict {
		t.Fatalf("revoke twice: %d %s", r.status, r.raw)
	}
	if r := sm.post(s.tenantB, "/admin/moderation/sanctions/"+muteID+":revoke", map[string]any{"reason": "tenant khac"}, 0); r.status != http.StatusNotFound {
		t.Fatalf("revoke from another tenant: %d %s", r.status, r.raw)
	}
	// Expiry: a short mute ends, sanction-expirer publishes sanction.lifted once.
	short := sm.post(s.tenantA, "/admin/moderation/sanctions", map[string]any{"user_id": an.id, "sanction_type": "mute", "duration_minutes": 1,
		"reason": "nhac nho ngan"}, 0)
	if short.status != http.StatusCreated {
		t.Fatalf("short mute: %d %s", short.status, short.raw)
	}
	s.clock.Add(2 * time.Minute)
	ctx := context.Background()
	if n, err := s.cmd.ExpireSanctions(ctx); err != nil || n != 1 {
		t.Fatalf("expirer: %d %v", n, err)
	}
	if n, err := s.cmd.ExpireSanctions(ctx); err != nil || n != 0 {
		t.Fatalf("expirer again: %d %v", n, err)
	}
	lifted := s.events("sanction.lifted")
	reasons := map[string]int{}
	for _, p := range lifted {
		reasons[p["lift_reason"].(string)]++
	}
	if reasons["revoked"] != 1 || reasons["expired"] != 1 {
		t.Fatalf("sanction.lifted: %v", reasons)
	}
	if n := s.checkEvents("sanction.lifted")["sanction.lifted"]; n != 2 {
		t.Fatalf("sanction.lifted schema: %d", n)
	}
	if !hasAudit(s.audits(), "moderation.sanction.applied") || !hasAudit(s.audits(), "moderation.sanction.revoked") {
		t.Fatal("sanction audits")
	}
	s.noContent("spam nhieu lan", "da hieu loi")
}

// T013 (release) / F121-AC-04: the hold of a p1 case is released when the case resolves (sanction.lifted
// case_resolved) unless it became a sanction; the student sends again.
func TestSafetyHoldReleasedOnResolve(t *testing.T) {
	s := setupSafety(t)
	an, binh := s.student(s.tenantA, s.schoolA), s.student(s.tenantA, s.schoolA)
	caseID, party, _ := s.partyCase(an, binh, "asked_personal_info")
	if r := s.sendWS(an, "c-an", party, "zqxin"); replyCode(r) != "CHAT_MUTED" {
		t.Fatalf("held: %+v", r)
	}
	sm := s.moderator("safety_moderator", false)
	path := "/admin/moderation/cases/" + caseID.String()
	v := version(sm.post(s.tenantA, path+":assign", map[string]any{"assignee_moderator_id": sm.id}, 0))
	if r := sm.post(s.tenantA, path+":act", map[string]any{"action": "dismiss", "note": "khong vi pham"}, v); r.status != http.StatusOK {
		t.Fatalf("dismiss: %d %s", r.status, r.raw)
	}
	if ev := s.events("sanction.lifted"); len(ev) != 1 || ev[0]["lift_reason"] != "case_resolved" {
		t.Fatalf("hold released: %v", ev)
	}
	if n := s.count(`SELECT count(*) FROM chat.reports WHERE case_id = $1 AND status = 'dismissed'`, caseID); n != 1 {
		t.Fatalf("dismissed report: %d", n)
	}
	if r := s.sendWS(an, "c-an", party, "zqxin lai"); r.Problem != nil {
		t.Fatalf("send after release: %+v", r.Problem)
	}
}
