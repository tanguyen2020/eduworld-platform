// Package modcall calls another module's facade in process (ADR-0074 §3, BACKEND_DESIGN §7.2, F003 US5):
// span <caller>→<callee>.<operationId>, deadline per kind (module_call_deadline), x-allowed-callers and
// x-context-required checks, the original Principal / scope / Idempotency-Key kept in the context, panic
// recovery and Problem mapping. No network, no token, no retry, no circuit breaker, no bulkhead.
package modcall

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Kind selects the deadline.
type Kind int

const (
	Read Kind = iota
	Write
	Saga
)

// DefaultDeadlines are module_call_deadline (CONFIG_DEFAULTS): 800 ms read, 2 s write, 5 s saga step.
var DefaultDeadlines = map[Kind]time.Duration{Read: 800 * time.Millisecond, Write: 2 * time.Second, Saga: 5 * time.Second}

// Op describes one facade call; AllowedCallers and ContextRequired come from the generated
// modules/<callee>/api maps.
type Op struct {
	Caller, Callee, Operation string
	Kind                      Kind
	AllowedCallers            []string
	ContextRequired           []string
	Deadline                  time.Duration // overrides the kind default (configuration, tests)
}

var (
	requests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_internal_client_requests_total", Help: "In-process facade calls by outcome.",
	}, []string{"caller_module", "callee_module", "operation", "outcome"})
	duration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "eduworld_internal_client_duration_seconds", Help: "In-process facade call latency.",
		Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, .8, 1, 2, 5},
	}, []string{"caller_module", "callee_module", "operation"})
)

// Requests exposes eduworld_internal_client_requests_total.
func Requests() *prometheus.CounterVec { return requests }

// Register adds the metrics to r (composition root).
func Register(r prometheus.Registerer) {
	_ = r.Register(requests)
	_ = r.Register(duration)
}

type callerKey struct{}
type idemKey struct{}

// CallerFrom returns the module that called the facade ("" outside a facade call).
func CallerFrom(ctx context.Context) string {
	c, _ := ctx.Value(callerKey{}).(string)
	return c
}

// WithIdempotencyKey carries the Idempotency-Key of the original request into facade calls.
func WithIdempotencyKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, idemKey{}, key)
}

// IdempotencyKeyFrom returns the carried Idempotency-Key.
func IdempotencyKeyFrom(ctx context.Context) (string, bool) {
	k, ok := ctx.Value(idemKey{}).(string)
	return k, ok && k != ""
}

type result[T any] struct {
	v   T
	err error
}

// Call runs fn (the callee facade method) with the call context. The Principal, scope and module of ctx
// are passed on unchanged (on-behalf); the caller module is added.
func Call[T any](ctx context.Context, op Op, fn func(ctx context.Context) (T, error)) (T, error) {
	var zero T
	start := time.Now()
	outcome := "ok"
	defer func() {
		requests.WithLabelValues(op.Caller, op.Callee, op.Operation, outcome).Inc()
		duration.WithLabelValues(op.Caller, op.Callee, op.Operation).Observe(time.Since(start).Seconds())
	}()
	ctx, span := otel.Tracer("eduworld/modcall").Start(ctx, op.Caller+"→"+op.Callee+"."+op.Operation)
	span.SetAttributes(attribute.String("eduworld.caller", op.Caller), attribute.String("eduworld.callee", op.Callee))
	defer span.End()
	fail := func(o string, err error) (T, error) {
		outcome = o
		span.SetStatus(codes.Error, o)
		return zero, err
	}

	if !slices.Contains(op.AllowedCallers, op.Caller) {
		return fail("denied", problem.New(problem.CallerNotAllowed, ""))
	}
	for _, req := range op.ContextRequired {
		switch req {
		case "tenant_scope":
			if _, ok := tenancy.ScopeFrom(ctx); !ok {
				return fail("bad_context", problem.New(problem.Internal, "").Wrap(fmt.Errorf("modcall: %s requires a tenant scope", op.Operation)))
			}
		case "idempotency_key":
			if _, ok := IdempotencyKeyFrom(ctx); !ok {
				return fail("bad_context", problem.New(problem.Internal, "").Wrap(fmt.Errorf("modcall: %s requires an Idempotency-Key", op.Operation)))
			}
		}
	}
	d := op.Deadline
	if d <= 0 {
		d = DefaultDeadlines[op.Kind]
	}
	if dl, ok := ctx.Deadline(); !ok || time.Until(dl) > d {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d)
		defer cancel()
	}
	ctx = context.WithValue(ctx, callerKey{}, op.Caller)

	ch := make(chan result[T], 1)
	go func() {
		defer func() {
			if p := recover(); p != nil {
				// counted and reported here with the callee module; the error is marked so the 500 it becomes at the
				// HTTP edge is not reported a second time (F131-AC-17)
				observability.RecoveredPanic(ctx, op.Callee, observability.EntrypointFacade, p)
				ch <- result[T]{err: observability.MarkReported(fmt.Errorf("modcall: panic in %s.%s: %v", op.Callee, op.Operation, p))}
			}
		}()
		v, err := fn(ctx)
		ch <- result[T]{v: v, err: err}
	}()
	select {
	case <-ctx.Done():
		return fail("unavailable", problem.New(problem.Unavailable, "").Retry().Wrap(ctx.Err()))
	case r := <-ch:
		if r.err == nil {
			return r.v, nil
		}
		var pe *problem.Error
		switch {
		case errors.As(r.err, &pe):
			if problem.Status(pe.Code) >= 500 {
				return fail("server_error", r.err)
			}
			return fail("client_error", r.err)
		case errors.Is(r.err, context.DeadlineExceeded) || errors.Is(r.err, context.Canceled):
			return fail("unavailable", problem.New(problem.Unavailable, "").Retry().Wrap(r.err))
		default:
			return fail("server_error", problem.New(problem.Internal, "").Wrap(r.err))
		}
	}
}
