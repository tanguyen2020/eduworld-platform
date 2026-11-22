package write

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
)

// PartyMember is the last applied change of one student of a party (ref.parties.member_versions): In tells whether the
// student is a member after that change, V is the party state_version of the change.
type PartyMember struct {
	V  int64 `json:"v"`
	In bool  `json:"in"`
}

// PartyState is the projection ref.parties of one party (F117-AC-03: a change older than the one applied for a student
// is ignored; a disbanded party never gets members again).
type PartyState struct {
	PartyID   uuid.UUID
	TenantID  uuid.UUID
	Disbanded bool
	Members   map[uuid.UUID]PartyMember
	Version   int64
}

// RetentionSettings are the projected gameplay chat retention settings of a tenant: Tenant is the tenant override
// (nil: inherit), Platform the platform chat_retention value (nil: not projected yet).
type RetentionSettings struct {
	Tenant   *int
	Platform json.RawMessage
}

// GameplayRepository writes the gameplay chat lifecycle (consumer chat.gameplay_channels, F117 US1) in the transaction
// of the event: the party projection, the retention of the tenant and the closing of a channel.
type GameplayRepository interface {
	// Party returns the projected party of tenant, nil when unknown.
	Party(ctx context.Context, tenant, party uuid.UUID) (*PartyState, error)
	// SaveParty upserts the projection.
	SaveParty(ctx context.Context, p PartyState) error
	// Retention returns the projected retention settings of tenant.
	Retention(ctx context.Context, tenant uuid.UUID) (RetentionSettings, error)
	// CloseChannel stores status closed, closed_at and purge_after of c (only while open; changed false otherwise).
	CloseChannel(ctx context.Context, c channel.Channel) (changed bool, err error)
	// ActiveMembers returns the ids of the active members of a channel of tenant.
	ActiveMembers(ctx context.Context, tenant, channel uuid.UUID) ([]uuid.UUID, error)
	// TenantRetention stores the tenant override gameplay_chat_retention_days (nil: inherit) when version is newer;
	// ok false when the tenant is not projected yet.
	TenantRetention(ctx context.Context, tenant uuid.UUID, days *int, version int64) (ok bool, err error)
	// MarkRead raises last_read_seq of the active member to seq (never back) and returns the unread count
	// (last_seq − last_read_seq) of that channel; seq is capped at GREATEST(last_seq, known) — known is the newest acked seq
	// of the recent buffer (not persisted yet); ok false when user is not an active member.
	MarkRead(ctx context.Context, tenant, channel, user uuid.UUID, seq, known int64, at time.Time) (unread int64, ok bool, err error)
}
