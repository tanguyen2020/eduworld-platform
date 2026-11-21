package consumer

import (
	"context"
	"database/sql"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// CaseOpener opens the case of a pipeline signal (command.Safety).
type CaseOpener interface {
	OpenCaseFromSignal(ctx context.Context, s command.Signal) (uuid.UUID, error)
}

// MessageHeld is the consumer chat.moderation_cases (F121-AC-03): every chat.message_held (hold_for_review,
// block_and_alert, owner F116) opens or joins the case of the held message; idempotent by event id (processed_events and
// the source_event_id of the case). The payload carries ids and codes only (R14).
type MessageHeld struct{ H CaseOpener }

// MessageHeldOptions returns its options.
func MessageHeldOptions(h MessageHeld, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "chat", Name: messaging.ModerationCasesName, Stream: messaging.ChatStream,
		Subjects: messaging.Subjects("chat", messaging.ModerationCasesEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c MessageHeld) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.EventType != command.EventMessageHeld {
		return nil
	}
	var p struct {
		TenantID    uuid.UUID `json:"tenant_id"`
		MessageID   uuid.UUID `json:"message_id"`
		ChannelID   uuid.UUID `json:"channel_id"`
		ChannelType string    `json:"channel_type"`
		SenderID    uuid.UUID `json:"sender_id"`
		Decision    string    `json:"decision"`
		Reasons     []string  `json:"reasons"`
		CreatedAt   time.Time `json:"created_at"`
	}
	if err := decode(e, &p); err != nil {
		return outbox.Permanent(err)
	}
	tenant, err := envTenant(e)
	if err != nil || p.TenantID != tenant {
		return outbox.Permanent(invalid(e.EventType, "tenant_id"))
	}
	if p.MessageID == uuid.Nil || p.ChannelID == uuid.Nil || p.SenderID == uuid.Nil ||
		!slices.Contains([]string{"hold_for_review", "block_and_alert"}, p.Decision) || len(p.Reasons) > 20 {
		return outbox.Permanent(invalid(e.EventType, "message_id/channel_id/sender_id/decision/reasons"))
	}
	for _, r := range p.Reasons {
		if !maxLen(&r, 64) {
			return outbox.Permanent(invalid(e.EventType, "reasons"))
		}
	}
	_, err = c.H.OpenCaseFromSignal(ctx, command.Signal{EventID: e.EventID, TenantID: tenant, MessageID: p.MessageID,
		ChannelID: p.ChannelID, ChannelType: p.ChannelType, SenderID: p.SenderID, Decision: p.Decision, Reasons: p.Reasons, At: p.CreatedAt})
	return classify("", e, false, err)
}
