package httpx_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// MOCK (AGENTS R17): sentryCapture replaces the HTTP transport of the Sentry client; events stay in memory.
type sentryCapture struct {
	mu     sync.Mutex
	events []*sentry.Event
}

func (c *sentryCapture) Flush(time.Duration) bool              { return true }
func (c *sentryCapture) FlushWithContext(context.Context) bool { return true }
func (c *sentryCapture) Configure(sentry.ClientOptions)        {}
func (c *sentryCapture) Close()                                {}
func (c *sentryCapture) SendEvent(e *sentry.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, e)
}

func (c *sentryCapture) take() []*sentry.Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := c.events
	c.events = nil
	return out
}

// F131-AC-17 (T033): a panic recovered by the HTTP middleware sends exactly one event with trace_id, service and
// module; an unexpected 500 sends one error event; an expected Problem (4xx/503) sends none.
func TestErrorTrackingFromHTTP(t *testing.T) {
	tr := &sentryCapture{}
	et, err := observability.NewErrorTracker(observability.ErrorTrackingOptions{DSN: "https://public@glitchtip.invalid/1",
		Binary: "core-api", Env: "staging", Version: "backend/v1.4.0", Transport: tr})
	if err != nil {
		t.Fatal(err)
	}
	observability.SetErrorTracker(et)
	t.Cleanup(func() { observability.SetErrorTracker(nil) })

	rec := tracetest.NewSpanRecorder()
	obs, err := observability.New(observability.Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), SpanProcessor: rec,
		Registerer: prometheus.NewRegistry(), Binary: "core-api"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(obs.Shutdown)
	p := newIDP(t)
	srv, err := httpx.NewServer(httpx.Deps{Auth: authCfg(p), Keys: noKeys{}, Resolver: &fakeResolver{},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Middlewares: []func(http.Handler) http.Handler{obs.HTTP}})
	if err != nil {
		t.Fatal(err)
	}
	handlers := map[string]http.HandlerFunc{
		"/boom": func(http.ResponseWriter, *http.Request) { panic("nil map write") },
		"/internal": func(w http.ResponseWriter, r *http.Request) {
			httpx.WriteError(w, r, errors.New("pq: connection reset"))
		},
		"/missing": func(w http.ResponseWriter, r *http.Request) {
			httpx.WriteError(w, r, problem.New(problem.NotFound, ""))
		},
		"/busy": func(w http.ResponseWriter, r *http.Request) {
			httpx.WriteError(w, r, problem.New(problem.Unavailable, "").Retry())
		},
	}
	for path, h := range handlers {
		op := httpx.Operation{ID: "op" + path[1:], Module: "learning", Method: http.MethodGet, Pattern: "/test" + path,
			Public: true, Tenant: httpx.TenantNone}
		if err := srv.Handle(op, h); err != nil {
			t.Fatal(err)
		}
	}
	call := func(path string) int {
		r := httptest.NewRecorder()
		srv.ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/api/v1/test"+path, nil))
		return r.Code
	}
	panics := observability.PanicsRecovered().WithLabelValues("learning", "http")
	before := testutil.ToFloat64(panics)

	if c := call("/boom"); c != http.StatusInternalServerError {
		t.Fatalf("panic status %d", c)
	}
	evs := tr.take()
	if len(evs) != 1 {
		t.Fatalf("panic: %d events, want exactly 1", len(evs))
	}
	spans := rec.Ended()
	traceID := spans[len(spans)-1].SpanContext().TraceID().String()
	for k, v := range map[string]string{"service": "core-api", "module": "learning", "entrypoint": "http", "version": "backend/v1.4.0", "trace_id": traceID} {
		if evs[0].Tags[k] != v {
			t.Errorf("tag %s = %q, want %q", k, evs[0].Tags[k], v)
		}
	}
	if evs[0].Level != sentry.LevelFatal || evs[0].Request != nil {
		t.Errorf("level %s, request %+v", evs[0].Level, evs[0].Request)
	}
	if got := testutil.ToFloat64(panics) - before; got != 1 {
		t.Errorf("eduworld_panics_recovered_total += %v", got)
	}

	if c := call("/internal"); c != http.StatusInternalServerError {
		t.Fatalf("internal status %d", c)
	}
	evs = tr.take()
	if len(evs) != 1 || evs[0].Level != sentry.LevelError || evs[0].Tags["module"] != "learning" {
		t.Fatalf("unexpected 5xx: %d events %+v", len(evs), evs)
	}

	for _, path := range []string{"/missing", "/busy"} {
		call(path)
	}
	call("/nope") // unmatched 404
	if evs := tr.take(); len(evs) != 0 {
		t.Fatalf("expected problems were reported: %d events", len(evs))
	}
}
