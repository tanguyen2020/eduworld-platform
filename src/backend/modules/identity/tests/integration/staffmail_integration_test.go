//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/safetymetrics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// recordingMailer is a TEST DOUBLE of port.StaffMailer (no email leaves the test; the SMTP rendering is covered by the
// adapter tests): it records the mails and fails while fail is set.
type recordingMailer struct {
	mu   sync.Mutex
	sent []port.StaffMail
	fail bool
}

func (m *recordingMailer) SendStaffMail(_ context.Context, s port.StaffMail) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return errors.New("mail: rcpt: smtp reply 451")
	}
	m.sent = append(m.sent, s)
	return nil
}

func (m *recordingMailer) to(addr string) []port.StaffMail {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []port.StaffMail
	for _, s := range m.sent {
		if s.To == addr {
			out = append(out, s)
		}
	}
	return out
}

type mailEnv struct {
	e      *env
	mailer *recordingMailer
	clock  time.Time
	mail   command.StaffMail
	c      consumer.StaffMailer
	m      *safetymetrics.Metrics
}

func newMailEnv(t *testing.T, e *env, maxAttempts int) *mailEnv {
	t.Helper()
	m, err := safetymetrics.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	me := &mailEnv{e: e, mailer: &recordingMailer{}, clock: time.Now().UTC(), m: m}
	me.mail = command.StaffMail{UoW: e.uow, Deliveries: write.EmailDeliveryRepo{}, Directory: write.StaffDirectoryRepo{}, Mailer: me.mailer,
		Audit: e.deps.Audit, Metrics: m, MaxAttempts: maxAttempts, Now: func() time.Time { return me.clock },
		Links: command.MailLinks{Base: "http://localhost:5173", SchoolNotifications: "/school-lms/safety-notifications",
			ModerationCases: "/live-ops/moderation-cases", AuditLogs: "/platform-admin/audit-logs"}}
	me.c = consumer.StaffMailer{Mail: me.mail}
	return me
}

// deliver hands one event to identity.staff_mailer in the scope pkg/outbox uses: the envelope tenant, or the platform
// scope of the consumer for a platform event.
func (me *mailEnv) deliver(ctx context.Context, t *testing.T, typ string, tenant *uuid.UUID, eventID uuid.UUID, payload map[string]any) error {
	t.Helper()
	raw, _ := json.Marshal(payload)
	env := outbox.Envelope{EventID: eventID, EventType: typ, OccurredAt: me.clock, TenantID: tenant, Payload: raw}
	if tenant == nil {
		ps, err := authz.SystemPlatformScope("consumer-identity-staff-mailer")
		if err != nil {
			t.Fatal(err)
		}
		return me.e.uow.InTenantTx(ctx, ps, func(ctx context.Context) error { return me.c.Handle(ctx, env) })
	}
	sc, err := authz.EventScope(*tenant)
	if err != nil {
		t.Fatal(err)
	}
	return me.e.uow.InTenantTx(ctx, sc, func(ctx context.Context) error { return me.c.Handle(ctx, env) })
}

