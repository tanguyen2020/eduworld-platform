package consumer

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/notification"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// NotificationCreator is the CreateNotifications use case (command.Notifications).
type NotificationCreator interface {
	FromEvent(ctx context.Context, src notification.Source, recipients []uuid.UUID, actor *uuid.UUID) (int, error)
	FromAssignment(ctx context.Context, src notification.Source, assignment uuid.UUID) (int, error)
}

// Notifier is the handler of the consumers chat.notify_* (F123 T001, T017): it decodes the source event (payload schema
// of its producer), checks the tenant of the payload against the envelope and hands the recipients to
// CreateNotifications. Idempotency is processed_events (same transaction) plus the unique key of social.notifications;
// a payload error goes to the DLQ at once; any other error (module learning down) is retried, then dead-lettered
// (F123-AC-07). Payloads carry ids only: nothing is logged (R14).
type Notifier struct{ H NotificationCreator }

// NotifierConfig overrides the delivery policy of the consumers (tests); zero values keep the pkg/outbox defaults.
type NotifierConfig struct {
	MaxDeliver int
	BackOff    []time.Duration
}

// NotifierNames are the durable consumers of the notifier, one per producer stream (+ the moderation notices of chat).
var NotifierNames = []string{messaging.NotifyFriendRequestName, messaging.NotifyModerationName, messaging.NotifyPartyInvitationName,
	messaging.NotifyAssignmentName, messaging.NotifyProgressionName}

