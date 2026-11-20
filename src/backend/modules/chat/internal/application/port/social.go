package port

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/social"
)

// CodeGenerator returns a new random friend code (crypto/rand, social.CodeAlphabet, social.CodeLength).
type CodeGenerator interface {
	NewCode() (string, error)
}

// PresenceStore keeps the last presence module chat received per student (Valkey ew:chat:social:presence:<id>, TTL
// presence_ttl, ADR-0004: ephemeral, never in PostgreSQL).
type PresenceStore interface {
	// Get returns the stored presence of each id (absent: none or expired).
	Get(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]social.Presence, error)
	// Put stores p for user unless the stored presence is newer (by Ts); returns the previous presence (nil when
	// none) and whether p was stored.
	Put(ctx context.Context, user uuid.UUID, p social.Presence) (*social.Presence, bool, error)
}

// FriendCache caches the friend ids of a student (Valkey ew:chat:social:friends:<id>, 10 min, invalidated when a
// relation changes).
type FriendCache interface {
	// Get returns the cached ids and whether the cache held them.
	Get(ctx context.Context, user uuid.UUID) ([]uuid.UUID, bool, error)
	Set(ctx context.Context, user uuid.UUID, ids []uuid.UUID) error
	Invalidate(ctx context.Context, users ...uuid.UUID) error
}

// RealtimePublisher sends presence.friend.updated to one student (ew:rt:user:<id>, src/backend/pkg/realtime).
type RealtimePublisher interface {
	FriendPresence(ctx context.Context, to, user uuid.UUID, p social.Presence) error
}

// Metrics of the social use cases (eduworld_social_*); labels never carry ids.
type Metrics interface {
	FriendRequest(result string)
	Block()
	PresenceFanout(n int)
}

// Results of eduworld_social_friend_requests_total.
const (
	ResultSent     = "sent"
	ResultAccepted = "accepted"
	ResultRejected = "rejected"
	ResultScope    = "scope_violation"
	ResultLimit    = "limit_reached"
	ResultRate     = "rate_limited"
)
