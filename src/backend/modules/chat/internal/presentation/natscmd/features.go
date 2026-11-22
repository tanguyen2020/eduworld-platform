package natscmd

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// Command types of the message features (AsyncAPI, routed to module chat by the gateway, F118).
const (
	TypeMessageEdit      = "chat.message.edit"
	TypeMessageRecall    = "chat.message.recall"
	TypeMessageForward   = "chat.message.forward"
	TypeReactionToggle   = "chat.reaction.toggle"
	TypePinAdd           = "chat.pin.add"
	TypePinRemove        = "chat.pin.remove"
	TypeTypingStart      = "chat.typing.start"
	TypeTypingStop       = "chat.typing.stop"
	TypeReceiptDelivered = "chat.receipt.delivered"
)

// Messaging is the use case set of the message features (command.Messaging).
type Messaging interface {
	Edit(ctx context.Context, s command.Sender, id uuid.UUID, body string, mentions []uuid.UUID, expected int) (command.EditResult, error)
	Recall(ctx context.Context, s command.Sender, id uuid.UUID) error
	Forward(ctx context.Context, s command.Sender, id uuid.UUID, targets []uuid.UUID, key string) ([]command.SendResult, error)
	ToggleReaction(ctx context.Context, s command.Sender, id uuid.UUID, emoji string) (port.ReactionResult, error)
	Pin(ctx context.Context, s command.Sender, ch, id uuid.UUID) error
	Unpin(ctx context.Context, s command.Sender, ch, id uuid.UUID) error
	TypingChanged(ctx context.Context, s command.Sender, ch uuid.UUID, start bool) error
}

// DeliveredReceipts is the delivered-cursor use case (command.Receipts).
type DeliveredReceipts interface {
	DeliveredReceipt(ctx context.Context, tenant, user, channel uuid.UUID, seq int64) error
}

func senderOf(c rt.Caller) command.Sender {
	return command.Sender{TenantID: c.TenantID, UserID: c.UserID, ConnID: c.ConnID}
}

// MessageEdit handles ew.cmd.chat.message.edit (F118-AC-01 … AC-03).
func MessageEdit(h Messaging) rt.CommandHandler {
	return func(ctx context.Context, c rt.Caller, payload json.RawMessage) rt.Reply {
		var p struct {
			MessageID uuid.UUID   `json:"message_id"`
			Body      string      `json:"body"`
			Revision  int         `json:"revision_expected"`
			Mentions  []uuid.UUID `json:"mentions"`
		}
		if err := json.Unmarshal(payload, &p); err != nil || p.MessageID == uuid.Nil || p.Revision < 1 {
			return invalid()
		}
		res, err := h.Edit(ctx, senderOf(c), p.MessageID, p.Body, p.Mentions, p.Revision)
		if err != nil {
			return failure(ctx, err)
		}
		state := string(res.Envelope.Moderation.State)
		return rt.OK(map[string]any{"message_id": p.MessageID, "channel_id": res.Envelope.ChannelID, "revision": res.Envelope.Revision,
			"moderation_state": state, "held": res.Held})
	}
}

// MessageRecall handles ew.cmd.chat.message.recall (F118-AC-04).
func MessageRecall(h Messaging) rt.CommandHandler {
	return func(ctx context.Context, c rt.Caller, payload json.RawMessage) rt.Reply {
		var p struct {
			MessageID uuid.UUID `json:"message_id"`
		}
		if err := json.Unmarshal(payload, &p); err != nil || p.MessageID == uuid.Nil {
			return invalid()
		}
		if err := h.Recall(ctx, senderOf(c), p.MessageID); err != nil {
			return failure(ctx, err)
		}
		return rt.OK(map[string]any{"message_id": p.MessageID})
	}
}

// MessageForward handles ew.cmd.chat.message.forward (F118-AC-09 … AC-11): a retry of the same forward republishes the
// same messages (client_message_id derived from source and target).
func MessageForward(h Messaging) rt.CommandHandler {
	return func(ctx context.Context, c rt.Caller, payload json.RawMessage) rt.Reply {
		var p struct {
			MessageID uuid.UUID   `json:"message_id"`
			Targets   []uuid.UUID `json:"target_channel_ids"`
		}
		if err := json.Unmarshal(payload, &p); err != nil || p.MessageID == uuid.Nil || len(p.Targets) == 0 {
			return invalid()
		}
		res, err := h.Forward(ctx, senderOf(c), p.MessageID, p.Targets, "ws")
		if err != nil {
			return failure(ctx, err)
		}
		out := make([]map[string]any, len(res))
		for i, r := range res {
			out[i] = ackedResult(r.Envelope)
		}
		return rt.OK(map[string]any{"message_id": p.MessageID, "messages": out})
	}
}

