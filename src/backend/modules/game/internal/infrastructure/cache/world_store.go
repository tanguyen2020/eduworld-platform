package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
)

// worldTimeout bounds every Valkey call of the world adapters: a slow Valkey never blocks a request for long.
const worldTimeout = 2 * time.Second

// PresenceKey is the Valkey key of the presence of a student (F071 data-model, ADR-0004: temporary data).
func PresenceKey(student uuid.UUID) string { return "ew:game:presence:" + student.String() }

// PresenceStore implements port.PresenceStore on Valkey.
type PresenceStore struct{ C valkey.Client }

var _ port.PresenceStore = PresenceStore{}

// Set implements port.PresenceStore.
func (s PresenceStore) Set(ctx context.Context, student uuid.UUID, p port.Presence, ttl time.Duration) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, worldTimeout)
	defer cancel()
	if err := s.C.Do(ctx, s.C.B().Set().Key(PresenceKey(student)).Value(string(b)).Ex(ttl).Build()).Error(); err != nil {
		return fmt.Errorf("presence: set: %w", err)
	}
	return nil
}

// Get implements port.PresenceStore (nil when expired or lost).
func (s PresenceStore) Get(ctx context.Context, student uuid.UUID) (*port.Presence, error) {
	ctx, cancel := context.WithTimeout(ctx, worldTimeout)
	defer cancel()
	b, err := s.C.Do(ctx, s.C.B().Get().Key(PresenceKey(student)).Build()).AsBytes()
	if valkey.IsValkeyNil(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("presence: get: %w", err)
	}
	var p port.Presence
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, nil //nolint:nilerr // a malformed presence is no presence: the client enters the area again
	}
	return &p, nil
}

// Touch implements port.PresenceStore (the heartbeat of the game session extends the presence).
func (s PresenceStore) Touch(ctx context.Context, student uuid.UUID, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, worldTimeout)
	defer cancel()
	if err := s.C.Do(ctx, s.C.B().Expire().Key(PresenceKey(student)).Seconds(int64(ttl/time.Second)).Build()).Error(); err != nil {
		return fmt.Errorf("presence: touch: %w", err)
	}
	return nil
}

// RateLimiter implements port.RateLimiter with a fixed-window counter ew:game:rl:<key>.
type RateLimiter struct{ C valkey.Client }

var _ port.RateLimiter = RateLimiter{}

// Allow implements port.RateLimiter.
func (r RateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, worldTimeout)
	defer cancel()
	k := "ew:game:rl:" + key
	res := r.C.DoMulti(ctx, r.C.B().Incr().Key(k).Build(), r.C.B().Expire().Key(k).Seconds(int64(window/time.Second)).Nx().Build(),
		r.C.B().Pttl().Key(k).Build())
	n, err := res[0].AsInt64()
	if err != nil {
		return false, 0, fmt.Errorf("rate limit: %w", err)
	}
	ttl, _ := res[2].AsInt64()
	wait := time.Duration(ttl) * time.Millisecond
	if wait <= 0 {
		wait = window
	}
	return n <= int64(limit), wait, nil
}

// GameSnapshotCache implements port.SnapshotCache: ew:game:cv:<type>:<id>:<version> (immutable, no expiry needed but
// bounded by content_snapshot_cache_ttl) and ew:game:current:<type>:<id>. Cache only: errors are dropped by the caller.
type GameSnapshotCache struct {
	C   valkey.Client
	TTL time.Duration
}

var _ port.SnapshotCache = GameSnapshotCache{}

func (s GameSnapshotCache) ttl() time.Duration {
	if s.TTL <= 0 {
		return 7 * 24 * time.Hour // content_snapshot_cache_ttl (CONFIG_DEFAULTS, owner CONTENT_MODEL §5.4)
	}
	return s.TTL
}

// PutSnapshot implements port.SnapshotCache.
func (s GameSnapshotCache) PutSnapshot(ctx context.Context, key string, payload []byte) error {
	ctx, cancel := context.WithTimeout(ctx, worldTimeout)
	defer cancel()
	return s.C.Do(ctx, s.C.B().Set().Key(key).Value(string(payload)).Ex(s.ttl()).Build()).Error()
}

// SetCurrent implements port.SnapshotCache.
func (s GameSnapshotCache) SetCurrent(ctx context.Context, itemType, itemID string, versionID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, worldTimeout)
	defer cancel()
	return s.C.Do(ctx, s.C.B().Set().Key("ew:game:current:"+itemType+":"+itemID).Value(versionID.String()).Build()).Error()
}