func (me *mailEnv) send(ctx context.Context, t *testing.T) int {
	t.Helper()
	n, err := me.mail.SendDue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// platformRole creates a platform role with permissions (the lead role of the Safety Team is a custom platform role).
func platformRole(ctx context.Context, t *testing.T, e *env, code string, perms ...string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := e.platform.ExecContext(ctx, `INSERT INTO identity.roles (role_id, tenant_id, created_at, updated_at, code, name_vi, name_en,
		role_scope_kind, is_system) VALUES ($1, NULL, now(), now(), $2, $2, $2, 'platform', false)`, id, code); err != nil {
		t.Fatal(err)
	}
	for _, p := range perms {
		if _, err := e.platform.ExecContext(ctx, `INSERT INTO identity.role_permissions (role_permission_id, tenant_id, created_at, updated_at,
			role_id, permission_code) VALUES ($1, NULL, now(), now(), $2, $3)`, uuid.New(), id, p); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

func grantCustom(ctx context.Context, t *testing.T, e *env, user, role uuid.UUID) {
	t.Helper()
	if _, err := e.platform.ExecContext(ctx, `INSERT INTO identity.user_roles (user_role_id, tenant_id, created_at, updated_at, user_id, role_id,
		scope_type, valid_from, granted_by, source) VALUES ($1, NULL, now(), now(), $2, $3, 'platform', now() - interval '1 day', $2, 'manual')`,
		uuid.New(), user, role); err != nil {
		t.Fatal(err)
	}
}

func schoolNotified(tenant, school, notification, caseID uuid.UUID, due time.Time) map[string]any {
	return map[string]any{"notification_id": notification, "case_id": caseID, "tenant_id": tenant, "school_id": school, "severity": "p1",
		"category_code": "asked_personal_info", "school_notified_at": due.Add(-24 * time.Hour), "ack_due_at": due}
}

// T020–T024 / F124-AC-07..AC-11 (TC-CHAT-043, TC-CHAT-009, TC-SVC-003, TC-TEN-003): moderation.school_notified emails the
// School Admins of the school only (never another school or tenant; a locked account is not a recipient), with the
// fixed template, the school name, the due instant and the portal link — the deliveries store recipient ids and
// allowed params only (no case id, category, student or address); one email per (event, recipient, template);
// moderation.school_notification_reminded emails the reminder n; SMTP failures retry with backoff until failed (metric);
// a recipient whose role ended before sending is skipped; a school without School Admin falls back to the Tenant Admins;
// without any recipient the no-recipient counter rises.
func TestStaffMailSchoolNotification(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	me := newMailEnv(t, e, 2)
	a := seedTenant(ctx, t, e.platform, "pgd-ml-a")
	b := seedTenant(ctx, t, e.platform, "pgd-ml-b")
	ta, sa, so := uuid.MustParse(a.tenant), uuid.MustParse(a.school), uuid.MustParse(a.otherSchool)
	tb := uuid.MustParse(b.tenant)
	adA := staffUser(ctx, t, e, "ad.truong.ml.a", &a.tenant)
	grantRole(ctx, t, e, adA, "school_admin", &a.tenant, "school", &a.school, "NULL")
	adA2 := staffUser(ctx, t, e, "ad.truong.ml.a2", &a.tenant) // role ends before sending → skipped
	grantRole(ctx, t, e, adA2, "school_admin", &a.tenant, "school", &a.school, "NULL")
	lockedAd := staffUser(ctx, t, e, "ad.truong.ml.khoa", &a.tenant)
	grantRole(ctx, t, e, lockedAd, "school_admin", &a.tenant, "school", &a.school, "NULL")
	if _, err := e.platform.ExecContext(ctx, `UPDATE identity.users SET status = 'locked' WHERE user_id = $1`, lockedAd); err != nil {
		t.Fatal(err)
	}
	adOther := staffUser(ctx, t, e, "ad.truong.ml.khac", &a.tenant)
	grantRole(ctx, t, e, adOther, "school_admin", &a.tenant, "school", &a.otherSchool, "NULL")
	tenantAd := staffUser(ctx, t, e, "ad.donvi.ml", &a.tenant)
	grantRole(ctx, t, e, tenantAd, "tenant_admin", &a.tenant, "tenant", nil, "NULL")
	adB := staffUser(ctx, t, e, "ad.truong.ml.b", &b.tenant)
	grantRole(ctx, t, e, adB, "school_admin", &b.tenant, "school", &b.school, "NULL")
	caseID, notification := uuid.MustParse("0193a6c1-0000-7000-8000-00000000c5e1"), uuid.New()
	due := me.clock.Add(24 * time.Hour)
	ev := uuid.New()

	// AC-07, AC-10: one delivery per recipient, the event delivered twice changes nothing.
	for i := 0; i < 2; i++ {
		if err := me.deliver(ctx, t, "moderation.school_notified", &ta, ev, schoolNotified(ta, sa, notification, caseID, due)); err != nil {
			t.Fatal(err)
		}
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.email_deliveries WHERE source_event_id = $1`, ev); n != 2 {
		t.Fatalf("deliveries: %d (school admins of the school, active only)", n)
	}
	// AC-11: the role of adA2 ends before sending (soft-deleted) → skipped.
	if _, err := e.platform.ExecContext(ctx, `UPDATE identity.user_roles SET deleted_at = now() WHERE user_id = $1`, adA2); err != nil {
		t.Fatal(err)
	}
	if n := me.send(ctx, t); n != 1 {
		t.Fatalf("sent: %d", n)
	}
	got := me.mailer.to("ad.truong.ml.a@test.vn")
	if len(got) != 1 || got[0].Template != "school_safety_notification" || got[0].Link != "http://localhost:5173/school-lms/safety-notifications" ||
		got[0].SchoolName == "" {
		t.Fatalf("school admin email: %+v", got)
	}
	for _, addr := range []string{"ad.truong.ml.a2@test.vn", "ad.truong.ml.khoa@test.vn", "ad.truong.ml.khac@test.vn", "ad.donvi.ml@test.vn",
		"ad.truong.ml.b@test.vn"} {
		if len(me.mailer.to(addr)) != 0 {
			t.Fatalf("%s must not receive the email", addr)
		}
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM identity.email_deliveries WHERE recipient_user_id = $1 AND status = 'skipped'`, adA2) != 1 ||
		count(ctx, t, e.platform, `SELECT count(*) FROM identity.email_deliveries WHERE recipient_user_id = $1 AND status = 'sent'`, adA) != 1 {
		t.Fatal("statuses")
	}
	// AC-09: no case id, category, student or address in deliveries, params, audit or the rendered parameters.
	var dump string
	if err := e.platform.QueryRowContext(ctx, `SELECT coalesce(string_agg(row_to_json(d)::text, ''), '') FROM identity.email_deliveries d`).Scan(&dump); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{caseID.String(), "asked_personal_info", "@test.vn", "Hoc Sinh"} {
		if strings.Contains(dump, bad) {
			t.Fatalf("email_deliveries carries %q: %s", bad, dump)
		}
	}
	if _, ok := got[0].Params["case_id"]; ok || got[0].Params["category_code"] != "" {
		t.Fatalf("mail params: %v", got[0].Params)
	}
	if sent := auditActions(ctx, t, e, "notification.email_sent"); len(sent) != 1 || sent[0]["recipient_user_id"] != adA.String() ||
		sent[0]["template"] != "school_safety_notification" {
		t.Fatalf("audit notification.email_sent: %v", sent)
	}
	var leak int
	if err := e.relay.QueryRowContext(ctx, `SELECT count(*) FROM infra.outbox_events WHERE envelope::text LIKE '%@test.vn%'`).Scan(&leak); err != nil || leak != 0 {
		t.Fatalf("an address reached the outbox: %d %v", leak, err)
	}
	// AC-08: the reminder n.
	if err := me.deliver(ctx, t, "moderation.school_notification_reminded", &ta, uuid.New(), map[string]any{"notification_id": notification,
		"case_id": caseID, "tenant_id": ta, "school_id": sa, "reminder_no": 1, "ack_due_at": due, "reminded_at": due}); err != nil {
		t.Fatal(err)
	}
	me.send(ctx, t)
	if r := me.mailer.to("ad.truong.ml.a@test.vn"); len(r) != 2 || r[1].Template != "school_safety_reminder" || r[1].Params["reminder_no"] != "1" {
		t.Fatalf("reminder: %+v", r)
	}
	// AC-10: SMTP down → retry with backoff, then failed (max 2 attempts here), metric failed; the consumer is never blocked.
	me.mailer.fail = true
	ev2 := uuid.New()
	if err := me.deliver(ctx, t, "moderation.school_notified", &ta, ev2, schoolNotified(ta, sa, uuid.New(), caseID, due)); err != nil {
		t.Fatal(err)
	}
	me.send(ctx, t)
	if count(ctx, t, e.platform, `SELECT count(*) FROM identity.email_deliveries WHERE source_event_id = $1 AND status = 'pending' AND attempts = 1
		AND next_attempt_at > $2 AND last_error_code IS NOT NULL`, ev2, me.clock) != 1 {
		t.Fatal("first failure: retry later")
	}
	if n := me.send(ctx, t); n != 0 {
		t.Fatal("not due before the backoff")
	}
	me.clock = me.clock.Add(time.Hour)
	me.send(ctx, t)
	if count(ctx, t, e.platform, `SELECT count(*) FROM identity.email_deliveries WHERE source_event_id = $1 AND status = 'failed' AND attempts = 2`, ev2) != 1 {
		t.Fatal("failed after the maximum")
	}
	if testutil.ToFloat64(me.m.Deliveries().WithLabelValues("school_safety_notification", "failed")) != 1 {
		t.Fatal("eduworld_email_deliveries_total{status=failed}")
	}
	me.mailer.fail = false
	// Fallback: the other school has a School Admin; tenant B's school without one → its Tenant Admin. No recipient at all → counter.
	tbAd := staffUser(ctx, t, e, "ad.donvi.ml.b", &b.tenant)
	grantRole(ctx, t, e, tbAd, "tenant_admin", &b.tenant, "tenant", nil, "NULL")
	if err := me.deliver(ctx, t, "moderation.school_notified", &tb, uuid.New(), schoolNotified(tb, uuid.MustParse(b.otherSchool), uuid.New(), caseID, due)); err != nil {
		t.Fatal(err)
	}
	me.send(ctx, t)
	if len(me.mailer.to("ad.donvi.ml.b@test.vn")) != 1 || len(me.mailer.to("ad.truong.ml.b@test.vn")) != 0 {
		t.Fatal("tenant admin fallback")
	}
	if _, err := e.platform.ExecContext(ctx, `UPDATE identity.user_roles SET deleted_at = now() WHERE user_id IN ($1, $2)`, adOther, tenantAd); err != nil {
		t.Fatal(err)
	}
	if err := me.deliver(ctx, t, "moderation.school_notified", &ta, uuid.New(), schoolNotified(ta, so, uuid.New(), caseID, due)); err != nil {
		t.Fatal(err)
	}
	if testutil.ToFloat64(me.m.NoRecipients().WithLabelValues("school_safety_notification")) != 1 {
		t.Fatal("no recipient counter (alert SchoolSafetyNotificationNoRecipient)")
	}
}

// T030, T038 (module identity side) / F124-AC-13, AC-17: moderation.oncall_paged emails the paged on-call moderator
// (primary/secondary), the team leads (moderation.case.lead through a platform role other than system_admin) or the System
// Admins — priority and case link only; break_glass.requested emails every other holder of platform.break_glass.approve,
// never the requester and never the reason.
func TestStaffMailOncallAndBreakGlass(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	me := newMailEnv(t, e, 8)
	sm := staffUser(ctx, t, e, "sm.ml.antoan", nil)
	grantRole(ctx, t, e, sm, "safety_moderator", nil, "platform", nil, "NULL")
	lead := staffUser(ctx, t, e, "sm.ml.truongca", nil)
	grantCustom(ctx, t, e, lead, platformRole(ctx, t, e, "safety_lead_ml", "moderation.case.lead", "moderation.case.act"))
	sa1 := staffUser(ctx, t, e, "sa.ml.1", nil)
	grantRole(ctx, t, e, sa1, "system_admin", nil, "platform", nil, "NULL")
	sa2 := staffUser(ctx, t, e, "sa.ml.2", nil)
	grantRole(ctx, t, e, sa2, "system_admin", nil, "platform", nil, "NULL")
	caseID, tenant := uuid.New(), uuid.New()
	paged := func(level string, mod *uuid.UUID) {
		t.Helper()
		if err := me.deliver(ctx, t, "moderation.oncall_paged", nil, uuid.New(), map[string]any{"case_id": caseID, "tenant_id": tenant,
			"severity": "p1", "level": level, "moderator_user_id": mod, "paged_at": me.clock}); err != nil {
			t.Fatal(err)
		}
	}
	paged("primary", &sm)
	paged("lead", nil)
	paged("system_admin", nil)
	me.send(ctx, t)
	if r := me.mailer.to("sm.ml.antoan@test.vn"); len(r) != 1 || r[0].Template != "moderation_oncall_page" || r[0].Params["severity"] != "p1" ||
		r[0].Link != "http://localhost:5173/live-ops/moderation-cases?case_id="+caseID.String() {
		t.Fatalf("primary page: %+v", r)
	}
	if r := me.mailer.to("sm.ml.truongca@test.vn"); len(r) != 1 {
		t.Fatalf("lead page: %+v", r)
	}
	if len(me.mailer.to("sa.ml.1@test.vn")) != 1 || len(me.mailer.to("sa.ml.2@test.vn")) != 1 {
		t.Fatal("system admin page (and system admins are not leads)")
	}
	if _, ok := me.mailer.to("sm.ml.antoan@test.vn")[0].Params["tenant_id"]; ok {
		t.Fatal("tenant in the page")
	}
	// A page event with a tenant envelope is not ours (platform event only) → DLQ.
	if err := me.deliver(ctx, t, "moderation.oncall_paged", &tenant, uuid.New(), map[string]any{"case_id": caseID, "tenant_id": tenant,
		"severity": "p1", "level": "lead", "paged_at": me.clock}); !errors.Is(err, outbox.ErrPermanent) {
		t.Fatalf("tenant envelope: %v", err)
	}
	grant := uuid.New()
	if err := me.deliver(ctx, t, "break_glass.requested", nil, uuid.New(), map[string]any{"grant_id": grant, "tenant_id": tenant,
		"requested_by": sa1, "requested_at": me.clock}); err != nil {
		t.Fatal(err)
	}
	me.send(ctx, t)
	if r := me.mailer.to("sa.ml.2@test.vn"); len(r) != 2 || r[1].Template != "break_glass_request" || r[1].Params["grant_id"] != grant.String() {
		t.Fatalf("break-glass email to the second admin: %+v", r)
	}
	for _, m := range me.mailer.to("sa.ml.1@test.vn") {
		if m.Template == "break_glass_request" {
			t.Fatal("the requester must not be emailed")
		}
	}
	if len(me.mailer.to("sm.ml.truongca@test.vn")) != 1 {
		t.Fatal("a lead without platform.break_glass.approve")
	}
}
