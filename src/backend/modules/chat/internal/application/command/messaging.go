package command

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/policy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Audit actions of the message features (ids only, never content — R14).
const (
	AuditMessageRecalled = "chat.message.recalled"
	AuditMessagePinned   = "chat.message.pinned"
	AuditMessageUnpinned = "chat.message.unpinned"
)

// Typing timings (CONFIG_DEFAULTS chat_typing_throttle [Confirmed]: once per 3 s, expires after 5 s).
const (
	TypingThrottle = 3 * time.Second
	TypingTTL      = 5 * time.Second
	TypingShown    = 3
)

// FeatureMetrics are the metrics of the message features (no user or content label, R14).
type FeatureMetrics interface {
	Event(event string)
	Typing()
}

// Messaging are the write use cases of the message features (F118, ADR-0034): edit, recall, forward, reactions, pins and
// typing. Every change of a message is published to the durable stream (ew.chat.<tenant>.evt.<channel>) and only after
// the JetStream ack is anyone told about it (R25, ADR-0035); chat-persister writes PostgreSQL. New content (an edit, a
// forward, a quote) goes through the moderation pipeline again (R15, R25); a recall keeps the original as evidence.
type Messaging struct {
	Send      Send // the single send path: forward creates new messages through it
	Lookup    service.Lookup
	Features  read.FeatureReader
	Changes   port.ChangeStream
	Gate      port.RevisionGate
	Reactions port.ReactionState
	Pins      port.PinState
	Typing    port.TypingState
	Fanout    port.FeatureFanout
	Emoji     port.EmojiSet
	Metrics   FeatureMetrics // nil: none
	Now       func() time.Time
}

func (h Messaging) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func (h Messaging) event(name string) {
	if h.Metrics != nil {
		h.Metrics.Event(name)
	}
}

// target is a message the caller acts on, with the facts of its channel.
type target struct {
	e      message.Envelope
	facts  *read.ChannelFacts
	member channel.Member
	eff    policy.Effective
	school *uuid.UUID
}

func disabled(feature string) error {
	return problem.New(problem.ChatFeatureDisabled, "").With("feature", feature)
}

// member loads the facts of a channel of the caller (404 when not an active member — never telling why, F118-AC-26).
func (h Messaging) member(ctx context.Context, s Sender, ch uuid.UUID) (*read.ChannelFacts, channel.Member, error) {
	if s.TenantID == uuid.Nil || s.UserID == uuid.Nil {
		return nil, channel.Member{}, problem.New(problem.Unauthenticated, "")
	}
	facts, err := h.Send.Facts.Channel(ctx, s.TenantID, ch)
	if err != nil {
		return nil, channel.Member{}, temporarily()
	}
	if facts == nil {
		return nil, channel.Member{}, notFound()
	}
	m, ok := facts.Members.Active(s.UserID)
	if !ok {
		return nil, channel.Member{}, notFound()
	}
	return facts, m, nil
}

// effective resolves the effective policy of the channel for the member (CHAT_NOT_ALLOWED when denied).
func (h Messaging) effective(ctx context.Context, s Sender, facts *read.ChannelFacts, m channel.Member) (policy.Effective, *uuid.UUID, error) {
	school := m.SchoolID
	if school == nil {
		school = facts.Channel.SchoolID
	}
	eff, err := h.Send.Facts.Effective(ctx, s.TenantID, school, facts.Channel)
	switch {
	case errors.Is(err, policy.ErrDenied):
		return policy.Effective{}, nil, problem.New(problem.ChatNotAllowed, "")
	case err != nil:
		return policy.Effective{}, nil, temporarily()
	}
	return eff, school, nil
}

