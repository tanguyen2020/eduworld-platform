package read

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// PinRow is one live pin of a channel (chat.pinned_messages).
type PinRow struct {
	MessageID uuid.UUID
	PinnedBy  uuid.UUID
	PinnedAt  time.Time
}

// ReactionRow is one live reaction (chat.message_reactions).
type ReactionRow struct {
	MessageID uuid.UUID
	UserID    uuid.UUID
	Emoji     string
}

// MemberCursor is the receipt cursor of one active member (chat.channel_members).
type MemberCursor struct {
	UserID    uuid.UUID
	Delivered int64
	Read      int64
	Mentions  int64
}

// Sticker is one sticker of a pack as a student sees it.
type Sticker struct {
	StickerID uuid.UUID
	AssetKey  string
	AltVI     string
	AltEN     *string
}

// StickerPackRow is a published pack assigned to the tenant, with the unlock state of the student.
type StickerPackRow struct {
	PackID   uuid.UUID
	Code     string
	NameVI   string
	NameEN   *string
	Locked   bool
	Unlocked bool
	HintVI   *string
	HintEN   *string
	Stickers []Sticker
}

// GifRow is a published GIF of the internal library.
type GifRow struct {
	GifID      uuid.UUID
	AssetKey   string
	PreviewKey string
	Width      int
	Height     int
	Tags       []string
	CreatedAt  time.Time
}

// FeatureReader reads the F118 read models of one tenant under entx.ReadSession: explicit SQL with tenant_id and
// deleted_at conditions on top of RLS (R27); the platform library tables are G tables (no tenant).
type FeatureReader interface {
	// MessageByID returns the stored message of tenant (display content only), nil when unknown.
	MessageByID(ctx context.Context, tenant, id uuid.UUID) (*StoredMessage, error)
	// MessagesByID returns the stored messages of one channel among ids (display content only).
	MessagesByID(ctx context.Context, tenant, channel uuid.UUID, gameplay bool, ids []uuid.UUID) (map[uuid.UUID]StoredMessage, error)
	// Pins returns the live pins of a channel, newest first.
	Pins(ctx context.Context, tenant, channel uuid.UUID) ([]PinRow, error)
	// Reactions returns the live reactions of messages of a channel.
	Reactions(ctx context.Context, tenant uuid.UUID, ids []uuid.UUID) ([]ReactionRow, error)
	// Cursors returns the receipt cursors of the active members of a channel.
	Cursors(ctx context.Context, tenant, channel uuid.UUID) ([]MemberCursor, error)
	// StickerPacks returns the published packs assigned to tenant with the unlock state of student.
	StickerPacks(ctx context.Context, tenant, student uuid.UUID) ([]StickerPackRow, error)
	// StickerAvailable reports a sticker of a published pack assigned to tenant and, when locked, unlocked by student.
	StickerAvailable(ctx context.Context, tenant, student, sticker uuid.UUID) (bool, error)
	// Gifs returns published GIFs (tag filter when set, newest first), after the keyset cursor (created_at, gif_id).
	Gifs(ctx context.Context, tag string, after *GifCursor, limit int) ([]GifRow, error)
	// GifAvailable reports a published GIF.
	GifAvailable(ctx context.Context, gif uuid.UUID) (bool, error)
}

// GifCursor is the keyset position of listGifs.
type GifCursor struct {
	CreatedAt time.Time `json:"t"`
	GifID     uuid.UUID `json:"id"`
}
