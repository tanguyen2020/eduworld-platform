//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
)

// schoolStaff registers a tenant staff member (role) limited to school (nil: the whole tenant) and returns its token.
func (s *safetyEnv) schoolStaff(tenant uuid.UUID, role string, school *uuid.UUID) string {
	var opts []authztest.Option
	if school != nil {
		opts = append(opts, authztest.AtSchool(*school))
	}
	sub := "kc-school-" + uuid.NewString()
	s.res.Set(sub, authztest.Staff(tenant, role, opts...).PrincipalData)
	return s.idp.Token(sub, httpxtest.PortalClient, time.Now())
}

func (s *safetyEnv) tenantGet(token string, tenant uuid.UUID, path string) resp {
	return s.do(http.MethodGet, path, token, map[string]string{"X-Tenant-Id": tenant.String()}, nil)
}

func (s *safetyEnv) tenantPost(token string, tenant uuid.UUID, path string, body any) resp {
	return s.do(http.MethodPost, path, token, map[string]string{"X-Tenant-Id": tenant.String(), "Idempotency-Key": newKey()}, body)
}

// multiCase: an (school A) is reported by binh (school A) and chi (school A2) — one p1 case of two schools.
func (s *safetyEnv) multiCase() (uuid.UUID, student, student, student) {
	s.t.Helper()
	an, binh, chi := s.student(s.tenantA, s.schoolA), s.student(s.tenantA, s.schoolA), s.student(s.tenantA, s.schoolA2)
	ch := s.open(channel.Team, an, binh, chi)
	target := s.msg(ch, false, an, 1, "zqxin so dien thoai")
	for _, rep := range []student{binh, chi} {
		if r := s.report(rep, newKey(), "vi", map[string]any{"target_type": "message", "target_id": target, "reason": "asked_personal_info"}); r.status != http.StatusCreated {
			s.t.Fatalf("report: %d %s", r.status, r.raw)
		}
	}
	return s.caseOf(target), an, binh, chi
}

