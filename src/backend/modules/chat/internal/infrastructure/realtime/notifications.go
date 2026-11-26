package realtime

import (
	"context"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/notification"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// TypeNotificationCreated is the server message of the AsyncAPI (NotificationItemCreated).
const TypeNotificationCreated = "notification.item.created"

// Notifications implements port.NotificationPusher on ew:rt:user:<recipient_id> (Valkey Pub/Sub; the gateway delivers it
// to every connection of the student and keeps it in the resume buffer: not droppable, F123-AC-04). The payload carries
// the kind, message code, params (nickname, content title, id — F123-AC-05), reference and deep link only.
type Notifications struct {
	P   rt.Publisher
	Now func() time.Time
}

var _ port.NotificationPusher = Notifications{}

// Payload renders AsyncAPI notification.item.created (also the REST Notification without read).
func Payload(n notification.Notification) map[string]any {
	p := map[string]any{"notification_id": n.ID, "kind": string(n.Kind), "message_code": n.MessageCode,
		"created_at": n.CreatedAt.UnixMilli()}
	if len(n.Params) > 0 {
		p["params"] = n.Params
	}
	if n.RefType != "" {
		p["ref_type"] = n.RefType
	}
	if n.RefID != nil {
		p["ref_id"] = *n.RefID
	}
	if n.Action != nil {
		p["action"] = n.Action
	}
	return p
}

// Created implements port.NotificationPusher.
func (p Notifications) Created(ctx context.Context, n notification.Notification) error {
	now := time.Now()
	if p.Now != nil {
		now = p.Now()
	}
	e, err := rt.NewMessage(TypeNotificationCreated, Payload(n), now)
	if err != nil {
		return err
	}
	return p.P.ToUser(ctx, n.RecipientID, e)
}
