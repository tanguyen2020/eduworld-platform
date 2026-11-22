package command

import (
	"context"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// Outbox events of the applied message changes (no content, EVENT_DESIGN §4.9).
const (
	EventMessageEdited   = "chat.message_edited"
	EventMessageRecalled = "chat.message_recalled"
	EventMessagePinned   = "chat.message_pinned"
	EventMessageUnpinned = "chat.message_unpinned"
)

// ApplyChange is the use case of chat-persister for one message change (F118, research R1): applied by revision in one
// tenant transaction with its outbox event; a held edit opens a case like a held message (chat.message_held).
type ApplyChange struct {
	UoW    write.UnitOfWork
	Store  write.ChangeStore
	Events write.EventPublisher
}

// Apply writes c and returns the outcome.
func (h ApplyChange) Apply(ctx context.Context, c message.Change) (out write.ChangeOutcome, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.ApplyChange")
	defer func() { done(err) }()
	err = h.UoW.InTenantTx(ctx, mustScope(c.TenantID), func(ctx context.Context) error {
		out, err = h.Store.Apply(ctx, c)
		if err != nil || out != write.ChangeApplied {
			return err
		}
		t := c.TenantID
		base := map[string]any{"tenant_id": c.TenantID, "message_id": c.MessageID, "channel_id": c.ChannelID, "channel_type": c.ChannelType,
			"actor_id": c.ActorID, "occurred_at": c.At.UTC()}
		typ := ""
		switch c.Op {
		case message.OpEdit:
			typ = EventMessageEdited
			base["revision"] = c.Revision
		case message.OpRecall:
			typ = EventMessageRecalled
			base["revision"] = c.Revision
		case message.OpPin:
			typ = EventMessagePinned
		case message.OpUnpin:
			typ = EventMessageUnpinned
		case message.OpEditHeld:
			reasons := []string{}
			if c.Moderation != nil && c.Moderation.CategoryCodes != nil {
				reasons = c.Moderation.CategoryCodes
			}
			var catalog *int64
			if c.Moderation != nil {
				catalog = c.Moderation.CatalogVersion
			}
			return h.Events.Publish(ctx, write.Event{Type: EventMessageHeld, SubjectType: "chat_message", SubjectID: c.MessageID.String(),
				PartitionKey: c.ChannelID.String(), TenantID: &t, OccurredAt: c.At, Payload: map[string]any{
					"tenant_id": c.TenantID, "message_id": c.MessageID, "channel_id": c.ChannelID, "channel_type": c.ChannelType,
					"sender_id": c.ActorID, "decision": string(c.Moderation.Decision), "reasons": reasons, "catalog_version": catalog,
					"created_at": c.At.UTC()}})
		default:
			return nil
		}
		return h.Events.Publish(ctx, write.Event{Type: typ, SubjectType: "chat_message", SubjectID: c.MessageID.String(),
			PartitionKey: c.ChannelID.String(), TenantID: &t, OccurredAt: c.At, Payload: base})
	})
	return out, err
}