// load finds a message of the caller's tenant in a channel the caller is an active member of and sees (held messages
// only for their sender, nothing from a blocked sender): 404 otherwise (F118-AC-26).
func (h Messaging) load(ctx context.Context, s Sender, id uuid.UUID) (*target, error) {
	if s.TenantID == uuid.Nil || s.UserID == uuid.Nil {
		return nil, problem.New(problem.Unauthenticated, "")
	}
	if id == uuid.Nil {
		return nil, notFound()
	}
	found, err := h.Lookup.Message(ctx, s.TenantID, id)
	if err != nil {
		return nil, temporarily()
	}
	if found == nil || found.Envelope.TenantID != s.TenantID {
		return nil, notFound()
	}
	facts, m, err := h.member(ctx, s, found.Envelope.ChannelID)
	if err != nil {
		return nil, err
	}
	e := found.Envelope
	if e.ChannelType == "" {
		e.ChannelType = string(facts.Channel.Type) // stored rows do not carry the channel type
	}
	e.IsGameplay = facts.Channel.IsGameplay
	if (!e.Visible(s.UserID) && !e.Recalled()) || facts.Hidden(s.UserID, e.SenderID) || e.CreatedAt.Before(m.JoinedAt) {
		return nil, notFound()
	}
	eff, school, err := h.effective(ctx, s, facts, m)
	if err != nil {
		return nil, err
	}
	return &target{e: e, facts: facts, member: m, eff: eff, school: school}, nil
}

func open(facts *read.ChannelFacts) error {
	if !facts.Channel.Open() {
		return problem.New(problem.ChatChannelClosed, "")
	}
	return nil
}

// recipients are the active members who see the messages of sender (block both ways, mute one way — F117-AC-12).
func recipients(facts *read.ChannelFacts, sender uuid.UUID) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(facts.Members))
	for _, id := range facts.Members.ActiveIDs() {
		if !facts.Hidden(id, sender) {
			out = append(out, id)
		}
	}
	return out
}

func (h Messaging) card(facts *read.ChannelFacts, user uuid.UUID) read.ChatCard {
	if c, ok := facts.Cards[user]; ok {
		return c
	}
	return read.ChatCard{UserID: user, Level: 1}
}

func (h Messaging) publish(ctx context.Context, c message.Change) error {
	c.V, c.Kind, c.ChangeID = 1, message.ChangeKind, ids.New()
	c.TraceID = traceID(ctx)
	if err := h.Changes.PublishChange(ctx, c); err != nil {
		if h.Send.Metrics != nil {
			h.Send.Metrics.PublishError()
		}
		return err
	}
	h.event(c.Op)
	return nil
}

func change(op string, t *target, actor uuid.UUID, now time.Time) message.Change {
	return message.Change{Op: op, TenantID: t.e.TenantID, ChannelID: t.e.ChannelID, ChannelType: t.e.ChannelType,
		IsGameplay: t.e.IsGameplay, MessageID: t.e.MessageID, ActorID: actor, At: now}
}

// audit records an action of the message features in its own transaction (best effort after the ack: the change is
// already durable; ids only).
func (h Messaging) audit(ctx context.Context, s Sender, action string, msg, ch uuid.UUID) {
	if h.Send.UoW == nil || h.Send.Audit == nil {
		return
	}
	err := h.Send.UoW.InTenantTx(context.WithoutCancel(ctx), mustScope(s.TenantID), func(ctx context.Context) error {
		return h.Send.Audit.Record(ctx, audit.Event{Action: action, TargetType: "chat_message", TargetID: msg.String(),
			Result: audit.ResultSuccess, Metadata: map[string]any{"channel_id": ch.String(), "actor_id": s.UserID.String()}})
	})
	if err != nil {
		slog.WarnContext(ctx, "chat: message action not audited", "action", action, "error", err)
	}
}

// EditResult is the outcome of an edit: the newest version as the sender sees it; Held when the new content waits for
// review (the other members keep the previous version, F118-AC-03).
type EditResult struct {
	Envelope message.Envelope
	Sender   read.ChatCard
	Held     bool
}

// previousDecision is the decision of a stored version (PostgreSQL rows carry only the state).
func previousDecision(e message.Envelope) message.Decision {
	if e.Moderation.Decision != "" {
		return e.Moderation.Decision
	}
	switch e.Moderation.State {
	case message.StateMasked:
		return message.Mask
	case message.StateUnchecked:
		return message.Unchecked
	}
	return message.Allow
}

