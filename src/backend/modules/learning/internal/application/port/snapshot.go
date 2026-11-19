package port

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// ErrCacheMiss is a key absent from the snapshot cache.
var ErrCacheMiss = errors.New("port: cache miss")

// SnapshotCache keeps published snapshots and release pointers in Valkey (F026 US3, VALKEY_DESIGN): a cache only (R9),
// every error falls back to PostgreSQL.
type SnapshotCache interface {
	// Get returns a snapshot (ErrCacheMiss when absent).
	Get(ctx context.Context, key string) ([]byte, error)
	Put(ctx context.Context, key string, b []byte) error
	// SetCurrent sets ew:learning:current:<tenant|_platform>:<item_type>:<item_id> to versionID (nil deletes the key).
	SetCurrent(ctx context.Context, tenant *uuid.UUID, itemType string, itemID uuid.UUID, versionID *uuid.UUID) error
	// Current reads a release pointer (ErrCacheMiss when absent).
	Current(ctx context.Context, tenant *uuid.UUID, itemType string, itemID uuid.UUID) (uuid.UUID, error)
}
