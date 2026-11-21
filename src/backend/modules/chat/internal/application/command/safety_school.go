package command

import (
	"context"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/safety"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// NotifyInput is SchoolSafetyNotificationCreate.
type NotifyInput struct {
	SchoolID   uuid.UUID
	StudentIDs []uuid.UUID
	Summary    string
}

// AckInput is SchoolSafetyNotificationAck.
type AckInput struct {
	Outcome string
	Note    *string
}

// NotifySchool implements notifySchoolForCase (F121-AC-17, AC-21, MODERATION §6.4): a template summary for one school
// of the case with only its students (each a student of the case — reported or reporter — of that school, else 422),
// ack_due_at by school_notification_ack_sla_hours, school_notified_at set once on the case, moderation.school_notified;
// an unacknowledged notification of the same school is 409. A case of the tenant common has no school: contact_path is
// set to platform_direct and the request is 422 (the temporary hold stays).
func (h Safety) NotifySchool(ctx context.Context, id uuid.UUID, in NotifyInput) (out read.NotificationRow, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.NotifySchool")
	defer func() { done(err) }()
	p, tenant, err := moderatorWrite(ctx, authz.PermModerationSchoolNotificationSend)
	if err != nil {
		return out, err
	}
	if n := len([]rune(in.Summary)); n < 10 || n > 1000 {
		return out, problem.Invalid("summary", problem.SubInvalid, "10 to 1000 characters")
	}
	if len(in.StudentIDs) == 0 || len(in.StudentIDs) > safety.MaxStudents {
		return out, problem.Invalid("student_ids", problem.SubInvalid, "1 to 20 students")
	}
	now := h.now()
	common := false
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := h.lockCase(ctx, tenant, id)
		if err != nil {
			return err
		}
		typ, err := h.Store.TenantType(ctx, tenant)
		if err != nil {
			return err
		}
		if typ == "common" {
			common = true
			if c.ContactPath == safety.PathPlatformDirect {
				return nil
			}
			c.ContactPath = safety.PathPlatformDirect
			c.Version++
			return h.Store.UpdateCase(ctx, *c, &p.UserID)
		}
		reports, err := h.Store.CaseReports(ctx, tenant, c.ID)
		if err != nil {
			return err
		}
		reporters := make([]uuid.UUID, 0, len(reports))
		for _, r := range reports {
			reporters = append(reporters, r.ReporterID)
		}
		students := make([]read.NotificationStudent, 0, len(in.StudentIDs))
		for _, sid := range in.StudentIDs {
			role := ""
			switch {
			case slices.Contains(c.Subjects, sid):
				role = safety.RoleOffender
			case slices.Contains(reporters, sid):
				role = safety.RoleAffected
			}
			st, err := h.Store.Student(ctx, tenant, sid)
			if err != nil {
				return err
			}
			if role == "" || st == nil || st.Deleted || st.SchoolID != in.SchoolID {
				return problem.Invalid("student_ids", problem.SubInvalid, "every student must be a student of the case in this school")
			}
			if !slices.ContainsFunc(students, func(s read.NotificationStudent) bool { return s.StudentID == sid }) {
				students = append(students, read.NotificationStudent{StudentID: sid, Role: role})
			}
		}
		pending, err := h.Store.PendingNotification(ctx, tenant, c.ID, in.SchoolID)
		if err != nil {
			return err
		}
		if pending != nil {
			return problem.New(problem.Conflict, "an unacknowledged notification of this school exists")
		}
		measures, err := h.measures(ctx, c)
		if err != nil {
			return err
		}
		out = read.NotificationRow{ID: ids.New(), TenantID: tenant, CaseID: c.ID, SchoolID: in.SchoolID, Severity: c.Severity,
			Category: c.Category, Students: students, MeasuresTaken: measures, Summary: in.Summary, SchoolNotifiedAt: now,
			AckDueAt: safety.AckDue(c.Severity, deref(c.Category), now), CreatedAt: now, Version: 1}
		if err := h.Store.InsertNotification(ctx, out, p.UserID); err != nil {
			return err
		}
		if c.SchoolNotifiedAt == nil {
			t := now
			c.SchoolNotifiedAt = &t
		}
		c.ContactPath = safety.PathSchool
		c.Version++
		if err := h.Store.UpdateCase(ctx, *c, &p.UserID); err != nil {
			return err
		}
		if err := h.action(ctx, c, safety.ActionNotifySchool, &p.UserID, "moderator", nil, nil, nil, now); err != nil {
			return err
		}
		if err := h.publish(ctx, EventSchoolNotified, "school_safety_notification", out.ID, tenant, c.ID.String(), now, map[string]any{
			"notification_id": out.ID, "case_id": c.ID, "tenant_id": tenant, "school_id": in.SchoolID, "severity": c.Severity,
			"category_code": c.Category, "school_notified_at": now, "ack_due_at": out.AckDueAt}); err != nil {
			return err
		}
		if err := h.record(ctx, AuditSchoolNotified, "school_safety_notification", out.ID, &in.SchoolID, map[string]any{
			"case_id": c.ID.String(), "tenant_id": tenant.String(), "students": len(students)}); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, service.NotificationBody(out))
	})
	if err != nil {
		return out, h.finish(ctx, p, id, err)
	}
	if common {
		return out, problem.Invalid("school_id", problem.SubInvalid, "a case of the common tenant has no school: contact_path platform_direct")
	}
	return out, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// measures is the server-built list of the measures taken on the case (sanction types, the temporary hold), ≤ 500.
