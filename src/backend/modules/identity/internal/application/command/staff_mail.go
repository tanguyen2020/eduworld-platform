package command

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/staffmail"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Audit and defaults of the staff emails (F124 US2–US4).
const (
	AuditEmailSent = "notification.email_sent"
	// DefaultEmailMaxAttempts is email_delivery_max_attempts (CONFIG_DEFAULTS; EW_EMAIL_DELIVERY_MAX_ATTEMPTS).
	DefaultEmailMaxAttempts = 8
	// EmailSenderInterval is the period of the job email-sender (≤ 5 minutes end to end, SC-003).
	EmailSenderInterval = 30 * time.Second
	emailLease          = 2 * time.Minute
	emailBatch          = 100
	permCaseLead        = "moderation.case.lead"
	permCaseAct         = "moderation.case.act"
	permBreakGlass      = "platform.break_glass.approve"
)

// MailMetrics observes the staff emails: eduworld_email_deliveries_total{template,status} and the school notifications
// without any recipient (alert SchoolSafetyNotificationNoRecipient).
type MailMetrics interface {
	Delivery(template, status string)
	NoRecipient(template string)
}

// MailLinks are the portal links of the emails: EW_PORTAL_BASE_URL + the route of the portal catalog (R3: no hardcoded
// environment URL).
type MailLinks struct {
	Base                string
	SchoolNotifications string // menu.school_lms.safety_notifications
	ModerationCases     string // menu.live_ops.moderation_cases
	AuditLogs           string // menu.platform_admin.audit_logs (break-glass approvals)
}

// StaffMail turns the events of the Safety Team into staff emails (F124-AC-07..AC-11, AC-13, AC-17): the consumers enqueue
// one delivery per (event, recipient, template) in the consumer transaction — recipient ids only — and the job
// email-sender sends them with retries, re-checking each recipient when sending. Sending never blocks a consumer.
type StaffMail struct {
	UoW         write.UnitOfWork
	Deliveries  write.EmailDeliveryRepository
	Directory   write.StaffDirectory
	Mailer      port.StaffMailer
	Audit       write.Auditor
	Metrics     MailMetrics // nil: none
	Links       MailLinks
	MaxAttempts int // email_delivery_max_attempts (0: default 8)
	Now         func() time.Time
}

func (h StaffMail) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func (h StaffMail) metric(template, status string) {
	if h.Metrics != nil {
		h.Metrics.Delivery(template, status)
	}
}

// SchoolEvent is moderation.school_notified / moderation.school_notification_reminded (ids and instants only).
type SchoolEvent struct {
	EventID        uuid.UUID
	TenantID       uuid.UUID
	SchoolID       uuid.UUID
	NotificationID uuid.UUID
	AckDueAt       time.Time
	ReminderNo     int // 0: the notification itself
}

// OncallEvent is moderation.oncall_paged.
type OncallEvent struct {
	EventID   uuid.UUID
	CaseID    uuid.UUID
	Severity  string
	Level     string
	Moderator *uuid.UUID
}

// BreakGlassEvent is break_glass.requested.
type BreakGlassEvent struct {
	EventID     uuid.UUID
	GrantID     uuid.UUID
	RequestedBy uuid.UUID
}

func (h StaffMail) enqueue(ctx context.Context, event uuid.UUID, tenant *uuid.UUID, template string, params map[string]string,
	recipients []uuid.UUID) (int, error) {
	n := 0
	seen := map[uuid.UUID]bool{}
	for _, r := range recipients {
		if seen[r] {
			continue
		}
		seen[r] = true
		ok, err := h.Deliveries.Enqueue(ctx, write.EmailDelivery{TenantID: tenant, SourceEventID: event, Template: template, RecipientID: r,
			Params: staffmail.Params(template, params), Locale: "vi", Status: staffmail.StatusPending, CreatedAt: h.now()})
		if err != nil {
			return n, err
		}
		if ok {
			n++
		}
	}
	return n, nil
}

// schoolRecipients: the School Admins of the school, otherwise the Tenant Admins of the tenant (F124-AC-07).
func (h StaffMail) schoolRecipients(ctx context.Context, tenant, school uuid.UUID) ([]uuid.UUID, error) {
	list, err := h.Directory.TenantRole(ctx, tenant, staffmail.RoleSchoolAdmin, &school)
	if err != nil || len(list) > 0 {
		return list, err
	}
	return h.Directory.TenantRole(ctx, tenant, staffmail.RoleTenantAdmin, nil)
}

