package modcall_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

func code(err error) problem.Code {
	var pe *problem.Error
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

func op(kind modcall.Kind, ctxReq ...string) modcall.Op {
	return modcall.Op{Caller: "game", Callee: "learning", Operation: "createAttempt", Kind: kind,
		AllowedCallers: []string{"game"}, ContextRequired: ctxReq}
}

// F003-AC-24, F003-AC-25 (TC-SVC-006): span, metric, deadline per kind, on-behalf context, allowlist,
// x-context-required.
func TestCallContextAndAllowlist(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	obs, err := observability.New(observability.Options{SpanProcessor: rec})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(obs.Shutdown)
	student := authztest.Student(authztest.TenantA)
	ctx := modcall.WithIdempotencyKey(authztest.Ctx(context.Background(), student), "0193f0a2-7c1e-7000-8000-000000000001")

	var seenDeadline time.Duration
	got, err := modcall.Call(ctx, op(modcall.Write, "tenant_scope", "idempotency_key"), func(ctx context.Context) (string, error) {
		dl, _ := ctx.Deadline()
		seenDeadline = time.Until(dl)
		p, _ := authz.PrincipalFrom(ctx)
		s, _ := tenancy.ScopeFrom(ctx)
		key, _ := modcall.IdempotencyKeyFrom(ctx)
		if p.UserID != student.UserID || s.TenantIDs()[0] != authztest.TenantA || modcall.CallerFrom(ctx) != "game" || key == "" {
			t.Errorf("callee context: principal %v scope %v caller %q key %q", p.UserID, s, modcall.CallerFrom(ctx), key)
		}
		return "ok", nil
	})
	if err != nil || got != "ok" {
		t.Fatalf("call: %q %v", got, err)
	}
	if seenDeadline <= 0 || seenDeadline > modcall.DefaultDeadlines[modcall.Write] {
		t.Fatalf("deadline %v for a write", seenDeadline)
	}
	var span bool
	for _, s := range rec.Ended() {
		if s.Name() == "game→learning.createAttempt" {
			span = true
		}
	}
	if !span {
		t.Fatal("span game→learning.createAttempt missing")
	}
	if n := testutil.ToFloat64(modcall.Requests().WithLabelValues("game", "learning", "createAttempt", "ok")); n < 1 {
		t.Fatal("metric not recorded")
	}

	// A shorter deadline already in ctx is kept.
	short, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	_, _ = modcall.Call(short, op(modcall.Saga), func(ctx context.Context) (int, error) {
		dl, _ := ctx.Deadline()
		if time.Until(dl) > 60*time.Millisecond {
			t.Error("caller's shorter deadline replaced")
		}
		return 0, nil
	})

	called := false
	fn := func(context.Context) (int, error) { called = true; return 1, nil }
	notAllowed := op(modcall.Read)
	notAllowed.Caller = "chat"
	if _, err := modcall.Call(ctx, notAllowed, fn); code(err) != problem.CallerNotAllowed || called {
		t.Fatalf("caller outside allowlist: %v called=%v", err, called)
	}
	if _, err := modcall.Call(context.Background(), op(modcall.Read, "tenant_scope"), fn); code(err) != problem.Internal || called {
		t.Fatalf("missing tenant scope: %v called=%v", err, called)
	}
	noKey := authztest.Ctx(context.Background(), student)
	if _, err := modcall.Call(noKey, op(modcall.Write, "idempotency_key"), fn); code(err) != problem.Internal || called {
		t.Fatalf("missing idempotency key: %v called=%v", err, called)
	}
}

// F003-AC-26, F003-AC-27 (TC-SVC-011): no retry, deadline → 503 retryable after one call, panic → 500,
// Problem codes kept, never fail-open.
func TestCallErrors(t *testing.T) {
	ctx := authztest.Ctx(context.Background(), authztest.Student(authztest.TenantA))
	var calls atomic.Int32
	o := op(modcall.Read)
	o.Deadline = 30 * time.Millisecond
	start := time.Now()
	_, err := modcall.Call(ctx, o, func(ctx context.Context) (int, error) {
		calls.Add(1)
		time.Sleep(200 * time.Millisecond) // ignores ctx on purpose
		return 1, nil
	})
	var pe *problem.Error
	if !errors.As(err, &pe) || pe.Code != problem.Unavailable || !pe.Retryable || calls.Load() != 1 || time.Since(start) > 150*time.Millisecond {
		t.Fatalf("slow callee: %v calls=%d after %v", err, calls.Load(), time.Since(start))
	}
	if _, err := modcall.Call(ctx, op(modcall.Read), func(context.Context) (int, error) { panic("boom") }); code(err) != problem.Internal {
		t.Fatalf("panic: %v", err)
	}
	if _, err := modcall.Call(ctx, op(modcall.Read), func(context.Context) (int, error) {
		return 0, problem.New(problem.NotFound, "attempt")
	}); code(err) != problem.NotFound {
		t.Fatalf("problem code not kept: %v", err)
	}
	if _, err := modcall.Call(ctx, op(modcall.Read), func(context.Context) (int, error) {
		return 0, context.DeadlineExceeded
	}); code(err) != problem.Unavailable {
		t.Fatalf("callee deadline: %v", err)
	}
	if _, err := modcall.Call(ctx, op(modcall.Read), func(context.Context) (int, error) { return 0, errors.New("pq: lost") }); code(err) != problem.Internal {
		t.Fatalf("unknown error: %v", err)
	}
}
