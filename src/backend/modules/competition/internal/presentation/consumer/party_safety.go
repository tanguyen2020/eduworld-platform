package consumer

import (
	"context"
	"database/sql"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// PartySafety is what the safety consumers apply to parties (command.PartySafety).
type PartySafety interface {
	Blocked(ctx context.Context, tenant, blocker, blocked uuid.UUID) (bool, error)
	Sanction(ctx context.Context, tenant, sanction, user uuid.UUID, typ string, until *time.Time, active bool, version int64) (bool, error)
	StudentSynced(ctx context.Context, tenant, student, school uuid.UUID, status string) (bool, error)
	StatusChanged(ctx context.Context, tenant *uuid.UUID, user uuid.UUID, to string) (bool, error)
	CleanedUp(ctx context.Context, tenant, student uuid.UUID) (bool, error)
}

// PartySafetyChat is the consumer competition.party_safety_chat (F100-AC-25): user.blocked (owner F095),
// sanction.applied / sanction.lifted (owner F121; payload of EVENT_DESIGN §4: sanction_id, user_id, tenant_id,
// sanction_type, until). Ids only (R14).
type PartySafetyChat struct{ S PartySafety }

// PartySafetyChatOptions returns its options.
func PartySafetyChatOptions(h PartySafetyChat, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "competition", Name: messaging.PartySafetyChatName, Stream: messaging.ChatStream,
		Subjects: messaging.Subjects("chat", messaging.PartySafetyChatEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c PartySafetyChat) Handle(ctx context.Context, e outbox.Envelope) error {
	var p struct {
		TenantID      uuid.UUID  `json:"tenant_id"`
		BlockerID     uuid.UUID  `json:"blocker_id"`
		BlockedID     uuid.UUID  `json:"blocked_id"`
		SanctionID    uuid.UUID  `json:"sanction_id"`
		UserID        uuid.UUID  `json:"user_id"`
		SanctionType  *string    `json:"sanction_type"`
		Until         *time.Time `json:"until"`
		SourceVersion *int64     `json:"source_version"`
	}
	if !slices.Contains(messaging.PartySafetyChatEvents, e.EventType) {
		return nil
	}
	if err := decode(e, &p); err != nil {
		return outbox.Permanent(err)
	}
	tenant, err := envTenant(e)
	if err != nil || p.TenantID != tenant {
		return outbox.Permanent(invalid(e.EventType, "tenant_id"))
	}
	if e.EventType == "user.blocked" {
		if p.BlockerID == uuid.Nil || p.BlockedID == uuid.Nil || p.BlockerID == p.BlockedID {
			return outbox.Permanent(invalid(e.EventType, "blocker_id/blocked_id"))
		}
		_, err := c.S.Blocked(ctx, tenant, p.BlockerID, p.BlockedID)
		return classify("", e, false, err)
	}
	if p.SanctionID == uuid.Nil || p.UserID == uuid.Nil || p.SanctionType == nil || !maxLen(p.SanctionType, 64) {
		return outbox.Permanent(invalid(e.EventType, "sanction_id/user_id/sanction_type"))
	}
	ok, err := c.S.Sanction(ctx, tenant, p.SanctionID, p.UserID, *p.SanctionType, p.Until, e.EventType == "sanction.applied",
		version(p.SourceVersion, e))
	return classify("sanctions", e, ok, err)
}

// PartySafetyIdentity is the consumer competition.party_safety_identity (F100 edge cases): student.synced (school
// change, deactivation), user.status_changed (locked, deactivated, anonymized), student.cleaned_up.
type PartySafetyIdentity struct{ S PartySafety }

// PartySafetyIdentityOptions returns its options (user.status_changed may be a platform event: Platform true).
func PartySafetyIdentityOptions(h PartySafetyIdentity, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "competition", Name: messaging.PartySafetyIdentityName, Stream: messaging.IdentityStream,
		Subjects: messaging.Subjects("identity", messaging.PartySafetyIdentityEvents), Platform: true, UoW: uow, DLQDB: dlq, JS: js,
		Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c PartySafetyIdentity) Handle(ctx context.Context, e outbox.Envelope) error {
	var p struct {
		StudentID uuid.UUID `json:"student_id"`
		SchoolID  uuid.UUID `json:"school_id"`
		TenantID  uuid.UUID `json:"tenant_id"`
		Action    string    `json:"action"`
		Status    *string   `json:"status"`
		UserID    uuid.UUID `json:"user_id"`
		To        string    `json:"to"`
	}
	if !slices.Contains(messaging.PartySafetyIdentityEvents, e.EventType) {
		return nil
	}
	if err := decode(e, &p); err != nil {
		return outbox.Permanent(err)
	}
	switch e.EventType {
	case "user.status_changed":
		if p.UserID == uuid.Nil || p.To == "" || len(p.To) > 64 {
			return outbox.Permanent(invalid(e.EventType, "user_id and to are required"))
		}
		_, err := c.S.StatusChanged(ctx, e.TenantID, p.UserID, p.To)
		return classify("", e, false, err)
	case "student.cleaned_up":
		tenant, err := envTenant(e)
		if err != nil || p.StudentID == uuid.Nil || p.TenantID != tenant {
			return outbox.Permanent(invalid(e.EventType, "student_id and tenant_id"))
		}
		_, err = c.S.CleanedUp(ctx, tenant, p.StudentID)
		return classify("", e, false, err)
	default: // student.synced
		tenant, err := envTenant(e)
		if err != nil || p.StudentID == uuid.Nil || p.SchoolID == uuid.Nil {
			return outbox.Permanent(invalid(e.EventType, "student_id and school_id are required"))
		}
		status := ""
		if p.Status != nil {
			status = *p.Status
		} else if p.Action == "deactivate" {
			status = "deactivated"
		}
		_, err = c.S.StudentSynced(ctx, tenant, p.StudentID, p.SchoolID, status)
		return classify("", e, false, err)
	}
}
