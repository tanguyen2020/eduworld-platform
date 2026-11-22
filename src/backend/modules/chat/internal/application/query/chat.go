package query

import (
	"context"
	"errors"
	"log/slog"
	"sort"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/policy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Limits of the history reads (AsyncAPI chat.history.fetch: 1..50, default 30; REST Limit: 1..100, default 20).
const (
	DefaultFetchLimit  = 30
	MaxFetchLimit      = 50
	DefaultPageLimit   = 20
	MaxPageLimit       = 100
	MaxSyncChannels    = 50
	MaxMyChannels      = 200
	defaultRecentLimit = 200
)

// Viewer is the calling student (principal for REST, gateway headers for WS; ConnID empty for REST).
type Viewer struct {
	TenantID uuid.UUID
	UserID   uuid.UUID
	ConnID   string
}

// ChatConfig is the configuration of the chat queries (chat_recent_buffer).
type ChatConfig struct {
	RecentBuffer int
}

// Chat are the read use cases of the chat core (F115 US2, US3): listMyChannels, listChannelMessages and the WS commands
// chat.channel.join/leave, chat.history.fetch, chat.channel.sync. Only an active member reads a channel (404 otherwise,
// never telling whether it exists — F115-AC-10, AC-11); history starts at joined_at, carries display content only,
// held messages only for their sender, nothing of a blocked sender (F115-AC-18).
type Chat struct {
	Read   service.ReadRunner
	Reader read.ChatReader
	Facts  service.Facts
	Recent port.RecentBuffer
	Fanout port.Fanout // WS pushes (chat.history.loaded); nil for REST only
	Config ChatConfig
}

// MessageView is one message of a history with the card of its sender.
type MessageView struct {
	Envelope message.Envelope
	Sender   read.ChatCard
	Recalled bool
}

// MessagePage is a page of history in seq order.
type MessagePage struct {
	Data    []MessageView
	HasMore bool
	Limit   int
}

// ChannelView is one channel of listMyChannels.
type ChannelView struct {
	Channel      channel.Channel
	AllowedKinds []policy.Kind
	UnreadCount  int64
	MyReadSeq    int64
}

func (q Chat) recentLimit() int {
	if q.Config.RecentBuffer > 0 {
		return q.Config.RecentBuffer
	}
	return defaultRecentLimit
}

func notFound() error { return problem.New(problem.NotFound, "") }

func unavailableProblem() error {
	return problem.New(problem.ChatTemporarilyUnavailable, "").Retry()
}

// member returns the facts of a channel the viewer is an active member of (NOT_FOUND otherwise).
func (q Chat) member(ctx context.Context, v Viewer, id uuid.UUID) (*read.ChannelFacts, channel.Member, error) {
	if v.TenantID == uuid.Nil || v.UserID == uuid.Nil {
		return nil, channel.Member{}, problem.New(problem.Unauthenticated, "")
	}
	facts, err := q.Facts.Channel(ctx, v.TenantID, id)
	if err != nil {
		return nil, channel.Member{}, unavailableProblem()
	}
	if facts == nil {
		return nil, channel.Member{}, notFound()
	}
	m, ok := facts.Members.Active(v.UserID)
	if !ok {
		return nil, channel.Member{}, notFound()
	}
	return facts, m, nil
}

// Join checks membership for chat.channel.join (the presentation answers subscribe ew:chat:fanout:<id>) and, with
// since, pushes the missing messages (≤ chat_recent_buffer) as chat.history.loaded (F115-AC-19).
func (q Chat) Join(ctx context.Context, v Viewer, id uuid.UUID, since *int64) (count int, hasMore bool, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.JoinChannel")
	defer func() { done(err) }()
	facts, m, err := q.member(ctx, v, id)
	if err != nil || since == nil {
		return 0, false, err
	}
	return q.push(ctx, v, facts, m, nil, since, q.recentLimit())
}

// Leave checks the channel id for chat.channel.leave (the presentation answers unsubscribe); leaving a subscription
// never changes membership.
func (q Chat) Leave(_ context.Context, v Viewer, id uuid.UUID) error {
	if v.TenantID == uuid.Nil || v.UserID == uuid.Nil || id == uuid.Nil {
		return problem.Invalid("channel_id", problem.SubInvalid, "must be a UUID")
	}
	return nil
}

// Fetch serves chat.history.fetch: up to limit messages before before (newest page when nil) pushed to the connection.
func (q Chat) Fetch(ctx context.Context, v Viewer, id uuid.UUID, before *int64, limit int) (count int, hasMore bool, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.FetchHistory")
	defer func() { done(err) }()
	if limit <= 0 {
		limit = DefaultFetchLimit
	}
	limit = min(limit, MaxFetchLimit)
	facts, m, err := q.member(ctx, v, id)
	if err != nil {
		return 0, false, err
	}
	return q.push(ctx, v, facts, m, before, nil, limit)
}

// SyncResult is the outcome of one channel of chat.channel.sync.
type SyncResult struct {
	ChannelID uuid.UUID
	Count     int
	HasMore   bool
	Code      string // NOT_FOUND for a channel the viewer is not a member of (no message pushed)
}

// Sync serves chat.channel.sync {channel_id: last_seen_seq}: for each channel the messages after last_seen, at most
// chat_recent_buffer, pushed as chat.history.loaded; more → has_more and the client loads the rest over REST.
func (q Chat) Sync(ctx context.Context, v Viewer, channels map[uuid.UUID]int64) (out []SyncResult, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.SyncChannels")
	defer func() { done(err) }()
	if len(channels) == 0 || len(channels) > MaxSyncChannels {
		return nil, problem.Invalid("channels", problem.SubInvalid, "1..50 channels")
	}
	ids := make([]uuid.UUID, 0, len(channels))
	for id := range channels {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	for _, id := range ids {
		after := max(channels[id], 0)
		facts, m, err := q.member(ctx, v, id)
		if err != nil {
			var pe *problem.Error
			if errors.As(err, &pe) && pe.Code == problem.NotFound {
				out = append(out, SyncResult{ChannelID: id, Code: string(problem.NotFound)})
				continue
			}
			return nil, err
		}
		n, more, err := q.push(ctx, v, facts, m, nil, &after, q.recentLimit())
		if err != nil {
			return nil, err
		}
		out = append(out, SyncResult{ChannelID: id, Count: n, HasMore: more})
	}
	return out, nil
}

func (q Chat) push(ctx context.Context, v Viewer, facts *read.ChannelFacts, m channel.Member, before, after *int64, limit int) (int, bool, error) {
	p, err := q.page(ctx, v, facts, m, before, after, limit)
	if err != nil {
		return 0, false, err
	}
	if q.Fanout != nil && v.ConnID != "" {
		items := make([]port.HistoryItem, len(p.Data))
		for i, x := range p.Data {
			items[i] = port.HistoryItem{Envelope: x.Envelope, Sender: x.Sender, Recalled: x.Recalled}
		}
		if err := q.Fanout.History(ctx, v.UserID, v.ConnID, facts.Channel.ID, items, p.HasMore); err != nil {
			return 0, false, unavailableProblem()
		}
	}
	return len(p.Data), p.HasMore, nil
}

// ListChannelMessages serves listChannelMessages (before_seq / after_seq exclusive; newest page without bounds).
func (q Chat) ListChannelMessages(ctx context.Context, v Viewer, id uuid.UUID, before, after *int64, limit int) (p MessagePage, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListChannelMessages")
	defer func() { done(err) }()
	if before != nil && after != nil {
		return MessagePage{}, problem.Invalid("after_seq", problem.SubInvalid, "before_seq and after_seq are exclusive")
	}
	if limit <= 0 {
		limit = DefaultPageLimit
	}
	limit = min(limit, MaxPageLimit)
	facts, m, err := q.member(ctx, v, id)
	if err != nil {
		return MessagePage{}, err
	}
	return q.page(ctx, v, facts, m, before, after, limit)
}

// page merges the stored messages with the recent buffer (messages not persisted yet, ADR-0035 §6): deduplicated by
// message_id, ordered by seq. When PostgreSQL cannot be read the recent buffer alone answers (F115-AC-16).
func (q Chat) page(ctx context.Context, v Viewer, facts *read.ChannelFacts, m channel.Member, before, after *int64, limit int) (MessagePage, error) {
	ch := facts.Channel
	var stored []read.StoredMessage
	s, err := authz.EventScope(v.TenantID)
	if err != nil {
		return MessagePage{}, err
	}
	err = q.Read.InTenantRead(ctx, s, func(ctx context.Context) error {
		var err error
		stored, err = q.Reader.Messages(ctx, read.MessageQuery{Tenant: v.TenantID, Channel: ch.ID, Gameplay: ch.IsGameplay, Reader: v.UserID,
			From: m.JoinedAt, Before: before, After: after, Limit: limit + 1})
		return err
	})
	if err != nil {
		slog.WarnContext(ctx, "chat: stored history unavailable, serving the recent buffer", "error", err)
		stored = nil
	}
	recent, rerr := q.Recent.List(ctx, ch.ID)
	if rerr != nil {
		if err != nil {
			return MessagePage{}, unavailableProblem()
		}
		slog.WarnContext(ctx, "chat: recent buffer unavailable", "error", rerr)
	}
	seen := map[uuid.UUID]bool{}
	var all []MessageView
	add := func(e message.Envelope, recalled bool) {
		if seen[e.MessageID] || facts.Hidden(v.UserID, e.SenderID) { // block both ways, mute one way (F117-AC-12)
			return
		}
		seen[e.MessageID] = true
		card, ok := facts.Cards[e.SenderID]
		if !ok {
			card = read.ChatCard{UserID: e.SenderID, Level: 1}
		}
		all = append(all, MessageView{Envelope: e, Sender: card, Recalled: recalled})
	}
	for _, x := range stored {
		add(x.Envelope, x.Recalled)
	}
	for _, e := range recent {
		if e.TenantID != v.TenantID || e.ChannelID != ch.ID || !e.Visible(v.UserID) || e.CreatedAt.Before(m.JoinedAt) ||
			(before != nil && e.Seq >= *before) || (after != nil && e.Seq <= *after) {
			continue
		}
		add(e, false)
	}
	asc := after != nil
	sort.Slice(all, func(i, j int) bool {
		if asc {
			return all[i].Envelope.Seq < all[j].Envelope.Seq
		}
		return all[i].Envelope.Seq > all[j].Envelope.Seq
	})
	more := len(all) > limit
	if more {
		all = all[:limit]
	}
	if !asc {
		for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
			all[i], all[j] = all[j], all[i]
		}
	}
	return MessagePage{Data: all, HasMore: more, Limit: limit}, nil
}

// ListMyChannels serves listMyChannels: the channels the caller is an active member of, with the kinds allowed by the
// effective policy (none when denied) and the unread count (last_seq − my_read_seq).
func (q Chat) ListMyChannels(ctx context.Context, v Viewer) (out []ChannelView, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListMyChannels")
	defer func() { done(err) }()
	if v.TenantID == uuid.Nil || v.UserID == uuid.Nil {
		return nil, problem.New(problem.Unauthenticated, "")
	}
	s, err := authz.EventScope(v.TenantID)
	if err != nil {
		return nil, err
	}
	var rows []read.MyChannel
	if err := q.Read.InTenantRead(ctx, s, func(ctx context.Context) error {
		var err error
		rows, err = q.Reader.MyChannels(ctx, v.TenantID, v.UserID, MaxMyChannels)
		return err
	}); err != nil {
		return nil, err
	}
	out = make([]ChannelView, 0, len(rows))
	for _, r := range rows {
		school := r.SchoolID
		if school == nil {
			school = r.Channel.SchoolID
		}
		kinds := []policy.Kind{}
		eff, err := q.Facts.Effective(ctx, v.TenantID, school, r.Channel)
		switch {
		case err == nil:
			kinds = eff.AllowedKinds
		case !errors.Is(err, policy.ErrDenied):
			return nil, unavailableProblem()
		}
		out = append(out, ChannelView{Channel: r.Channel, AllowedKinds: kinds, UnreadCount: max(r.Channel.LastSeq-r.MyReadSeq, 0),
			MyReadSeq: r.MyReadSeq})
	}
	return out, nil
}
