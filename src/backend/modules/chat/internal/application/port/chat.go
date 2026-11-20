package port

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
)

// ValidationInput is what the synchronous moderation pipeline checks for one new message (MODERATION §3): the text
// (body or emoji code), the quick message code and the context rules (channel, strictness). No grade (R15).
type ValidationInput struct {
	TenantID       uuid.UUID
	SchoolID       *uuid.UUID
	ChannelID      uuid.UUID
	ChannelType    string
	SenderID       uuid.UUID
	Kind           message.Kind
	Text           string
	QuickMessageID string
	Strictness     string
}

// Verdict is the pipeline result: the decision, the display text (masked when mask) and metadata without content
// (no matched term or position is ever returned to a client, ADR-0030).
type Verdict struct {
	Decision       message.Decision
	Display        string
	CategoryCodes  []string
	StepsSkipped   []string
	CatalogVersion *int64
}

// ContentValidator is the synchronous moderation pipeline every message goes through before a seq is allocated
// (NFR-SAFE-001, R15, F115-AC-06). F116 implements it (pipeline.Validator); module chat refuses to start without a real
// one outside EW_ENV=test.
type ContentValidator interface {
	Validate(ctx context.Context, in ValidationInput) (Verdict, error)
}

// TestDouble marks a ContentValidator that is a labelled test double (R17): never accepted outside EW_ENV=test.
type TestDouble interface {
	ContentValidator
	TestDouble() bool
}

// ErrUnavailable is returned by an infrastructure port of the send path that cannot answer (NATS, Valkey, PostgreSQL
// without a cached value): the send fails with CHAT_TEMPORARILY_UNAVAILABLE and nothing is delivered (F115-AC-03).
var ErrUnavailable = errors.New("chat: dependency unavailable")

// SeqAllocator allocates the next seq of a channel in Valkey (seq_next.lua, VALKEY_DESIGN §4.9). floor is called only
// when the counter is missing (first message, Valkey lost the key): GREATEST(channels.last_seq, max seq of the recent
// buffer) (F115-AC-20).
type SeqAllocator interface {
	Next(ctx context.Context, channel uuid.UUID, floor func(ctx context.Context) (int64, error)) (int64, error)
}

// ChatStream publishes an envelope to the durable stream CHAT_MESSAGES (subject ew.chat.<tenant>.msg.<channel>,
// Nats-Msg-Id = message_id) and returns only after the PubAck; a failure or no ack within the timeout is ErrUnavailable.
type ChatStream interface {
	Publish(ctx context.Context, e message.Envelope) error
}

// HistoryItem is one message pushed with chat.history.loaded: the envelope (display content only), the sender card and
// the recall state.
type HistoryItem struct {
	Envelope message.Envelope
	Sender   read.ChatCard
	Recalled bool
}

// Fanout pushes realtime events through Valkey Pub/Sub after the JetStream ack (R25) and renders the AsyncAPI payloads:
// chat.message.created to the recipients of a channel (ew:chat:fanout:<channel_id>), chat.message.acked,
// chat.message.held and chat.history.loaded to one connection of a user (ew:rt:user:<id>, target conn_id).
type Fanout interface {
	Created(ctx context.Context, e message.Envelope, sender read.ChatCard, recipients []uuid.UUID) error
	Acked(ctx context.Context, user uuid.UUID, conn string, e message.Envelope) error
	Held(ctx context.Context, user uuid.UUID, conn string, e message.Envelope) error
	History(ctx context.Context, user uuid.UUID, conn string, channel uuid.UUID, items []HistoryItem, hasMore bool) error
}

// RecentBuffer is ew:chat:recent:<channel_id>: the newest acked envelopes, read by history and sync while
// chat-persister catches up (ADR-0035 §6).
type RecentBuffer interface {
	Push(ctx context.Context, e message.Envelope) error
	List(ctx context.Context, channel uuid.UUID) ([]message.Envelope, error)
}

// RateLimiter is the sliding window of chat sends (VALKEY_DESIGN §4.5): perChannel messages per minute in one channel
// and perUser across every channel; ok false returns when the next send is possible.
type RateLimiter interface {
	Allow(ctx context.Context, user, channel, msg uuid.UUID, perChannel, perUser int, now time.Time) (ok bool, retry time.Duration, err error)
}

// SendRecord is the idempotency record of one client_message_id (ew:chat:cmid:<sender>:<client_message_id>): pending
// before the ack (with the envelope once the seq is allocated, so a retry republishes the same message_id and seq), done
// after it.
type SendRecord struct {
	Done     bool              `json:"done"`
	Envelope *message.Envelope `json:"envelope,omitempty"`
}

// SendDedupe keeps the SendRecords (SET NX, TTL 10 min).
type SendDedupe interface {
	// Reserve creates a pending record; when one exists it is returned (created false).
	Reserve(ctx context.Context, sender, cmid uuid.UUID) (prev SendRecord, created bool, err error)
	Save(ctx context.Context, sender, cmid uuid.UUID, r SendRecord) error
	Release(ctx context.Context, sender, cmid uuid.UUID) error
}

// Cache is the read-through cache of the facts of the send path (channel and members, effective policy, curfew):
// a value older than fresh is reloaded; when the loader fails the stale value is served, so sending keeps working while
// PostgreSQL is down (F115-AC-16). Keys are logical ("chan:<id>"); the adapter adds the module prefix.
type Cache interface {
	Get(ctx context.Context, key string, fresh time.Duration, load func(ctx context.Context) ([]byte, error)) ([]byte, error)
	Invalidate(ctx context.Context, keys ...string) error
}

// ChatMetrics are the chat metrics (no user or content label, R14).
type ChatMetrics interface {
	Sent(channelType, kind, decision string)
	Rejected(code string)
	SendLatency(d time.Duration)
	PublishError()
}
