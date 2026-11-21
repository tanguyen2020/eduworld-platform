package command

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// Outbox events of a persisted message (no content, EVENT_DESIGN §4.9).
const (
	EventMessageCreated = "chat.message_created"
	// EventMessageHeld: a held message (hold_for_review / block_and_alert) was stored (F116-AC-06); consumed by F121.
	EventMessageHeld = "chat.message_held"
)

// Persist is the use case of chat-persister (F115 US3, R25): one batch of envelopes of one tenant is written in one
// transaction — rows idempotent per message_id, channels.last_seq raised with GREATEST, gameplay partitions recorded —
// with one chat.message_created per newly inserted row in the outbox of the same transaction. The worker acks the
// stream only after this commit.
type Persist struct {
	UoW    write.UnitOfWork
	Store  write.MessageStore
	Events write.EventPublisher
}

// Persist writes es (all of tenant) and returns the result of the batch.
func (h Persist) Persist(ctx context.Context, tenant uuid.UUID, es []message.Envelope) (res write.PersistResult, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.Persist")
	defer func() { done(err) }()
	err = h.UoW.InTenantTx(ctx, mustScope(tenant), func(ctx context.Context) error {
		res, err = h.Store.Insert(ctx, tenant, es)
		if err != nil {
			return err
		}
		for _, e := range res.Inserted {
			t := e.TenantID
			if err := h.Events.Publish(ctx, write.Event{Type: EventMessageCreated, SubjectType: "chat_message", SubjectID: e.MessageID.String(),
				PartitionKey: e.ChannelID.String(), TenantID: &t, OccurredAt: e.CreatedAt, Payload: map[string]any{
					"tenant_id": e.TenantID, "message_id": e.MessageID, "channel_id": e.ChannelID, "channel_type": e.ChannelType,
					"seq": e.Seq, "sender_id": e.SenderID, "kind": string(e.MsgKind), "moderation_state": string(e.Moderation.State),
					"created_at": e.CreatedAt.UTC()}}); err != nil {
				return err
			}
			if e.Moderation.State == message.StateHeld {
				reasons := e.Moderation.CategoryCodes
				if reasons == nil {
					reasons = []string{}
				}
				if err := h.Events.Publish(ctx, write.Event{Type: EventMessageHeld, SubjectType: "chat_message", SubjectID: e.MessageID.String(),
					PartitionKey: e.ChannelID.String(), TenantID: &t, OccurredAt: e.CreatedAt, Payload: map[string]any{
						"tenant_id": e.TenantID, "message_id": e.MessageID, "channel_id": e.ChannelID, "channel_type": e.ChannelType,
						"sender_id": e.SenderID, "decision": string(e.Moderation.Decision), "reasons": reasons,
						"catalog_version": e.Moderation.CatalogVersion, "created_at": e.CreatedAt.UTC()}}); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return write.PersistResult{}, err
	}
	return res, nil
}
