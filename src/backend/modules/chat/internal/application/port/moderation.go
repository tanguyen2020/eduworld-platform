package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
)

// SpamWindow is the Valkey state of the spam step (MODERATION §3.4; VALKEY_DESIGN ew:chat:spam:*): fingerprints only,
// never the content (R14).
type SpamWindow interface {
	// Record counts the message: identical fingerprints of the sender in this channel within SpamRepeatWindow and
	// distinct channels with this fingerprint within SpamChannelWindow (the current message included).
	Record(ctx context.Context, user, channel uuid.UUID, fingerprint string, now time.Time) (moderation.SpamCounts, error)
	// Block counts one blocked message of the sender and returns the blocks within SpamViolationWindow.
	Block(ctx context.Context, user uuid.UUID, now time.Time) (int, error)
}

// Sanction is an active restriction of a sender (chat.sanctions).
type Sanction struct {
	ID        uuid.UUID  `json:"sanction_id"`
	Type      string     `json:"type"`
	Scope     string     `json:"scope"`
	ChannelID *uuid.UUID `json:"channel_id,omitempty"`
	EndsAt    *time.Time `json:"ends_at,omitempty"`
}

// Applies reports a sanction that refuses a message of channel at now (mute; scope all or this channel).
func (s Sanction) Applies(channel uuid.UUID, now time.Time) bool {
	if s.EndsAt != nil && !now.Before(*s.EndsAt) {
		return false
	}
	if s.Type != "mute" {
		return false
	}
	return s.Scope == "all" || (s.ChannelID != nil && *s.ChannelID == channel)
}

// SanctionCache is ew:chat:sanction:<user_id> (TTL = ends_at): the active mutes of a user, rebuilt from PostgreSQL.
type SanctionCache interface {
	// Get returns the cached list (ok false: not cached).
	Get(ctx context.Context, user uuid.UUID) (list []Sanction, ok bool, err error)
	Set(ctx context.Context, user uuid.UUID, list []Sanction, ttl time.Duration) error
	Invalidate(ctx context.Context, user uuid.UUID) error
}

// DecisionRecord is one row of the decision log chat.moderation_decisions: metadata only, no content (F116-AC-09, R14).
type DecisionRecord struct {
	DecisionID             uuid.UUID
	TenantID               uuid.UUID
	ChannelID              uuid.UUID
	MessageID              *uuid.UUID
	SenderID               uuid.UUID
	Decision               string
	CategoryCodes          []string
	StepsRun               []string
	StepsSkipped           []string
	CatalogVersionPlatform *int64
	CatalogVersionTenant   *int64
	SwitchSnapshotID       *uuid.UUID
	LatencyMS              int
	CreatedAt              time.Time
}

// DecisionLog writes the decisions of blocked messages (no envelope exists for them, research R2): buffered, batched,
// best effort with a metric when dropped. Decisions of stored messages are written by chat-persister.
type DecisionLog interface {
	Record(ctx context.Context, d DecisionRecord)
}

// ModerationMetrics are the metrics of the pipeline (labels without user or content, R14).
type ModerationMetrics interface {
	FilterLatency(d time.Duration)
	Match(category, action string)
	Unchecked(tenant uuid.UUID)
	RE2Timeouts(n int)
	DecisionDropped()
}
