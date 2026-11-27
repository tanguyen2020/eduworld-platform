package read

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
)

// SearchQuery is one searchMessages page (F120-AC-22, AC-23): tsquery of the words (unaccented in the database), the
// caller and its tenant only, optional channel/sender/time filters, keyset after (created_at, message_id) descending.
type SearchQuery struct {
	Tenant   uuid.UUID
	Reader   uuid.UUID
	TSQuery  string
	Channel  *uuid.UUID
	Sender   *uuid.UUID
	From, To *time.Time
	AfterAt  *time.Time
	AfterID  uuid.UUID
	Limit    int
}

// SearchHit is one matching message with its channel type and sender card.
type SearchHit struct {
	Message     StoredMessage
	ChannelType string
	Sender      ChatCard
	// Highlight and Ranges are set by the query (plain display text + matches of the search words, delta F112-8).
	Highlight string
	Ranges    []message.Range
}

// SearchReader reads the full-text matches of the persistent channels the reader is an active member of (from its
// joined_at), on body_display only (never the original of a masked message), without recalled, removed, held, blocked
// or anonymised messages, never World/Area and never another tenant (explicit tenant + RLS, R27).
type SearchReader interface {
	Search(ctx context.Context, q SearchQuery) ([]SearchHit, error)
}