// T031–T033 / F121-AC-17, AC-18, AC-19 (TC-CHAT-043): the Safety Team notifies each school of a case with its own
// students only (422 for a student of another school or outside the case), ack_due_at by severity, one pending
// notification per school (409), moderation.school_notified; a school admin sees the notifications of their school only
// (a tenant admin every school of the tenant, another tenant nothing), never message content; the school acknowledges
// once (409 the second time, 404 for another school), moderation.school_acknowledged.
func TestSafetySchoolNotifications(t *testing.T) {
	authztest.Cover(t, "notifySchoolForCase", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listSchoolSafetyNotifications", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "acknowledgeSchoolSafetyNotification", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listSchoolSafetyNotificationsPlatform", authztest.CrossTenant, authztest.RoleDenied)
	s := setupSafety(t)
	sent0, ack0 := schoolNotifications("sent"), schoolNotifications("acknowledged")
	caseID, an, binh, chi := s.multiCase()
	sm := s.moderator("safety_moderator", false)
	path := "/admin/moderation/cases/" + caseID.String() + ":notify-school"
	summary := "Hoc sinh bi xin thong tin ca nhan trong chat lop, da tam han che nguoi gui."
	if r := sm.post(s.tenantA, path, map[string]any{"school_id": s.schoolA, "student_ids": []uuid.UUID{an.id, chi.id}, "summary": summary}, 0); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("student of another school: %d %s", r.status, r.raw)
	}
	stranger := s.student(s.tenantA, s.schoolA)
	if r := sm.post(s.tenantA, path, map[string]any{"school_id": s.schoolA, "student_ids": []uuid.UUID{stranger.id}, "summary": summary}, 0); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("student outside the case: %d %s", r.status, r.raw)
	}
	if r := sm.post(s.tenantA, path, map[string]any{"school_id": s.schoolA, "student_ids": []uuid.UUID{an.id}, "summary": "ngan"}, 0); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("summary < 10: %d %s", r.status, r.raw)
	}
	if r := sm.post(s.tenantB, path, map[string]any{"school_id": s.schoolA, "student_ids": []uuid.UUID{an.id}, "summary": summary}, 0); r.status != http.StatusForbidden ||
		r.code() != "TENANT_MISMATCH" {
		t.Fatalf("X-Tenant-Id of another tenant: %d %s", r.status, r.raw)
	}
	nA := sm.post(s.tenantA, path, map[string]any{"school_id": s.schoolA, "student_ids": []uuid.UUID{an.id, binh.id}, "summary": summary}, 0)
	if nA.status != http.StatusCreated {
		t.Fatalf("notify school A: %d %s", nA.status, nA.raw)
	}
	due, _ := time.Parse(time.RFC3339Nano, nA.body["ack_due_at"].(string))
	if !due.Equal(s.clock.Now().Add(24*time.Hour)) || nA.body["measures_taken"] != "case_hold" {
		t.Fatalf("ack_due_at p1 / measures: %s", nA.raw)
	}
	roles := map[string]string{}
	for _, st := range nA.body["students"].([]any) {
		m := st.(map[string]any)
		roles[m["student_id"].(string)] = m["role"].(string)
	}
	if roles[an.id.String()] != "offender" || roles[binh.id.String()] != "affected" || len(roles) != 2 {
		t.Fatalf("students: %v", roles)
	}
	if r := sm.post(s.tenantA, path, map[string]any{"school_id": s.schoolA, "student_ids": []uuid.UUID{an.id}, "summary": summary}, 0); r.status != http.StatusConflict {
		t.Fatalf("pending notification of the school: %d %s", r.status, r.raw)
	}
	nA2 := sm.post(s.tenantA, path, map[string]any{"school_id": s.schoolA2, "student_ids": []uuid.UUID{chi.id}, "summary": summary}, 0)
	if nA2.status != http.StatusCreated {
		t.Fatalf("notify school A2: %d %s", nA2.status, nA2.raw)
	}
	if n := s.checkEvents("moderation.school_notified")["moderation.school_notified"]; n != 2 {
		t.Fatalf("moderation.school_notified: %d", n)
	}
	if c := s.caseRow(caseID); c["contact_path"] != "school" {
		t.Fatalf("contact_path: %v", c)
	}
	var notified *time.Time
	_ = s.su.QueryRowContext(context.Background(), `SELECT school_notified_at FROM chat.moderation_cases WHERE case_id = $1`, caseID).Scan(&notified)
	if notified == nil || !notified.Equal(s.clock.Now()) {
		t.Fatalf("school_notified_at: %v", notified)
	}
	// F121-AC-18: scope of the school list.
	adA, adA2 := s.schoolStaff(s.tenantA, "school_admin", &s.schoolA), s.schoolStaff(s.tenantA, "school_admin", &s.schoolA2)
	list := "/tenant/safety/school-notifications"
	ids := func(r resp) []string {
		var out []string
		for _, d := range r.data() {
			out = append(out, d["notification_id"].(string))
		}
		return out
	}
	idA, idA2 := nA.body["notification_id"].(string), nA2.body["notification_id"].(string)
	if got := ids(s.tenantGet(adA, s.tenantA, list)); len(got) != 1 || got[0] != idA {
		t.Fatalf("school admin A: %v", got)
	}
	if got := ids(s.tenantGet(adA2, s.tenantA, list)); len(got) != 1 || got[0] != idA2 {
		t.Fatalf("school admin A2: %v", got)
	}
	if got := ids(s.tenantGet(s.schoolStaff(s.tenantA, "tenant_admin", nil), s.tenantA, list)); len(got) != 2 {
		t.Fatalf("tenant admin: %v", got)
	}
	if got := s.tenantGet(s.schoolStaff(s.tenantB, "school_admin", &s.schoolB), s.tenantB, list); got.status != http.StatusOK || len(got.data()) != 0 {
		t.Fatalf("tenant B: %d %s", got.status, got.raw)
	}
	if r := s.tenantGet(adA, s.tenantA, list+"?school_id="+s.schoolA2.String()); r.status != http.StatusForbidden {
		t.Fatalf("school admin A asks school A2: %d %s", r.status, r.raw)
	}
	raw := s.tenantGet(adA, s.tenantA, list).raw
	if strings.Contains(raw, "zqxin") || strings.Contains(raw, `"body"`) || strings.Contains(raw, chi.id.String()) {
		t.Fatalf("school list leaks content or another school: %s", raw)
	}
	// F121-AC-19: acknowledgement.
	ack := list + "/" + idA + ":acknowledge"
	if r := s.tenantPost(adA2, s.tenantA, ack, map[string]any{"contact_outcome": "parent_contacted"}); r.status != http.StatusNotFound {
		t.Fatalf("other school acknowledges: %d %s", r.status, r.raw)
	}
	if r := s.tenantPost(s.schoolStaff(s.tenantB, "school_admin", &s.schoolB), s.tenantB, ack, map[string]any{"contact_outcome": "parent_contacted"}); r.status != http.StatusNotFound {
		t.Fatalf("other tenant acknowledges: %d %s", r.status, r.raw)
	}
	if r := s.tenantPost(adA, s.tenantA, ack, map[string]any{"contact_outcome": "parent_contacted", "note": strings.Repeat("a", 501)}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("note > 500: %d %s", r.status, r.raw)
	}
	a := s.tenantPost(adA, s.tenantA, ack, map[string]any{"contact_outcome": "parent_contacted", "note": "Da goi phu huynh"})
	if a.status != http.StatusOK || a.body["contact_outcome"] != "parent_contacted" || a.body["school_ack_at"] == nil {
		t.Fatalf("acknowledge: %d %s", a.status, a.raw)
	}
	if r := s.tenantPost(adA, s.tenantA, ack, map[string]any{"contact_outcome": "not_needed"}); r.status != http.StatusConflict {
		t.Fatalf("acknowledge twice: %d %s", r.status, r.raw)
	}
	// MONITORING §3: two notifications sent (the 409 duplicate and the refusals count nothing), one acknowledged.
	if d := schoolNotifications("sent") - sent0; d != 2 {
		t.Fatalf("school_notifications_total{sent} delta %v", d)
	}
	if d := schoolNotifications("acknowledged") - ack0; d != 1 {
		t.Fatalf("school_notifications_total{acknowledged} delta %v", d)
	}
	if c := s.caseRow(caseID); c["status"] == "" {
		t.Fatal("case")
	}
	if n := s.count(`SELECT count(*) FROM chat.moderation_cases WHERE case_id = $1 AND school_ack_at IS NOT NULL
		AND contact_outcome = 'parent_contacted'`, caseID); n != 1 {
		t.Fatalf("case acknowledged: %d", n)
	}
	if n := s.checkEvents("moderation.school_acknowledged")["moderation.school_acknowledged"]; n != 1 {
		t.Fatalf("moderation.school_acknowledged: %d", n)
	}
	if !hasAudit(s.audits(), "moderation.school_notification.sent") || !hasAudit(s.audits(), "moderation.school_notification.acknowledged") {
		t.Fatal("notification audits")
	}
	s.noContent("zqxin so dien thoai", summary, "Da goi phu huynh")
}

