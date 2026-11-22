package write

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/chatasset"
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

// ChangeOutcome is the result of applying one message change.
type ChangeOutcome int

// Outcomes of ChangeStore.Apply.
const (
	// ChangeApplied: the change was written (its outbox events follow).
	ChangeApplied ChangeOutcome = iota + 1
	// ChangeStale: older than the stored state (late or redelivered): nothing to do but the evidence rows.
	ChangeStale
	// ChangeMissing: the message is not persisted yet; chat-persister retries later (research R1).
	ChangeMissing
)

// ChangeStore is the write side of the message changes (F118): chat-persister applies edits, recalls, reactions and pins
// by revision in the transaction of the tenant (R25).
type ChangeStore interface {
	Apply(ctx context.Context, c message.Change) (ChangeOutcome, error)
}

// RoleRepository sets the role of a member (F118: the party leader may pin, CHAT_SYSTEM §7.8).
type RoleRepository interface {
	// SetRole sets the role of the active member (changed false when not an active member or already that role).
	SetRole(ctx context.Context, tenant, channel, user uuid.UUID, role string) (changed bool, err error)
}

// AssetRepository writes the platform sticker/GIF library (G tables, chat_platform, PlatformScope) for Game Studio.
type AssetRepository interface {
	// CreateItem inserts the item (sticker pack or GIF, status draft) and its version 1.
	CreateItem(ctx context.Context, v port.AssetVersion, pack *chatasset.Pack) error
	// CodeTaken reports a live pack with this code other than id.
	CodeTaken(ctx context.Context, code string, id uuid.UUID) (bool, error)
	// Version returns one version locked for update (nil when unknown).
	Version(ctx context.Context, t chatasset.Type, id uuid.UUID, version int) (*port.AssetVersion, error)
	// Latest returns the newest version number of the item (0: unknown).
	Latest(ctx context.Context, t chatasset.Type, id uuid.UUID) (int, error)
	// InsertVersion adds a new version (draft) of an item.
	InsertVersion(ctx context.Context, v port.AssetVersion) error
	// SaveVersion stores status, payload, title, reviewer and publish fields of a version.
	SaveVersion(ctx context.Context, v port.AssetVersion) error
	// Publish materializes v as the current published state of the item (pack + stickers, GIF + tags), archives the
	// version published before (previous, nil: none) and appends the release.
	Publish(ctx context.Context, v port.AssetVersion, previous *port.AssetVersion, r port.AssetRelease) error
	// Published returns the published version of the item (nil: none).
	Published(ctx context.Context, t chatasset.Type, id uuid.UUID) (*port.AssetVersion, error)
}