// Edit is editMessage / chat.message.edit (F118-AC-01 … AC-03).
func (h Messaging) Edit(ctx context.Context, s Sender, id uuid.UUID, body string, mentions []uuid.UUID, expected int) (res EditResult, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.EditMessage")
	defer func() { done(err) }()
	t, err := h.load(ctx, s, id)
	if err != nil {
		return EditResult{}, err
	}
	if err := open(t.facts); err != nil {
		return EditResult{}, err
	}
	if !t.eff.Features.Edit {
		return EditResult{}, disabled(policy.FeatureEdit)
	}
	now := h.now()
	if err := t.e.CanEdit(s.UserID, now, t.eff.Features.EditWindow()); err != nil {
		return EditResult{}, problem.New(problem.ChatEditWindowExpired, "")
	}
	if expected != max(t.e.Revision, 1) {
		return EditResult{}, problem.New(problem.VersionMismatch, "")
	}
	if strings.TrimSpace(body) == "" || (t.eff.MaxTextLength > 0 && utf8.RuneCountInString(body) > t.eff.MaxTextLength) {
		return EditResult{}, problem.Invalid("body", problem.SubInvalid, "1..max_text_length characters")
	}
	filtered, err := h.mentions(t, s, mentions)
	if err != nil {
		return EditResult{}, err
	}
	// New content: the whole pipeline again (R25, F118-AC-01); a block keeps the previous version (F118-AC-03).
	verdict, err := h.Send.Validator.Validate(ctx, port.ValidationInput{TenantID: s.TenantID, SchoolID: t.school, ChannelID: t.e.ChannelID,
		ChannelType: t.e.ChannelType, SenderID: s.UserID, Kind: message.KindText, Text: body, Mentions: len(filtered),
		Strictness: t.eff.Strictness})
	var rejected *problem.Error
	if errors.As(err, &rejected) {
		return EditResult{}, rejected
	}
	if err != nil {
		slog.WarnContext(ctx, "chat: content validator failed", "error", err)
		return EditResult{}, temporarily()
	}
	state, deliver, stored, err := verdict.Decision.Outcome()
	if err != nil {
		return EditResult{}, temporarily()
	}
	if !stored {
		pe := problem.New(problem.ChatContentBlocked, "")
		if len(verdict.CategoryCodes) > 0 {
			pe = pe.With("category_code", verdict.CategoryCodes[0])
		}
		return EditResult{}, pe
	}
	display := verdict.Display
	if display == "" {
		display = body
	}
	var original *string
	if display != body {
		o := body
		original = &o
	}
	mod := message.Moderation{Decision: verdict.Decision, State: state, CategoryCodes: verdict.CategoryCodes, StepsRun: verdict.StepsRun,
		StepsSkipped: verdict.StepsSkipped, CatalogVersion: verdict.CatalogVersion, CatalogVersionTenant: verdict.CatalogVersionTenant,
		SwitchSnapshotID: verdict.SwitchSnapshotID, LatencyMS: verdict.LatencyMS}
	next := t.e.Edited(display, original, filtered, mod, now)
	prev := &message.Previous{Revision: max(t.e.Revision, 1), BodyDisplay: t.e.BodyDisplay, BodyOriginal: t.e.BodyOriginal,
		Mentions: t.e.Mentions, Decision: previousDecision(t.e)}
	c := change(message.OpEdit, t, s.UserID, now)
	c.Revision, c.BodyDisplay, c.BodyOriginal, c.Mentions, c.Moderation, c.Previous = next.Revision, next.BodyDisplay, next.BodyOriginal,
		next.Mentions, &mod, prev
	card := h.card(t.facts, s.UserID)
	if !deliver {
		// hold_for_review / block_and_alert: the proposal is kept as evidence for the Safety Team, the other members keep
		// the previous version (F118-AC-03); the revision does not move.
		c.Op = message.OpEditHeld
		if err := h.publish(ctx, c); err != nil {
			return EditResult{}, temporarily()
		}
		if err := h.Fanout.HeldEdit(context.WithoutCancel(ctx), s.UserID, s.ConnID, next); err != nil {
			slog.WarnContext(ctx, "chat: held edit not pushed", "error", err)
		}
		return EditResult{Envelope: next, Sender: card, Held: true}, nil
	}
	ok, _, err := h.Gate.Advance(ctx, id, expected, max(t.e.Revision, 1))
	if err != nil {
		return EditResult{}, temporarily()
	}
	if !ok {
		return EditResult{}, problem.New(problem.VersionMismatch, "")
	}
	if err := h.publish(ctx, c); err != nil {
		if rerr := h.Gate.Restore(context.WithoutCancel(ctx), id, next.Revision, expected); rerr != nil {
			slog.WarnContext(ctx, "chat: revision gate not restored", "error", rerr)
		}
		return EditResult{}, temporarily()
	}
	bg := context.WithoutCancel(ctx)
	if err := h.Send.Recent.Push(bg, next); err != nil {
		slog.WarnContext(ctx, "chat: recent buffer not updated", "error", err)
	}
	cards := make([]read.ChatCard, len(filtered))
	for i, u := range filtered {
		cards[i] = h.card(t.facts, u)
	}
	if err := h.Fanout.Edited(bg, next, cards, recipients(t.facts, s.UserID)); err != nil {
		slog.WarnContext(ctx, "chat: edit fan-out failed", "error", err)
	}
	h.notifyMentions(bg, t.facts, next, without(filtered, t.e.Mentions))
	return EditResult{Envelope: next, Sender: card}, nil
}