func (h Safety) measures(ctx context.Context, c *read.CaseRow) (*string, error) {
	list, err := h.Store.CaseSanctions(ctx, c.TenantID, c.ID)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, s := range list {
		name := s.Type
		if safety.Holds(s.ReasonCode) {
			name = safety.CodeCaseHold
		}
		seen[name] = true
	}
	if len(seen) == 0 {
		return nil, nil
	}
	names := make([]string, 0, len(seen))
	for k := range seen {
		names = append(names, k)
	}
	sort.Strings(names)
	m := strings.Join(names, ", ")
	return &m, nil
}

// schoolGrant authorizes perm for a school of the tenant of a school staff member: 403 without the permission, false
// when the school is outside the grants (the caller answers 404, never revealing another school).
func schoolGrant(ctx context.Context, perm authz.Permission) (authz.Principal, uuid.UUID, error) {
	p, err := principal(ctx)
	if err != nil {
		return p, uuid.Nil, err
	}
	if p.TenantID == nil {
		return p, uuid.Nil, problem.New(problem.TenantHeaderRequired, "")
	}
	tenant := *p.TenantID
	if err := authz.Require(ctx, p, perm, authz.Resource{TenantID: &tenant}); err != nil {
		if !p.Has(perm) {
			return p, uuid.Nil, err
		}
		// limited to schools: the school of the notification decides below.
	}
	if p.UserType != authz.UserTypeStaff {
		return p, uuid.Nil, problem.New(problem.PermissionDenied, "")
	}
	return p, tenant, nil
}

// CoversSchool reports a grant of perm covering school (no limit: the whole tenant).
func CoversSchool(p authz.Principal, perm authz.Permission, school uuid.UUID) bool {
	for _, g := range p.Grants {
		if slices.Contains(g.Permissions, perm) && (g.SchoolID == nil || *g.SchoolID == school) {
			return true
		}
	}
	return false
}

