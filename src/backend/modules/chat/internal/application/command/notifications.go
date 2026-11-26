package command

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/notification"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// MaxRecipientPages bounds the pages read from module learning for one assignment (500 students each).
const MaxRecipientPages = 20

// Notifications are the write use cases of the in-game notifications (F123): CreateNotifications from a domain event
// (consumers chat.notify_*), MarkNotificationsRead (REST one id, WS ≤ 100 ids) and MarkAllNotificationsRead. PostgreSQL
// first, then — after the commit only — the push of notification.item.created and the unread cache (R9, F123-AC-04).
// No audit per notification (volume, not an important operation: plan F123 §Bảo mật); metrics instead. Never a push
// outside the game (FR-NTF-003).
type Notifications struct {
	UoW         write.UnitOfWork
	AfterCommit write.AfterCommit
	Store       write.NotificationStore
	Push        port.NotificationPusher   // nil: stored only
	Unread      port.UnreadCounter        // nil: no cache
	Recipients  port.AssignmentRecipients // learning facade (assignment events)
	Metrics     port.NotificationMetrics  // nil: no metrics
	Now         func() time.Time
}

func (h Notifications) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

// FromEvent creates the notification of src for each recipient that the RecipientPolicy accepts (active student of the
// tenant of the event, F123-AC-06); actor (inviter, requester) gives the nickname param. It runs in the transaction of
// the consumer (processed_events in the same transaction) and returns the number of rows created now (0 when the event
// was already handled: unique key, F123-AC-03).
func (h Notifications) FromEvent(ctx context.Context, src notification.Source, recipients []uuid.UUID, actor *uuid.UUID) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.CreateNotifications")
	defer func() { done(err) }()
	return h.create(ctx, src, recipients, actor)
}

// FromAssignment creates assignment_new / assignment_results_released for the students module learning returns for the
// assignment (listAssignmentRecipients, every page; the excused students are not returned, F123-AC-02). A failure of
// learning is returned as is (the consumer retries, then dead-letters: no partial batch, F123-AC-07); an assignment
// learning does not know in the tenant notifies nobody.
func (h Notifications) FromAssignment(ctx context.Context, src notification.Source, assignment uuid.UUID) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.CreateNotifications")
	defer func() { done(err) }()
	if h.Recipients == nil {
		return 0, errors.New("chat: no assignment recipients port")
	}
	var all []uuid.UUID
	title, cursor, complete := "", "", false
	for range MaxRecipientPages {
		p, err := h.Recipients.Page(ctx, assignment, cursor)
		if errors.Is(err, port.ErrAssignmentNotFound) {
			slog.WarnContext(ctx, "assignment unknown to module learning: no notification", "event_type", src.EventType)
			return 0, nil
		}
		if err != nil {
			return 0, err
		}
		title = p.Title
		all = append(all, p.StudentIDs...)
		if p.Next == "" {
			complete = true
			break
		}
		cursor = p.Next
	}
	if !complete {
		// A school assignment targets at most one school (≤ 9 grades); more than MaxRecipientPages pages means a cursor
		// that does not advance: stop and keep the pages read (logged, ids only — R14).
		slog.ErrorContext(ctx, "assignment recipients exceed the page bound", "event_type", src.EventType, "pages", MaxRecipientPages)
	}
	src.Params = map[string]string{"title": title}
	return h.create(ctx, src, all, nil)
}