func without(a, b []uuid.UUID) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(a))
	for _, x := range a {
		found := false
		for _, y := range b {
			if x == y {
				found = true
				break
			}
		}
		if !found {
			out = append(out, x)
		}
	}
	return out
}

// mentions applies CHAT_SYSTEM §7.7 to the mentions of a message of t (F118-AC-18).
func (h Messaging) mentions(t *target, s Sender, ids []uuid.UUID) ([]uuid.UUID, error) {
	return filterMentions(t.facts, t.eff, s.UserID, ids)
}

func filterMentions(facts *read.ChannelFacts, eff policy.Effective, sender uuid.UUID, ids []uuid.UUID) ([]uuid.UUID, error) {
	if len(ids) == 0 {
		return []uuid.UUID{}, nil
	}
	if !eff.Features.Mention {
		return nil, disabled(policy.FeatureMention)
	}
	out, err := message.FilterMentions(ids, sender, eff.Features.MentionsMax(), func(u uuid.UUID) bool {
		_, ok := facts.Members.Active(u)
		return ok
	}, facts.Blocked)
	if err != nil {
		return nil, problem.New(problem.ChatMentionLimit, "")
	}
	return out, nil
}

// notifyMentions sends chat.mention.created to each mentioned member who sees the sender (best effort; the counter
// unread_mentions is raised by chat-persister with the row, F118-AC-19).
func (h Messaging) notifyMentions(ctx context.Context, facts *read.ChannelFacts, e message.Envelope, users []uuid.UUID) {
	notifyMentions(ctx, h.Fanout, facts, e, users)
}

func notifyMentions(ctx context.Context, f port.FeatureFanout, facts *read.ChannelFacts, e message.Envelope, users []uuid.UUID) {
	if f == nil || len(users) == 0 {
		return
	}
	by, ok := facts.Cards[e.SenderID]
	if !ok {
		by = read.ChatCard{UserID: e.SenderID, Level: 1}
	}
	preview := ""
	if e.BodyDisplay != nil && e.MsgKind == message.KindText {
		preview = message.Preview(*e.BodyDisplay, message.MentionPreviewMx)
	}
	for _, u := range users {
		if facts.Hidden(u, e.SenderID) {
			continue
		}
		if err := f.Mention(ctx, u, e.ChannelID, e.MessageID, e.Seq, by, preview); err != nil {
			slog.WarnContext(ctx, "chat: mention not pushed", "error", err)
		}
	}
}

