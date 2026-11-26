// Package saga is the saga engine of module identity (F016 T006, shared with F045, ADR-0042, ADR-0074): a
// cross-module operation runs its steps in order (facade calls of other modules through ports), each step at
// most once successfully (stable Idempotency-Key per step), retries transient failures with backoff, and on a
// "blocked" answer or a permanent failure compensates the done steps in reverse order. The local identity
// change runs last (Finish), in one transaction, only when every step is done or had nothing to do.
package saga

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Step outcomes returned by a step (StudentLifecycleResult.status of the facades).
const (
	Done        = "done"
	Blocked     = "blocked"
	NothingToDo = "nothing_to_do"
)

// Run statuses.
const (
	StatusRunning      = "running"
	StatusCompensating = "compensating"
	StatusSucceeded    = "succeeded"
	StatusBlocked      = "blocked"
	StatusFailed       = "failed"
)

// Result of one step call.
type Result struct {
	Outcome  string // Done | Blocked | NothingToDo
	Response any    // stored in saga_steps.response (no PII)
}

// Step is one participant call and its compensation (nil when nothing to undo).
type Step struct {
	Service, Action string
	Do              func(ctx context.Context, run write.SagaRun) (Result, error)
	Undo            func(ctx context.Context, run write.SagaRun) error
}

// Definition describes a saga type.
type Definition struct {
	Type  string
	Steps []Step
	// Finish applies the change of module identity in one transaction (the step results are in run.Steps).
	Finish func(ctx context.Context, run write.SagaRun) error
}

// Outcome of Execute.
type Outcome struct {
	Status string // succeeded | blocked | failed | running (retry later)
	Run    write.SagaRun
}

// Metrics of the engine.
type Metrics interface {
	Finished(sagaType, status string)
	Stuck(n int)
}

// Engine runs sagas.
type Engine struct {
	UoW         write.UnitOfWork
	Repo        write.SagaRepository
	Metrics     Metrics
	MaxAttempts int           // per run before failing (default 8)
	Backoff     time.Duration // first retry delay, doubled per attempt (default 30 s, capped at 1 h)
	Clock       func() time.Time
}

func (e Engine) now() time.Time {
	if e.Clock != nil {
		return e.Clock().UTC()
	}
	return time.Now().UTC()
}

func (e Engine) inPlatform(ctx context.Context, fn func(ctx context.Context) error) error {
	sc, err := authz.SystemPlatformScope("saga")
	if err != nil {
		return err
	}
	return e.UoW.Do(tenancy.WithScope(ctx, sc), fn)
}

// StepKey is the Idempotency-Key of a step: a UUIDv7 with the start time of the saga and a hash of
// (saga, step), stable across retries (facades replay their stored result).
func StepKey(run write.SagaRun, step int) uuid.UUID {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", run.ID, step)))
	var u uuid.UUID
	ms := uint64(run.StartedAt.UnixMilli())
	binary.BigEndian.PutUint64(u[0:8], ms<<16)
	copy(u[6:], h[:10])
	u[6] = (u[6] & 0x0f) | 0x70 // version 7
	u[8] = (u[8] & 0x3f) | 0x80 // variant RFC 4122
	return u
}

// Start creates the run (or returns the existing one of the same type and idempotency key) and executes it.
func (e Engine) Start(ctx context.Context, def Definition, tenant *uuid.UUID, subject any, idemKey string, by *uuid.UUID) (out Outcome, err error) {
	raw, err := json.Marshal(subject)
	if err != nil {
		return out, err
	}
	steps := make([]write.SagaStep, len(def.Steps))
	for i, s := range def.Steps {
		steps[i] = write.SagaStep{No: i, Service: s.Service, Action: s.Action, Status: "pending"}
	}
	run := write.SagaRun{ID: uuid.Must(uuid.NewV7()), Type: def.Type, TenantID: tenant, Subject: raw, Status: StatusRunning,
		RequestedBy: by, IdempotencyKey: idemKey, StartedAt: e.now(), Steps: steps}
	var created bool
	if err := e.inPlatform(ctx, func(ctx context.Context) error {
		run, created, err = e.Repo.Create(ctx, run)
		return err
	}); err != nil {
		return out, err
	}
	if !created && run.Status != StatusRunning && run.Status != StatusCompensating {
		return Outcome{Status: run.Status, Run: run}, nil // finished before: idempotent
	}
	return e.Execute(ctx, def, run.ID)
}