// T034 / F121-AC-20 (TC-CHAT-043): an unacknowledged notification is reminded once past ack_due_at and once more (team
// lead) one SLA period later; the platform list filters overdue and acknowledged.
func TestSafetySchoolReminder(t *testing.T) {
	s := setupSafety(t)
	overdue0 := schoolNotifications("overdue")
	caseID, an, _, _ := s.multiCase()
	sm := s.moderator("safety_moderator", false)
	n := sm.post(s.tenantA, "/admin/moderation/cases/"+caseID.String()+":notify-school", map[string]any{"school_id": s.schoolA,
		"student_ids": []uuid.UUID{an.id}, "summary": "Tom tat theo mau cho nha truong."}, 0)
	if n.status != http.StatusCreated {
		t.Fatalf("notify: %d %s", n.status, n.raw)
	}
	id := n.body["notification_id"].(string)
	ctx := context.Background()
	platform := "/admin/safety/school-notifications"
	if got := sm.get(platform + "?overdue=true"); len(got.data()) != 0 {
		t.Fatalf("not overdue yet: %s", got.raw)
	}
	if k, err := s.cmd.RemindOverdueNotifications(ctx); err != nil || k != 0 {
		t.Fatalf("early reminder: %d %v", k, err)
	}
	s.clock.Add(25 * time.Hour)
	if k, err := s.cmd.RemindOverdueNotifications(ctx); err != nil || k != 1 {
		t.Fatalf("first reminder: %d %v", k, err)
	}
	if k, _ := s.cmd.RemindOverdueNotifications(ctx); k != 0 {
		t.Fatalf("reminded twice in the same period: %d", k)
	}
	over := sm.get(platform + "?overdue=true&acknowledged=false")
	if len(over.data()) != 1 || over.data()[0]["notification_id"] != id || over.data()[0]["reminder_count"] != float64(1) {
		t.Fatalf("overdue list: %s", over.raw)
	}
	if got := sm.get(platform + "?acknowledged=true"); len(got.data()) != 0 {
		t.Fatalf("acknowledged filter: %s", got.raw)
	}
	s.clock.Add(24 * time.Hour)
	if k, err := s.cmd.RemindOverdueNotifications(ctx); err != nil || k != 1 {
		t.Fatalf("second reminder: %d %v", k, err)
	}
	if k := s.count(`SELECT count(*) FROM chat.moderation_actions WHERE case_id = $1 AND action = 'school_reminder_lead'`, caseID); k != 1 {
		t.Fatalf("team lead reminder: %d", k)
	}
	if k, _ := s.cmd.RemindOverdueNotifications(ctx); k != 0 {
		t.Fatalf("no third reminder: %d", k)
	}
	// MONITORING §3: overdue is counted once per notification (first reminder), not again for the team lead reminder.
	if d := schoolNotifications("overdue") - overdue0; d != 1 {
		t.Fatalf("school_notifications_total{overdue} delta %v", d)
	}
}