// Recall is recallMessage / chat.message.recall (F118-AC-04): soft (recalled_at/by), the original and every revision
// stay as evidence until the channel retention ends (F118-AC-06); a pinned message is unpinned.
func (h Messaging) Recall(ctx context.Context, s Sender, id uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.RecallMessage")
	defer func() { done(err) }()
	t, err := h.load(ctx, s, id)
	if err != nil {
		return err
	}
	if err := open(t.facts); err != nil {
		return err
	}
	if !t.eff.Features.Recall {
		return disabled(policy.FeatureRecall)
	}
	now := h.now()
	switch err := t.e.CanRecall(s.UserID, now, t.eff.Features.RecallWindow()); {
	case errors.Is(err, message.ErrRecalled):
		return nil // idempotent: already recalled
	case err != nil:
		return problem.New(problem.ChatRecallWindowExpired, "")
	}
	cur := max(t.e.Revision, 1)
	ok, _, err := h.Gate.Advance(ctx, id, cur, cur)
	if err != nil {
		return temporarily()
	}
	if !ok {
		return problem.New(problem.VersionMismatch, "")
	}
	next := t.e.WithRecall(s.UserID, now)
	c := change(message.OpRecall, t, s.UserID, now)
	c.Revision = next.Revision
	if err := h.publish(ctx, c); err != nil {
		if rerr := h.Gate.Restore(context.WithoutCancel(ctx), id, next.Revision, cur); rerr != nil {
			slog.WarnContext(ctx, "chat: revision gate not restored", "error", rerr)
		}
		return temporarily()
	}
	bg := context.WithoutCancel(ctx)
	if err := h.Send.Recent.Push(bg, next); err != nil {
		slog.WarnContext(ctx, "chat: recent buffer not updated", "error", err)
	}
	all := t.facts.Members.ActiveIDs()
	if err := h.Fanout.Recalled(bg, next, all); err != nil {
		slog.WarnContext(ctx, "chat: recall fan-out failed", "error", err)
	}
	// A recalled pinned message is unpinned (CHAT_SYSTEM §7.3); chat-persister soft-deletes the pin row with the recall.
	if h.Pins != nil {
		removed, pins, err := h.Pins.Remove(bg, t.e.ChannelID, id, h.pinLoader(t.e.TenantID, t.e.ChannelID))
		if err == nil && removed {
			if err := h.Fanout.Pins(bg, t.e.ChannelID, pins, read.ChatCard{}, all); err != nil {
				slog.WarnContext(ctx, "chat: pin fan-out failed", "error", err)
			}
		}
	}
	h.audit(ctx, s, AuditMessageRecalled, id, t.e.ChannelID)
	return nil
}

func (h Messaging) pinLoader(tenant, ch uuid.UUID) func(ctx context.Context) ([]uuid.UUID, error) {
	return func(ctx context.Context) ([]uuid.UUID, error) {
		var rows []read.PinRow
		err := h.Lookup.Read.InTenantRead(ctx, mustScope(tenant), func(ctx context.Context) error {
			var err error
			rows, err = h.Features.Pins(ctx, tenant, ch)
			return err
		})
		if err != nil {
			return nil, port.ErrUnavailable
		}
		out := make([]uuid.UUID, len(rows))
		for i, r := range rows {
			out[i] = r.MessageID
		}
		return out, nil
	}
}

// forwardNamespace derives the client_message_id of each forwarded copy: a retry (same Idempotency-Key, or the same
// source and target over WS) republishes the same messages instead of creating new ones (F118-AC-11).
var forwardNamespace = uuid.MustParse("6f1d3c2a-9b8e-5d47-a1f0-2c3b4d5e6f70")

