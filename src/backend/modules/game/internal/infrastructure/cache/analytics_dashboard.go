package cache

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
)

// Dashboard keys of module game (DASHBOARD_DATA §9): data version of a tenant (no expiry), cached answers, in-flight
// counters of the concurrency limits.
const (
	dashVersionPrefix  = "ew:game:dash:ver:"
	dashInflightPrefix = "ew:game:dash:inflight:"
	dashInflightTTL    = 30 * time.Second
	dashTimeout        = 300 * time.Millisecond
)

// Dashboard implements port.DashboardCache on Valkey; every error is a cache miss (ADR-0004: answers stay correct).
type Dashboard struct{ C valkey.Client }

var _ port.DashboardCache = Dashboard{}

// Version implements port.DashboardCache (0 while never bumped).
func (d Dashboard) Version(ctx context.Context, tenant uuid.UUID) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, dashTimeout)
	defer cancel()
	v, err := d.C.Do(ctx, d.C.B().Get().Key(dashVersionPrefix+tenant.String()).Build()).AsInt64()
	if valkey.IsValkeyNil(err) {
		return 0, nil
	}
	return v, err
}

// Get implements port.DashboardCache.
func (d Dashboard) Get(ctx context.Context, key string) ([]byte, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, dashTimeout)
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

// Set implements port.DashboardCache.
func (d Dashboard) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, dashTimeout)
	defer cancel()
	return d.C.Do(ctx, d.C.B().Set().Key(key).Value(valkey.BinaryString(value)).Ex(ttl).Build()).Error()
}

// Bump implements port.DashboardCache: cached answers of the previous version are never read again (F080-AC-09).
func (d Dashboard) Bump(ctx context.Context, tenant uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, dashTimeout)
	defer cancel()
	return d.C.Do(ctx, d.C.B().Incr().Key(dashVersionPrefix+tenant.String()).Build()).Error()
}

// DashLimiter implements port.DashboardLimiter: Valkey counters per user and tenant (INCR + EXPIRE 30 s, DECR on release);
// per process when Valkey fails (DASHBOARD_DATA §9: tighter limits, never skipped).
type DashLimiter struct {
	C     valkey.Client
	mu    sync.Mutex
	local map[string]int
}

var _ port.DashboardLimiter = (*DashLimiter)(nil)

// NewDashLimiter returns a limiter on c (nil = per process only).
func NewDashLimiter(c valkey.Client) *DashLimiter { return &DashLimiter{C: c} }

type slotLimit struct {
	key   string
	limit int
}

// Acquire implements port.DashboardLimiter.
func (l *DashLimiter) Acquire(ctx context.Context, user, tenant uuid.UUID, perUser, perTenant int) (func(), bool) {
	slots := []slotLimit{{dashInflightPrefix + user.String(), perUser}}
	if tenant != uuid.Nil {
		slots = append(slots, slotLimit{dashInflightPrefix + "t:" + tenant.String(), perTenant})
	}
	if l.C == nil {
		return l.acquireLocal(slots)
	}
	cctx, cancel := context.WithTimeout(ctx, dashTimeout)
	defer cancel()
	var taken []string
	release := func() {
		rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), dashTimeout)
		defer cancel()
		for _, k := range taken {
			_ = l.C.Do(rctx, l.C.B().Decr().Key(k).Build()).Error()
		}
	}
	for _, s := range slots {
		if s.limit <= 0 {
			continue
		}
		n, err := l.C.Do(cctx, l.C.B().Incr().Key(s.key).Build()).AsInt64()
		if err != nil {
			release()
			slog.WarnContext(ctx, "game dashboard inflight counter failed: per-process limits apply", "error", err)
			return l.acquireLocal(slots)
		}
		taken = append(taken, s.key)
		_ = l.C.Do(cctx, l.C.B().Expire().Key(s.key).Seconds(int64(dashInflightTTL/time.Second)).Build()).Error()
		if n > int64(s.limit) {
			release()
			return func() {}, false
		}
	}
	return release, true
}

func (l *DashLimiter) acquireLocal(slots []slotLimit) (func(), bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.local == nil {
		l.local = map[string]int{}
	}
	for _, s := range slots {
		if s.limit > 0 && l.local[s.key] >= s.limit {
			return func() {}, false
		}
	}
	for _, s := range slots {
		l.local[s.key]++
	}
	return func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		for _, s := range slots {
			l.local[s.key]--
		}
	}, true
}
