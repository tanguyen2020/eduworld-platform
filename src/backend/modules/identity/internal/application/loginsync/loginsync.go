// Package loginsync keeps the SIS data of a student fresh at login (SIS_INTEGRATION §4.5, ADR-0061),
// used by the game login of F013: branch A (complete and fresh: no SIS call), B (no effective school/grade —
// ADR-0095: blocking lookup through SisLookupPort with timeout, per-connection concurrency limit and
// circuit breaker, then the SIS pipeline) and C (complete but stale: one deduplicated login_refresh job).
// There is never a synchronous retry on the login path.
package loginsync

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Branches and results of eduworld_sis_login_sync_total.
const (
	BranchFresh      = "fresh"
	BranchBlocking   = "blocking"
	BranchBackground = "background"

	ResultOK               = "ok"
	ResultTimeout          = "timeout"
	ResultError            = "error"
	ResultMissingPlacement = "missing_placement" // no effective school/grade after the lookup (ADR-0095 D6)
	ResultThrottled        = "throttled"
)

// Defaults [Proposed] until the tenant/platform settings are read by the caller (CONFIG_DEFAULTS).
const (
	DefaultRefreshAfter = 24 * time.Hour          // sis_login_refresh_after_hours
	DefaultTimeout      = 3000 * time.Millisecond // sis_login_sync_timeout_ms
	DefaultConcurrency  = 20                      // sis_login_sync_concurrency
)

// Applier applies a looked-up Student record (command.SisRun).
type Applier interface {
	ApplyLookup(ctx context.Context, tenantID, connectionID, schoolID uuid.UUID, record map[string]any) (command.RunImportResult, error)
}

// Result is the branch taken and its outcome.
type Result struct {
	Branch          string
	Outcome         string
	SisConnectionID *uuid.UUID // recorded in login_events (ADR-0046 §8)
}

// LoginSync implements the branches. The caller sets the tenant scope of the student on ctx.
type LoginSync struct {
	UoW          write.UnitOfWork
	State        write.LoginStateReader
	Lookup       port.SisLookupPort
	Apply        Applier
	Jobs         port.SisJobQueue
	Metrics      port.LoginSyncMetrics
	Guard        *Guard
	RefreshAfter time.Duration
	Timeout      time.Duration
	Clock        func() time.Time
}

func (s LoginSync) now() time.Time {
	if s.Clock != nil {
		return s.Clock()
	}
	return time.Now()
}

func (s LoginSync) load(ctx context.Context, f func(ctx context.Context) (*write.LoginState, error)) (st *write.LoginState, err error) {
	err = s.UoW.Do(ctx, func(ctx context.Context) error {
		st, err = f(ctx)
		return err
	})
	return st, err
}

// Ensure runs login sync for a student. A student without an active SIS link is not synced (fresh, ok).
// Branch B without an effective school/grade afterwards returns NO_ACTIVE_ENROLLMENT (code kept, ADR-0095).
func (s LoginSync) Ensure(ctx context.Context, userID uuid.UUID) (res Result, err error) {
	ctx, done := observability.UseCase(ctx, "identity.loginsync.Ensure")
	defer func() { done(err) }()
	start := s.now()
	st, err := s.load(ctx, func(ctx context.Context) (*write.LoginState, error) { return s.State.LoginState(ctx, userID) })
	if err != nil {
		return res, err
	}
	if st == nil {
		return Result{Branch: BranchFresh, Outcome: ResultOK}, nil
	}
	conn := st.ConnectionID
	res.SisConnectionID = &conn
	refresh := s.RefreshAfter
	if refresh <= 0 {
		refresh = DefaultRefreshAfter
	}
	if st.Placed {
		res.Branch, res.Outcome = BranchFresh, ResultOK
		if st.LastSyncedAt == nil || start.Sub(*st.LastSyncedAt) > refresh {
			// C: continue at once; the refresh runs in the background (F011-AC-21).
			res.Branch = BranchBackground
			if err := s.Jobs.EnqueueLoginRefresh(ctx, port.LoginRefreshJob{TenantID: st.TenantID, ConnectionID: conn, StudentExternalID: st.ExternalID}); err != nil {
				res.Outcome = ResultError // the login still continues with the current data
			}
		}
		s.observe(res, start)
		return res, nil
	}
	// B: blocking lookup (F011-AC-19, AC-20, AC-22).
	res.Branch, res.Outcome = BranchBlocking, s.lookupAndApply(ctx, st)
	after, err := s.load(ctx, func(ctx context.Context) (*write.LoginState, error) { return s.State.LoginState(ctx, userID) })
	if err != nil {
		return res, err
	}
	s.observe(res, start)
	if after != nil && after.Placed {
		return res, nil
	}
	if res.Outcome == ResultOK {
		res.Outcome = ResultMissingPlacement
	}
	msg := ""
	if res.Outcome != ResultMissingPlacement {
		msg = "retry in a few minutes"
	}
	return res, problem.New(problem.NoActiveEnrollment, msg)
}

