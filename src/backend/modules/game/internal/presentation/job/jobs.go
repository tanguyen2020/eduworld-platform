// Package job holds the periodic jobs of module game run by the worker profile jobs (F014): session-timeout
// (game_session_heartbeat_timeout, scan every 15 s) and curfew-session-closer (every minute, ADR-0078). Each job is
// one RunOnce use case; Every schedules it. Several worker instances may run them: each session is ended in its own
// transaction after a FOR UPDATE re-check, so a session is never ended twice.
package job

import (
	"context"
	"log/slog"
	"time"
)

// Names of the jobs (logs, metrics, SystemPlatformScope actor).
const (
	SessionTimeout      = "session-timeout"
	CurfewSessionCloser = "curfew-session-closer"
)

// Intervals [Proposed]: the curfew closer ends sessions at most one minute after the start of the curfew
// (F014-AC-26); the timeout scan comes from EW_GAME_SESSION_TIMEOUT_SCAN.
const (
	CurfewCloserInterval = time.Minute
	Batch                = 500
)

// RunOnce is one execution of a job; it returns the number of sessions ended.
type RunOnce func(ctx context.Context) (int, error)

// Runner is the use case behind both jobs (command.Sessions).
type Runner interface {
	TimeoutStaleSessions(ctx context.Context, batch int) (int, error)
	CloseSessionsAtCurfew(ctx context.Context, batch int) (int, error)
}

// SessionTimeoutJob is one run of session-timeout.
func SessionTimeoutJob(r Runner) RunOnce {
	return func(ctx context.Context) (int, error) { return r.TimeoutStaleSessions(ctx, Batch) }
}

// CurfewSessionCloserJob is one run of curfew-session-closer.
func CurfewSessionCloserJob(r Runner) RunOnce {
	return func(ctx context.Context) (int, error) { return r.CloseSessionsAtCurfew(ctx, Batch) }
}

// Every runs job at start and then every interval until ctx ends; a failed run is logged and retried at the next
// tick (never in a tight loop).
func Every(ctx context.Context, name string, interval time.Duration, job RunOnce) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if n, err := job(ctx); err != nil {
			slog.WarnContext(ctx, "job failed", "job", name, "error", err)
		} else if n > 0 {
			slog.InfoContext(ctx, "job done", "job", name, "count", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
