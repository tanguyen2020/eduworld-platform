package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/notification"
	chatrealtime "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

// NotificationCommands are the inbox write use cases (command.Notifications, F123 US2).
type NotificationCommands interface {
	MarkOne(ctx context.Context, id uuid.UUID) (notification.Notification, error)
	MarkAll(ctx context.Context) (int, error)
}

// NotificationQueries is listNotifications (query.Notifications).
type NotificationQueries interface {
	List(ctx context.Context, unread bool, limit int, cursor string) (query.InboxPage, error)
}

// NotificationHandlers serve the inbox routes of F123 (API_DESIGN §9.11): any authenticated user of the session tenant,
// X-Tenant-Id required (F123-AC-11), only the caller's own notifications. Mark read is idempotent by state (no
// Idempotency-Key in the contract). There is no route to register a push token (FR-NTF-003).
type NotificationHandlers struct {
	Commands NotificationCommands
	Queries  NotificationQueries
}

// NotificationOperations are the F123 operations (checked against the spec by the contract test).
var NotificationOperations = []string{"listNotifications", "markNotificationRead", "markAllNotificationsRead"}

func init() {
	for _, o := range []httpx.Operation{
		{ID: "listNotifications", Method: http.MethodGet, Pattern: "/notifications", Tenant: httpx.TenantRequired, SuccessStatus: http.StatusOK},
		{ID: "markNotificationRead", Method: http.MethodPost, Pattern: "/notifications/{notification_id}:read", Tenant: httpx.TenantRequired,
			SuccessStatus: http.StatusOK},
		{ID: "markAllNotificationsRead", Method: http.MethodPost, Pattern: "/notifications:read-all", Tenant: httpx.TenantRequired,
			SuccessStatus: http.StatusOK},
	} {
		o.Module = "chat"
		Operations[o.ID] = o
	}
}

// registerNotifications mounts the F123 operations.
func registerNotifications(s *httpx.Server, h *NotificationHandlers) error {
	if h.Commands == nil || h.Queries == nil {
		return errors.New("chat: the notification handlers are required")
	}
	routes := map[string]http.HandlerFunc{
		"listNotifications":        h.list,
		"markNotificationRead":     h.markOne,
		"markAllNotificationsRead": h.markAll,
	}
	for _, id := range NotificationOperations {
		if err := s.Handle(Operations[id], routes[id]); err != nil {
			return err
		}
	}
	return nil
}

// notificationJSON renders the REST Notification (AsyncAPI payload + read).
func notificationJSON(n notification.Notification) map[string]any {
	m := chatrealtime.Payload(n)
	m["created_at"] = n.CreatedAt.UTC()
	m["read"] = n.Read()
	return m
}

func (h NotificationHandlers) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	n, _ := strconv.Atoi(q.Get("limit"))
	unread := q.Get("unread") == "true"
	p, err := h.Queries.List(r.Context(), unread, n, q.Get("cursor"))
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(p.Data))
	for i, it := range p.Data {
		data[i] = notificationJSON(it)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "page": p.Page, "unread_count": p.UnreadCount})
}

func (h NotificationHandlers) markOne(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "notification_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	n, err := h.Commands.MarkOne(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, notificationJSON(n))
}

func (h NotificationHandlers) markAll(w http.ResponseWriter, r *http.Request) {
	left, err := h.Commands.MarkAll(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"unread_count": left})
}
