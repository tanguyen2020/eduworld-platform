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
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// shift posts createModerationOncallShift with token (platform data: no X-Tenant-Id) and a new Idempotency-Key.
func (s *safetyEnv) shift(token string, moderator uuid.UUID, role string, from, to time.Time, key string) resp {
	s.t.Helper()
	if key == "" {
		key = newKey()
	}
	return s.do(http.MethodPost, "/admin/moderation/oncall-shifts", token, map[string]string{"Idempotency-Key": key},
		map[string]any{"moderator_user_id": moderator, "oncall_role": role, "starts_at": from.UTC().Format(time.RFC3339),
			"ends_at": to.UTC().Format(time.RFC3339), "note": "Ca trực thử"})
}

func (s *safetyEnv) caseLevel(id uuid.UUID) (level, via string, assignee *string) {
	s.t.Helper()
	var l, v, a *string
	if err := s.su.QueryRowContext(context.Background(), `SELECT oncall_page_level, assigned_via, assignee_moderator_id::text
		FROM chat.moderation_cases WHERE case_id = $1`, id).Scan(&l, &v, &a); err != nil {
		s.t.Fatal(err)
	}
	if l != nil {
		level = *l
	}
	if v != nil {
		via = *v
	}
	return level, via, a
}

// pagedLevels returns the levels of the moderation.oncall_paged events of a case, in order.
func (s *safetyEnv) pagedLevels(id uuid.UUID) []string {
	var out []string
	for _, e := range s.envelopes("moderation.oncall_paged") {
		var p map[string]any
		_ = json.Unmarshal(e.Payload, &p)
		if p["case_id"] == id.String() {
			if e.TenantID != nil {
				s.t.Fatalf("moderation.oncall_paged must be a platform event (recipients are platform staff): %v", e.TenantID)
			}
			out = append(out, p["level"].(string))
		}
	}
	return out
}

