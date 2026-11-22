package command

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ReceiptWindow is the coalescing window of read receipts (CHAT_SYSTEM §8.2: ≤ 1 durable write per 2 s per member).
const ReceiptWindow = 2 * time.Second

// Receipts is chat.receipt.read (F117 US3, F117-AC-13): the read cursor of a member only grows; the first receipt of a
// window is written at once, the following ones of the window are coalesced into one trailing write of the newest seq.
// After each write every device of the member gets chat.unread.updated. A non-member gets NOT_FOUND (F115-AC-10).
type Receipts struct {
	UoW      write.UnitOfWork
	Facts    service.Facts // membership of the reader (cached facts of the send path)
	Repo     write.GameplayRepository
	Buffer   port.CursorBuffer
	Recent   port.RecentBuffer
	Push     port.UnreadPush
	Window   time.Duration                   // default ReceiptWindow
	Schedule func(d time.Duration, f func()) // trailing flush timer (default time.AfterFunc)
	Now      func() time.Time
}

func (h Receipts) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func (h Receipts) window() time.Duration {
	if h.Window > 0 {
		return h.Window
	}
	return ReceiptWindow
}

// Read handles one chat.receipt.read of (tenant, user).
func (h Receipts) Read(ctx context.Context, tenant, user, ch uuid.UUID, seq int64) (err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.MarkRead")
	defer func() { done(err) }()
	if tenant == uuid.Nil || user == uuid.Nil {
		return problem.New(problem.Unauthenticated, "")
	}
	if ch == uuid.Nil || seq < 0 {
		return problem.Invalid("seq", problem.SubInvalid, "channel_id and seq ≥ 0")
	}
	facts, err := h.Facts.Channel(ctx, tenant, ch)
	if err != nil {
		return problem.New(problem.ChatTemporarilyUnavailable, "").Retry()
	}
	if facts == nil {
		return problem.New(problem.NotFound, "")
	}
	if _, ok := facts.Members.Active(user); !ok {
		return problem.New(problem.NotFound, "")
	}
	first, err := h.Buffer.Gate(ctx, ch, user, h.window())
	if err != nil {
		return problem.New(problem.ChatTemporarilyUnavailable, "").Retry()
	}
	if first {
		return h.flush(ctx, tenant, user, ch, seq)
	}
	schedule, err := h.Buffer.Hold(ctx, ch, user, seq, h.window())
	if err != nil {
		return problem.New(problem.ChatTemporarilyUnavailable, "").Retry()
	}
	if schedule {
		run := h.Schedule
		if run == nil {
			run = func(d time.Duration, f func()) { time.AfterFunc(d, f) }
		}
		bg := context.WithoutCancel(ctx)
		run(h.window(), func() { h.trailing(bg, tenant, user, ch) })
	}
	return nil
}

// trailing writes the newest buffered seq of the window (best effort: a lost flush is recovered by the next receipt).
func (h Receipts) trailing(ctx context.Context, tenant, user, ch uuid.UUID) {
	seq, ok, err := h.Buffer.Take(ctx, ch, user)
	if err != nil || !ok {
		return
	}
	if err := h.flush(ctx, tenant, user, ch, seq); err != nil {
		slog.WarnContext(ctx, "chat: trailing read receipt not written", "error", err)
	}
}

func (h Receipts) flush(ctx context.Context, tenant, user, ch uuid.UUID, seq int64) error {
	var known int64
	if h.Recent != nil {
		if recent, err := h.Recent.List(ctx, ch); err == nil {
			for _, e := range recent {
				if e.TenantID == tenant {
					known = max(known, e.Seq)
				}
			}
		}
	}
	var unread int64
	var member bool
	err := h.UoW.InTenantTx(ctx, mustScope(tenant), func(ctx context.Context) error {
		var err error
		unread, member, err = h.Repo.MarkRead(ctx, tenant, ch, user, seq, known, h.now())
		return err
	})
	if err != nil {
		return problem.New(problem.ChatTemporarilyUnavailable, "").Retry()
	}
	if !member {
		return problem.New(problem.NotFound, "")
	}
	if h.Push != nil {
		if err := h.Push.Unread(context.WithoutCancel(ctx), user, map[uuid.UUID]int64{ch: unread}); err != nil {
			slog.WarnContext(ctx, "chat: unread not pushed", "error", err)
		}
	}
	return nil
}
