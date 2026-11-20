// Package roomhost holds the building blocks of a room host (REALTIME_ARCHITECTURE §5.1, F091 T002): a fixed-rate tick
// loop with its duration budget and a lease keeper that renews the ownership of a room and reports its loss. The room
// state, the lease keys and what a tick does belong to the module that hosts the room (competition: hub instance in
// F091; lobby, battle, co-op, boss later). No I/O here besides calling the functions it is given.
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
