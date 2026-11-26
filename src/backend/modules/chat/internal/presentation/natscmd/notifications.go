package natscmd

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/notification"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// TypeNotificationMarkRead is the WS command of the AsyncAPI (NotificationItemMarkRead, x-routing module chat).
const TypeNotificationMarkRead = "notification.item.mark_read"

// NotificationReader marks notifications read for the ticket user (command.Notifications.MarkRead).
type NotificationReader interface {
	MarkRead(ctx context.Context, tenant, user uuid.UUID, ids []uuid.UUID) (int, error)
}

// NotificationMarkRead handles ew.cmd.chat.notification.item.mark_read (F123-AC-09): ≤ 100 ids; ids of anyone else are
// ignored; the reply result is {read: <number read now>}.
func NotificationMarkRead(h NotificationReader) rt.CommandHandler {
	return func(ctx context.Context, c rt.Caller, payload json.RawMessage) rt.Reply {
		var p struct {
			NotificationIDs []uuid.UUID `json:"notification_ids"`
		}
		if err := json.Unmarshal(payload, &p); err != nil || len(p.NotificationIDs) == 0 || len(p.NotificationIDs) > notification.MaxMarkRead {
			return invalid()
		}
		n, err := h.MarkRead(ctx, c.TenantID, c.UserID, p.NotificationIDs)
		if err != nil {
			return failure(ctx, err)
		}
		return rt.OK(map[string]any{"read": n})
	}
}

// ServeNotifications serves the notification commands of the gateway until ctx ends (core-api).
func ServeNotifications(ctx context.Context, nc *nats.Conn, h NotificationReader) error {
	return rt.ServeCommand(ctx, nc, Module, TypeNotificationMarkRead, NotificationMarkRead(h), SendTimeout)
}