func schoolNotifications(status string) float64 {
	return testutil.ToFloat64(observability.SchoolNotifications().WithLabelValues(status))
}

// T035 / F121-AC-21: a case of the tenant common has no school — contact_path platform_direct, notifySchoolForCase 422,
// no moderation.school_notified; the temporary hold of the p1 case stays.
func TestSafetyCommonTenant(t *testing.T) {
	s := setupSafety(t)
	common, school := uuid.New(), uuid.New()
	s.tenant(common, "common", nil)
	an, binh := s.student(common, school), s.student(common, school)
	ch := s.open(channel.Party, an, binh)
	target := s.msg(ch, true, an, 1, "zqhen gap")
	reports0 := testutil.ToFloat64(observability.Reports().WithLabelValues("message", "asked_personal_info"))
	sent0 := schoolNotifications("sent")
	if r := s.report(binh, newKey(), "vi", map[string]any{"target_type": "message", "target_id": target, "reason": "asked_personal_info"}); r.status != http.StatusCreated {
		t.Fatalf("report: %d %s", r.status, r.raw)
	}
	// MONITORING §3: eduworld_reports_total{target_type,reason}.
	if d := testutil.ToFloat64(observability.Reports().WithLabelValues("message", "asked_personal_info")) - reports0; d != 1 {
		t.Fatalf("reports_total delta %v", d)
	}
	defer func() {
		if d := schoolNotifications("sent") - sent0; d != 0 { // the common tenant has no school notification
			t.Errorf("school_notifications_total{sent} delta %v", d)
		}
	}()
	caseID := s.caseOf(target)
	if c := s.caseRow(caseID); c["contact_path"] != "platform_direct" || c["hold"] == "" {
		t.Fatalf("common case: %v", c)
	}
	sm := s.moderator("safety_moderator", false)
	r := sm.post(common, "/admin/moderation/cases/"+caseID.String()+":notify-school", map[string]any{"school_id": school,
		"student_ids": []uuid.UUID{an.id}, "summary": "Tom tat theo mau cho nha truong."}, 0)
	if r.status != http.StatusUnprocessableEntity || r.code() != "VALIDATION_FAILED" {
		t.Fatalf("notify common: %d %s", r.status, r.raw)
	}
	if len(s.events("moderation.school_notified")) != 0 || s.count(`SELECT count(*) FROM chat.school_safety_notifications`) != 0 {
		t.Fatal("no notification for the tenant common")
	}
	if c := s.caseRow(caseID); c["contact_path"] != "platform_direct" || c["hold"] == "" {
		t.Fatalf("hold stays: %v", c)
	}
}

