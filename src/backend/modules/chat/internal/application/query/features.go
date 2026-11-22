package query

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/policy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// PerMemberMax is the largest channel whose receipts are listed member by member (CHAT_SYSTEM §8.1: ≤ 8).
const PerMemberMax = 8

// Gif page limits (REST Limit 1..100, default 20).
const (
	DefaultGifLimit = 20
	MaxGifLimit     = 100
)

// Features are the read use cases of the message features (F118): pins, receipts, the sticker packs and GIFs of the
// internal library, and the decoration of history pages (reply excerpts, reactions, pins). Reads only; no write, no event.
type Features struct {
	Read   service.ReadRunner
	Reader read.FeatureReader
	Facts  service.Facts
	Lookup service.Lookup
	Recent port.RecentBuffer
	Now    func() time.Time
}

func (q Features) scope(tenant uuid.UUID, fn func(ctx context.Context) error, ctx context.Context) error {
	s, err := authz.EventScope(tenant)
	if err != nil {
		return err
	}
	return q.Read.InTenantRead(ctx, s, fn)
}

func (q Features) member(ctx context.Context, v Viewer, ch uuid.UUID) (*read.ChannelFacts, policy.Effective, error) {
	c := Chat{Facts: q.Facts}
	facts, m, err := c.member(ctx, v, ch)
	if err != nil {
		return nil, policy.Effective{}, err
	}
	school := m.SchoolID
	if school == nil {
		school = facts.Channel.SchoolID
	}
	eff, err := q.Facts.Effective(ctx, v.TenantID, school, facts.Channel)
	switch {
	case errors.Is(err, policy.ErrDenied):
		return nil, policy.Effective{}, problem.New(problem.ChatNotAllowed, "")
	case err != nil:
		return nil, policy.Effective{}, unavailableProblem()
	}
	return facts, eff, nil
}

// PinView is one pinned message of listChannelPins.
type PinView struct {
	Message  MessageView
	PinnedBy read.ChatCard
	PinnedAt time.Time
}

// ListPins serves listChannelPins: the live pins of a channel the caller is an active member of (404 otherwise);
// recalled messages are never pinned, messages of a hidden sender are left out.
func (q Features) ListPins(ctx context.Context, v Viewer, ch uuid.UUID) (out []PinView, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListChannelPins")
	defer func() { done(err) }()
	facts, _, err := q.member(ctx, v, ch)
	if err != nil {
		return nil, err
	}
	var pins []read.PinRow
	if err := q.scope(v.TenantID, func(ctx context.Context) error {
		var err error
		pins, err = q.Reader.Pins(ctx, v.TenantID, ch)
		return err
	}, ctx); err != nil {
		return nil, unavailableProblem()
	}
	ids := make([]uuid.UUID, len(pins))
	for i, p := range pins {
		ids[i] = p.MessageID
	}
	recent, _ := q.Recent.List(ctx, ch)
	refs, err := q.Lookup.Refs(ctx, v.TenantID, ch, facts.Channel.IsGameplay, ids, recent)
	if err != nil {
		return nil, unavailableProblem()
	}
	views := make([]MessageView, 0, len(pins))
	for _, p := range pins {
		e, ok := refs[p.MessageID]
		if !ok || !e.Delivered() || facts.Hidden(v.UserID, e.SenderID) {
			continue
		}
		views = append(views, MessageView{Envelope: e, Sender: memberCard(facts, e.SenderID), Pinned: true})
		out = append(out, PinView{PinnedBy: memberCard(facts, p.PinnedBy), PinnedAt: p.PinnedAt})
	}
	if err := q.Decorate(ctx, v, facts, views, recent); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Message = views[i]
	}
	return out, nil
}

func memberCard(facts *read.ChannelFacts, user uuid.UUID) read.ChatCard {
	if c, ok := facts.Cards[user]; ok {
		return c
	}
	return read.ChatCard{UserID: user, Level: 1}
}