// AcknowledgeSchoolNotification implements acknowledgeSchoolSafetyNotification (F121-AC-19): the school admin of the
// notified school (or the tenant admin) records that the parents were contacted — school_ack_at, acknowledged_by,
// contact_outcome on the notification and (first) on the case, moderation.school_acknowledged. A notification of
// another school or tenant is 404; a second acknowledgement is 409.
func (h Safety) AcknowledgeSchoolNotification(ctx context.Context, id uuid.UUID, in AckInput) (out read.NotificationRow, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.AcknowledgeSchoolNotification")
	defer func() { done(err) }()
	p, tenant, err := schoolGrant(ctx, authz.PermModerationSchoolNotificationAcknowledge)
	if err != nil {
		return out, err
	}
	if !slices.Contains(safety.Outcomes, in.Outcome) {
		return out, problem.Invalid("contact_outcome", problem.SubInvalid, "unknown outcome")
	}
	if in.Note != nil && len([]rune(*in.Note)) > 500 {
		return out, problem.Invalid("note", problem.SubMaxLength, "at most 500 characters")
	}
	now := h.now()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		n, err := h.Store.LockNotification(ctx, tenant, id)
		if err != nil {
			return err
		}
		if n == nil || !CoversSchool(p, authz.PermModerationSchoolNotificationAcknowledge, n.SchoolID) {
			return notFound()
		}
		if n.SchoolAckAt != nil {
			return problem.New(problem.Conflict, "the notification is already acknowledged")
		}
		t, by, outcome := now, p.UserID, in.Outcome
		n.SchoolAckAt, n.AcknowledgedBy, n.ContactOutcome, n.AckNote = &t, &by, &outcome, in.Note
		n.Version++
		if err := h.Store.UpdateNotification(ctx, *n, &p.UserID); err != nil {
			return err
		}
		c, err := h.Store.LockCase(ctx, tenant, n.CaseID)
		if err != nil {
			return err
		}
		if c != nil {
			if c.SchoolAckAt == nil {
				c.SchoolAckAt, c.AcknowledgedBy, c.ContactOutcome = &t, &by, &outcome
				c.Version++
				if err := h.Store.UpdateCase(ctx, *c, &p.UserID); err != nil {
					return err
				}
			}
			if err := h.action(ctx, c, safety.ActionSchoolAck, &p.UserID, "school", nil, nil, nil, now); err != nil {
				return err
			}
		}
		if err := h.publish(ctx, EventSchoolAcknowledged, "school_safety_notification", n.ID, tenant, n.CaseID.String(), now, map[string]any{
			"notification_id": n.ID, "case_id": n.CaseID, "tenant_id": tenant, "school_id": n.SchoolID, "school_ack_at": now,
			"acknowledged_by": p.UserID, "contact_outcome": in.Outcome}); err != nil {
			return err
		}
		out = *n
		if err := h.record(ctx, AuditSchoolAcknowledged, "school_safety_notification", n.ID, &n.SchoolID, map[string]any{
			"case_id": n.CaseID.String(), "contact_outcome": in.Outcome}); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, service.NotificationBody(out))
	})
	return out, err
}

// RemindOverdueNotifications is one run of the job school-notification-reminder (F121-AC-20, MODERATION §6.4): an
// unacknowledged notification past ack_due_at is reminded once (reminder_count 1: the school sees it again in the portal,
// the assignee in the case history); past a second SLA period it is reminded again for the team lead (reminder_count 2,
// action school_reminder_lead). Work list under tenancy.Bypass(platform_job); each item in the scope of its tenant.
func (h Safety) RemindOverdueNotifications(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.RemindOverdueNotifications")
	defer func() { done(err) }()
	if ok, err := h.lease(ctx, "school-notification-reminder"); err != nil || !ok {
		return 0, err
	}
	now := h.now()
	var due []read.NotificationRow
	if err := h.platformRead(ctx, func(ctx context.Context) error {
		var e error
		due, e = h.Store.DueNotifications(ctx, now, 500)
		return e
	}); err != nil {
		return 0, err
	}
	for _, d := range due {
		sc, err := authz.EventScope(d.TenantID)
		if err != nil {
			continue
		}
		err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
			cur, err := h.Store.LockNotification(ctx, d.TenantID, d.ID)
			if err != nil || cur == nil || cur.SchoolAckAt != nil || !reminderDue(*cur, now) {
				return err
			}
			cur.ReminderCount++
			t := now
			cur.LastRemindedAt = &t
			cur.Version++
			if err := h.Store.UpdateNotification(ctx, *cur, nil); err != nil {
				return err
			}
			act := safety.ActionSchoolReminder
			if cur.ReminderCount >= 2 {
				act = safety.ActionSchoolReminderLd
			}
			if c, err := h.Store.LockCase(ctx, d.TenantID, cur.CaseID); err != nil {
				return err
			} else if c != nil {
				if err := h.action(ctx, c, act, nil, "system", nil, nil, nil, now); err != nil {
					return err
				}
			}
			n++
			return h.record(ctx, AuditSchoolReminded, "school_safety_notification", cur.ID, &cur.SchoolID, map[string]any{
				"case_id": cur.CaseID.String(), "reminder_no": cur.ReminderCount})
		})
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

// reminderDue: first reminder at ack_due_at, the second (team lead) one SLA period later.
func reminderDue(n read.NotificationRow, now time.Time) bool {
	switch n.ReminderCount {
	case 0:
		return !now.Before(n.AckDueAt)
	case 1:
		return !now.Before(n.AckDueAt.Add(n.AckDueAt.Sub(n.SchoolNotifiedAt)))
	}
	return false
}