// T039 / F121-AC-22, AC-23: moderation-sla-escalator escalates a case past its first-response deadline (p1 after 1 hour,
// p2 after 24 hours) once, with a system action and audit, never a sanction; it publishes the queue gauges
// (eduworld_moderation_cases_open, oldest unclaimed age for ModerationP1Unclaimed after 15 minutes).
func TestSafetySLAEscalation(t *testing.T) {
	s := setupSafety(t)
	p1Case, _, _ := s.reportedCase(s.student(s.tenantA, s.schoolA), s.student(s.tenantA, s.schoolA), "asked_personal_info")
	p2Case, _, _ := s.reportedCase(s.student(s.tenantB, s.schoolB), s.student(s.tenantB, s.schoolB), "bullying")
	sanctions := s.count(`SELECT count(*) FROM chat.sanctions`)
	ctx := context.Background()
	gauges := observability.Safety{}
	s.clock.Add(16 * time.Minute)
	if n, err := s.cmd.EscalateOverdueCases(ctx, gauges, s.reads.QueueCounts); err != nil || n != 0 {
		t.Fatalf("nothing overdue at 16 min: %d %v", n, err)
	}
	if v := testutil.ToFloat64(observability.CasesOpen().WithLabelValues("p1", "false")); v != 1 {
		t.Fatalf("cases_open p1 unassigned: %v", v)
	}
	if v := testutil.ToFloat64(observability.UnclaimedAge().WithLabelValues("p1")); v < 900 {
		t.Fatalf("p1 unclaimed age past p1_unclaimed_alert_minutes: %v", v)
	}
	s.clock.Add(45 * time.Minute)
	if n, err := s.cmd.EscalateOverdueCases(ctx, gauges, s.reads.QueueCounts); err != nil || n != 1 {
		t.Fatalf("p1 escalated after 1 h: %d %v", n, err)
	}
	if c := s.caseRow(p1Case); c["status"] != "escalated" {
		t.Fatalf("p1: %v", c)
	}
	if c := s.caseRow(p2Case); c["status"] != "open" {
		t.Fatalf("p2 not yet: %v", c)
	}
	s.clock.Add(24 * time.Hour)
	if n, err := s.cmd.EscalateOverdueCases(ctx, gauges, s.reads.QueueCounts); err != nil || n != 1 {
		t.Fatalf("p2 escalated after 24 h: %d %v", n, err)
	}
	if n, _ := s.cmd.EscalateOverdueCases(ctx, gauges, s.reads.QueueCounts); n != 0 {
		t.Fatalf("escalated once: %d", n)
	}
	if n := s.count(`SELECT count(*) FROM chat.moderation_actions WHERE action = 'system_escalate' AND actor_kind = 'system'`); n != 2 {
		t.Fatalf("system actions: %d", n)
	}
	if n := s.count(`SELECT count(*) FROM chat.sanctions`); n != sanctions {
		t.Fatalf("no automatic sanction: %d → %d", sanctions, n)
	}
	if !hasAudit(s.audits(), "moderation.case.escalated") {
		t.Fatal("audit moderation.case.escalated")
	}
}

// T043 / F121-AC-25, AC-26 (TC-CHAT-022, TC-CHAT-009): once a gameplay channel is gone (purged by F117; simulated by
// soft-deleting the channel and its messages) the moderator sees only the stored evidence; the evidence is immutable
// (append-only) for the application roles; no log line, event or audit of the flow carries message content.
func TestSafetyEvidenceAfterPurge(t *testing.T) {
	logs := captureLogs(t)
	s := setupSafety(t)
	an, binh := s.student(s.tenantA, s.schoolA), s.student(s.tenantA, s.schoolA)
	caseID, ch, _ := s.partyCase(an, binh, "bullying")
	sm := s.moderator("safety_moderator", false)
	path := "/admin/moderation/cases/" + caseID.String()
	sm.post(s.tenantA, path+":assign", map[string]any{"assignee_moderator_id": sm.id}, 0)
	before := sm.get(path + "/messages")
	if len(before.data()) != 5 {
		t.Fatalf("before purge: %s", before.raw)
	}
	for i := 6; i <= 8; i++ { // later messages: context only (not in the snapshot)
		s.msg(ch, true, an, int64(i), "zqsau")
	}
	if got := sm.get(path + "/messages"); len(got.data()) != 8 {
		t.Fatalf("context while the channel exists: %d", len(got.data()))
	}
	// Purge simulated (F117 not yet implemented): channel and messages gone for the application.
	at := s.clock.Now()
	s.exec(`UPDATE chat.gameplay_messages SET deleted_at = $2 WHERE channel_id = $1 AND tenant_id = $3 AND deleted_at IS NULL`, ch, at, s.tenantA)
	s.exec(`UPDATE chat.channels SET deleted_at = $2 WHERE channel_id = $1 AND tenant_id = $3 AND deleted_at IS NULL`, ch, at, s.tenantA)
	after := sm.get(path + "/messages")
	if after.status != http.StatusOK || len(after.data()) != 5 {
		t.Fatalf("after purge: evidence only: %d %s", after.status, after.raw)
	}
	for _, m := range after.data() {
		if m["in_snapshot"] != true || m["body"] == nil {
			t.Fatalf("evidence message: %v", m)
		}
	}
	if _, err := s.app.ExecContext(context.Background(), `UPDATE chat.moderation_evidence SET sha256 = repeat('0', 64)
		WHERE case_id = $1 AND tenant_id = $2`, caseID, s.tenantA); err == nil {
		t.Fatal("evidence updated by the application role")
	}
	if n := s.count(`SELECT count(*) FROM chat.moderation_evidence WHERE case_id = $1`, caseID); n != 1 {
		t.Fatalf("evidence kept: %d", n)
	}
	s.noContent("zqparty", "zqsau")
	if strings.Contains(logs.String(), "zqparty") || strings.Contains(logs.String(), "zqsau") {
		t.Fatal("a log line carries message content")
	}
}