// Refresh handles a login_refresh job (the caller sets the tenant scope of the job).
func (s LoginSync) Refresh(ctx context.Context, job port.LoginRefreshJob) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.loginsync.Refresh")
	defer func() { done(err) }()
	start := s.now()
	st, err := s.load(ctx, func(ctx context.Context) (*write.LoginState, error) {
		return s.State.LoginStateBySource(ctx, job.ConnectionID, job.StudentExternalID)
	})
	if err != nil || st == nil {
		return err
	}
	res := Result{Branch: BranchBackground, Outcome: s.lookupAndApply(ctx, st)}
	s.observe(res, start)
	return nil // never retried: the next login queues a new job
}

// lookupAndApply calls the SIS of the student's connection and applies the record.
func (s LoginSync) lookupAndApply(ctx context.Context, st *write.LoginState) string {
	if !st.StudentLookup || st.SchoolExternalID == "" {
		return ResultMissingPlacement // e.g. csv_import: no student_lookup, use the last synced data (F011-AC-20)
	}
	release, ok := s.Guard.Acquire(st.ConnectionID, s.now())
	if !ok {
		return ResultThrottled
	}
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	lctx, cancel := context.WithTimeout(ctx, timeout)
	rec, err := s.Lookup.Lookup(lctx, st.ConnectionID, st.SchoolExternalID, st.Username)
	cancel()
	switch {
	case errors.Is(err, port.ErrSisLookupNotFound):
		release(true, s.now())
		return ResultMissingPlacement
	case errors.Is(err, port.ErrSisLookupTimeout) || errors.Is(err, context.DeadlineExceeded):
		release(false, s.now())
		return ResultTimeout
	case err != nil:
		release(false, s.now())
		return ResultError
	}
	release(true, s.now())
	if _, err := s.Apply.ApplyLookup(ctx, st.TenantID, st.ConnectionID, st.SchoolID, rec); err != nil {
		return ResultError
	}
	return ResultOK
}

func (s LoginSync) observe(r Result, start time.Time) {
	if s.Metrics != nil {
		s.Metrics.Observe(r.Branch, r.Outcome, s.now().Sub(start))
	}
}

// Guard limits concurrent lookups per connection and opens a circuit after consecutive failures
// (bulkhead + breaker of SIS_INTEGRATION §4.5): a refused call is throttled, never queued.
type Guard struct {
	mu        sync.Mutex
	limit     int
	threshold int
	openFor   time.Duration
	conns     map[uuid.UUID]*connGuard
}

type connGuard struct {
	inFlight  int
	failures  int
	openUntil time.Time
}

// NewGuard returns a guard; threshold consecutive failures open the circuit for openFor [Proposed: 5, 30 s].
func NewGuard(concurrency, threshold int, openFor time.Duration) *Guard {
	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}
	return &Guard{limit: concurrency, threshold: threshold, openFor: openFor, conns: map[uuid.UUID]*connGuard{}}
}

// Acquire takes a slot of the connection; release reports whether the SIS answered.
func (g *Guard) Acquire(conn uuid.UUID, now time.Time) (release func(ok bool, at time.Time), allowed bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	c := g.conns[conn]
	if c == nil {
		c = &connGuard{}
		g.conns[conn] = c
	}
	if now.Before(c.openUntil) || c.inFlight >= g.limit {
		return nil, false
	}
	c.inFlight++
	return func(ok bool, at time.Time) {
		g.mu.Lock()
		defer g.mu.Unlock()
		c.inFlight--
		if ok {
			c.failures = 0
			return
		}
		c.failures++
		if g.threshold > 0 && c.failures >= g.threshold {
			c.openUntil, c.failures = at.Add(g.openFor), 0
		}
	}, true
}
