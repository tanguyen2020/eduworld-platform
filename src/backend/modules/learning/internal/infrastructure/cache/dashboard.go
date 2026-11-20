package cache

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
)

// Dashboard keys of module learning (DASHBOARD_DATA §9): the data version of a tenant (no expiry), the cached answers of a
// version and the in-flight counters of the concurrency limits.
const (
	DashVersionPrefix  = "ew:learning:dash:ver:"
	DashInflightPrefix = "ew:learning:dash:inflight:"
	// dashInflightTTL bounds a counter left by a crashed query (DASHBOARD_DATA §9: 30 s).
	dashInflightTTL = 30 * time.Second
)

// Dashboard implements query.DashboardCache and command.DashboardVersions on Valkey. Every call is bounded by PoolTimeout;
// an error is a cache miss (ADR-0004: Valkey down → answers come from the replica).
type Dashboard struct{ C valkey.Client }

var (
	_ query.DashboardCache      = Dashboard{}
	_ command.DashboardVersions = Dashboard{}
)

// Version implements query.DashboardCache (0 while the tenant was never bumped).
func (d Dashboard) Version(ctx context.Context, tenantID uuid.UUID) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, PoolTimeout)
	defer cancel()
	v, err := d.C.Do(ctx, d.C.B().Get().Key(DashVersionPrefix+tenantID.String()).Build()).AsInt64()
	if valkey.IsValkeyNil(err) {
		return 0, nil
	}
	return v, err
}

// Get implements query.DashboardCache.
func (d Dashboard) Get(ctx context.Context, key string) ([]byte, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, PoolTimeout)
	defer cancel()
	b, err := d.C.Do(ctx, d.C.B().Get().Key(key).Build()).AsBytes()
	if valkey.IsValkeyNil(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

// Set implements query.DashboardCache.
func (d Dashboard) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, PoolTimeout)
	defer cancel()
	return d.C.Do(ctx, d.C.B().Set().Key(key).Value(valkey.BinaryString(value)).Ex(ttl).Build()).Error()
}

// Bump implements command.DashboardVersions: the cached answers of the previous version are never read again (F043-AC-20).
func (d Dashboard) Bump(ctx context.Context, tenantID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, PoolTimeout)
	defer cancel()
	return d.C.Do(ctx, d.C.B().Incr().Key(DashVersionPrefix+tenantID.String()).Build()).Error()
}

// Limiter implements query.DashboardLimiter: counters ew:learning:dash:inflight:<user_id> and
// ew:learning:dash:inflight:t:<tenant_id> on Valkey (INCR + EXPIRE 30 s, DECR on release). When Valkey fails the limits are
// kept per process (DASHBOARD_DATA §9: tighter limits without Valkey), never skipped.
type Limiter struct {
	C     valkey.Client
	local localLimiter
}

var _ query.DashboardLimiter = (*Limiter)(nil)

// NewLimiter returns a limiter on c (nil = per process only).
func NewLimiter(c valkey.Client) *Limiter { return &Limiter{C: c} }

// Acquire implements query.DashboardLimiter.
func (l *Limiter) Acquire(ctx context.Context, userID, tenantID uuid.UUID, perUser, perTenant int) (func(), bool) {
	keys := []slot{{DashInflightPrefix + userID.String(), perUser}}
	if tenantID != uuid.Nil {
		keys = append(keys, struct {
			key   string
			limit int
		}{DashInflightPrefix + "t:" + tenantID.String(), perTenant})
	}
	if l.C == nil {
		return l.local.acquire(keys[0].key, keys[0].limit, tenantKey(keys))
	}
	cctx, cancel := context.WithTimeout(ctx, PoolTimeout)
	defer cancel()
	taken := []string{}
	release := func() {
		rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), PoolTimeout)
		defer cancel()
		for _, k := range taken {
			_ = l.C.Do(rctx, l.C.B().Decr().Key(k).Build()).Error()
		}
	}
	for _, k := range keys {
		if k.limit <= 0 {
			continue
		}
		n, err := l.C.Do(cctx, l.C.B().Incr().Key(k.key).Build()).AsInt64()
		if err != nil {
			release()
			slog.WarnContext(ctx, "dashboard inflight counter failed: per-process limits apply", "error", err)
			return l.local.acquire(keys[0].key, keys[0].limit, tenantKey(keys))
		}
		taken = append(taken, k.key)
		_ = l.C.Do(cctx, l.C.B().Expire().Key(k.key).Seconds(int64(dashInflightTTL/time.Second)).Build()).Error()
		if n > int64(k.limit) {
			release()
			return func() {}, false
		}
	}
	return release, true
}

func tenantKey(keys []slot) *slot {
	if len(keys) < 2 {
		return nil
	}
	return &keys[1]
}

// slot is one counter and its limit.
type slot struct {
	key   string
	limit int
}

// localLimiter keeps the limits in process memory when Valkey is unavailable.
type localLimiter struct {
	mu sync.Mutex
	n  map[string]int
}

func (l *localLimiter) acquire(user string, perUser int, tenant *slot) (func(), bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.n == nil {
		l.n = map[string]int{}
	}
	if perUser > 0 && l.n[user] >= perUser {
		return func() {}, false
	}
	if tenant != nil && tenant.limit > 0 && l.n[tenant.key] >= tenant.limit {
		return func() {}, false
	}
	l.n[user]++
	if tenant != nil {
		l.n[tenant.key]++
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			defer l.mu.Unlock()
			l.n[user]--
			if tenant != nil {
				l.n[tenant.key]--
			}
		})
	}, true
}
