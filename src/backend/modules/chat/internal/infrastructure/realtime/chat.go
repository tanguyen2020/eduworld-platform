package realtime

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// Server message types of the AsyncAPI pushed by the chat core.
const (
	TypeMessageCreated = "chat.message.created"
	TypeMessageAcked   = "chat.message.acked"
	TypeMessageHeld    = "chat.message.held"
	TypeHistoryLoaded  = "chat.history.loaded"
	TypeSanction       = "chat.sanction.applied"
	TypeMessageRemoved = "chat.message.removed"
)

// maxTargets is the bound of target.user_ids of RtPubSubEnvelope.
const maxTargets = 500

// ChatFanout implements port.Fanout on Valkey Pub/Sub (the gateway assigns seq and delivers, F090): the members of a
// channel on ew:chat:fanout:<channel_id> restricted to the recipients (members, minus blocked pairs), one connection on
// ew:rt:user:<id> with target conn_id. Payloads carry ids, nickname and avatar only (R16) and display content only.
type ChatFanout struct {
	P   rt.Publisher
	Now func() time.Time
}

var _ port.Fanout = ChatFanout{}

func (f ChatFanout) now() time.Time {
	if f.Now != nil {
		return f.Now()
	}
	return time.Now()
}

// PublicPlayer renders a card as AsyncAPI PublicPlayer (nickname required, "" until projected).
func PublicPlayer(c read.ChatCard) map[string]any {
	m := map[string]any{"user_id": c.UserID, "nickname": "", "level": max(c.Level, 1)}
	if c.Nickname != nil {
		m["nickname"] = *c.Nickname
	}
	if len(c.Avatar) > 0 && string(c.Avatar) != "null" {
		m["avatar_config"] = c.Avatar
	}
	return m
}

// ChatMessage renders AsyncAPI ChatMessage: body_display for text, the code for emoji/quick, never the original body.
func ChatMessage(e message.Envelope, sender read.ChatCard, recalled bool) map[string]any {
	m := map[string]any{"message_id": e.MessageID, "channel_id": e.ChannelID, "seq": e.Seq, "sender": PublicPlayer(sender),
		"kind": string(e.MsgKind), "revision": max(e.Revision, 1), "moderation_state": string(e.Moderation.State),
		"created_at": e.CreatedAt.UnixMilli()}
	switch {
	case recalled:
		m["recalled"] = true
	case e.MsgKind == message.KindText && e.BodyDisplay != nil:
		m["body"] = *e.BodyDisplay
	case e.MsgKind == message.KindEmoji && e.BodyDisplay != nil:
		m["emoji"] = *e.BodyDisplay
	}
	if e.QuickMessageID != nil {
		m["quick_message_id"] = *e.QuickMessageID
	}
	if len(e.Params) > 0 {
		m["params"] = e.Params
	}
	if e.StickerID != nil {
		m["sticker_id"] = *e.StickerID
	}
	if e.GifID != nil {
		m["gif_id"] = *e.GifID
	}
	if e.ReplyToMessageID != nil {
		m["reply_to_message_id"] = *e.ReplyToMessageID
	}
	return m
}

// Created implements port.Fanout.
func (f ChatFanout) Created(ctx context.Context, e message.Envelope, sender read.ChatCard, recipients []uuid.UUID) error {
	env, err := rt.NewMessage(TypeMessageCreated, ChatMessage(e, sender, false), f.now())
	if err != nil {
		return err
	}
	if len(recipients) <= maxTargets {
		env.Target = &rt.Target{UserIDs: append([]uuid.UUID{}, recipients...)}
		if len(recipients) == 0 {
			return nil
		}
	}
	return f.P.Publish(ctx, rt.ChatFanoutChannel(e.ChannelID.String()), env)
}

func (f ChatFanout) toConn(ctx context.Context, user uuid.UUID, conn, typ string, payload any) error {
	env, err := rt.NewMessage(typ, payload, f.now())
	if err != nil {
		return err
	}
	env.Target = &rt.Target{ConnID: conn}
	return f.P.ToUser(ctx, user, env)
}

// Acked implements port.Fanout: "Sent" for the sending connection only.
func (f ChatFanout) Acked(ctx context.Context, user uuid.UUID, conn string, e message.Envelope) error {
	return f.toConn(ctx, user, conn, TypeMessageAcked, AckedPayload(e))
}

// AckedPayload is the payload of chat.message.acked (also the result of system.command.acked).
func AckedPayload(e message.Envelope) map[string]any {
	p := map[string]any{"message_id": e.MessageID, "channel_id": e.ChannelID, "seq": e.Seq, "moderation_state": string(e.Moderation.State)}
	if e.ClientMessageID != nil {
		p["client_message_id"] = *e.ClientMessageID
	}
	return p
}

// Held implements port.Fanout: only the sender learns the message waits for review (the client builds the notice).
func (f ChatFanout) Held(ctx context.Context, user uuid.UUID, conn string, e message.Envelope) error {
	return f.toConn(ctx, user, conn, TypeMessageHeld, map[string]any{"message_id": e.MessageID, "channel_id": e.ChannelID})
}

// History implements port.Fanout: chat.history.loaded for the connection that asked (F115-AC-18, AC-19).
func (f ChatFanout) History(ctx context.Context, user uuid.UUID, conn string, channel uuid.UUID, items []port.HistoryItem, hasMore bool) error {
	msgs := make([]map[string]any, len(items))
	for i, it := range items {
		msgs[i] = ChatMessage(it.Envelope, it.Sender, it.Recalled)
	}
	return f.toConn(ctx, user, conn, TypeHistoryLoaded, map[string]any{"channel_id": channel, "messages": msgs, "has_more": hasMore})
}

// SanctionApplied pushes chat.sanction.applied to every connection of the sanctioned student (F116: system mute; the
// UI locks the input). payload: sanction_type, until (epoch ms) — never the reason content.
func (f ChatFanout) SanctionApplied(ctx context.Context, user uuid.UUID, payload map[string]any) error {
	env, err := rt.NewMessage(TypeSanction, payload, f.now())
	if err != nil {
		return err
	}
	return f.P.ToUser(ctx, user, env)
}

// MessageRemoved pushes chat.message.removed to the members of the channel (F121-AC-12): the client shows a placeholder;
// ids only, never the content.
func (f ChatFanout) MessageRemoved(ctx context.Context, channel, msg uuid.UUID) error {
	env, err := rt.NewMessage(TypeMessageRemoved, map[string]any{"message_id": msg, "channel_id": channel, "reason": "moderation"}, f.now())
	if err != nil {
		return err
	}
	return f.P.Publish(ctx, rt.ChatFanoutChannel(channel.String()), env)
}

// TypeUnreadUpdated is chat.unread.updated (AsyncAPI ChatUnreadUpdated).
const TypeUnreadUpdated = "chat.unread.updated"

// Unread implements port.UnreadPush: chat.unread.updated {counts} to every connection of the user (all devices,
// F117-AC-13); counts only, no content.
func (f ChatFanout) Unread(ctx context.Context, user uuid.UUID, counts map[uuid.UUID]int64) error {
	c := make(map[string]int64, len(counts))
	for id, n := range counts {
		c[id.String()] = n
	}
	env, err := rt.NewMessage(TypeUnreadUpdated, map[string]any{"counts": c}, f.now())
	if err != nil {
		return err
	}
	return f.P.ToUser(ctx, user, env)
}