// Decorate adds to a page the reply excerpts (≤ 80 characters of the moderated original, nil when the original is
// recalled/removed/held — F118-AC-05, AC-07), the reactions and the pin flag of each message.
func (q Features) Decorate(ctx context.Context, v Viewer, facts *read.ChannelFacts, views []MessageView, recent []message.Envelope) error {
	if len(views) == 0 {
		return nil
	}
	var refIDs, msgIDs []uuid.UUID
	for _, m := range views {
		msgIDs = append(msgIDs, m.Envelope.MessageID)
		if m.Envelope.ReplyToMessageID != nil {
			refIDs = append(refIDs, *m.Envelope.ReplyToMessageID)
		}
		if m.Envelope.Quote != nil {
			refIDs = append(refIDs, m.Envelope.Quote.MessageID)
		}
	}
	refs, err := q.Lookup.Refs(ctx, v.TenantID, facts.Channel.ID, facts.Channel.IsGameplay, refIDs, recent)
	if err != nil {
		return unavailableProblem()
	}
	var reactions []read.ReactionRow
	var pins []read.PinRow
	if err := q.scope(v.TenantID, func(ctx context.Context) error {
		var err error
		if reactions, err = q.Reader.Reactions(ctx, v.TenantID, msgIDs); err != nil {
			return err
		}
		pins, err = q.Reader.Pins(ctx, v.TenantID, facts.Channel.ID)
		return err
	}, ctx); err != nil {
		return unavailableProblem()
	}
	pinned := map[uuid.UUID]bool{}
	for _, p := range pins {
		pinned[p.MessageID] = true
	}
	counts := map[uuid.UUID]map[string]int{}
	mine := map[uuid.UUID][]string{}
	for _, r := range reactions {
		if counts[r.MessageID] == nil {
			counts[r.MessageID] = map[string]int{}
		}
		counts[r.MessageID][r.Emoji]++
		if r.UserID == v.UserID {
			mine[r.MessageID] = append(mine[r.MessageID], r.Emoji)
		}
	}
	for i := range views {
		e := &views[i].Envelope
		if views[i].Recalled || e.Recalled() {
			views[i].Recalled = true
			continue
		}
		views[i].Pinned = pinned[e.MessageID]
		views[i].Reactions = counts[e.MessageID]
		views[i].MyReactions = mine[e.MessageID]
		for _, u := range e.Mentions {
			views[i].Mentions = append(views[i].Mentions, memberCard(facts, u))
		}
		if e.ReplyToMessageID != nil {
			if o, ok := refs[*e.ReplyToMessageID]; ok && o.Quotable() && !facts.Hidden(v.UserID, o.SenderID) {
				ex := message.Preview(*o.BodyDisplay, message.ReplyExcerptMax)
				views[i].ReplyExcerpt = &ex
			}
		}
		if e.Quote != nil {
			if o, ok := refs[e.Quote.MessageID]; !ok || !o.Quotable() || facts.Hidden(v.UserID, o.SenderID) {
				q := *e.Quote
				q.Excerpt = "" // the original is gone: placeholder "Tin nhắn gốc không còn" (F118-AC-05)
				e.Quote = &q
			}
		}
	}
	return nil
}

// ReceiptMember is the status of one member for getMessageReceipts.
type ReceiptMember struct {
	Member read.ChatCard
	Status string // sent, delivered, read
}

// ReceiptsView is MessageReceipts.
type ReceiptsView struct {
	MessageID      uuid.UUID
	Mode           string
	DeliveredCount int
	ReadCount      int
	Members        []ReceiptMember
}

