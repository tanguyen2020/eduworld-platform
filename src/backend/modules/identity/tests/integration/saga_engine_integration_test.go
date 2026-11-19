//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/saga"
	pw "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// participant is a TEST DOUBLE of a module facade in a saga: it records calls and Idempotency-Keys and answers
// with the programmed outcome or error.
type participant struct {
	calls, undos int
	keys         []string
	outcome      string
	err          error
}

func (p *participant) step(service string) saga.Step {
	return saga.Step{Service: service, Action: "cleanupStudentData",
		Do: func(ctx context.Context, _ write.SagaRun) (saga.Result, error) {
			p.calls++
			k, _ := modcall.IdempotencyKeyFrom(ctx)
			p.keys = append(p.keys, k)
			if p.err != nil {
				return saga.Result{}, p.err
			}
			return saga.Result{Outcome: p.outcome, Response: map[string]any{"status": p.outcome}}, nil
		},
		Undo: func(context.Context, write.SagaRun) error { p.undos++; return nil }}
}

// F016 T006 (F016-AC-13..AC-15 engine part): steps in order, finish once, compensation on blocked and on a
// permanent failure, retry with backoff on a transient failure with the same Idempotency-Key, idempotent start.
func TestSagaEngine(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	now := time.Now().UTC()
	eng := saga.Engine{UoW: e.uow, Repo: pw.SagaRepo{}, Backoff: time.Minute, Clock: func() time.Time { return now }}
	run := func(game, learning *participant, finished *int, key string) saga.Outcome {
		def := saga.Definition{Type: "test_cleanup", Steps: []saga.Step{game.step("game"), learning.step("learning")},
			Finish: func(context.Context, write.SagaRun) error { *finished++; return nil }}
		out, err := eng.Start(ctx, def, nil, map[string]any{"student_id": uuid.NewString()}, key, nil)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}

	// Success, then an idempotent restart with the same key.
	g, l, fin := &participant{outcome: saga.Done}, &participant{outcome: saga.NothingToDo}, 0
	if out := run(g, l, &fin, "k-ok"); out.Status != saga.StatusSucceeded || fin != 1 || g.calls != 1 || l.calls != 1 {
		t.Fatalf("success: %s fin=%d", out.Status, fin)
	}
	if out := run(g, l, &fin, "k-ok"); out.Status != saga.StatusSucceeded || fin != 1 || g.calls != 1 {
		t.Fatal("restart ran the saga twice")
	}
	if g.keys[0] == l.keys[0] || len(g.keys[0]) != 36 {
		t.Fatalf("step keys %v %v", g.keys, l.keys)
	}

	// Blocked at the second step: the first is compensated, nothing finished.
	g, l, fin = &participant{outcome: saga.Done}, &participant{outcome: saga.Blocked}, 0
	if out := run(g, l, &fin, "k-blocked"); out.Status != saga.StatusBlocked || g.undos != 1 || fin != 0 {
		t.Fatalf("blocked: %s undos=%d", out.Status, g.undos)
	}

	// Permanent failure (4xx): compensated, failed.
	g, l, fin = &participant{outcome: saga.Done}, &participant{err: problem.New(problem.ValidationFailed, "")}, 0
	if out := run(g, l, &fin, "k-perm"); out.Status != saga.StatusFailed || g.undos != 1 {
		t.Fatalf("permanent: %s", out.Status)
	}

	// Transient failure: stays running with a next attempt; the job resumes it with the same key.
	g, l, fin = &participant{outcome: saga.Done}, &participant{err: errors.New("learning down")}, 0
	out := run(g, l, &fin, "k-retry")
	if out.Status != saga.StatusRunning || out.Run.NextAttemptAt == nil {
		t.Fatalf("transient: %+v", out)
	}
	l.err, l.outcome = nil, saga.Done
	now = now.Add(2 * time.Minute)
	defs := map[string]saga.Definition{"test_cleanup": {Type: "test_cleanup", Steps: []saga.Step{g.step("game"), l.step("learning")},
		Finish: func(context.Context, write.SagaRun) error { fin++; return nil }}}
	if n, err := eng.ResumeDue(ctx, defs, 10, time.Hour); err != nil || n != 1 {
		t.Fatalf("resume = %d %v", n, err)
	}
	if g.calls != 1 || l.calls != 2 || l.keys[0] != l.keys[1] || fin != 1 {
		t.Fatalf("resume calls game=%d learning=%d keys=%v fin=%d", g.calls, l.calls, l.keys, fin)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM platform.saga_runs WHERE idempotency_key = 'k-retry' AND status = 'succeeded'`); n != 1 {
		t.Fatal("resumed run not succeeded")
	}
	// The application role cannot touch saga rows.
	if _, err := e.app.ExecContext(ctx, `UPDATE platform.saga_runs SET status = 'failed'`); err == nil {
		t.Fatal("identity_app updated saga runs")
	}
}
