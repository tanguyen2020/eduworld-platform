package write

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/notification"
)

// NotificationStore persists social.notifications (F123) in the transaction of ctx under its TenantScope, with explicit
// tenant_id and deleted_at conditions (R27); it never deletes a row (R21).
type NotificationStore interface {
	// Recipients returns the projected students among ids (ref.students of tenant; absent = unknown in that tenant).
	Recipients(ctx context.Context, tenant uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]notification.Recipient, error)
	// Nickname returns the public nickname of a student of tenant ("" when not projected yet) — the only personal data a
	// notification may carry (R16).
	Nickname(ctx context.Context, tenant, student uuid.UUID) (string, error)
	// Insert stores the drafts; a draft whose (recipient_id, source_event_id, kind) exists already is skipped (ON
	// CONFLICT DO NOTHING). It returns the rows inserted now.
	Insert(ctx context.Context, drafts []notification.Draft) ([]notification.Notification, error)
	// MarkRead sets read_at = at on the unread rows among ids that belong to recipient in tenant; it returns every row
	// among ids of that recipient (read now or before) and the number read now. Ids of anyone else are ignored.
	MarkRead(ctx context.Context, tenant, recipient uuid.UUID, ids []uuid.UUID, at time.Time) ([]notification.Notification, int, error)
	// MarkAllRead sets read_at = at on every unread row of recipient created up to before; returns the number read now.
	MarkAllRead(ctx context.Context, tenant, recipient uuid.UUID, before, at time.Time) (int, error)
	// UnreadCount counts the unread rows of recipient.
	UnreadCount(ctx context.Context, tenant, recipient uuid.UUID) (int, error)
}