// Execute advances a run: runs the pending steps, compensates on blocked/permanent failure, finishes or
// schedules a retry on a transient failure.
func (e Engine) Execute(ctx context.Context, def Definition, id uuid.UUID) (out Outcome, err error) {
	ctx, done := observability.UseCase(ctx, "identity.saga."+def.Type)
	defer func() { done(err) }()
	var run *write.SagaRun
	if err := e.inPlatform(ctx, func(ctx context.Context) error {
		run, err = e.Repo.Lock(ctx, id)
		return err
	}); err != nil {
		return out, err
	}
	if run == nil {
		return out, problem.New(problem.NotFound, "")
	}
	if run.Status == StatusSucceeded || run.Status == StatusBlocked || run.Status == StatusFailed {
		return Outcome{Status: run.Status, Run: *run}, nil
	}
	if run.Status == StatusRunning {
		for i := range run.Steps {
			st := &run.Steps[i]
			if st.Status == Done || st.Status == NothingToDo {
				continue
			}
			res, err := e.call(ctx, def.Steps[i], *run, i)
			st.Attempts++
			switch {
			case err != nil && permanent(err):
				st.Status, st.Error = "failed", trim(err.Error())
				if err := e.saveStep(ctx, run.ID, *st); err != nil {
					return out, err
				}
				return e.compensate(ctx, def, run, StatusFailed, err.Error())
			case err != nil:
				st.Error = trim(err.Error())
				if err := e.saveStep(ctx, run.ID, *st); err != nil {
					return out, err
				}
				return e.retryLater(ctx, run, err)
			}
			raw, _ := json.Marshal(res.Response)
			st.Status, st.Response, st.Error = res.Outcome, raw, ""
			if err := e.saveStep(ctx, run.ID, *st); err != nil {
				return out, err
			}
			if res.Outcome == Blocked {
				return e.compensate(ctx, def, run, StatusBlocked, def.Steps[i].Service+" blocked")
			}
		}
		if def.Finish != nil {
			if err := def.Finish(ctx, *run); err != nil {
				if permanent(err) {
					return e.compensate(ctx, def, run, StatusFailed, err.Error())
				}
				return e.retryLater(ctx, run, err)
			}
		}
		return e.finish(ctx, def, run, StatusSucceeded, "")
	}
	return e.compensate(ctx, def, run, StatusFailed, run.LastError) // resume an interrupted compensation
}

func (e Engine) call(ctx context.Context, s Step, run write.SagaRun, i int) (res Result, err error) {
	defer func() {
		if p := recover(); p != nil {
			observability.RecoveredPanic(ctx, "identity", observability.EntrypointJob, p) // metric + GlitchTip (F131-AC-17)
			err = fmt.Errorf("saga step %s.%s panicked: %v", s.Service, s.Action, p)
		}
	}()
	return s.Do(modcall.WithIdempotencyKey(ctx, StepKey(run, i).String()), run)
}

// permanent: a 4xx problem (validation, not found, conflict) is not retried; anything else is transient.
func permanent(err error) bool {
	var pe *problem.Error
	if !errors.As(err, &pe) {
		return false
	}
	s := problem.Status(pe.Code)
	return s >= 400 && s < 500 && s != 409 && s != 429
}

func trim(s string) string {
	if len(s) > 1000 {
		return s[:1000]
	}
	return s
}

func (e Engine) saveStep(ctx context.Context, id uuid.UUID, s write.SagaStep) error {
	return e.inPlatform(ctx, func(ctx context.Context) error { return e.Repo.SaveStep(ctx, id, s) })
}

func (e Engine) retryLater(ctx context.Context, run *write.SagaRun, cause error) (Outcome, error) {
	max := e.MaxAttempts
	if max <= 0 {
		max = 8
	}
	run.Attempts++
	run.LastError = trim(cause.Error())
	if run.Attempts >= max {
		slog.WarnContext(ctx, "saga gave up after retries", "saga_id", run.ID.String(), "type", run.Type)
		return Outcome{}, errors.Join(cause, e.saveRun(ctx, run, StatusFailed))
	}
	base := e.Backoff
	if base <= 0 {
		base = 30 * time.Second
	}
	delay := base << (run.Attempts - 1)
	if delay > time.Hour || delay <= 0 {
		delay = time.Hour
	}
	next := e.now().Add(delay)
	run.NextAttemptAt = &next
	if err := e.saveRun(ctx, run, run.Status); err != nil {
		return Outcome{}, err
	}
	return Outcome{Status: StatusRunning, Run: *run}, nil
}

// compensate undoes the done steps in reverse order; a failed undo is retried later (status compensating).
func (e Engine) compensate(ctx context.Context, def Definition, run *write.SagaRun, final, reason string) (Outcome, error) {
	if run.Status != StatusCompensating {
		run.LastError = trim(reason)
		if err := e.saveRun(ctx, run, StatusCompensating); err != nil {
			return Outcome{}, err
		}
	}
	for i := len(run.Steps) - 1; i >= 0; i-- {
		st := &run.Steps[i]
		if st.Status != Done || def.Steps[i].Undo == nil {
			continue
		}
		if err := def.Steps[i].Undo(modcall.WithIdempotencyKey(ctx, StepKey(*run, i+25).String()), *run); err != nil {
			return e.retryLater(ctx, run, fmt.Errorf("compensate %s.%s: %w", st.Service, st.Action, err))
		}
		st.Status = "compensated"
		if err := e.saveStep(ctx, run.ID, *st); err != nil {
			return Outcome{}, err
		}
	}
	if final == StatusFailed && run.LastError != "" && reason == "" {
		reason = run.LastError
	}
	return e.finish(ctx, def, run, final, reason)
}

