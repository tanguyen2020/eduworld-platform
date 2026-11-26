package query

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/notification"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Notifications reads the inbox of the caller (listNotifications, F123-AC-08): only the rows whose recipient_id is the
// caller in the session tenant, newest first, with the unread count (Valkey cache, recounted from PostgreSQL on a miss —
// R9). No write to the database, no event (R26).
type Notifications struct {
	Read    ReadSession
	Reader  read.NotificationReader
	Unread  port.UnreadCounter // nil: always counted
	Cursors *page.Codec
}

// InboxPage is one page of the inbox.
type InboxPage struct {
	Data        []notification.Notification
	Page        page.Info
	UnreadCount int
}

// List implements listNotifications: any authenticated user (staff get an empty inbox: F123 only notifies students).
func (q Notifications) List(ctx context.Context, unread bool, limit int, cursor string) (out InboxPage, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListNotifications")
	defer func() { done(err) }()
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return out, problem.New(problem.Unauthenticated, "")
	}
	if p.TenantID == nil || *p.TenantID == uuid.Nil {
		return out, problem.New(problem.TenantHeaderRequired, "")
	}
	tenant, me := *p.TenantID, p.UserID
	limit = page.Limit(limit)
	cur, err := q.Cursors.Decode(cursor)
	if err != nil {
		return out, problem.New(problem.BadRequest, "invalid cursor")
	}
	var after *read.After
	if cur != nil {
		at, err := time.Parse(time.RFC3339Nano, cur.Key)
		if err != nil {
			return out, problem.New(problem.BadRequest, "invalid cursor")
		}
		after = &read.After{At: at, ID: cur.ID}
	}
	count, cached := 0, false
	if q.Unread != nil {
		if count, cached, err = q.Unread.Get(ctx, me); err != nil {
			slog.WarnContext(ctx, "unread cache read failed", "error", err)
			cached = false
		}
	}
	var rows []notification.Notification
	if err := q.Read.Read(ctx, func(ctx context.Context) error {
		var err error
		if rows, err = q.Reader.Inbox(ctx, tenant, me, unread, after, limit+1); err != nil {
			return err
		}
		if !cached {
			count, err = q.Reader.UnreadCount(ctx, tenant, me)
		}
		return err
	}); err != nil {
		return out, err
	}
	if !cached && q.Unread != nil {
		if err := q.Unread.Fill(ctx, me, count); err != nil {
			slog.WarnContext(ctx, "unread cache fill failed", "error", err)
		}
	}
	out.Page = page.Info{Limit: limit}
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		next := q.Cursors.Encode(page.Cursor{Key: last.CreatedAt.UTC().Format(time.RFC3339Nano), ID: last.ID})
		out.Page.NextCursor, out.Page.HasMore = &next, true
	}
	out.Data, out.UnreadCount = rows, count
	return out, nil
}