// T029, T033 / F124-AC-12, AC-16 (TC-CHAT-007, TC-CMS-WS-02): the team lead creates on-call shifts of platform staff
// holding moderation.case.act (checked through resolvePrincipal), overlaps of one role are 409 ONCALL_SHIFT_OVERLAP, the
// Idempotency-Key replays, delete is a soft-delete; a moderator without moderation.case.lead or a tenant role is 403;
// listing needs moderation.case.read; every write is audited.
func TestSafetyOncallShifts(t *testing.T) {
	authztest.Cover(t, "listModerationOncallShifts", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createModerationOncallShift", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "deleteModerationOncallShift", authztest.CrossTenant, authztest.RoleDenied)
	s := setupSafetyWith(t, true)
	lead, sm, sm2 := s.moderator("safety_moderator", true), s.moderator("safety_moderator", false), s.moderator("safety_moderator", false)
	now := s.clock.Now()
	// AC-16: no moderation.case.lead → 403; tenant roles → 403 (no route on the public gateway; called directly here).
	if r := s.shift(sm.token, sm.id, "primary", now, now.Add(time.Hour), ""); r.status != http.StatusForbidden || r.code() != "PERMISSION_DENIED" {
		t.Fatalf("moderator creates a shift: %d %s", r.status, r.raw)
	}
	for _, tok := range []string{s.staff(s.tenantA, "tenant_admin"), s.staff(s.tenantB, "school_admin")} {
		if r := s.shift(tok, sm.id, "primary", now, now.Add(time.Hour), ""); r.status != http.StatusForbidden {
			t.Fatalf("tenant role creates a shift: %d %s", r.status, r.raw)
		}
		if r := s.do(http.MethodGet, "/admin/moderation/oncall-shifts", tok, nil, nil); r.status != http.StatusForbidden {
			t.Fatalf("tenant role lists shifts: %d %s", r.status, r.raw)
		}
	}
	// AC-12: create, replay, overlap, unknown moderator, moderator without moderation.case.act.
	key := newKey()
	r := s.shift(lead.token, sm.id, "primary", now.Add(-time.Hour), now.Add(23*time.Hour), key)
	if r.status != http.StatusCreated || r.body["oncall_role"] != "primary" || r.body["moderator_user_id"] != sm.id.String() {
		t.Fatalf("create: %d %s", r.status, r.raw)
	}
	primary := r.body["shift_id"].(string)
	if r2 := s.shift(lead.token, sm.id, "primary", now.Add(-time.Hour), now.Add(23*time.Hour), key); r2.status != http.StatusCreated ||
		r2.body["shift_id"] != primary {
		t.Fatalf("replay: %d %s", r2.status, r2.raw)
	}
	if r := s.shift(lead.token, sm2.id, "primary", now.Add(22*time.Hour), now.Add(30*time.Hour), ""); r.status != http.StatusConflict ||
		r.code() != "ONCALL_SHIFT_OVERLAP" {
		t.Fatalf("overlap: %d %s", r.status, r.raw)
	}
	if r := s.shift(lead.token, sm2.id, "primary", now.Add(23*time.Hour), now.Add(30*time.Hour), ""); r.status != http.StatusCreated {
		t.Fatalf("adjacent shift: %d %s", r.status, r.raw)
	}
	if r := s.shift(lead.token, lead.id, "secondary", now.Add(-time.Hour), now.Add(23*time.Hour), ""); r.status != http.StatusCreated {
		t.Fatalf("secondary: %d %s", r.status, r.raw)
	}
	if r := s.shift(lead.token, uuid.New(), "secondary", now.Add(24*time.Hour), now.Add(25*time.Hour), ""); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown moderator: %d %s", r.status, r.raw)
	}
	content := s.moderator("content_admin", false)
	if r := s.shift(lead.token, content.id, "secondary", now.Add(24*time.Hour), now.Add(25*time.Hour), ""); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("moderator without moderation.case.act: %d %s", r.status, r.raw)
	}
	if r := s.shift(lead.token, sm2.id, "secondary", now.Add(24*time.Hour), now.Add(24*time.Hour+169*time.Hour), ""); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("longer than oncall_shift_max_hours: %d %s", r.status, r.raw)
	}
	if n := s.count(`SELECT count(*) FROM chat.moderation_oncall_shifts WHERE deleted_at IS NULL`); n != 3 {
		t.Fatalf("shifts: %d", n)
	}
	if len(s.auditMeta("moderation.oncall_shift.created")) != 3 {
		t.Fatal("audit moderation.oncall_shift.created")
	}
	// listModerationOncallShifts: moderation.case.read, keyset pages in SQL.
	l := sm.get("/admin/moderation/oncall-shifts?oncall_role=primary&limit=1")
	pg, _ := l.body["page"].(map[string]any)
	cursor, _ := pg["next_cursor"].(string)
	if l.status != http.StatusOK || len(l.data()) != 1 || l.data()[0]["shift_id"] != primary || cursor == "" {
		t.Fatalf("list page 1: %d %s", l.status, l.raw)
	}
	if l2 := sm.get("/admin/moderation/oncall-shifts?oncall_role=primary&limit=1&cursor=" + cursor); len(l2.data()) != 1 ||
		l2.data()[0]["shift_id"] == primary {
		t.Fatalf("list page 2: %s", l2.raw)
	}
	// Delete: soft-delete + audit; a moderator cannot, a tenant role of another tenant cannot; unknown → 404.
	del := func(tok, id string) resp {
		return s.do(http.MethodDelete, "/admin/moderation/oncall-shifts/"+id, tok, map[string]string{"Idempotency-Key": newKey()}, nil)
	}
	if r := del(sm.token, primary); r.status != http.StatusForbidden {
		t.Fatalf("moderator deletes: %d", r.status)
	}
	if r := del(s.staff(s.tenantB, "tenant_admin"), primary); r.status != http.StatusForbidden {
		t.Fatalf("tenant B deletes: %d", r.status)
	}
	if r := del(lead.token, primary); r.status != http.StatusNoContent {
		t.Fatalf("delete: %d %s", r.status, r.raw)
	}
	if r := del(lead.token, primary); r.status != http.StatusNotFound {
		t.Fatalf("delete twice: %d %s", r.status, r.raw)
	}
	if s.count(`SELECT count(*) FROM chat.moderation_oncall_shifts WHERE shift_id = $1 AND deleted_at IS NOT NULL AND deleted_by = $2`,
		primary, lead.id) != 1 || len(s.auditMeta("moderation.oncall_shift.deleted")) != 1 {
		t.Fatal("soft-delete + audit")
	}
}