// ReactionToggle handles ew.cmd.chat.reaction.toggle (F118-AC-16).
func ReactionToggle(h Messaging) rt.CommandHandler {
	return func(ctx context.Context, c rt.Caller, payload json.RawMessage) rt.Reply {
		var p struct {
			MessageID uuid.UUID `json:"message_id"`
			Reaction  string    `json:"reaction"`
		}
		if err := json.Unmarshal(payload, &p); err != nil || p.MessageID == uuid.Nil || p.Reaction == "" || len(p.Reaction) > 64 {
			return invalid()
		}
		res, err := h.ToggleReaction(ctx, senderOf(c), p.MessageID, p.Reaction)
		if err != nil {
			return failure(ctx, err)
		}
		return rt.OK(map[string]any{"message_id": p.MessageID, "active": res.Set, "reactions": res.Counts})
	}
}

// Pin handles ew.cmd.chat.pin.add / remove (F118-AC-20).
func Pin(h Messaging, add bool) rt.CommandHandler {
	return func(ctx context.Context, c rt.Caller, payload json.RawMessage) rt.Reply {
		var p struct {
			ChannelID uuid.UUID `json:"channel_id"`
			MessageID uuid.UUID `json:"message_id"`
		}
		if err := json.Unmarshal(payload, &p); err != nil || p.ChannelID == uuid.Nil || p.MessageID == uuid.Nil {
			return invalid()
		}
		var err error
		if add {
			err = h.Pin(ctx, senderOf(c), p.ChannelID, p.MessageID)
		} else {
			err = h.Unpin(ctx, senderOf(c), p.ChannelID, p.MessageID)
		}
		if err != nil {
			return failure(ctx, err)
		}
		return rt.OK(map[string]any{"channel_id": p.ChannelID, "message_id": p.MessageID})
	}
}

// Typing handles ew.cmd.chat.typing.start / stop (F118-AC-21).
func Typing(h Messaging, start bool) rt.CommandHandler {
	return func(ctx context.Context, c rt.Caller, payload json.RawMessage) rt.Reply {
		var p struct {
			ChannelID uuid.UUID `json:"channel_id"`
		}
		if err := json.Unmarshal(payload, &p); err != nil || p.ChannelID == uuid.Nil {
			return invalid()
		}
		if err := h.TypingChanged(ctx, senderOf(c), p.ChannelID, start); err != nil {
			return failure(ctx, err)
		}
		return rt.OK(map[string]any{"channel_id": p.ChannelID})
	}
}

// ReceiptDelivered handles ew.cmd.chat.receipt.delivered (F118-AC-22).
func ReceiptDelivered(h DeliveredReceipts) rt.CommandHandler {
	return func(ctx context.Context, c rt.Caller, payload json.RawMessage) rt.Reply {
		var p struct {
			ChannelID uuid.UUID `json:"channel_id"`
			Seq       *int64    `json:"seq"`
		}
		if err := json.Unmarshal(payload, &p); err != nil || p.ChannelID == uuid.Nil || p.Seq == nil || *p.Seq < 0 {
			return invalid()
		}
		if err := h.DeliveredReceipt(ctx, c.TenantID, c.UserID, p.ChannelID, *p.Seq); err != nil {
			return failure(ctx, err)
		}
		return rt.OK(map[string]any{"channel_id": p.ChannelID, "seq": *p.Seq})
	}
}

// ServeFeatures answers the F118 commands until ctx ends (queue group chat).
func ServeFeatures(ctx context.Context, nc *nats.Conn, h Messaging, receipts DeliveredReceipts) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	handlers := map[string]rt.CommandHandler{TypeMessageEdit: MessageEdit(h), TypeMessageRecall: MessageRecall(h),
		TypeMessageForward: MessageForward(h), TypeReactionToggle: ReactionToggle(h), TypePinAdd: Pin(h, true), TypePinRemove: Pin(h, false),
		TypeTypingStart: Typing(h, true), TypeTypingStop: Typing(h, false), TypeReceiptDelivered: ReceiptDelivered(receipts)}
	errs := make(chan error, len(handlers))
	for typ, fn := range handlers {
		go func() { errs <- rt.ServeCommand(ctx, nc, Module, typ, fn, SendTimeout) }()
	}
	var first error
	for range handlers {
		if err := <-errs; err != nil && first == nil {
			first = err
			cancel()
		}
	}
	return first
}