// Forward is forwardMessage / chat.message.forward (F118-AC-09 … AC-11): a new message in each target channel of the
// same tenant, each through the send path of its channel (policy, rate, the whole pipeline again).
func (h Messaging) Forward(ctx context.Context, s Sender, id uuid.UUID, targets []uuid.UUID, key string) (out []SendResult, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.ForwardMessage")
	defer func() { done(err) }()
	t, err := h.load(ctx, s, id)
	if err != nil {
		return nil, err
	}
	notAllowed := problem.New(problem.ChatForwardNotAllowed, "")
	if t.facts.Channel.Type.Implicit() || !t.eff.Features.Forward {
		return nil, notAllowed
	}
	seen := map[uuid.UUID]bool{}
	for _, x := range targets {
		if x == uuid.Nil || seen[x] {
			return nil, problem.Invalid("target_channel_ids", problem.SubInvalid, "distinct channel ids")
		}
		seen[x] = true
	}
	if len(targets) == 0 || len(targets) > t.eff.Features.ForwardTargetsMax() {
		return nil, problem.Invalid("target_channel_ids", problem.SubInvalid, "1..chat_forward_targets_max channels")
	}
	if t.e.Forwardable() != nil || (t.e.MsgKind == message.KindText || t.e.MsgKind == message.KindEmoji) && t.e.BodyDisplay == nil {
		return nil, notAllowed
	}
	// Every target first: same tenant (a channel of another tenant is unknown in the session tenant), an active member
	// allowed to send there (open channel, policy allows the kind). One refusal refuses the whole forward.
	for _, x := range targets {
		facts, err := h.Send.Facts.Channel(ctx, s.TenantID, x)
		if err != nil {
			return nil, temporarily()
		}
		if facts == nil || facts.Channel.TenantID != s.TenantID || !facts.Channel.Open() || facts.Channel.Type.Implicit() {
			return nil, notAllowed
		}
		m, ok := facts.Members.Active(s.UserID)
		if !ok {
			return nil, notAllowed
		}
		eff, _, err := h.effective(ctx, s, facts, m)
		if err != nil {
			var pe *problem.Error
			if errors.As(err, &pe) && pe.Code == problem.ChatNotAllowed {
				return nil, notAllowed
			}
			return nil, err
		}
		if !eff.Allows(policy.Kind(t.e.MsgKind)) {
			return nil, notAllowed
		}
	}
	nick := ""
	if c := h.card(t.facts, t.e.SenderID); c.Nickname != nil {
		nick = *c.Nickname
	}
	from := &message.ForwardedFrom{MessageID: t.e.MessageID, ChannelType: t.e.ChannelType, SenderNickname: nick}
	for _, x := range targets {
		req := message.Request{ChannelID: x, Kind: t.e.MsgKind, StickerID: t.e.StickerID, GifID: t.e.GifID, ForwardedFrom: from,
			ClientMessageID: uuid.NewSHA1(forwardNamespace, []byte(key+"|"+s.UserID.String()+"|"+id.String()+"|"+x.String()))}
		switch t.e.MsgKind {
		case message.KindText:
			req.Body = *t.e.BodyDisplay
		case message.KindEmoji:
			req.Emoji = *t.e.BodyDisplay
		case message.KindQuick:
			if t.e.QuickMessageID != nil {
				req.QuickMessageID = *t.e.QuickMessageID
			}
			req.Params = t.e.Params
		}
		res, err := h.Send.Send(ctx, Sender{TenantID: s.TenantID, UserID: s.UserID}, req)
		if err != nil {
			return out, err
		}
		out = append(out, res)
	}
	h.event("forward")
	return out, nil
}

