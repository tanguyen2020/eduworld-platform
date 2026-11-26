package read

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/notification"
)

// NotificationReader reads the inbox of one user (social.notifications) in the read transaction of ctx, with explicit
// tenant_id, recipient_id and deleted_at conditions (R27).
type NotificationReader interface {
	// Inbox returns up to limit rows of recipient, newest first (created_at, notification_id), after the keyset position;
	// unread keeps the unread ones only.
	Inbox(ctx context.Context, tenant, recipient uuid.UUID, unread bool, after *After, limit int) ([]notification.Notification, error)
	// UnreadCount counts the unread rows of recipient.
	UnreadCount(ctx context.Context, tenant, recipient uuid.UUID) (int, error)
}
