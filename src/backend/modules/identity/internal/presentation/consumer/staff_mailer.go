package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// StaffMailerName is the durable consumer that turns the events of the Safety Team into staff emails (F124 US2–US4).
const StaffMailerName = "identity.staff_mailer"

// StaffMailerSubjects are the events it handles (stream EVT_CHAT): school notifications and reminders (tenant events),
// on-call pages and break-glass requests (platform events: their recipients are platform staff).
var StaffMailerSubjects = []string{"ew.evt.chat.moderation.school_notified", "ew.evt.chat.moderation.school_notification_reminded",
	"ew.evt.chat.moderation.oncall_paged", "ew.evt.chat.break_glass.requested"}

// StaffMailer dispatches the events to command.StaffMail in the consumer transaction (one delivery per event, recipient and
// template; sending happens in the job email-sender, never here).
type StaffMailer struct {
	Mail command.StaffMail
}

type staffMailPayload struct {
	NotificationID  uuid.UUID  `json:"notification_id"`
	TenantID        uuid.UUID  `json:"tenant_id"`
	SchoolID        uuid.UUID  `json:"school_id"`
	AckDueAt        time.Time  `json:"ack_due_at"`
	ReminderNo      int        `json:"reminder_no"`
	CaseID          uuid.UUID  `json:"case_id"`
	Severity        string     `json:"severity"`
	Level           string     `json:"level"`
	ModeratorUserID *uuid.UUID `json:"moderator_user_id"`
	GrantID         uuid.UUID  `json:"grant_id"`
	RequestedBy     uuid.UUID  `json:"requested_by"`
}

// Handle implements outbox.Handler.
func (c StaffMailer) Handle(ctx context.Context, e outbox.Envelope) error {
	var p staffMailPayload
	if err := json.Unmarshal(e.Payload, &p); err != nil {
		return outbox.Permanent(fmt.Errorf("%s: invalid payload", e.EventType))
	}
	switch e.EventType {
	case "moderation.school_notified", "moderation.school_notification_reminded":
		if e.TenantID == nil || *e.TenantID != p.TenantID || p.SchoolID == uuid.Nil || p.NotificationID == uuid.Nil {
			return outbox.Permanent(fmt.Errorf("%s: invalid payload or tenant", e.EventType))
		}
		ev := command.SchoolEvent{EventID: e.EventID, TenantID: p.TenantID, SchoolID: p.SchoolID, NotificationID: p.NotificationID,
			AckDueAt: p.AckDueAt}
		if e.EventType == "moderation.school_notification_reminded" {
			if p.ReminderNo < 1 {
				return outbox.Permanent(fmt.Errorf("%s: reminder_no", e.EventType))
			}
			ev.ReminderNo = p.ReminderNo
		}
		return c.Mail.School(ctx, ev)
	case "moderation.oncall_paged":
		if e.TenantID != nil || p.CaseID == uuid.Nil || p.Level == "" {
			return outbox.Permanent(fmt.Errorf("%s: invalid payload or not a platform event", e.EventType))
		}
		return c.Mail.Oncall(ctx, command.OncallEvent{EventID: e.EventID, CaseID: p.CaseID, Severity: p.Severity, Level: p.Level,
			Moderator: p.ModeratorUserID})
	case "break_glass.requested":
		if e.TenantID != nil || p.GrantID == uuid.Nil || p.RequestedBy == uuid.Nil {
			return outbox.Permanent(fmt.Errorf("%s: invalid payload or not a platform event", e.EventType))
		}
		return c.Mail.BreakGlass(ctx, command.BreakGlassEvent{EventID: e.EventID, GrantID: p.GrantID, RequestedBy: p.RequestedBy})
	}
	return outbox.Permanent(fmt.Errorf("unexpected event %s", e.EventType))
}
