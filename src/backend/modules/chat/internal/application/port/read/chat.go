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

// Mute is one live mute (muter → muted) between two members: only the messages of muted are hidden from muter.
type Mute struct {
	Muter uuid.UUID `json:"muter"`
	Muted uuid.UUID `json:"muted"`
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
	Mutes   []Mute                 `json:"mutes,omitempty"`
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

// Hidden reports that viewer must not see the messages of sender in this channel (F117-AC-12): a block in either direction
// or a mute of sender by viewer. Nobody leaves the channel for it.
func (f ChannelFacts) Hidden(viewer, sender uuid.UUID) bool {
	if viewer == sender {
		return false
	}
	if f.Blocked(viewer, sender) {
		return true
	}
	for _, m := range f.Mutes {
		if m.Muter == viewer && m.Muted == sender {
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
	// PolicyLevels returns the live documents of the levels platform, tenant and school (when school is set).
	PolicyLevels(ctx context.Context, tenant uuid.UUID, school *uuid.UUID) (policy.Levels, error)
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

// ImplicitReader reads what the implicit channels (world, area — F120-AC-12, AC-13) need: the projected school of a
// student, the tenant type and the world_scope level, and the public card of a sender (no member rows).
type ImplicitReader interface {
	// StudentSchool returns the school of an active projected student of tenant (nil: unknown, cleaned up, not active).
	StudentSchool(ctx context.Context, tenant, student uuid.UUID) (*uuid.UUID, error)
	// Card returns the public card of a student (nil when no profile).
	Card(ctx context.Context, tenant, student uuid.UUID) (*ChatCard, error)
	// Relations returns the live blocks (either direction) and mutes (both directions) of a student in tenant: what the
	// World/Area fan-out and history filter need without a member list (F120 T061).
	Relations(ctx context.Context, tenant, student uuid.UUID) (UserRelations, error)
}

// UserRelations are the live block and mute relations of one student (social.blocks, social.mutes of its tenant).
type UserRelations struct {
	Blocked []uuid.UUID `json:"blocked,omitempty"`  // a block in either direction
	Muting  []uuid.UUID `json:"muting,omitempty"`   // the students this student muted
	MutedBy []uuid.UUID `json:"muted_by,omitempty"` // the students who muted this student
}

// HiddenFrom are the students who must not receive a message of this student (a block either way, or they muted it).
func (r UserRelations) HiddenFrom() []uuid.UUID {
	seen := make(map[uuid.UUID]bool, len(r.Blocked)+len(r.MutedBy))
	out := make([]uuid.UUID, 0, len(r.Blocked)+len(r.MutedBy))
	for _, ids := range [][]uuid.UUID{r.Blocked, r.MutedBy} {
		for _, id := range ids {
			if !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out
}

// HidesSender reports that this student must not see a message of sender (a block either way, or it muted sender).
func (r UserRelations) HidesSender(sender uuid.UUID) bool {
	for _, ids := range [][]uuid.UUID{r.Blocked, r.Muting} {
		for _, id := range ids {
			if id == sender {
				return true
			}
		}
	}
	return false
}
