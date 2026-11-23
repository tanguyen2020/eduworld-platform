// Package roomhost is the shared room host library of the server-authoritative rooms (REALTIME_ARCHITECTURE §5.1,
// CHALLENGE_BOSS_SYSTEM §4; F091 T002, F076 T032): a fixed-rate tick loop and a lease keeper (hub rooms of module
// competition), and Host — one owner node per room through a Valkey lease (ValkeyRegistry), failover from the
// checkpoint the module keeps in its database (CheckpointStore), timers on the server clock (pkg/clock) with a grace per
// room type, used by the solo challenges of module game (F076) and reused by boss (F103) and battle (F105). No business
// code: what a room does is the Handler of the module (R13); Valkey holds the registry and the lease only (ADR-0004).
package roomhost

import (
	"context"
	"time"
)

// Tick runs fn at hz until ctx ends, passing the tick time, and reports the duration of every tick to observe (may be
// nil). A tick slower than the period delays the next one (no catch-up burst).
func Tick(ctx context.Context, hz int, now func() time.Time, fn func(at time.Time), observe func(d time.Duration)) {
	if hz <= 0 {
		hz = 10
	}
	if now == nil {
		now = time.Now
	}
	t := time.NewTicker(time.Second / time.Duration(hz))
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			start := time.Now()
			fn(now())
			if observe != nil {
				observe(time.Since(start))
			}
		}
	}
}

// KeepLease calls renew every period until ctx ends; when renew reports the lease lost (false) or fails for longer
// than ttl, lost is called once and KeepLease returns. A transient error is retried at the next period: the lease only
// expires in Valkey after ttl, so a short outage does not move the room.
func KeepLease(ctx context.Context, period, ttl time.Duration, renew func(ctx context.Context) (bool, error), lost func()) {
	t := time.NewTicker(period)
	defer t.Stop()
	lastOK := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			rctx, cancel := context.WithTimeout(ctx, period)
			ok, err := renew(rctx)
			cancel()
			switch {
			case err == nil && ok:
				lastOK = time.Now()
			case err == nil && !ok, time.Since(lastOK) > ttl:
				if ctx.Err() == nil {
					lost()
				}
				return
			}
		}
	}
}