// Receipts serves getMessageReceipts (F118-AC-23, AC-24): computed from the member cursors (no row per message ×
// member); member by member in channels of ≤ 8 members, counts only above; never the status of a member in a block
// relation with the caller; world/area → CHAT_FEATURE_DISABLED.
func (q Features) Receipts(ctx context.Context, v Viewer, ch, id uuid.UUID) (out ReceiptsView, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.GetMessageReceipts")
	defer func() { done(err) }()
	facts, eff, err := q.member(ctx, v, ch)
	if err != nil {
		return out, err
	}
	if facts.Channel.Type.Implicit() || eff.Features.ReceiptMode() == policy.ReceiptsNone {
		return out, problem.New(problem.ChatFeatureDisabled, "").With("feature", policy.FeatureReceipts)
	}
	found, err := q.Lookup.Message(ctx, v.TenantID, id)
	if err != nil {
		return out, unavailableProblem()
	}
	if found == nil || found.Envelope.ChannelID != ch || !found.Envelope.Visible(v.UserID) || facts.Hidden(v.UserID, found.Envelope.SenderID) {
		return out, notFound()
	}
	e := found.Envelope
	var cursors []read.MemberCursor
	if err := q.scope(v.TenantID, func(ctx context.Context) error {
		var err error
		cursors, err = q.Reader.Cursors(ctx, v.TenantID, ch)
		return err
	}, ctx); err != nil {
		return out, unavailableProblem()
	}
	out = ReceiptsView{MessageID: id, Mode: policy.ReceiptsPerMember}
	if len(facts.Members.ActiveIDs()) > PerMemberMax {
		out.Mode = policy.ReceiptsCount
	}
	sort.Slice(cursors, func(i, j int) bool { return cursors[i].UserID.String() < cursors[j].UserID.String() })
	for _, c := range cursors {
		if c.UserID == e.SenderID || facts.Blocked(v.UserID, c.UserID) {
			continue
		}
		status := "sent"
		switch {
		case c.Read >= e.Seq:
			status = "read"
			out.ReadCount++
			out.DeliveredCount++
		case c.Delivered >= e.Seq:
			status = "delivered"
			out.DeliveredCount++
		}
		if out.Mode == policy.ReceiptsPerMember {
			out.Members = append(out.Members, ReceiptMember{Member: memberCard(facts, c.UserID), Status: status})
		}
	}
	return out, nil
}

// StickerPacks serves listStickerPacks (F118-AC-14): published packs assigned to the tenant with the unlock state of
// the caller.
func (q Features) StickerPacks(ctx context.Context, v Viewer) (out []read.StickerPackRow, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListStickerPacks")
	defer func() { done(err) }()
	if v.TenantID == uuid.Nil || v.UserID == uuid.Nil {
		return nil, problem.New(problem.Unauthenticated, "")
	}
	err = q.scope(v.TenantID, func(ctx context.Context) error {
		var err error
		out, err = q.Reader.StickerPacks(ctx, v.TenantID, v.UserID)
		return err
	}, ctx)
	if err != nil {
		return nil, unavailableProblem()
	}
	return out, nil
}

// GifPage is a page of listGifs.
type GifPage struct {
	Data    []read.GifRow
	Next    *read.GifCursor
	HasMore bool
	Limit   int
}

// Gifs serves listGifs (F118-AC-13): published GIFs of the internal library only, by tag; no third-party URL.
func (q Features) Gifs(ctx context.Context, v Viewer, tag string, after *read.GifCursor, limit int) (p GifPage, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListGifs")
	defer func() { done(err) }()
	if v.TenantID == uuid.Nil || v.UserID == uuid.Nil {
		return p, problem.New(problem.Unauthenticated, "")
	}
	if limit <= 0 {
		limit = DefaultGifLimit
	}
	limit = min(limit, MaxGifLimit)
	var rows []read.GifRow
	err = q.scope(v.TenantID, func(ctx context.Context) error {
		var err error
		rows, err = q.Reader.Gifs(ctx, tag, after, limit+1)
		return err
	}, ctx)
	if err != nil {
		return p, unavailableProblem()
	}
	p.Limit = limit
	if len(rows) > limit {
		rows, p.HasMore = rows[:limit], true
		last := rows[len(rows)-1]
		p.Next = &read.GifCursor{CreatedAt: last.CreatedAt, GifID: last.GifID}
	}
	p.Data = rows
	return p, nil
}
