package command

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/policy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ReceiptWindow is the coalescing window of receipts (CHAT_SYSTEM §8.2, chat_receipt_coalesce: ≤ 1 durable write per 2 s
// per member per channel).
const ReceiptWindow = 2 * time.Second

// MentionPush pushes chat.unread.updated {counts, mentions} to every connection of a user (F118-AC-25).
type MentionPush interface {
	UnreadMentions(ctx context.Context, user uuid.UUID, counts, mentions map[uuid.UUID]int64) error
}

// Receipts are chat.receipt.read (F117 US3, F117-AC-13) and chat.receipt.delivered (F118-AC-22): the cursors of a member
// only grow; the first receipt of a window is written at once, the following ones of the window are coalesced into one
// trailing write of the newest seq; no row per message × member. After a read write every device of the member gets
// chat.unread.updated (unread + mention count, F118-AC-25); after each write the members of a channel with receipts get
// chat.receipt.updated (Valkey Pub/Sub only, never between members in a block relation — F118-AC-24). A non-member gets
// NOT_FOUND (F115-AC-10).
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
	// F118: delivered cursors, the mention counter and the receipt fan-out (nil: F117 behaviour only).
	Cursors   write.CursorRepository
	Delivered port.CursorBuffer
	Mentions  MentionPush
	Fanout    port.FeatureFanout
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
	return h.receipt(ctx, h.Buffer, false, tenant, user, ch, seq)
}

// DeliveredReceipt handles one chat.receipt.delivered of (tenant, user) (F118-AC-22).
func (h Receipts) DeliveredReceipt(ctx context.Context, tenant, user, ch uuid.UUID, seq int64) (err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.MarkDelivered")
	defer func() { done(err) }()
	if h.Delivered == nil || h.Cursors == nil {
		return problem.New(problem.ChatTemporarilyUnavailable, "").Retry()
	}
	return h.receipt(ctx, h.Delivered, true, tenant, user, ch, seq)
}

func (h Receipts) receipt(ctx context.Context, buf port.CursorBuffer, delivered bool, tenant, user, ch uuid.UUID, seq int64) error {
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
	first, err := buf.Gate(ctx, ch, user, h.window())
	if err != nil {
		return problem.New(problem.ChatTemporarilyUnavailable, "").Retry()
	}
	if first {
		return h.flush(ctx, delivered, tenant, user, ch, seq)
	}
	schedule, err := buf.Hold(ctx, ch, user, seq, h.window())
	if err != nil {
		return problem.New(problem.ChatTemporarilyUnavailable, "").Retry()
	}
	if schedule {
		run := h.Schedule
		if run == nil {
			run = func(d time.Duration, f func()) { time.AfterFunc(d, f) }
		}
		bg := context.WithoutCancel(ctx)
		run(h.window(), func() { h.trailing(bg, buf, delivered, tenant, user, ch) })
	}
	return nil
}

// trailing writes the newest buffered seq of the window (best effort: a lost flush is recovered by the next receipt).
func (h Receipts) trailing(ctx context.Context, buf port.CursorBuffer, delivered bool, tenant, user, ch uuid.UUID) {
	seq, ok, err := buf.Take(ctx, ch, user)
	if err != nil || !ok {
		return
	}
	if err := h.flush(ctx, delivered, tenant, user, ch, seq); err != nil {
		slog.WarnContext(ctx, "chat: trailing receipt not written", "error", err)
	}
}

func (h Receipts) flush(ctx context.Context, delivered bool, tenant, user, ch uuid.UUID, seq int64) error {
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
	var unread, deliveredSeq, readSeq, mentions int64
	var member bool
	err := h.UoW.InTenantTx(ctx, mustScope(tenant), func(ctx context.Context) error {
		var err error
		if delivered {
			deliveredSeq, readSeq, member, err = h.Cursors.MarkDelivered(ctx, tenant, ch, user, seq, known, h.now())
			return err
		}
		unread, member, err = h.Repo.MarkRead(ctx, tenant, ch, user, seq, known, h.now())
		if err != nil || !member || h.Cursors == nil {
			return err
		}
		deliveredSeq, readSeq, mentions, err = h.Cursors.ResetMentions(ctx, tenant, ch, user)
		return err
	})
	if err != nil {
		return problem.New(problem.ChatTemporarilyUnavailable, "").Retry()
	}
	if !member {
		return problem.New(problem.NotFound, "")
	}
	bg := context.WithoutCancel(ctx)
	if !delivered {
		switch {
		case h.Mentions != nil && h.Cursors != nil:
			if err := h.Mentions.UnreadMentions(bg, user, map[uuid.UUID]int64{ch: unread}, map[uuid.UUID]int64{ch: mentions}); err != nil {
				slog.WarnContext(ctx, "chat: unread not pushed", "error", err)
			}
		case h.Push != nil:
			if err := h.Push.Unread(bg, user, map[uuid.UUID]int64{ch: unread}); err != nil {
				slog.WarnContext(ctx, "chat: unread not pushed", "error", err)
			}
		}
	}
	h.broadcast(bg, tenant, user, ch, deliveredSeq, readSeq)
	return nil
}

// broadcast sends chat.receipt.updated of user to the members of a channel with receipts, except members in a block
// relation with user (F118-AC-22, AC-24). Best effort (pub/sub).
func (h Receipts) broadcast(ctx context.Context, tenant, user, ch uuid.UUID, delivered, readSeq int64) {
	if h.Fanout == nil || h.Cursors == nil {
		return
	}
	facts, err := h.Facts.Channel(ctx, tenant, ch)
	if err != nil || facts == nil {
		return
	}
	m, ok := facts.Members.Active(user)
	if !ok {
		return
	}
	school := m.SchoolID
	if school == nil {
		school = facts.Channel.SchoolID
	}
	eff, err := h.Facts.Effective(ctx, tenant, school, facts.Channel)
	if err != nil {
		if !errors.Is(err, policy.ErrDenied) {
			slog.WarnContext(ctx, "chat: receipt policy unavailable", "error", err)
		}
		return
	}
	if eff.Features.ReceiptMode() == policy.ReceiptsNone {
		return
	}
	var to []uuid.UUID
	for _, id := range facts.Members.ActiveIDs() {
		if id != user && !facts.Blocked(id, user) {
			to = append(to, id)
		}
	}
	card, ok := facts.Cards[user]
	if !ok {
		card = read.ChatCard{UserID: user, Level: 1}
	}
	if err := h.Fanout.Receipt(ctx, ch, card, delivered, readSeq, to); err != nil {
		slog.WarnContext(ctx, "chat: receipt fan-out failed", "error", err)
	}
}
