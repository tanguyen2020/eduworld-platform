package query

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"time"

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
	// Decor decorates the pages (F118: reply excerpts, reactions, pins); nil: none.
	Decor *Features
	// Implicit resolves World/Area (F120-AC-12, AC-13); Ensure creates them lazily (command.Persistent.EnsureImplicit,
	// called by chat.channel.join before the query). nil: no implicit channel.
	Implicit *service.Implicit
	// Sanctions are the active sanctions of the caller (service.Sanctions, cached ew:chat:sanction): they narrow
	// Channel.sendable_kinds and features (delta F119). nil: no sanction considered.
	Sanctions SanctionReader
	Now       func() time.Time
}

// SanctionReader returns the active sanctions of a user (service.Sanctions).
type SanctionReader interface {
	Active(ctx context.Context, tenant, user uuid.UUID, now time.Time) ([]port.Sanction, error)
}

func (q Chat) now() time.Time {
	if q.Now != nil {
		return q.Now()
	}
	return time.Now()
}

// MessageView is one message of a history with the card of its sender.
type MessageView struct {
	Envelope message.Envelope
	Sender   read.ChatCard
	Recalled bool
	// F118 decoration (Features.Decorate): reply excerpt of the moderated original (nil: none or gone), reactions, pin.
	ReplyExcerpt *string
	Reactions    map[string]int
	MyReactions  []string
	Pinned       bool
	Mentions     []read.ChatCard
}

// MessagePage is a page of history in seq order.
type MessagePage struct {
	Data    []MessageView
	HasMore bool
	Limit   int
}

// ChannelView is one channel of listMyChannels.
type ChannelView struct {
	Channel     channel.Channel
	UnreadCount int64
	MyReadSeq   int64
	// NotificationLevel of the caller ("" for World/Area: no membership — delta F112-9).
	NotificationLevel string
	// Peer is the public card of the other student of a direct channel (nil otherwise or unknown — delta F112-5).
	Peer *read.ChatCard
	ChannelCaps
}

// ChannelCaps is what the caller may do in a channel now (delta F119, CHAT_SYSTEM §3, §7.1): the kinds of the effective
// policy, the kinds the caller can send under its active sanctions, max_text_length and the features narrowed to those
// kinds. Allowed false (policy denied): no kind, no length, no features.
type ChannelCaps struct {
	Allowed       bool
	AllowedKinds  []policy.Kind
	SendableKinds []policy.Kind
	MaxTextLength int
	Features      policy.Features
}

// caps derives the ChannelCaps of ch from its effective policy (nil: denied) and the active sanctions of the caller.
func caps(ch channel.Channel, eff *policy.Effective, sanctions []port.Sanction, now time.Time) ChannelCaps {
	if eff == nil {
		return ChannelCaps{AllowedKinds: []policy.Kind{}, SendableKinds: []policy.Kind{}}
	}
	sendable := make([]policy.Kind, 0, len(eff.AllowedKinds))
	for _, k := range eff.AllowedKinds {
		ok := true
		for _, s := range sanctions {
			if s.Refusal(ch.ID, string(k), string(ch.Type), now) != "" {
				ok = false
				break
			}
		}
		if ok {
			sendable = append(sendable, k)
		}
	}
	return ChannelCaps{Allowed: true, AllowedKinds: eff.AllowedKinds, SendableKinds: sendable, MaxTextLength: eff.MaxTextLength,
		Features: eff.Features.ForCaller(sendable, ch.Type.Implicit())}
}

