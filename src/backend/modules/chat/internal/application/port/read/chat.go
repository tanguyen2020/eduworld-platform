package read

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/policy"
)

// ChatCard is the public card of a member (ref.player_profiles): nickname, level and avatar only — never a real name,
// school or class (R16, F115-AC-14).
type ChatCard struct {
	UserID   uuid.UUID       `json:"user_id"`
	Nickname *string         `json:"nickname,omitempty"`
	Level    int             `json:"level"`
	Avatar   json.RawMessage `json:"avatar,omitempty"`
}

// Block is one live block (blocker → blocked) between two members of a channel: messages are hidden both ways.
type Block struct {
	Blocker uuid.UUID `json:"blocker"`
	Blocked uuid.UUID `json:"blocked"`
}

// ChannelFacts is a channel with its members (active and left), the cards of the members and the live blocks between
// them: everything the send path and the history need about a channel, cached as one value.
type ChannelFacts struct {
	Channel channel.Channel        `json:"channel"`
	Members channel.Members        `json:"members"`
	Cards   map[uuid.UUID]ChatCard `json:"cards"`
	Blocks  []Block                `json:"blocks,omitempty"`
}

// Blocked reports a live block between a and b in either direction.
func (f ChannelFacts) Blocked(a, b uuid.UUID) bool {
	for _, x := range f.Blocks {
		if (x.Blocker == a && x.Blocked == b) || (x.Blocker == b && x.Blocked == a) {
			return true
		}
	}
	return false
}

// CurfewWindow is a {start, end} window of the identity projections ("HH:MM").
type CurfewWindow struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// MyChannel is one channel of listMyChannels with the cursor of the caller.
type MyChannel struct {
	Channel   channel.Channel
	SchoolID  *uuid.UUID // school of the membership (policy level)
	MyReadSeq int64
	JoinedAt  time.Time
}

// MessageQuery selects stored messages of one channel visible to Reader, by seq (exclusive bounds), from From
// (joined_at of the reader) on: newest first when Before is set or no bound is given, oldest first with After.
type MessageQuery struct {
	Tenant   uuid.UUID
	Channel  uuid.UUID
	Gameplay bool
	Reader   uuid.UUID
	From     time.Time
	Before   *int64
	After    *int64
	Limit    int
}

// StoredMessage is a persisted message as an envelope (body_display only; the original body is never read for a user,
// F115-AC-18) plus recall state.
type StoredMessage struct {
	Envelope message.Envelope
	Recalled bool
}

// ChatReader reads the chat facts and read models of one tenant under entx.ReadSession: explicit SQL with tenant_id and
// deleted_at conditions on top of RLS (R27).
type ChatReader interface {
	// ChannelFacts returns the channel of tenant with its members, cards and blocks; nil when unknown or deleted.
	ChannelFacts(ctx context.Context, tenant, channel uuid.UUID) (*ChannelFacts, error)
	// PolicyLevels returns the live documents of the levels platform, tenant, school (when school is set) and class.
	PolicyLevels(ctx context.Context, tenant uuid.UUID, school, class *uuid.UUID) (policy.Levels, error)
	// CurfewWindows returns platform_curfew projected from identity (nil: not projected) and the curfew_window of the
	// school (nil: none).
	CurfewWindows(ctx context.Context, tenant uuid.UUID, school *uuid.UUID) (platform, schoolWindow *CurfewWindow, err error)
	// LastSeq returns channels.last_seq (0 when unknown).
	LastSeq(ctx context.Context, tenant, channel uuid.UUID) (int64, error)
	// MyChannels lists the open or closed channels user is an active member of, newest activity first, at most limit.
	MyChannels(ctx context.Context, tenant, user uuid.UUID, limit int) ([]MyChannel, error)
	// Messages returns at most q.Limit stored messages of q (held ones only for their sender, blocked senders excluded
	// by the caller).
	Messages(ctx context.Context, q MessageQuery) ([]StoredMessage, error)
}