// ToggleReaction is chat.reaction.toggle / toggleMessageReaction (F118-AC-16): on/off, at most chat_reactions_per_user
// per member per message, never on a recalled message; counts pushed to every member.
func (h Messaging) ToggleReaction(ctx context.Context, s Sender, id uuid.UUID, emoji string) (res port.ReactionResult, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.ToggleReaction")
	defer func() { done(err) }()
	t, err := h.load(ctx, s, id)
	if err != nil {
		return port.ReactionResult{}, err
	}
	if err := open(t.facts); err != nil {
		return port.ReactionResult{}, err
	}
	if !t.eff.Features.Reaction {
		return port.ReactionResult{}, disabled(policy.FeatureReaction)
	}
	if !message.ValidEmojiCode(emoji) || h.Emoji == nil || !h.Emoji.Reaction(emoji) {
		return port.ReactionResult{}, problem.Invalid("reaction", problem.SubInvalid, "not in the allowed reaction set")
	}
	if !t.e.Delivered() {
		return port.ReactionResult{}, problem.Invalid("message_id", problem.SubInvalid, "recalled or not delivered message")
	}
	tenant := t.e.TenantID
	res, err = h.Reactions.Toggle(ctx, id, s.UserID, emoji, t.eff.Features.ReactionsPerUser(), func(ctx context.Context) ([]port.Reaction, error) {
		var rows []read.ReactionRow
		err := h.Lookup.Read.InTenantRead(ctx, mustScope(tenant), func(ctx context.Context) error {
			var err error
			rows, err = h.Features.Reactions(ctx, tenant, []uuid.UUID{id})
			return err
		})
		if err != nil {
			return nil, port.ErrUnavailable
		}
		out := make([]port.Reaction, len(rows))
		for i, r := range rows {
			out[i] = port.Reaction{UserID: r.UserID, Emoji: r.Emoji}
		}
		return out, nil
	})
	if err != nil {
		return port.ReactionResult{}, temporarily()
	}
	if res.Limited {
		return res, problem.New(problem.ChatReactionLimit, "")
	}
	op := message.OpReactionUnset
	if res.Set {
		op = message.OpReactionSet
	}
	c := change(op, t, s.UserID, h.now())
	c.Emoji = emoji
	if err := h.publish(ctx, c); err != nil {
		if uerr := h.Reactions.Undo(context.WithoutCancel(ctx), id, s.UserID, emoji, res.Set); uerr != nil {
			slog.WarnContext(ctx, "chat: reaction not undone", "error", uerr)
		}
		return port.ReactionResult{}, temporarily()
	}
	if err := h.Fanout.Reactions(context.WithoutCancel(ctx), t.e.ChannelID, id, res.Counts, t.facts.Members.ActiveIDs()); err != nil {
		slog.WarnContext(ctx, "chat: reaction fan-out failed", "error", err)
	}
	return res, nil
}

// pinTarget checks a pin/unpin of message id in channel ch by the caller (F118-AC-20).
func (h Messaging) pinTarget(ctx context.Context, s Sender, ch, id uuid.UUID) (*target, error) {
	facts, m, err := h.member(ctx, s, ch)
	if err != nil {
		return nil, err
	}
	if facts.Channel.Type.Implicit() {
		return nil, disabled(policy.FeaturePin)
	}
	t, err := h.load(ctx, s, id)
	if err != nil {
		return nil, err
	}
	if t.e.ChannelID != ch {
		return nil, notFound()
	}
	if err := open(t.facts); err != nil {
		return nil, err
	}
	if !t.eff.Features.Pin {
		return nil, disabled(policy.FeaturePin)
	}
	// Rights by channel type: the leader in a party; any player in match/team; both members in a direct channel.
	if facts.Channel.Type == channel.Party && m.Role != channel.RoleLeader {
		return nil, problem.New(problem.PermissionDenied, "")
	}
	return t, nil
}

// Pin is pinMessage / chat.pin.add (F118-AC-20): at most chat_pins_max live pins (409 CHAT_PIN_LIMIT_REACHED).
func (h Messaging) Pin(ctx context.Context, s Sender, ch, id uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.PinMessage")
	defer func() { done(err) }()
	t, err := h.pinTarget(ctx, s, ch, id)
	if err != nil {
		return err
	}
	if !t.e.Delivered() {
		return problem.Invalid("message_id", problem.SubInvalid, "recalled or not delivered message")
	}
	added, full, pins, err := h.Pins.Add(ctx, ch, id, t.eff.Features.PinsMax(), h.pinLoader(t.e.TenantID, ch))
	switch {
	case err != nil:
		return temporarily()
	case full:
		return problem.New(problem.ChatPinLimitReached, "")
	case !added:
		return nil // already pinned
	}
	if err := h.publish(ctx, change(message.OpPin, t, s.UserID, h.now())); err != nil {
		if _, _, rerr := h.Pins.Remove(context.WithoutCancel(ctx), ch, id, h.pinLoader(t.e.TenantID, ch)); rerr != nil {
			slog.WarnContext(ctx, "chat: pin not undone", "error", rerr)
		}
		return temporarily()
	}
	if err := h.Fanout.Pins(context.WithoutCancel(ctx), ch, pins, h.card(t.facts, s.UserID), t.facts.Members.ActiveIDs()); err != nil {
		slog.WarnContext(ctx, "chat: pin fan-out failed", "error", err)
	}
	h.audit(ctx, s, AuditMessagePinned, id, ch)
	return nil
}

