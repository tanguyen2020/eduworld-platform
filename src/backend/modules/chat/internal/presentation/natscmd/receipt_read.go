package natscmd

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"

	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// TypeReceiptRead is chat.receipt.read (AsyncAPI ChatReceiptRead, routed to module chat by the gateway).
const TypeReceiptRead = "chat.receipt.read"

// ReceiptReader is the read-cursor use case (command.Receipts).
type ReceiptReader interface {
	Read(ctx context.Context, tenant, user, channel uuid.UUID, seq int64) error
}

// ReceiptRead handles ew.cmd.chat.receipt.read (F117-AC-13): the cursor of the caller only grows; every device of the
// caller then gets chat.unread.updated. A channel the caller is not an active member of is NOT_FOUND.
func ReceiptRead(h ReceiptReader) rt.CommandHandler {
	return func(ctx context.Context, c rt.Caller, payload json.RawMessage) rt.Reply {
		var p struct {
			ChannelID uuid.UUID `json:"channel_id"`
			Seq       *int64    `json:"seq"`
		}
		if err := json.Unmarshal(payload, &p); err != nil || p.ChannelID == uuid.Nil || p.Seq == nil || *p.Seq < 0 {
			return invalid()
		}
		if err := h.Read(ctx, c.TenantID, c.UserID, p.ChannelID, *p.Seq); err != nil {
			return failure(ctx, err)
		}
		return rt.OK(map[string]any{"channel_id": p.ChannelID, "seq": *p.Seq})
	}
}

// ServeReceipts answers ew.cmd.chat.receipt.read until ctx ends (queue group chat).
func ServeReceipts(ctx context.Context, nc *nats.Conn, h ReceiptReader) error {
	return rt.ServeCommand(ctx, nc, Module, TypeReceiptRead, ReceiptRead(h), SendTimeout)
}