func (h Notifications) create(ctx context.Context, src notification.Source, recipients []uuid.UUID, actor *uuid.UUID) (int, error) {
	var created []notification.Notification
	err := h.UoW.Do(ctx, func(ctx context.Context) error {
		params := map[string]string{}
		maps.Copy(params, src.Params)
		if actor != nil {
			nick, err := h.Store.Nickname(ctx, src.TenantID, *actor)
			if err != nil {
				return err
			}
			if nick != "" {
				params["nickname"] = nick
			}
		}
		src.Params = params
		ids := make([]uuid.UUID, 0, len(recipients))
		for _, id := range recipients {
			if id != uuid.Nil && !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
		found, err := h.Store.Recipients(ctx, src.TenantID, ids)
		if err != nil {
			return err
		}
		drafts := make([]notification.Draft, 0, len(ids))
		for _, id := range ids {
			r, ok := found[id]
			if !ok || !notification.Eligible(&r, src.TenantID) {
				continue
			}
			school := r.SchoolID
			d, err := notification.Build(src, id, &school)
			if err != nil {
				return err
			}
			drafts = append(drafts, d)
		}
		if len(drafts) == 0 {
			return nil
		}
		if created, err = h.Store.Insert(ctx, drafts); err != nil {
			return err
		}
		if len(created) == 0 || h.AfterCommit == nil {
			return nil
		}
		list, occurred := created, src.OccurredAt
		return h.AfterCommit(ctx, func(ctx context.Context) { h.deliver(ctx, list, occurred) })
	})
	if err != nil {
		return 0, err
	}
	return len(created), nil
}

// deliver pushes notification.item.created and moves the unread cache after the commit (best effort: the row is stored;
// a lost push is recovered by the client from listNotifications after a resync).
func (h Notifications) deliver(ctx context.Context, list []notification.Notification, occurred time.Time) {
	for _, n := range list {
		if h.Push != nil {
			if err := h.Push.Created(ctx, n); err != nil {
				slog.WarnContext(ctx, "notification push failed", "kind", string(n.Kind), "error", err)
			}
		}
		if h.Unread != nil {
			if err := h.Unread.Add(ctx, n.RecipientID, 1); err != nil {
				slog.WarnContext(ctx, "unread cache update failed", "error", err)
			}
		}
	}
	if h.Metrics != nil && len(list) > 0 {
		h.Metrics.Created(string(list[0].Kind), len(list))
		h.Metrics.Delivered(h.now().Sub(occurred))
	}
}

// owner is the caller of the inbox routes: any authenticated user of the session tenant (the routes only ever touch
// the rows whose recipient_id is the caller: ownership is the authorization, F123-AC-08/09).
func owner(ctx context.Context) (uuid.UUID, uuid.UUID, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return uuid.Nil, uuid.Nil, problem.New(problem.Unauthenticated, "")
	}
	if p.TenantID == nil || *p.TenantID == uuid.Nil {
		return uuid.Nil, uuid.Nil, problem.New(problem.TenantHeaderRequired, "")
	}
	return *p.TenantID, p.UserID, nil
}

// afterRead moves the unread cache and the metric once the transaction committed.
func (h Notifications) afterRead(ctx context.Context, user uuid.UUID, changed int) error {
	if changed == 0 || h.AfterCommit == nil {
		return nil
	}
	return h.AfterCommit(ctx, func(ctx context.Context) {
		if h.Unread != nil {
			if err := h.Unread.Add(ctx, user, -changed); err != nil {
				slog.WarnContext(ctx, "unread cache update failed", "error", err)
			}
		}
		if h.Metrics != nil {
			h.Metrics.Read(changed)
		}
	})
}

// MarkOne implements markNotificationRead (F123-AC-09): read_at set once; a notification of anyone else (or of another
// tenant) is 404 NOT_FOUND.
func (h Notifications) MarkOne(ctx context.Context, id uuid.UUID) (out notification.Notification, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.MarkNotificationRead")
	defer func() { done(err) }()
	tenant, me, err := owner(ctx)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		rows, changed, err := h.Store.MarkRead(ctx, tenant, me, []uuid.UUID{id}, h.now())
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return problem.New(problem.NotFound, "")
		}
		out = rows[0]
		return h.afterRead(ctx, me, changed)
	})
	return out, err
}

// MarkAll implements markAllNotificationsRead (F123-AC-10): every unread notification of the caller created up to now;
// returns the unread count left (0 unless one arrived meanwhile).
func (h Notifications) MarkAll(ctx context.Context) (left int, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.MarkAllNotificationsRead")
	defer func() { done(err) }()
	tenant, me, err := owner(ctx)
	if err != nil {
		return 0, err
	}
	now := h.now()
	var changed int
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if changed, err = h.Store.MarkAllRead(ctx, tenant, me, now, now); err != nil {
			return err
		}
		if left, err = h.Store.UnreadCount(ctx, tenant, me); err != nil {
			return err
		}
		if h.AfterCommit == nil {
			return nil
		}
		n := left
		return h.AfterCommit(ctx, func(ctx context.Context) {
			if h.Unread != nil {
				if err := h.Unread.Set(ctx, me, n); err != nil {
					slog.WarnContext(ctx, "unread cache update failed", "error", err)
				}
			}
			if h.Metrics != nil && changed > 0 {
				h.Metrics.Read(changed)
			}
		})
	})
	return left, err
}

// MarkRead implements the WS command notification.item.mark_read (F123-AC-09): the tenant and user are those of the
// ticket (gateway headers); ≤ 100 ids; ids of anyone else are ignored. Returns the number read now.
func (h Notifications) MarkRead(ctx context.Context, tenant, user uuid.UUID, ids []uuid.UUID) (changed int, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.MarkNotificationsRead")
	defer func() { done(err) }()
	list, err := notification.MarkReadIDs(ids)
	if err != nil {
		return 0, problem.New(problem.ValidationFailed, err.Error())
	}
	if len(list) == 0 {
		return 0, nil
	}
	sc, err := authz.EventScope(tenant)
	if err != nil {
		return 0, problem.New(problem.TenantHeaderRequired, "")
	}
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		var err error
		if _, changed, err = h.Store.MarkRead(ctx, tenant, user, list, h.now()); err != nil {
			return err
		}
		return h.afterRead(ctx, user, changed)
	})
	return changed, err
}