// sanctions loads the active sanctions of the viewer once per request (none without a reader); a failed read is
// CHAT_TEMPORARILY_UNAVAILABLE like the policy (never report a muted student as free to send).
func (q Chat) sanctions(ctx context.Context, v Viewer, now time.Time) ([]port.Sanction, error) {
	if q.Sanctions == nil {
		return nil, nil
	}
	list, err := q.Sanctions.Active(ctx, v.TenantID, v.UserID, now)
	if err != nil {
		return nil, unavailableProblem()
	}
	return list, nil
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
	if facts.Channel.Type.Implicit() { // World/Area: access from the projections, no history window (F120)
		if q.Implicit == nil {
			return nil, channel.Member{}, notFound()
		}
		ok, school, err := q.Implicit.Access(ctx, v.TenantID, v.UserID, facts.Channel)
		if err != nil {
			return nil, channel.Member{}, unavailableProblem()
		}
		if !ok {
			return nil, channel.Member{}, notFound()
		}
		return facts, channel.Member{ChannelID: id, UserID: v.UserID, TenantID: v.TenantID, SchoolID: school, Role: channel.RoleMember}, nil
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
			items[i] = port.HistoryItem{Envelope: x.Envelope, Sender: x.Sender, Recalled: x.Recalled, Extras: x.Extras()}
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
	if !ch.Type.Implicit() { // World/Area are ephemeral: the recent buffer only (chat.ephemeral_messages is not a history)
		err = q.Read.InTenantRead(ctx, s, func(ctx context.Context) error {
			var err error
			stored, err = q.Reader.Messages(ctx, read.MessageQuery{Tenant: v.TenantID, Channel: ch.ID, Gameplay: ch.IsGameplay, Reader: v.UserID,
				From: m.JoinedAt, Before: before, After: after, Limit: limit + 1})
			return err
		})
	}
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
	// World/Area have no member list, so the channel facts carry no blocks: the relations of the viewer hide the senders it
	// blocked, was blocked by or muted (F120 T061). Unreadable relations → nothing is served (fail closed).
	var rel read.UserRelations
	if ch.Type.Implicit() {
		if q.Implicit == nil {
			return MessagePage{}, notFound()
		}
		var relErr error
		if rel, relErr = q.Implicit.Relations(ctx, v.TenantID, v.UserID); relErr != nil {
			return MessagePage{}, unavailableProblem()
		}
	}
	seen := map[uuid.UUID]int{}
	var all []MessageView
	add := func(e message.Envelope, recalled bool) {
		// block both ways, mute one way (F117-AC-12; World/Area through the viewer's relations, F120 T061)
		if facts.Hidden(v.UserID, e.SenderID) || (e.SenderID != v.UserID && rel.HidesSender(e.SenderID)) {
			return
		}
		recalled = recalled || e.Recalled()
		if i, ok := seen[e.MessageID]; ok {
			// The newest revision wins (an edit or recall not applied by chat-persister yet, F118); a recall seen
			// anywhere stays a recall and never shows content again.
			prev := all[i]
			if e.Revision > prev.Envelope.Revision {
				all[i].Envelope = e
			}
			if recalled || prev.Recalled {
				all[i].Recalled = true
				all[i].Envelope.BodyDisplay, all[i].Envelope.BodyOriginal, all[i].Envelope.Quote = nil, nil, nil
			}
			return
		}
		seen[e.MessageID] = len(all)
		card, ok := facts.Cards[e.SenderID]
		if !ok {
			card = read.ChatCard{UserID: e.SenderID, Level: 1}
		}
		if recalled {
			e.BodyDisplay, e.BodyOriginal, e.Quote = nil, nil, nil
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
	if q.Decor != nil {
		if err := q.Decor.Decorate(ctx, v, facts, all, recent); err != nil {
			slog.WarnContext(ctx, "chat: history decoration unavailable", "error", err)
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
	now := q.now()
	sanctions, err := q.sanctions(ctx, v, now)
	if err != nil {
		return nil, err
	}
	out = make([]ChannelView, 0, len(rows))
	for _, r := range rows {
		school := r.SchoolID
		if school == nil {
			school = r.Channel.SchoolID
		}
		var effective *policy.Effective
		eff, err := q.Facts.Effective(ctx, v.TenantID, school, r.Channel)
		switch {
		case err == nil:
			effective = &eff
		case !errors.Is(err, policy.ErrDenied):
			return nil, unavailableProblem()
		}
		out = append(out, ChannelView{Channel: r.Channel, UnreadCount: max(r.Channel.LastSeq-r.MyReadSeq, 0),
			MyReadSeq: r.MyReadSeq, NotificationLevel: r.NotificationLevel, Peer: q.Peer(ctx, v, r.Channel),
			ChannelCaps: caps(r.Channel, effective, sanctions, now)})
	}
	implicit, err := q.implicitChannels(ctx, v, sanctions, now)
	if err != nil {
		return nil, err
	}
	return append(out, implicit...), nil
}

// implicitChannels are the World channel of the caller and the Area channel of its hub instance (F120-AC-12, AC-13), when
// the effective policy allows them (a tenant common has no World until System Admin re-enables it, F120-AC-28). Their ids
// are deterministic: the row is created at the first send or join.
func (q Chat) implicitChannels(ctx context.Context, v Viewer, sanctions []port.Sanction, now time.Time) ([]ChannelView, error) {
	if q.Implicit == nil {
		return nil, nil
	}
	var out []ChannelView
	add := func(ch channel.Channel, school *uuid.UUID) error {
		eff, err := q.Facts.Effective(ctx, v.TenantID, school, ch)
		switch {
		case errors.Is(err, policy.ErrDenied):
			return nil
		case err != nil:
			return unavailableProblem()
		}
		if facts, err := q.Facts.Channel(ctx, v.TenantID, ch.ID); err == nil && facts != nil {
			ch = facts.Channel
		}
		out = append(out, ChannelView{Channel: ch, ChannelCaps: caps(ch, &eff, sanctions, now)})
		return nil
	}
	world, scope, ok, err := q.Implicit.World(ctx, v.TenantID, v.UserID)
	if err != nil {
		return nil, unavailableProblem()
	}
	if !ok {
		return nil, nil
	}
	sc, err := q.Implicit.School(ctx, v.TenantID, v.UserID)
	if err != nil {
		return nil, unavailableProblem()
	}
	if err := add(channel.Channel{ID: world, TenantID: v.TenantID, SchoolID: scope, Type: channel.World,
		ScopeRef: channel.WorldScope(v.TenantID, scope), Status: channel.StatusOpen}, sc); err != nil {
		return nil, err
	}
	area, room, ok, err := q.Implicit.Area(ctx, v.TenantID, v.UserID)
	if err != nil {
		return nil, unavailableProblem()
	}
	if ok {
		if err := add(channel.Channel{ID: area, TenantID: v.TenantID, Type: channel.Area, ScopeRef: channel.AreaScope(room),
			Status: channel.StatusOpen}, sc); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Peer returns the public card of the other student of a direct channel (delta F112-5): user id, nickname, level and
// avatar only — never a real name, school or grade (R16). nil for another channel type, when the facts cannot be read
// (best effort: the field is optional) or when the other student has no player profile yet.
func (q Chat) Peer(ctx context.Context, v Viewer, ch channel.Channel) *read.ChatCard {
	if ch.Type != channel.Direct {
		return nil
	}
	facts, err := q.Facts.Channel(ctx, v.TenantID, ch.ID)
	if err != nil || facts == nil {
		if err != nil {
			slog.WarnContext(ctx, "chat: direct peer card unavailable", "error", err)
		}
		return nil
	}
	for _, m := range facts.Members {
		if m.UserID == v.UserID {
			continue
		}
		c, ok := facts.Cards[m.UserID]
		if !ok || c.Nickname == nil {
			return nil
		}
		c.UserID = m.UserID
		return &c
	}
	return nil
}

// Extras returns the read-time parts of the view.
func (m MessageView) Extras() port.MessageExtras {
	return port.MessageExtras{ReplyExcerpt: m.ReplyExcerpt, Reactions: m.Reactions, MyReactions: m.MyReactions, Pinned: m.Pinned,
		Mentions: m.Mentions}
}

// Caps returns what the viewer may do in ch now (openDirectChannel, delta F119): the kinds of the effective policy, the
// sendable kinds under its active sanctions, max_text_length and features. Best effort: a denied policy or an
// unavailable read gives no kind and no features (the commands still check every request).
func (q Chat) Caps(ctx context.Context, v Viewer, ch channel.Channel) ChannelCaps {
	school := ch.SchoolID
	if q.Implicit != nil {
		if s, err := q.Implicit.School(ctx, v.TenantID, v.UserID); err == nil && s != nil {
			school = s
		}
	}
	now := q.now()
	eff, err := q.Facts.Effective(ctx, v.TenantID, school, ch)
	if err != nil {
		return caps(ch, nil, nil, now)
	}
	sanctions, err := q.sanctions(ctx, v, now)
	if err != nil { // sanctions unknown: the policy kinds stay, nothing is reported as sendable (fail closed)
		return ChannelCaps{Allowed: true, AllowedKinds: eff.AllowedKinds, SendableKinds: []policy.Kind{}, MaxTextLength: eff.MaxTextLength,
			Features: eff.Features.ForCaller(nil, ch.Type.Implicit())}
	}
	return caps(ch, &eff, sanctions, now)
}
