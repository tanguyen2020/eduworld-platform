package cache

import (
	"context"
	"fmt"
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

// Get implements port.SnapshotCache (the TTL slides on every hit).
func (s SnapshotCache) Get(ctx context.Context, key string) ([]byte, error) {
	b, err := s.C.Do(ctx, s.C.B().Getex().Key(key).Ex(s.ttl()).Build()).AsBytes()
	if valkey.IsValkeyNil(err) {
		return nil, port.ErrCacheMiss
	}
	if err != nil {
		return nil, fmt.Errorf("cache: get snapshot: %w", err)
	}
	return b, nil
}

// Put implements port.SnapshotCache.
func (s SnapshotCache) Put(ctx context.Context, key string, b []byte) error {
	if err := s.C.Do(ctx, s.C.B().Set().Key(key).Value(string(b)).Ex(s.ttl()).Build()).Error(); err != nil {
		return fmt.Errorf("cache: put snapshot: %w", err)
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

// Current implements port.SnapshotCache.
func (s SnapshotCache) Current(ctx context.Context, tenant *uuid.UUID, itemType string, itemID uuid.UUID) (uuid.UUID, error) {
	v, err := s.C.Do(ctx, s.C.B().Get().Key(currentKey(tenant, itemType, itemID)).Build()).ToString()
	if valkey.IsValkeyNil(err) {
		return uuid.Nil, port.ErrCacheMiss
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("cache: get release pointer: %w", err)
	}
	return uuid.Parse(v)
}