// Unpin is unpinMessage / chat.pin.remove (F118-AC-20): a soft delete of the pin.
func (h Messaging) Unpin(ctx context.Context, s Sender, ch, id uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.UnpinMessage")
	defer func() { done(err) }()
	t, err := h.pinTarget(ctx, s, ch, id)
	if err != nil {
		return err
	}
	removed, pins, err := h.Pins.Remove(ctx, ch, id, h.pinLoader(t.e.TenantID, ch))
	if err != nil {
		return temporarily()
	}
	if !removed {
		return nil
	}
	if err := h.publish(ctx, change(message.OpUnpin, t, s.UserID, h.now())); err != nil {
		if _, _, _, rerr := h.Pins.Add(context.WithoutCancel(ctx), ch, id, policy.CeilingPins, h.pinLoader(t.e.TenantID, ch)); rerr != nil {
			slog.WarnContext(ctx, "chat: unpin not undone", "error", rerr)
		}
		return temporarily()
	}
	if err := h.Fanout.Pins(context.WithoutCancel(ctx), ch, pins, h.card(t.facts, s.UserID), t.facts.Members.ActiveIDs()); err != nil {
		slog.WarnContext(ctx, "chat: pin fan-out failed", "error", err)
	}
	h.audit(ctx, s, AuditMessageUnpinned, id, ch)
	return nil
}

// Typing is chat.typing.start / chat.typing.stop (F118-AC-21): throttled once per 3 s, expires after 5 s, Valkey Pub/Sub
// only (no JetStream, no PostgreSQL), never to a member in a block relation, not in world/area; at most 3 nicknames.
func (h Messaging) TypingChanged(ctx context.Context, s Sender, ch uuid.UUID, start bool) (err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.Typing")
	defer func() { done(err) }()
	facts, m, err := h.member(ctx, s, ch)
	if err != nil {
		return err
	}
	if facts.Channel.Type.Implicit() {
		return disabled(policy.FeatureTyping)
	}
	if err := open(facts); err != nil {
		return err
	}
	eff, _, err := h.effective(ctx, s, facts, m)
	if err != nil {
		return err
	}
	if !eff.Features.Typing {
		return disabled(policy.FeatureTyping)
	}
	now := h.now()
	var changed bool
	var typing []uuid.UUID
	if start {
		changed, typing, err = h.Typing.Start(ctx, ch, s.UserID, now, TypingThrottle, TypingTTL)
	} else {
		changed, typing, err = h.Typing.Stop(ctx, ch, s.UserID, now, TypingTTL)
	}
	if err != nil {
		return temporarily()
	}
	if !changed {
		return nil
	}
	if h.Metrics != nil {
		h.Metrics.Typing()
	}
	// Each recipient sees the typing members it has no block relation with (and never itself).
	groups := map[string][]uuid.UUID{}
	lists := map[string][]read.ChatCard{}
	for _, r := range facts.Members.ActiveIDs() {
		var cards []read.ChatCard
		var key strings.Builder
		for _, u := range typing {
			if u == r || facts.Hidden(r, u) || facts.Blocked(r, u) {
				continue
			}
			if len(cards) < TypingShown {
				cards = append(cards, h.card(facts, u))
				key.WriteString(u.String())
			}
		}
		// Nothing for a recipient who sees no typist and must not hear about the one who changed (itself, a block or mute).
		if len(cards) == 0 && (r == s.UserID || facts.Blocked(r, s.UserID) || facts.Hidden(r, s.UserID)) {
			continue
		}
		k := key.String()
		groups[k] = append(groups[k], r)
		lists[k] = cards
	}
	bg := context.WithoutCancel(ctx)
	for k, rs := range groups {
		if err := h.Fanout.Typing(bg, ch, lists[k], rs); err != nil {
			slog.WarnContext(ctx, "chat: typing fan-out failed", "error", err)
		}
	}
	return nil
}