// School handles moderation.school_notified (ReminderNo 0, template school_safety_notification) and
// moderation.school_notification_reminded (school_safety_reminder) in the consumer transaction (tenant scope).
func (h StaffMail) School(ctx context.Context, ev SchoolEvent) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.StaffMail.School")
	defer func() { done(err) }()
	template := staffmail.TemplateSchoolNotification
	params := map[string]string{"school_id": ev.SchoolID.String(), "notification_id": ev.NotificationID.String(),
		"ack_due_at": ev.AckDueAt.UTC().Format(time.RFC3339)}
	if ev.ReminderNo > 0 {
		template = staffmail.TemplateSchoolReminder
		params["reminder_no"] = strconv.Itoa(ev.ReminderNo)
	}
	recipients, err := h.schoolRecipients(ctx, ev.TenantID, ev.SchoolID)
	if err != nil {
		return err
	}
	if len(recipients) == 0 {
		h.metric(template, staffmail.StatusSkipped)
		if h.Metrics != nil {
			h.Metrics.NoRecipient(template)
		}
		return nil
	}
	tenant := ev.TenantID
	_, err = h.enqueue(ctx, ev.EventID, &tenant, template, params, recipients)
	return err
}

// Oncall handles moderation.oncall_paged (platform event, platform scope): the on-call moderator of primary/secondary,
// the team leads (moderation.case.lead, not through the System Admin role) or the System Admins (F124-AC-13..AC-15).
func (h StaffMail) Oncall(ctx context.Context, ev OncallEvent) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.StaffMail.Oncall")
	defer func() { done(err) }()
	var recipients []uuid.UUID
	switch ev.Level {
	case "primary", "secondary":
		if ev.Moderator != nil {
			recipients = []uuid.UUID{*ev.Moderator}
		}
	case "lead":
		recipients, err = h.Directory.PlatformPermission(ctx, permCaseLead, staffmail.RoleSystemAdmin)
	case "system_admin":
		recipients, err = h.Directory.PlatformRole(ctx, staffmail.RoleSystemAdmin)
	}
	if err != nil {
		return err
	}
	if len(recipients) == 0 {
		h.metric(staffmail.TemplateOncallPage, staffmail.StatusSkipped)
		return nil
	}
	_, err = h.enqueue(ctx, ev.EventID, nil, staffmail.TemplateOncallPage, map[string]string{"case_id": ev.CaseID.String(),
		"severity": ev.Severity, "level": ev.Level}, recipients)
	return err
}

// BreakGlass handles break_glass.requested (platform event): every other holder of platform.break_glass.approve
// (F124-AC-17) — never the reason or the scope.
func (h StaffMail) BreakGlass(ctx context.Context, ev BreakGlassEvent) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.StaffMail.BreakGlass")
	defer func() { done(err) }()
	all, err := h.Directory.PlatformPermission(ctx, permBreakGlass, "")
	if err != nil {
		return err
	}
	var recipients []uuid.UUID
	for _, u := range all {
		if u != ev.RequestedBy {
			recipients = append(recipients, u)
		}
	}
	if len(recipients) == 0 {
		h.metric(staffmail.TemplateBreakGlassRequest, staffmail.StatusSkipped)
		return nil
	}
	_, err = h.enqueue(ctx, ev.EventID, nil, staffmail.TemplateBreakGlassRequest, map[string]string{"grant_id": ev.GrantID.String()}, recipients)
	return err
}

// eligible re-checks a recipient when sending (F124-AC-11): still holding the role/permission of the template, in the
// tenant/school of the event.
func (h StaffMail) eligible(ctx context.Context, d write.EmailDelivery) (bool, error) {
	in := func(list []uuid.UUID, err error) (bool, error) {
		for _, u := range list {
			if u == d.RecipientID {
				return true, err
			}
		}
		return false, err
	}
	switch d.Template {
	case staffmail.TemplateSchoolNotification, staffmail.TemplateSchoolReminder:
		school, ok := parseID(d.Params["school_id"])
		if !ok || d.TenantID == nil {
			return false, nil // a malformed delivery is never sent (skipped), not retried
		}
		if ok, err := in(h.Directory.TenantRole(ctx, *d.TenantID, staffmail.RoleSchoolAdmin, &school)); ok || err != nil {
			return ok, err
		}
		return in(h.Directory.TenantRole(ctx, *d.TenantID, staffmail.RoleTenantAdmin, nil))
	case staffmail.TemplateOncallPage:
		switch d.Params["level"] {
		case "lead":
			return in(h.Directory.PlatformPermission(ctx, permCaseLead, staffmail.RoleSystemAdmin))
		case "system_admin":
			return in(h.Directory.PlatformRole(ctx, staffmail.RoleSystemAdmin))
		}
		return in(h.Directory.PlatformPermission(ctx, permCaseAct, ""))
	case staffmail.TemplateBreakGlassRequest:
		return in(h.Directory.PlatformPermission(ctx, permBreakGlass, ""))
	}
	return false, nil
}