// T030, T031 / F124-AC-13, AC-14 (TC-CHAT-042): a new p1 case is assigned to the primary on-call moderator by rotation
// (still open) with moderation.case_assigned + moderation.oncall_paged{primary}; the job oncall-escalator pages secondary
// at 1×, lead at 2× p1_unclaimed_alert_minutes and the System Admin past the first response of report_sla — each level
// once, one oncall_page action each; a claimed case is never paged again.
func TestSafetyOncallRotationAndLadder(t *testing.T) {
	s := setupSafetyWith(t, true)
	lead, sm := s.moderator("safety_moderator", true), s.moderator("safety_moderator", false)
	now := s.clock.Now()
	if r := s.shift(lead.token, sm.id, "primary", now.Add(-time.Hour), now.Add(48*time.Hour), ""); r.status != http.StatusCreated {
		t.Fatal(r.raw)
	}
	if r := s.shift(lead.token, lead.id, "secondary", now.Add(-time.Hour), now.Add(48*time.Hour), ""); r.status != http.StatusCreated {
		t.Fatal(r.raw)
	}
	caseID, _, _ := s.reportedCase(s.student(s.tenantA, s.schoolA), s.student(s.tenantA, s.schoolA), "asked_personal_info")
	row := s.caseRow(caseID)
	level, via, assignee := s.caseLevel(caseID)
	if row["status"] != "open" || level != "primary" || via != "rotation" || assignee == nil || *assignee != sm.id.String() {
		t.Fatalf("rotation: %v %s %s %v", row["status"], level, via, assignee)
	}
	if got := s.pagedLevels(caseID); len(got) != 1 || got[0] != "primary" {
		t.Fatalf("paged: %v", got)
	}
	if n := s.checkEvents("moderation.case_assigned", "moderation.oncall_paged"); n["moderation.case_assigned"] != 1 || n["moderation.oncall_paged"] != 1 {
		t.Fatalf("events: %v", n)
	}
	s.noContent("zqtin") // ids and levels only (R14)
	// A p2 case is not paged.
	// Other tenants for the next cases: a direct channel scope is unique per tenant within the test (channels_persistent_scope_uq).
	p2, _, _ := s.reportedCase(s.student(s.tenantB, s.schoolB), s.student(s.tenantB, s.schoolB), "bullying")
	if l, _, a := s.caseLevel(p2); l != "" || a != nil {
		t.Fatalf("p2 paged: %s %v", l, a)
	}
	run := func() int {
		n, err := s.cmd.EscalateUnclaimedP1(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	s.clock.Add(14 * time.Minute)
	if n := run(); n != 0 {
		t.Fatalf("before 1×: %d", n)
	}
	s.clock.Add(time.Minute) // 15 min: secondary (the paged moderator becomes the rotation assignee to claim it)
	if n := run(); n != 1 {
		t.Fatalf("1×: %d", n)
	}
	if l, v, a := s.caseLevel(caseID); l != "secondary" || v != "rotation" || a == nil || *a != lead.id.String() {
		t.Fatalf("secondary: %s %s %v", l, v, a)
	}
	if n := run(); n != 0 {
		t.Fatal("secondary paged twice")
	}
	s.clock.Add(15 * time.Minute) // 30 min: lead
	if n := run(); n != 1 {
		t.Fatalf("2×: %d", n)
	}
	s.clock.Add(30 * time.Minute) // 60 min: first response of report_sla p1 → System Admin
	if n := run(); n != 1 {
		t.Fatalf("report_sla: %d", n)
	}
	s.clock.Add(3 * time.Hour)
	if n := run(); n != 0 {
		t.Fatal("paged after system_admin")
	}
	if got := s.pagedLevels(caseID); strings.Join(got, ",") != "primary,secondary,lead,system_admin" {
		t.Fatalf("ladder: %v", got)
	}
	if n := s.count(`SELECT count(*) FROM chat.moderation_actions WHERE case_id = $1 AND action = 'oncall_page'`, caseID); n != 4 {
		t.Fatalf("oncall_page actions: %d", n)
	}
	if testutil.ToFloat64(observability.OncallPages().WithLabelValues("system_admin")) < 1 {
		t.Fatal("eduworld_moderation_oncall_pages_total")
	}
	// A claimed case (assignModerationCase → in_review) is never paged again.
	claimed, _, _ := s.reportedCase(s.student(s.tenantU, s.schoolU1), s.student(s.tenantU, s.schoolU1), "asked_personal_info")
	if r := sm.post(s.tenantU, "/admin/moderation/cases/"+claimed.String()+":assign", map[string]any{"assignee_moderator_id": sm.id}, 0); r.status != http.StatusOK ||
		r.body["status"] != "in_review" || r.body["assigned_via"] != "manual" {
		t.Fatalf("claim the rotation case: %d %s", r.status, r.raw)
	}
	s.clock.Add(2 * time.Hour)
	if n := run(); n != 0 {
		t.Fatalf("claimed case paged: %d", n)
	}
	if got := s.pagedLevels(claimed); len(got) != 1 {
		t.Fatalf("claimed ladder: %v", got)
	}
}

// T032 / F124-AC-15: without a current primary (or secondary) shift the next level with people is paged at once; the
// coverage gauge reads 0 for the empty role (alert ModerationOncallGap); a shift deleted while its case is open leaves the
// case to the next level.
func TestSafetyOncallCoverageGap(t *testing.T) {
	s := setupSafetyWith(t, true)
	lead, sm := s.moderator("safety_moderator", true), s.moderator("safety_moderator", false)
	now := s.clock.Now()
	caseID, _, _ := s.reportedCase(s.student(s.tenantA, s.schoolA), s.student(s.tenantA, s.schoolA), "asked_personal_info")
	if l, _, a := s.caseLevel(caseID); l != "lead" || a != nil {
		t.Fatalf("no shift: lead at once, no assignee: %s %v", l, a)
	}
	if _, err := s.cmd.EscalateUnclaimedP1(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"primary", "secondary"} {
		if v := testutil.ToFloat64(observability.OncallCoverage().WithLabelValues(role)); v != 0 {
			t.Fatalf("coverage %s: %v", role, v)
		}
	}
	// Only a secondary shift: a new p1 case pages the secondary at once.
	r := s.shift(lead.token, sm.id, "secondary", now.Add(-time.Hour), now.Add(24*time.Hour), "")
	if r.status != http.StatusCreated {
		t.Fatal(r.raw)
	}
	second, _, _ := s.reportedCase(s.student(s.tenantB, s.schoolB), s.student(s.tenantB, s.schoolB), "asked_personal_info")
	if l, v, a := s.caseLevel(second); l != "secondary" || v != "rotation" || a == nil || *a != sm.id.String() {
		t.Fatalf("secondary at once: %s %s %v", l, v, a)
	}
	if _, err := s.cmd.EscalateUnclaimedP1(context.Background()); err != nil {
		t.Fatal(err)
	}
	if testutil.ToFloat64(observability.OncallCoverage().WithLabelValues("secondary")) != 1 ||
		testutil.ToFloat64(observability.OncallCoverage().WithLabelValues("primary")) != 0 {
		t.Fatal("coverage gauge")
	}
	// The shift is deleted while the case is open: the next level (lead) is paged at its mark.
	if r := s.do(http.MethodDelete, "/admin/moderation/oncall-shifts/"+r.body["shift_id"].(string), lead.token,
		map[string]string{"Idempotency-Key": newKey()}, nil); r.status != http.StatusNoContent {
		t.Fatal(r.raw)
	}
	s.clock.Add(30 * time.Minute)
	if _, err := s.cmd.EscalateUnclaimedP1(context.Background()); err != nil {
		t.Fatal(err)
	}
	if l, _, _ := s.caseLevel(second); l != "lead" {
		t.Fatalf("after the shift is deleted: %s", l)
	}
}

// T025 / F124-AC-08 (producer side, TC-CHAT-043): the reminder job of F121 publishes moderation.school_notification_reminded
// (ids, reminder number, ack_due_at — no content) for module identity to email the school again.
func TestSafetySchoolReminderEvent(t *testing.T) {
	s := setupSafetyWith(t, true)
	caseID, an, _, _ := s.multiCase()
	sm := s.moderator("safety_moderator", false)
	r := sm.post(s.tenantA, "/admin/moderation/cases/"+caseID.String()+":notify-school", map[string]any{"school_id": s.schoolA,
		"student_ids": []uuid.UUID{an.id}, "summary": "Tom tat zqmau cho nha truong."}, 0)
	if r.status != http.StatusCreated {
		t.Fatalf("notify: %d %s", r.status, r.raw)
	}
	s.clock.Add(73 * time.Hour)
	if n, err := s.cmd.RemindOverdueNotifications(context.Background()); err != nil || n != 1 {
		t.Fatalf("remind: %d %v", n, err)
	}
	evs := s.envelopes("moderation.school_notification_reminded")
	if len(evs) != 1 || evs[0].TenantID == nil || *evs[0].TenantID != s.tenantA {
		t.Fatalf("reminded event: %+v", evs)
	}
	if n := s.checkEvents("moderation.school_notification_reminded"); n["moderation.school_notification_reminded"] != 1 {
		t.Fatalf("schema: %v", n)
	}
	var p map[string]any
	_ = json.Unmarshal(evs[0].Payload, &p)
	if p["reminder_no"] != float64(1) || p["school_id"] != s.schoolA.String() {
		t.Fatalf("payload: %v", p)
	}
	s.noContent("zqmau")
}
