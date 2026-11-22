package realtime

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// Server message types of the F118 features (AsyncAPI).
const (
	TypeMessageEdited   = "chat.message.edited"
	TypeMessageRecalled = "chat.message.recalled"
	TypeReactionUpdated = "chat.reaction.updated"
	TypePinUpdated      = "chat.pin.updated"
	TypeTypingUpdated   = "chat.typing.updated"
	TypeReceiptUpdated  = "chat.receipt.updated"
	TypeMentionCreated  = "chat.mention.created"
)

var _ port.FeatureFanout = ChatFanout{}

// toChannel publishes typ on ew:chat:fanout:<channel_id> to the recipients only (nothing when none).
func (f ChatFanout) toChannel(ctx context.Context, channel uuid.UUID, typ string, payload any, recipients []uuid.UUID) error {
	if len(recipients) == 0 {
		return nil
	}
	env, err := rt.NewMessage(typ, payload, f.now())
	if err != nil {
		return err
	}
	for start := 0; start < len(recipients); start += maxTargets {
		end := min(start+maxTargets, len(recipients))
		e := env
		e.Target = &rt.Target{UserIDs: append([]uuid.UUID{}, recipients[start:end]...)}
		if err := f.P.Publish(ctx, rt.ChatFanoutChannel(channel.String()), e); err != nil {
			return err
		}
	}
	return nil
}

// Edited implements port.FeatureFanout: chat.message.edited with the display body only (F118-AC-01).
func (f ChatFanout) Edited(ctx context.Context, e message.Envelope, mentions []read.ChatCard, recipients []uuid.UUID) error {
	body := ""
	if e.BodyDisplay != nil {
		body = *e.BodyDisplay
	}
	ms := make([]map[string]any, len(mentions))
	for i, c := range mentions {
		ms[i] = PublicPlayer(c)
	}
	edited := e.CreatedAt
	if e.EditedAt != nil {
		edited = *e.EditedAt
	}
	return f.toChannel(ctx, e.ChannelID, TypeMessageEdited, map[string]any{"message_id": e.MessageID, "channel_id": e.ChannelID,
		"seq": e.Seq, "body": body, "mentions": ms, "revision": e.Revision, "edited_at": edited.UnixMilli()}, recipients)
}

// Recalled implements port.FeatureFanout: chat.message.recalled without content (F118-AC-04).
func (f ChatFanout) Recalled(ctx context.Context, e message.Envelope, recipients []uuid.UUID) error {
	p := map[string]any{"message_id": e.MessageID, "channel_id": e.ChannelID, "seq": e.Seq}
	if e.RecalledAt != nil {
		p["recalled_at"] = e.RecalledAt.UnixMilli()
	}
	return f.toChannel(ctx, e.ChannelID, TypeMessageRecalled, p, recipients)
}

// Reactions implements port.FeatureFanout: chat.reaction.updated {reactions: emoji_code → count} (F118-AC-16).
func (f ChatFanout) Reactions(ctx context.Context, channel, msg uuid.UUID, counts map[string]int, recipients []uuid.UUID) error {
	if counts == nil {
		counts = map[string]int{}
	}
	return f.toChannel(ctx, channel, TypeReactionUpdated, map[string]any{"message_id": msg, "channel_id": channel, "reactions": counts},
		recipients)
}

// Pins implements port.FeatureFanout: chat.pin.updated (F118-AC-20).
func (f ChatFanout) Pins(ctx context.Context, channel uuid.UUID, ids []uuid.UUID, actor read.ChatCard, recipients []uuid.UUID) error {
	if ids == nil {
		ids = []uuid.UUID{}
	}
	p := map[string]any{"channel_id": channel, "pinned_message_ids": ids}
	if actor.UserID != uuid.Nil {
		p["actor"] = PublicPlayer(actor)
	}
	return f.toChannel(ctx, channel, TypePinUpdated, p, recipients)
}

// Typing implements port.FeatureFanout: chat.typing.updated (pub/sub only, never JetStream nor PostgreSQL).
func (f ChatFanout) Typing(ctx context.Context, channel uuid.UUID, typing []read.ChatCard, recipients []uuid.UUID) error {
	ps := make([]map[string]any, len(typing))
	for i, c := range typing {
		ps[i] = PublicPlayer(c)
	}
	return f.toChannel(ctx, channel, TypeTypingUpdated, map[string]any{"channel_id": channel, "typing": ps}, recipients)
}

// Receipt implements port.FeatureFanout: chat.receipt.updated of one member (pub/sub only).
func (f ChatFanout) Receipt(ctx context.Context, channel uuid.UUID, member read.ChatCard, delivered, readSeq int64, recipients []uuid.UUID) error {
	return f.toChannel(ctx, channel, TypeReceiptUpdated, map[string]any{"channel_id": channel, "member": PublicPlayer(member),
		"delivered_seq": delivered, "read_seq": readSeq}, recipients)
}

// Mention implements port.FeatureFanout: chat.mention.created to every connection of the mentioned student; the card
// carries the nickname of the game profile, never a login name (F118-AC-19, R16).
func (f ChatFanout) Mention(ctx context.Context, user, channel, msg uuid.UUID, seq int64, by read.ChatCard, preview string) error {
	p := map[string]any{"channel_id": channel, "message_id": msg, "seq": seq, "by": PublicPlayer(by)}
	if preview != "" {
		p["preview"] = preview
	}
	env, err := rt.NewMessage(TypeMentionCreated, p, f.now())
	if err != nil {
		return err
	}
	return f.P.ToUser(ctx, user, env)
}

// HeldEdit implements port.FeatureFanout: only the sender learns its edit waits for review (chat.message.held).
func (f ChatFanout) HeldEdit(ctx context.Context, user uuid.UUID, conn string, e message.Envelope) error {
	if conn == "" {
		return nil
	}
	return f.Held(ctx, user, conn, e)
}

// UnreadMentions pushes chat.unread.updated {counts, mentions} to every connection of a user (F118-AC-25).
func (f ChatFanout) UnreadMentions(ctx context.Context, user uuid.UUID, counts, mentions map[uuid.UUID]int64) error {
	c := make(map[string]int64, len(counts))
	for id, n := range counts {
		c[id.String()] = n
	}
	m := make(map[string]int64, len(mentions))
	for id, n := range mentions {
		m[id.String()] = n
	}
	env, err := rt.NewMessage(TypeUnreadUpdated, map[string]any{"counts": c, "mentions": m}, f.now())
	if err != nil {
		return err
	}
	return f.P.ToUser(ctx, user, env)
}