func (h StaffMail) link(d write.EmailDelivery) string {
	switch d.Template {
	case staffmail.TemplateSchoolNotification, staffmail.TemplateSchoolReminder:
		return h.Links.Base + h.Links.SchoolNotifications
	case staffmail.TemplateOncallPage:
		return h.Links.Base + h.Links.ModerationCases + "?case_id=" + url.QueryEscape(d.Params["case_id"])
	}
	return h.Links.Base + h.Links.AuditLogs
}

type prepared struct {
	mail    port.StaffMail
	skip    bool
	skipWhy string
}

// SendDue is one run of the job email-sender (F124-AC-10, AC-11): claims the due deliveries (lease), re-checks each
// recipient and reads its address (skipped otherwise), sends outside any transaction, then records sent / retry with
// exponential backoff / failed after email_delivery_max_attempts (metric + alert EmailDeliveryFailing). Platform scope
// (job): every statement carries its explicit tenant condition. Logs carry ids only, never an address (R14).
func (h StaffMail) SendDue(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.StaffMail.SendDue")
	defer func() { done(err) }()
	sc, err := authz.SystemPlatformScope("email-sender")
	if err != nil {
		return 0, err
	}
	ctx = tenancy.WithScope(ctx, sc)
	max := h.MaxAttempts
	if max <= 0 {
		max = DefaultEmailMaxAttempts
	}
	now := h.now()
	var due []write.EmailDelivery
	if err := h.UoW.Do(ctx, func(ctx context.Context) error {
		var e error
		due, e = h.Deliveries.Claim(ctx, now, emailLease, emailBatch)
		return e
	}); err != nil {
		return 0, err
	}
	for _, d := range due {
		var p prepared
		if err := h.UoW.Do(ctx, func(ctx context.Context) error {
			ok, err := h.eligible(ctx, d)
			if err != nil {
				return err
			}
			if !ok {
				p.skip, p.skipWhy = true, "not_eligible"
				return nil
			}
			c, err := h.Directory.Contact(ctx, d.RecipientID)
			if err != nil {
				return err
			}
			if c == nil {
				p.skip, p.skipWhy = true, "no_contact"
				return nil
			}
			p.mail = port.StaffMail{To: c.Email, Locale: c.Locale, Template: d.Template, Params: d.Params, Link: h.link(d)}
			if staffmail.School(d.Template) && d.TenantID != nil {
				if school, err := uuid.Parse(d.Params["school_id"]); err == nil {
					if p.mail.SchoolName, err = h.Directory.SchoolName(ctx, *d.TenantID, school); err != nil {
						return err
					}
				}
			}
			return nil
		}); err != nil {
			return n, err
		}
		if p.skip {
			why := p.skipWhy
			if err := h.UoW.Do(ctx, func(ctx context.Context) error {
				return h.Deliveries.Finish(ctx, d.ID, staffmail.StatusSkipped, d.Attempts, &why, now)
			}); err != nil {
				return n, err
			}
			h.metric(d.Template, staffmail.StatusSkipped)
			continue
		}
		sendErr := h.Mailer.SendStaffMail(ctx, p.mail)
		attempts := d.Attempts + 1
		err := h.UoW.Do(ctx, func(ctx context.Context) error {
			if sendErr == nil {
				if err := h.Deliveries.Finish(ctx, d.ID, staffmail.StatusSent, attempts, nil, h.now()); err != nil {
					return err
				}
				return h.Audit.Record(ctx, audit.Event{Action: AuditEmailSent, TargetType: "email_delivery", TargetID: d.ID.String(),
					Result: audit.ResultSuccess, Metadata: map[string]any{"template": d.Template, "recipient_user_id": d.RecipientID.String()}})
			}
			code := errorCode(sendErr)
			if staffmail.Exhausted(attempts, max) {
				return h.Deliveries.Finish(ctx, d.ID, staffmail.StatusFailed, attempts, &code, h.now())
			}
			return h.Deliveries.Retry(ctx, d.ID, attempts, now.Add(staffmail.Backoff(attempts)), code)
		})
		if err != nil {
			return n, err
		}
		switch {
		case sendErr == nil:
			h.metric(d.Template, staffmail.StatusSent)
			n++
		case staffmail.Exhausted(attempts, max):
			h.metric(d.Template, staffmail.StatusFailed)
		default:
			h.metric(d.Template, "retry")
		}
	}
	return n, nil
}

// errorCode keeps a short code of a send error (the SMTP adapter already reduces replies to their code, R14).
func errorCode(err error) string {
	var c interface{ Code() string }
	if errors.As(err, &c) {
		return c.Code()
	}
	s := err.Error()
	if len(s) > 64 {
		s = s[:64]
	}
	return s
}

// parseID parses a uuid parameter of a delivery (false when absent or malformed).
func parseID(s string) (uuid.UUID, bool) {
	id, err := uuid.Parse(s)
	return id, err == nil
}
