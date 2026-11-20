package write

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
)

// ChannelRepository writes chat.channels and chat.channel_members in the transaction (internal commands OpenChannel,
// AddMember, RemoveMember — called by the consumers of F117/F120 and by test fixtures, never by a public API).
type ChannelRepository interface {
	// CreateChannel inserts c; an existing channel with the same id is left as is (created false).
	CreateChannel(ctx context.Context, c channel.Channel) (created bool, err error)
	// Channel returns the live channel of tenant, nil when unknown.
	Channel(ctx context.Context, tenant, id uuid.UUID) (*channel.Channel, error)
	// StudentSchool returns the school of an active projected student of tenant (ok false: unknown, cleaned up or not
	// active).
	StudentSchool(ctx context.Context, tenant, student uuid.UUID) (school uuid.UUID, ok bool, err error)
	// JoinMember makes m an active member (insert, or clear left_at of a member who left: joined_at = m.JoinedAt);
	// changed false when already active.
	JoinMember(ctx context.Context, m channel.Member) (changed bool, err error)
	// LeaveMember sets left_at of an active member; changed false when not an active member.
	LeaveMember(ctx context.Context, tenant, channel, user uuid.UUID, at time.Time) (changed bool, err error)
}

// PersistResult is the outcome of one persister batch of a tenant: the envelopes newly inserted (events are written for
// them), and those whose channel does not exist in the tenant of the envelope (dropped to the DLQ, never retried).
type PersistResult struct {
	Inserted []message.Envelope
	Orphans  []message.Envelope
}

// MessageStore is the write side of chat-persister: the only writer of chat.messages / chat.gameplay_messages (R25,
// F115-AC-21). Insert is idempotent per message_id (ON CONFLICT DO NOTHING) and raises channels.last_seq with GREATEST.
type MessageStore interface {
	Insert(ctx context.Context, tenant uuid.UUID, es []message.Envelope) (PersistResult, error)
}
