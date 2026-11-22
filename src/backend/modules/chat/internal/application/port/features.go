package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
)

// ChangeStream publishes a message change to the durable stream CHAT_MESSAGES (subject ew.chat.<tenant>.evt.<channel>,
// Nats-Msg-Id = change_id) and returns only after the PubAck (R25, ADR-0035): nobody is told about an edit, recall,
// reaction or pin before it is durable.
type ChangeStream interface {
	PublishChange(ctx context.Context, c message.Change) error
}

// MessageIndex maps a message id to its tenant and channel while the message may still be changed (ew:chat:msgidx,
// TTL chat_recall_window + margin): the commands addressed by message_id find messages not persisted yet.
type MessageIndex interface {
	Put(ctx context.Context, msg, tenant, channel uuid.UUID) error
	Get(ctx context.Context, msg uuid.UUID) (tenant, channel uuid.UUID, ok bool, err error)
}

// RevisionGate serializes the changes of one message (ew:chat:msgrev:<id>, compare-and-set): Advance moves the revision
// from expected to expected+1 when the current one (at least floor, the newest known) equals expected; ok false returns
// the current revision. Restore moves it back when the publish of the change failed.
type RevisionGate interface {
	Advance(ctx context.Context, msg uuid.UUID, expected, floor int) (ok bool, current int, err error)
	Restore(ctx context.Context, msg uuid.UUID, from, to int) error
}

// Reaction is one live reaction of a member on a message.
type Reaction struct {
	UserID uuid.UUID
	Emoji  string
}

// ReactionResult is the state of the reactions of a message after a toggle.
type ReactionResult struct {
	Set     bool           // the reaction is on after the toggle
	Limited bool           // refused: the member already has limit reactions on the message
	Counts  map[string]int // emoji_code → count
	Mine    []string       // the reactions of the member
}

// ReactionState is the live reaction state of a message (ew:chat:rx:<message_id>, loaded from PostgreSQL when missing);
// the durable rows are chat.message_reactions written by chat-persister.
type ReactionState interface {
	Toggle(ctx context.Context, msg, user uuid.UUID, emoji string, limit int, load func(ctx context.Context) ([]Reaction, error)) (ReactionResult, error)
	// Undo reverts a toggle whose change could not be published.
	Undo(ctx context.Context, msg, user uuid.UUID, emoji string, set bool) error
}

// PinState is the live pin set of a channel (ew:chat:pins:<channel_id>, loaded from PostgreSQL when missing).
type PinState interface {
	// Add pins msg unless limit pins are live (full true); added false when already pinned.
	Add(ctx context.Context, channel, msg uuid.UUID, limit int, load func(ctx context.Context) ([]uuid.UUID, error)) (added, full bool, ids []uuid.UUID, err error)
	// Remove unpins msg; removed false when it was not pinned.
	Remove(ctx context.Context, channel, msg uuid.UUID, load func(ctx context.Context) ([]uuid.UUID, error)) (removed bool, ids []uuid.UUID, err error)
}

// TypingState keeps who is typing in a channel (ew:chat:typing:<channel_id>, never in PostgreSQL — CHAT_SYSTEM §7.10):
// Start is throttled to once per throttle per member (changed false: nothing to broadcast); entries expire after ttl.
type TypingState interface {
	Start(ctx context.Context, channel, user uuid.UUID, now time.Time, throttle, ttl time.Duration) (changed bool, typing []uuid.UUID, err error)
	Stop(ctx context.Context, channel, user uuid.UUID, now time.Time, ttl time.Duration) (changed bool, typing []uuid.UUID, err error)
}

// FeatureFanout pushes the realtime events of the message features through Valkey Pub/Sub after the JetStream ack
// (typing and receipts are pub/sub only): display content only, cards with nickname/level/avatar only (R16).
type FeatureFanout interface {
	Edited(ctx context.Context, e message.Envelope, mentions []read.ChatCard, recipients []uuid.UUID) error
	Recalled(ctx context.Context, e message.Envelope, recipients []uuid.UUID) error
	Reactions(ctx context.Context, channel, msg uuid.UUID, counts map[string]int, recipients []uuid.UUID) error
	Pins(ctx context.Context, channel uuid.UUID, ids []uuid.UUID, actor read.ChatCard, recipients []uuid.UUID) error
	Typing(ctx context.Context, channel uuid.UUID, typing []read.ChatCard, recipients []uuid.UUID) error
	Receipt(ctx context.Context, channel uuid.UUID, member read.ChatCard, delivered, readSeq int64, recipients []uuid.UUID) error
	Mention(ctx context.Context, user, channel, msg uuid.UUID, seq int64, by read.ChatCard, preview string) error
	HeldEdit(ctx context.Context, user uuid.UUID, conn string, e message.Envelope) error
}

// AssetChecker tells whether a sticker or GIF may be sent (F118-AC-12, AC-13): a sticker of a published pack assigned to
// the tenant and owned by the student when the pack is locked; a published GIF of the internal library.
type AssetChecker interface {
	Sticker(ctx context.Context, tenant, student, sticker uuid.UUID) (bool, error)
	Gif(ctx context.Context, tenant, gif uuid.UUID) (bool, error)
}

// EmojiSet is the platform set of allowed emoji / reaction codes (seed emoji_sets.json, CHAT_SYSTEM §7.5).
type EmojiSet interface {
	Emoji(code string) bool
	Reaction(code string) bool
}

// ExtraRate is the sticker/GIF ceiling chat_sticker_gif_rate (4 per minute per sender per channel, sticker and GIF
// counted together), checked on top of the channel rate.
type ExtraRate interface {
	AllowAsset(ctx context.Context, user, channel, msg uuid.UUID, perMinute int, now time.Time) (ok bool, retry time.Duration, err error)
}
