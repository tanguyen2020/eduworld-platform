package cache

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
)

// SnapshotTTL is content_snapshot_cache_ttl (CONFIG_DEFAULTS, owner CONTENT_MODEL §5.4: 7 days, sliding) [Proposed].
const SnapshotTTL = 7 * 24 * time.Hour

// CurrentPrefix is the prefix of the release pointers: ew:learning:current:<tenant|_platform>:<item_type>:<item_id>.
const CurrentPrefix = "ew:learning:current:"

// SnapshotCache implements port.SnapshotCache on Valkey (cache only, R9).
type SnapshotCache struct {
	C   valkey.Client
	TTL time.Duration
}

var _ port.SnapshotCache = SnapshotCache{}

func (s SnapshotCache) ttl() time.Duration {
	if s.TTL == 0 {
		return SnapshotTTL
	}
	return s.TTL
}

// Get implements port.SnapshotCache (the TTL slides on every hit). The call is bounded by PoolTimeout and any error is
// a miss: the caller reads PostgreSQL (ADR-0004, OQ F031 owner 2026-11-20).
func (s SnapshotCache) Get(ctx context.Context, key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, PoolTimeout)
	defer cancel()
	b, err := s.C.Do(ctx, s.C.B().Getex().Key(key).Ex(s.ttl()).Build()).AsBytes()
	if err != nil {
		if !valkey.IsValkeyNil(err) {
			slog.WarnContext(ctx, "content snapshot cache read failed", "error", err)
		}
		return nil, port.ErrCacheMiss
	}
	return b, nil
}

// Put implements port.SnapshotCache: bounded by PoolTimeout; a failed write is logged, never an error (cache only).
func (s SnapshotCache) Put(ctx context.Context, key string, b []byte) error {
	ctx, cancel := context.WithTimeout(ctx, PoolTimeout)
	defer cancel()
	if err := s.C.Do(ctx, s.C.B().Set().Key(key).Value(string(b)).Ex(s.ttl()).Build()).Error(); err != nil {
		slog.WarnContext(ctx, "content snapshot cache write failed", "error", err)
	}
	return nil
}

func currentKey(tenant *uuid.UUID, itemType string, itemID uuid.UUID) string {
	t := "_platform"
	if tenant != nil {
		t = tenant.String()
	}
	return CurrentPrefix + t + ":" + itemType + ":" + itemID.String()
}

// SetCurrent implements port.SnapshotCache.
func (s SnapshotCache) SetCurrent(ctx context.Context, tenant *uuid.UUID, itemType string, itemID uuid.UUID, versionID *uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, PoolTimeout)
	defer cancel()
	key := currentKey(tenant, itemType, itemID)
	var err error
	if versionID == nil {
		err = s.C.Do(ctx, s.C.B().Del().Key(key).Build()).Error()
	} else {
		err = s.C.Do(ctx, s.C.B().Set().Key(key).Value(versionID.String()).Build()).Error()
	}
	if err != nil {
		return fmt.Errorf("cache: set release pointer: %w", err)
	}
	return nil
}

// Current implements port.SnapshotCache: bounded by PoolTimeout, any error is a miss (the pointer is read from PostgreSQL).
func (s SnapshotCache) Current(ctx context.Context, tenant *uuid.UUID, itemType string, itemID uuid.UUID) (uuid.UUID, error) {
	ctx, cancel := context.WithTimeout(ctx, PoolTimeout)
	defer cancel()
	v, err := s.C.Do(ctx, s.C.B().Get().Key(currentKey(tenant, itemType, itemID)).Build()).ToString()
	if err != nil {
		if !valkey.IsValkeyNil(err) {
			slog.WarnContext(ctx, "release pointer cache read failed", "error", err)
		}
		return uuid.Nil, port.ErrCacheMiss
	}
	id, err := uuid.Parse(v)
	if err != nil {
		return uuid.Nil, port.ErrCacheMiss
	}
	return id, nil
}
