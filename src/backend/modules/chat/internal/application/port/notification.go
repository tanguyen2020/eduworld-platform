package port

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/notification"
)

// NotificationPusher sends notification.item.created to the recipient (ew:rt:user:<id>, not droppable: the gateway keeps
// it in the resume buffer, F123-AC-04). Called after the commit only (R9).
type NotificationPusher interface {
	Created(ctx context.Context, n notification.Notification) error
}

// UnreadCounter caches the unread count of a user (Valkey ew:chat:ntf:unread:<user_id>, VALKEY_DESIGN §3.1): never the
// source of truth (R9) — a miss or an error is recounted from PostgreSQL.
type UnreadCounter interface {
	// Get returns the cached count and whether it was cached.
	Get(ctx context.Context, user uuid.UUID) (int, bool, error)
	// Fill caches a count read from PostgreSQL unless a value is cached already.
	Fill(ctx context.Context, user uuid.UUID, n int) error
	// Add moves a cached count by delta (no-op when nothing is cached; a negative result drops the key).
	Add(ctx context.Context, user uuid.UUID, delta int) error
	// Set caches an exact count (after markAllNotificationsRead).
	Set(ctx context.Context, user uuid.UUID, n int) error
}

// ErrAssignmentNotFound: module learning knows no such assignment in the tenant (permanent for the consumer).
var ErrAssignmentNotFound = errors.New("notification: assignment not found in module learning")

// AssignmentRecipientPage is one page of listAssignmentRecipients (facade of module learning).
type AssignmentRecipientPage struct {
	Title      string
	StudentIDs []uuid.UUID
	Next       string // "" on the last page
}

// AssignmentRecipients reads the students receiving an assignment from module learning (facade listAssignmentRecipients,
// R29: never the database of learning). The tenant is the TenantScope of ctx. Any failure other than
// ErrAssignmentNotFound is retryable (the consumer does not ack, F123-AC-07).
type AssignmentRecipients interface {
	Page(ctx context.Context, assignment uuid.UUID, cursor string) (AssignmentRecipientPage, error)
}

// NotificationMetrics are eduworld_notifications_* (labels never carry ids).
type NotificationMetrics interface {
	Created(kind string, n int)
	Read(n int)
	Delivered(latency time.Duration)
}