// NotifierOptions returns the options of the consumer name (one of NotifierNames).
func NotifierOptions(name string, h Notifier, cfg NotifierConfig, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	r := messaging.Notifiers[name]
	return outbox.ConsumerOptions{Module: "chat", Name: name, Stream: r.Stream, Subjects: messaging.Subjects(r.Producer, r.Events),
		MaxDeliver: cfg.MaxDeliver, BackOff: cfg.BackOff, UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c Notifier) Handle(ctx context.Context, e outbox.Envelope) error {
	if _, ok := notification.MessageCatalog[e.EventType]; !ok {
		return nil
	}
	tenant, err := envTenant(e)
	if err != nil {
		return outbox.Permanent(err)
	}
	src := notification.Source{EventID: e.EventID, EventType: e.EventType, TenantID: tenant, OccurredAt: e.OccurredAt}
	switch e.EventType {
	case notification.EventFriendRequestSent:
		var p struct {
			RequestID   uuid.UUID `json:"request_id"`
			TenantID    uuid.UUID `json:"tenant_id"`
			RequesterID uuid.UUID `json:"requester_id"`
			AddresseeID uuid.UUID `json:"addressee_id"`
		}
		if err := decode(e, &p); err != nil {
			return outbox.Permanent(err)
		}
		if p.TenantID != tenant || p.RequestID == uuid.Nil || p.RequesterID == uuid.Nil || p.AddresseeID == uuid.Nil {
			return outbox.Permanent(invalid(e.EventType, "tenant_id/request_id/requester_id/addressee_id"))
		}
		src.RefID = &p.RequestID
		return c.one(ctx, src, p.AddresseeID, &p.RequesterID)
	case notification.EventPartyInvitation:
		var p struct {
			InvitationID uuid.UUID `json:"invitation_id"`
			TenantID     uuid.UUID `json:"tenant_id"`
			InviterID    uuid.UUID `json:"inviter_id"`
			InviteeID    uuid.UUID `json:"invitee_id"`
		}
		if err := decode(e, &p); err != nil {
			return outbox.Permanent(err)
		}
		if p.TenantID != tenant || p.InvitationID == uuid.Nil || p.InviterID == uuid.Nil || p.InviteeID == uuid.Nil {
			return outbox.Permanent(invalid(e.EventType, "tenant_id/invitation_id/inviter_id/invitee_id"))
		}
		src.RefID = &p.InvitationID
		return c.one(ctx, src, p.InviteeID, &p.InviterID)
	case notification.EventAssignmentPublish, notification.EventAssignmentResults:
		var p struct {
			AssignmentID uuid.UUID `json:"assignment_id"`
			Status       *string   `json:"status"`
		}
		if err := decode(e, &p); err != nil {
			return outbox.Permanent(err)
		}
		if p.AssignmentID == uuid.Nil {
			return outbox.Permanent(invalid(e.EventType, "assignment_id"))
		}
		// assignment.published carries the status: only an open assignment is new work (F123-AC-02).
		if e.EventType == notification.EventAssignmentPublish && (p.Status == nil || *p.Status != "open") {
			return nil
		}
		src.RefID = &p.AssignmentID
		_, err := c.H.FromAssignment(ctx, src, p.AssignmentID)
		return notifyResult(err)
	case notification.EventLevelReached, notification.EventAchievementUnlock:
		return c.progression(ctx, e, src)
	case notification.EventReportResolved:
		var p struct {
			TenantID   uuid.UUID `json:"tenant_id"`
			ReportID   uuid.UUID `json:"report_id"`
			ReporterID uuid.UUID `json:"reporter_id"`
		}
		if err := decode(e, &p); err != nil {
			return outbox.Permanent(err)
		}
		if p.TenantID != tenant || p.ReportID == uuid.Nil || p.ReporterID == uuid.Nil {
			return outbox.Permanent(invalid(e.EventType, "tenant_id/report_id/reporter_id"))
		}
		src.RefID = &p.ReportID
		return c.one(ctx, src, p.ReporterID, nil)
	}
	return nil
}

func (c Notifier) one(ctx context.Context, src notification.Source, recipient uuid.UUID, actor *uuid.UUID) error {
	_, err := c.H.FromEvent(ctx, src, []uuid.UUID{recipient}, actor)
	return notifyResult(err)
}

// progression handles level.reached and achievement.unlocked (owner F074/F075, payload schemas of module game): the
// student is student_id of the payload, else the envelope subject of type student; params level (to_level) / tier.
func (c Notifier) progression(ctx context.Context, e outbox.Envelope, src notification.Source) error {
	var p struct {
		StudentID     uuid.UUID       `json:"student_id"`
		ToLevel       *int            `json:"to_level"`
		AchievementID uuid.UUID       `json:"achievement_id"`
		Tier          json.RawMessage `json:"tier"`
	}
	if err := decode(e, &p); err != nil {
		return outbox.Permanent(err)
	}
	student := p.StudentID
	if student == uuid.Nil && e.Subject.Type == "student" {
		if id, err := uuid.Parse(e.Subject.ID); err == nil {
			student = id
		}
	}
	if student == uuid.Nil {
		return outbox.Permanent(invalid(e.EventType, "student_id"))
	}
	src.Params = map[string]string{}
	if e.EventType == notification.EventLevelReached {
		if p.ToLevel == nil || *p.ToLevel < 1 {
			return outbox.Permanent(invalid(e.EventType, "to_level"))
		}
		src.Params["level"] = strconv.Itoa(*p.ToLevel)
	} else {
		if p.AchievementID == uuid.Nil {
			return outbox.Permanent(invalid(e.EventType, "achievement_id"))
		}
		src.RefID = &p.AchievementID
		if tier := tierOf(p.Tier); tier != "" {
			src.Params["tier"] = tier
		}
	}
	return c.one(ctx, src, student, nil)
}

// tierOf reads the tier of achievement.unlocked as a short string or a number.
func tierOf(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil && len(s) <= 64 {
		return s
	}
	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		return n.String()
	}
	return ""
}

// notifyResult keeps a domain refusal of the payload permanent (DLQ at once); anything else is retried.
func notifyResult(err error) error {
	if errors.Is(err, notification.ErrParam) || errors.Is(err, notification.ErrUnknownSource) {
		return outbox.Permanent(err)
	}
	return err
}