func (e Engine) finish(ctx context.Context, def Definition, run *write.SagaRun, status, reason string) (Outcome, error) {
	if reason != "" {
		run.LastError = trim(reason)
	}
	if err := e.saveRun(ctx, run, status); err != nil {
		return Outcome{}, err
	}
	if e.Metrics != nil {
		e.Metrics.Finished(def.Type, status)
	}
	return Outcome{Status: status, Run: *run}, nil
}

func (e Engine) saveRun(ctx context.Context, run *write.SagaRun, status string) error {
	run.Status = status
	if status == StatusSucceeded || status == StatusBlocked || status == StatusFailed {
		t := e.now()
		run.FinishedAt, run.NextAttemptAt = &t, nil
	}
	return e.inPlatform(ctx, func(ctx context.Context) error { return e.Repo.SaveRun(ctx, run) })
}

// ResumeDue executes the runs whose retry is due with their definition (job saga-retry) and reports the
// number of runs older than stuckAfter still not finished (alert SagaStuck).
func (e Engine) ResumeDue(ctx context.Context, defs map[string]Definition, limit int, stuckAfter time.Duration) (int, error) {
	var ids []uuid.UUID
	var stuck int
	if err := e.inPlatform(ctx, func(ctx context.Context) error {
		var err error
		if ids, err = e.Repo.Due(ctx, e.now(), limit); err != nil {
			return err
		}
		stuck, err = e.Repo.Stuck(ctx, e.now().Add(-stuckAfter))
		return err
	}); err != nil {
		return 0, err
	}
	if e.Metrics != nil {
		e.Metrics.Stuck(stuck)
	}
	n := 0
	for _, id := range ids {
		var run *write.SagaRun
		if err := e.inPlatform(ctx, func(ctx context.Context) error {
			var err error
			run, err = e.Repo.Lock(ctx, id)
			return err
		}); err != nil || run == nil {
			continue
		}
		def, ok := defs[run.Type]
		if !ok {
			continue
		}
		if _, err := e.Execute(ctx, def, id); err != nil {
			slog.WarnContext(ctx, "saga resume failed", "saga_id", id.String(), "error", err)
			continue
		}
		n++
	}
	return n, nil
}

// Enqueue creates the run (or returns the existing one of the same type and key) without executing it: the
// job running ResumeDue executes it (operations answering 202, F045). Call it inside a UnitOfWork with a
// platform scope so the run is created in the caller's transaction; created is false for a replay.
func (e Engine) Enqueue(ctx context.Context, def Definition, tenant *uuid.UUID, subject any, idemKey string, by *uuid.UUID) (run write.SagaRun, created bool, err error) {
	raw, err := json.Marshal(subject)
	if err != nil {
		return run, false, err
	}
	steps := make([]write.SagaStep, len(def.Steps))
	for i, s := range def.Steps {
		steps[i] = write.SagaStep{No: i, Service: s.Service, Action: s.Action, Status: "pending"}
	}
	now := e.now()
	run = write.SagaRun{ID: uuid.Must(uuid.NewV7()), Type: def.Type, TenantID: tenant, Subject: raw, Status: StatusRunning,
		RequestedBy: by, IdempotencyKey: idemKey, StartedAt: now, Steps: steps}
	if run, created, err = e.Repo.Create(ctx, run); err != nil || !created {
		return run, created, err
	}
	run.NextAttemptAt = &now
	return run, true, e.Repo.SaveRun(ctx, &run)
}

// Reopen puts a blocked or failed run back to running with its unfinished steps pending, due now (a System
// Admin re-runs it after fixing the cause, F045-AC-29); done steps are never run again. Call it inside a
// UnitOfWork with a platform scope; a running or succeeded run is returned unchanged.
func (e Engine) Reopen(ctx context.Context, id uuid.UUID) (*write.SagaRun, error) {
	run, err := e.Repo.Lock(ctx, id)
	if err != nil || run == nil || (run.Status != StatusBlocked && run.Status != StatusFailed) {
		return run, err
	}
	for i := range run.Steps {
		st := &run.Steps[i]
		if st.Status == Done || st.Status == NothingToDo {
			continue
		}
		st.Status = "pending"
		if err := e.Repo.SaveStep(ctx, run.ID, *st); err != nil {
			return nil, err
		}
	}
	now := e.now()
	run.Status, run.Attempts, run.FinishedAt, run.NextAttemptAt = StatusRunning, 0, nil, &now
	return run, e.Repo.SaveRun(ctx, run)
}
