//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// T010–T011, T013–T014 / F121-AC-01, AC-02, AC-04, AC-05 (TC-CHAT-006, TC-CHAT-030, TC-CHAT-043, TC-TEN-024): a student
// reports a message of a channel they belong to — 201 with a localized thank-you (Idempotency-Key replays), a ±10
// snapshot with the original content (also of a recalled message), one p1 case of the tenant with the schools, report_sla
// deadlines, one count per reporter, report.created; the p1 case holds the reported student at once (no message, no
// friend request); unknown targets, foreign channels and other tenants are 404.
func TestSafetyReportCreatesCase(t *testing.T) {
	authztest.Cover(t, "createReport", authztest.CrossTenant, authztest.RoleDenied)
	s := setupSafety(t)
	an, binh, chi := s.student(s.tenantA, s.schoolA), s.student(s.tenantA, s.schoolA), s.student(s.tenantA, s.schoolA2)
	ch := s.open(channel.Team, an, binh, chi)
	var target uuid.UUID
	for seq := int64(1); seq <= 25; seq++ {
		sender := binh
		if seq%2 == 1 {
			sender = an
		}
		id := s.msg(ch, false, sender, seq, "zqtin "+jsonNum(seq))
		if seq == 13 {
			target = id
		}
	}
	// The reported message was recalled (F118): the report is still accepted and the evidence keeps the original.
	s.exec(`UPDATE chat.messages SET recalled_at = $2, recalled_by = $3 WHERE message_id = $1 AND tenant_id = $4 AND deleted_at IS NULL`,
		target, s.clock.Now(), an.id, s.tenantA)
	body := map[string]any{"target_type": "message", "target_id": target, "reason": "asked_personal_info", "context": map[string]any{"channel_id": ch}}
	key := newKey()
	r := s.report(binh, key, "vi", body)
	if r.status != http.StatusCreated || r.body["status"] != "open" || !strings.Contains(r.raw, "Cảm ơn") || r.header.Get("Content-Language") != "vi" {
		t.Fatalf("createReport: %d %s", r.status, r.raw)
	}
	reportID := r.body["report_id"]
	if again := s.report(binh, key, "vi", body); again.status != http.StatusCreated || again.body["report_id"] != reportID ||
		again.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %d %s", again.status, again.raw)
	}
	if en := s.report(chi, newKey(), "en", body); en.status != http.StatusCreated || !strings.Contains(en.raw, "Thank you") {
		t.Fatalf("english: %d %s", en.status, en.raw)
	}
	// The same reporter again (new key): counted once.
	if r2 := s.report(binh, newKey(), "vi", body); r2.status != http.StatusCreated || r2.body["report_id"] != reportID {
		t.Fatalf("same reporter: %d %s", r2.status, r2.raw)
	}
	caseID := s.caseOf(target)
	c := s.caseRow(caseID)
	now := s.clock.Now()
	if c["tenant_id"] != s.tenantA || c["severity"] != "p1" || c["status"] != "open" || c["source"] != "report" || c["report_count"] != 2 ||
		!strings.Contains(c["school_ids"].(string), s.schoolA.String()) || !strings.Contains(c["school_ids"].(string), s.schoolA2.String()) ||
		!strings.Contains(c["subjects"].(string), an.id.String()) || strings.Contains(c["subjects"].(string), binh.id.String()) {
		t.Fatalf("case: %v", c)
	}
	if !c["sla"].(time.Time).Equal(now.Add(24*time.Hour)) || !c["first_response"].(time.Time).Equal(now.Add(time.Hour)) {
		t.Fatalf("report_sla p1: %v %v", c["first_response"], c["sla"])
	}
	// Evidence: ±10 messages (seq 3..23) with the original body of the recalled message.
	var snap []byte
	if err := s.su.QueryRowContext(context.Background(), `SELECT snapshot FROM chat.moderation_evidence WHERE case_id = $1
		ORDER BY created_at LIMIT 1`, caseID).Scan(&snap); err != nil {
		t.Fatal(err)
	}
	var ev struct {
		Messages []struct {
			MessageID  uuid.UUID  `json:"message_id"`
			Seq        int64      `json:"seq"`
			Body       *string    `json:"body"`
			RecalledAt *time.Time `json:"recalled_at"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(snap, &ev); err != nil {
		t.Fatal(err)
	}
	if len(ev.Messages) != 21 || ev.Messages[0].Seq != 3 || ev.Messages[20].Seq != 23 {
		t.Fatalf("snapshot ±10: %d messages", len(ev.Messages))
	}
	if m := ev.Messages[10]; m.MessageID != target || m.Body == nil || *m.Body != "zqtin 13" || m.RecalledAt == nil {
		t.Fatalf("snapshot target: %+v", m)
	}
	if n := s.count(`SELECT count(*) FROM chat.moderation_evidence WHERE case_id = $1`, caseID); n != 2 {
		t.Fatalf("one evidence per counted report: %d", n)
	}
	if n := s.checkEvents("report.created", "sanction.applied")["report.created"]; n != 2 {
		t.Fatalf("report.created: %d", n)
	}
	if !hasAudit(s.audits(), "moderation.report.created") {
		t.Fatal("audit moderation.report.created")
	}
	// F121-AC-04: hold of the reported student (system mute of every channel, case_hold, no end) linked to the case.
	var holdID uuid.UUID
	var reasonCode string
	if err := s.su.QueryRowContext(context.Background(), `SELECT sanction_id, reason_code FROM chat.sanctions WHERE user_id = $1
		AND case_id = $2 AND type = 'mute' AND scope = 'all' AND ends_at IS NULL AND revoked_at IS NULL`, an.id, caseID).Scan(&holdID,
		&reasonCode); err != nil || reasonCode != "case_hold" {
		t.Fatalf("hold: %v %s", err, reasonCode)
	}
	if c["hold"] != holdID.String() {
		t.Fatalf("hold_sanction_id: %v", c["hold"])
	}
	if ev := s.events("sanction.applied"); len(ev) != 1 || ev[0]["reason_code"] != "case_hold" || ev[0]["sanction_type"] != "mute" {
		t.Fatalf("sanction.applied: %v", ev)
	}
	party := s.open(channel.Party, an, binh)
	if rep := s.sendWS(an, "c-an", party, "zqxin chao"); replyCode(rep) != "CHAT_MUTED" {
		t.Fatalf("held student sends: %+v", rep)
	}
	if fr := s.call(an, http.MethodPost, "/social/friend-requests", newKey(), map[string]any{"addressee_id": chi.id}); fr.status != http.StatusForbidden {
		t.Fatalf("held student friend request: %d %s", fr.status, fr.raw)
	}
	// Validation: note > 500 → 422; unknown reason → 422; own message → 422.
	long := map[string]any{"target_type": "message", "target_id": target, "reason": "spam", "note": strings.Repeat("a", 501)}
	if v := s.report(binh, newKey(), "vi", long); v.status != http.StatusUnprocessableEntity {
		t.Fatalf("note > 500: %d %s", v.status, v.raw)
	}
	if v := s.report(binh, newKey(), "vi", map[string]any{"target_type": "message", "target_id": target, "reason": "bad_words"}); v.status != http.StatusUnprocessableEntity {
		t.Fatalf("old reason: %d %s", v.status, v.raw)
	}
	own := s.msg(ch, false, binh, 26, "zqcua toi")
	if v := s.report(binh, newKey(), "vi", map[string]any{"target_type": "message", "target_id": own, "reason": "spam"}); v.status != http.StatusUnprocessableEntity {
		t.Fatalf("own message: %d %s", v.status, v.raw)
	}
	// F121-AC-05: 404 for a message of a channel the reporter never joined, of another tenant, unknown ids, users elsewhere.
	other := s.open(channel.Direct, an, chi)
	foreign := s.msg(other, false, an, 1, "zqkhac")
	bStudent, bFriend := s.student(s.tenantB, s.schoolB), s.student(s.tenantB, s.schoolB)
	bCh := s.open(channel.Direct, bStudent, bFriend)
	bMsg := s.msg(bCh, false, bStudent, 1, "zqtenantb")
	for name, b := range map[string]map[string]any{
		"foreign channel": {"target_type": "message", "target_id": foreign, "reason": "spam"},
		"other tenant":    {"target_type": "message", "target_id": bMsg, "reason": "spam"},
		"unknown message": {"target_type": "message", "target_id": uuid.New(), "reason": "spam"},
		"user of B":       {"target_type": "user", "target_id": bStudent.id, "reason": "bullying"},
		"user channel":    {"target_type": "user", "target_id": an.id, "reason": "bullying", "context": map[string]any{"channel_id": bCh}},
	} {
		if v := s.report(binh, newKey(), "vi", b); v.status != http.StatusNotFound {
			t.Errorf("%s: %d %s", name, v.status, v.raw)
		}
	}
	if s.caseOf(bMsg) != uuid.Nil || s.count(`SELECT count(*) FROM chat.reports WHERE tenant_id = $1`, s.tenantB) != 0 {
		t.Fatal("nothing written in tenant B")
	}
	// Reporting a student (context channel) opens a case of that user; a gameplay message (not purged) is reportable.
	if v := s.report(chi, newKey(), "vi", map[string]any{"target_type": "user", "target_id": an.id, "reason": "rude_language",
		"context": map[string]any{"channel_id": ch}}); v.status != http.StatusCreated {
		t.Fatalf("user report: %d %s", v.status, v.raw)
	}
	if uc := s.caseRow(s.caseOf(an.id)); uc["severity"] != "p3" || uc["report_count"] != 1 {
		t.Fatalf("user case: %v", uc)
	}
	match := s.open(channel.Match, an, binh)
	gm := s.msg(match, true, an, 1, "zqtran dau")
	if v := s.report(binh, newKey(), "vi", map[string]any{"target_type": "message", "target_id": gm, "reason": "bullying",
		"context": map[string]any{"match_id": uuid.New()}}); v.status != http.StatusCreated {
		t.Fatalf("gameplay report: %d %s", v.status, v.raw)
	}
	if gc := s.caseRow(s.caseOf(gm)); gc["severity"] != "p2" || gc["tenant_id"] != s.tenantA {
		t.Fatalf("gameplay case: %v", gc)
	}
	// Staff and other roles cannot report (game_player guard).
	if v := s.do(http.MethodPost, "/chat/reports", s.staff(s.tenantA, "school_admin"), map[string]string{"X-Tenant-Id": s.tenantA.String(),
		"Idempotency-Key": newKey()}, body); v.status != http.StatusForbidden {
		t.Fatalf("staff report: %d %s", v.status, v.raw)
	}
	s.noContent("zqtin 13", "zqtran dau")
}

// T012 / F121-AC-03, AC-24 (TC-SVC-003): chat.message_held of the real pipeline opens a case — block_and_alert of a
// grooming placeholder: filter_alert p1 holding the sender through the existing block_and_alert mute (no second hold);
// block_and_alert of a self-harm placeholder: self_harm_signal p1 and the child is NOT penalised (the mute is lifted);
// hold_for_review of a high placeholder: filter_alert p2 without hold; a redelivered event opens no second case.
func TestSafetyMessageHeldOpensCase(t *testing.T) {
	s := setupSafety(t)
	catGrooming := uuid.MustParse("01936a00-0000-7000-8000-000000000107")
	s.publishPlatform(append(defaultTerms(), placeholder(catGrooming, "zqgroom", "vi"))...)
	an, binh, chi := s.student(s.tenantA, s.schoolA), s.student(s.tenantA, s.schoolA), s.student(s.tenantA, s.schoolA)
	party := s.open(channel.Party, an, binh, chi)
	for _, m := range []struct {
		st   student
		text string
	}{{an, "zqgroom"}, {binh, "zqhigh"}, {chi, "zqcrit"}} {
		if r := s.sendWS(m.st, "c-"+m.text, party, m.text); r.Problem != nil {
			t.Fatalf("send %s: %+v", m.text, r)
		}
	}
	deadline := time.Now().Add(10 * time.Second)
	for len(s.envelopes("chat.message_held")) < 3 && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	held := s.envelopes("chat.message_held")
	if len(held) != 3 {
		t.Fatalf("chat.message_held: %d", len(held))
	}
	ctx := context.Background()
	bySender := map[uuid.UUID]uuid.UUID{}
	for _, e := range held {
		for i := 0; i < 2; i++ { // redelivery
			if err := s.consumer.Handle(ctx, e); err != nil {
				t.Fatal(err)
			}
		}
		var p struct {
			MessageID uuid.UUID `json:"message_id"`
			SenderID  uuid.UUID `json:"sender_id"`
		}
		_ = json.Unmarshal(e.Payload, &p)
		bySender[p.SenderID] = p.MessageID
	}
	if n := s.count(`SELECT count(*) FROM chat.moderation_cases WHERE tenant_id = $1`, s.tenantA); n != 3 {
		t.Fatalf("one case per held message: %d", n)
	}
	groom := s.caseRow(s.caseOf(bySender[an.id]))
	if groom["source"] != "filter_alert" || groom["severity"] != "p1" || groom["category"] != "grooming_scam" {
		t.Fatalf("grooming case: %v", groom)
	}
	var muteID uuid.UUID
	var caseRef *uuid.UUID
	if err := s.su.QueryRowContext(ctx, `SELECT sanction_id, case_id FROM chat.sanctions WHERE user_id = $1 AND reason = 'block_and_alert_hold'`,
		an.id).Scan(&muteID, &caseRef); err != nil {
		t.Fatal(err)
	}
	if n := s.count(`SELECT count(*) FROM chat.sanctions WHERE user_id = $1`, an.id); n != 1 || caseRef == nil ||
		*caseRef != s.caseOf(bySender[an.id]) || groom["hold"] != muteID.String() {
		t.Fatalf("hold reuses the block_and_alert mute: %d %v %v", n, caseRef, groom["hold"])
	}
	high := s.caseRow(s.caseOf(bySender[binh.id]))
	if high["source"] != "filter_alert" || high["severity"] != "p2" || high["hold"] != "" {
		t.Fatalf("hold_for_review case: %v", high)
	}
	if n := s.count(`SELECT count(*) FROM chat.sanctions WHERE user_id = $1`, binh.id); n != 0 {
		t.Fatalf("no sanction for a p2 signal: %d", n)
	}
	selfHarm := s.caseRow(s.caseOf(bySender[chi.id]))
	if selfHarm["source"] != "self_harm_signal" || selfHarm["severity"] != "p1" || selfHarm["category"] != "self_harm" || selfHarm["hold"] != "" {
		t.Fatalf("self-harm case: %v", selfHarm)
	}
	if n := s.count(`SELECT count(*) FROM chat.sanctions WHERE user_id = $1 AND revoked_at IS NULL`, chi.id); n != 0 {
		t.Fatalf("self-harm sender penalised: %d active sanctions", n)
	}
	if r := s.sendWS(chi, "c-chi", party, "zqxin chao"); r.Problem != nil {
		t.Fatalf("self-harm sender can still talk: %+v", r.Problem)
	}
	if n := s.count(`SELECT count(*) FROM chat.moderation_evidence WHERE case_id = $1`, s.caseOf(bySender[an.id])); n != 1 {
		t.Fatalf("signal evidence: %d", n)
	}
	// F121-AC-24: the case of a self-harm signal carries the support guide for the moderator.
	lead := s.moderator("safety_moderator", true)
	if d := lead.get("/admin/moderation/cases/" + s.caseOf(bySender[chi.id]).String()); d.status != http.StatusOK ||
		d.body["support_guide"] != "child_protection_111" {
		t.Fatalf("support guide: %d %s", d.status, d.raw)
	}
	s.noContent("zqcrit", "zqhigh", "zqgroom")
}
