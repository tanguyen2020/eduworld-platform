package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
)

// PoolTTL is the lifetime of a cached question pool (QUESTION_BANK §6: 5 minutes).
const PoolTTL = 5 * time.Minute

// PoolTimeout bounds one cache call: a Valkey that is down or slow never blocks a delivery, it is a cache miss
// (ADR-0004, NFR-AVAIL-003, F031-AC-06) [Proposed].
const PoolTimeout = 200 * time.Millisecond

// PoolPrefix is the key prefix of the question pools: ew:learning:qb:pool:<tenant_id>:<filter_hash>.
const PoolPrefix = "ew:learning:qb:pool:"

// Pool implements read.PoolCache on Valkey (cache only: errors are logged and treated as misses, ADR-0004).
type Pool struct{ C valkey.Client }

var _ read.PoolCache = Pool{}

func poolKey(tenant uuid.UUID, key string) string { return PoolPrefix + tenant.String() + ":" + key }

// Get implements read.PoolCache.
func (p Pool) Get(ctx context.Context, tenant uuid.UUID, key string) ([]selection.Candidate, bool) {
	ctx, cancel := context.WithTimeout(ctx, PoolTimeout)
	defer cancel()
	b, err := p.C.Do(ctx, p.C.B().Get().Key(poolKey(tenant, key)).Build()).AsBytes()
	if err != nil {
		if !valkey.IsValkeyNil(err) {
			slog.WarnContext(ctx, "question pool cache read failed", "error", err)
		}
		return nil, false
	}
	var out []selection.Candidate
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, false
	}
	return out, true
}

// Put implements read.PoolCache.
func (p Pool) Put(ctx context.Context, tenant uuid.UUID, key string, pool []selection.Candidate) {
	ctx, cancel := context.WithTimeout(ctx, PoolTimeout)
	defer cancel()
	b, err := json.Marshal(pool)
	if err != nil {
		return
	}
	if err := p.C.Do(ctx, p.C.B().Set().Key(poolKey(tenant, key)).Value(string(b)).Ex(PoolTTL).Build()).Error(); err != nil {
		slog.WarnContext(ctx, "question pool cache write failed", "error", err)
	}
}

// InvalidateAll unlinks the question pools of every tenant (a platform question changed: every tenant it is
// assigned to may hold it).
func (i Invalidator) InvalidateAll(ctx context.Context) error {
	var cursor uint64
	for {
		e, err := i.C.Do(ctx, i.C.B().Scan().Cursor(cursor).Match(PoolPrefix+"*").Count(200).Build()).AsScanEntry()
		if err != nil {
			return fmt.Errorf("cache: scan pools: %w", err)
		}
		if len(e.Elements) > 0 {
			if err := i.C.Do(ctx, i.C.B().Unlink().Key(e.Elements...).Build()).Error(); err != nil {
				return fmt.Errorf("cache: unlink pools: %w", err)
			}
		}
		if e.Cursor == 0 {
			return nil
		}
		cursor = e.Cursor
	}
}
