// Package cache holds the Valkey adapters of module learning. Valkey is only a cache (ADR-0004): losing an
// entry never loses data, so failures here are reported but never fail a committed use case.
package cache

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
)

// TenantPrefixes are the key prefixes of per-tenant read models of module learning (ew:learning:*, plan F020):
// the question pools built from assigned platform content (F022) change when an assignment changes.
var TenantPrefixes = []string{"ew:learning:qb:pool:"}

// Invalidator implements write.CacheInvalidator: it unlinks every key ew:learning:qb:pool:<tenant_id>:*.
type Invalidator struct{ C valkey.Client }

var _ write.CacheInvalidator = Invalidator{}

// InvalidateTenant implements write.CacheInvalidator (SCAN + UNLINK, never KEYS).
func (i Invalidator) InvalidateTenant(ctx context.Context, tenant uuid.UUID) error {
	for _, p := range TenantPrefixes {
		pattern := p + tenant.String() + ":*"
		var cursor uint64
		for {
			e, err := i.C.Do(ctx, i.C.B().Scan().Cursor(cursor).Match(pattern).Count(200).Build()).AsScanEntry()
			if err != nil {
				return fmt.Errorf("cache: scan %s: %w", pattern, err)
			}
			if len(e.Elements) > 0 {
				if err := i.C.Do(ctx, i.C.B().Unlink().Key(e.Elements...).Build()).Error(); err != nil {
					return fmt.Errorf("cache: unlink %s: %w", pattern, err)
				}
			}
			if e.Cursor == 0 {
				break
			}
			cursor = e.Cursor
		}
	}
	return nil
}
