package observability

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.opentelemetry.io/otel/trace"
)

// MOCK (AGENTS R17): captureTransport replaces the HTTP transport of the Sentry client and keeps the events
// that would have been sent to GlitchTip.
type captureTransport struct {
	mu     sync.Mutex
	events []*sentry.Event
}

func (c *captureTransport) Flush(time.Duration) bool              { return true }
func (c *captureTransport) FlushWithContext(context.Context) bool { return true }
func (c *captureTransport) Configure(sentry.ClientOptions)        {}
func (c *captureTransport) Close()                                {}
func (c *captureTransport) SendEvent(e *sentry.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, e)
}

func (c *captureTransport) Events() []*sentry.Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]*sentry.Event(nil), c.events...)
}

const testDSN = "https://public@glitchtip.invalid/1"

func newTestTracker(t *testing.T) (*ErrorTracker, *captureTransport) {
	t.Helper()
	tr := &captureTransport{}
	et, err := NewErrorTracker(ErrorTrackingOptions{DSN: testDSN, Binary: "core-api", Env: "production", Version: "backend/v1.4.0", Transport: tr})
	if err != nil || et == nil {
		t.Fatalf("tracker: %v", err)
	}
	return et, tr
}

func spanCtx(t *testing.T) (context.Context, trace.TraceID) {
	t.Helper()
	tid, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	sid, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
	sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: tid, SpanID: sid, TraceFlags: trace.FlagsSampled})
	return trace.ContextWithSpanContext(context.Background(), sc), tid
}

// F131-AC-17 (T033): beforeSend drops request body/headers/cookies/query string, user, server name,
// breadcrumb data and PII-bearing contexts, redacts messages and keeps the tags.
func TestBeforeSendStripsPII(t *testing.T) {
	e := &sentry.Event{
		Message:    "login failed for student@example.com",
		ServerName: "app-node-3",
		User:       sentry.User{ID: "u1", Email: "student@example.com", IPAddress: "10.1.2.3", Username: "hs.nguyen"},
		Request: &sentry.Request{URL: "https://api.example/api/v1/auth/session", Method: "POST",
			Data: `{"password":"secret"}`, QueryString: "token=abc", Cookies: "session=xyz",
			Headers: map[string]string{"Authorization": "Bearer eyJabc", "User-Agent": "Godot", "X-Forwarded-For": "1.2.3.4"},
			Env:     map[string]string{"REMOTE_ADDR": "1.2.3.4"}},
		Exception:   []sentry.Exception{{Type: "*errors.errorString", Value: "user hs@example.com not found at 192.168.1.10"}},
		Breadcrumbs: []*sentry.Breadcrumb{{Message: "GET /x?email=a@b.co", Data: map[string]any{"body": "secret"}}},
		Contexts: map[string]sentry.Context{"trace": {"trace_id": "t"}, "runtime": {"name": "go"},
			"os": {"name": "linux"}, "device": {"hostname": "app-node-3"}, "request": {"ip": "1.2.3.4"}},
		Tags:    map[string]string{"service": "core-api", "module": "identity", "trace_id": "t", "version": "v1"},
		Modules: map[string]string{"github.com/x/y": "v1"},
	}
	out := beforeSend(e, nil)
	if out == nil {
		t.Fatal("event dropped")
	}
	if out.Request != nil || !out.User.IsEmpty() || out.ServerName != "" || out.Modules != nil {
		t.Errorf("request/user/server/modules kept: %+v %+v %q", out.Request, out.User, out.ServerName)
	}
	if out.Breadcrumbs[0].Data != nil || strings.Contains(out.Breadcrumbs[0].Message, "a@b.co") {
		t.Errorf("breadcrumb kept data: %+v", out.Breadcrumbs[0])
	}
	for k := range out.Contexts {
		if k != "trace" && k != "runtime" {
			t.Errorf("context %q kept", k)
		}
	}
	if strings.Contains(out.Message, "student@example.com") || strings.Contains(out.Exception[0].Value, "hs@example.com") ||
		strings.Contains(out.Exception[0].Value, "192.168.1.10") {
		t.Errorf("message not redacted: %q / %q", out.Message, out.Exception[0].Value)
	}
	for _, k := range []string{"service", "module", "trace_id", "version"} {
		if out.Tags[k] == "" {
			t.Errorf("tag %s dropped", k)
		}
	}
}

func TestCapturePanicTagsAndPipeline(t *testing.T) {
	et, tr := newTestTracker(t)
	SetErrorTracker(et)
	t.Cleanup(func() { SetErrorTracker(nil) })
	ctx, tid := spanCtx(t)
	before := testutil.ToFloat64(panicsRecovered.WithLabelValues("learning", EntrypointConsumer))

	RecoveredPanic(ctx, "learning", EntrypointConsumer, "nil map write for user a@b.co")
	et.Flush(time.Second)

	evs := tr.Events()
	if len(evs) != 1 {
		t.Fatalf("events = %d, want 1", len(evs))
	}
	ev := evs[0]
	want := map[string]string{"service": "core-api", "module": "learning", "version": "backend/v1.4.0",
		"entrypoint": "consumer", "trace_id": tid.String()}
	for k, v := range want {
		if ev.Tags[k] != v {
			t.Errorf("tag %s = %q, want %q", k, ev.Tags[k], v)
		}
	}
	if ev.Level != sentry.LevelFatal || ev.Environment != "production" || ev.Release != "backend/v1.4.0" {
		t.Errorf("level/env/release: %s %s %s", ev.Level, ev.Environment, ev.Release)
	}
	if ev.ServerName != "" || ev.Request != nil {
		t.Errorf("server name or request sent: %q %+v", ev.ServerName, ev.Request)
	}
	if len(ev.Exception) == 0 || strings.Contains(ev.Exception[0].Value, "a@b.co") {
		t.Errorf("exception: %+v", ev.Exception)
	}
	if fmt.Sprint(ev.Contexts["trace"]["trace_id"]) != tid.String() {
		t.Errorf("trace context: %+v", ev.Contexts["trace"])
	}
	dc := et.client.GetDataCollection() // send_default_pii=false: nothing collected automatically
	if dc.UserInfo.Value || len(dc.HTTPBodies) != 0 || dc.Cookies.Mode != sentry.CollectionOff ||
		dc.QueryParams.Mode != sentry.CollectionOff || dc.HTTPHeaders.Request.Mode != sentry.CollectionOff {
		t.Errorf("data collection not off: %+v", dc)
	}
	if got := testutil.ToFloat64(panicsRecovered.WithLabelValues("learning", EntrypointConsumer)) - before; got != 1 {
		t.Errorf("eduworld_panics_recovered_total += %v", got)
	}
}

func TestReportUnexpectedSkipsReported(t *testing.T) {
	et, tr := newTestTracker(t)
	SetErrorTracker(et)
	t.Cleanup(func() { SetErrorTracker(nil) })

	ReportUnexpected(context.Background(), "chat", EntrypointHTTP, MarkReported(errors.New("panic in facade")))
	ReportUnexpected(context.Background(), "chat", EntrypointHTTP, nil)
	ReportUnexpected(context.Background(), "chat", EntrypointHTTP, errors.New("db: connection reset"))
	et.Flush(time.Second)
	evs := tr.Events()
	if len(evs) != 1 || evs[0].Level != sentry.LevelError || evs[0].Tags["module"] != "chat" || evs[0].Tags["trace_id"] != "" {
		t.Fatalf("events: %d %+v", len(evs), evs)
	}
	wrapped := errors.Join(errors.New("outer"), MarkReported(errors.New("inner")))
	if !IsReported(wrapped) || IsReported(errors.New("x")) {
		t.Error("IsReported does not follow the error chain")
	}
}

// Without a DSN no Sentry client is built: no network, no global hub, nil-safe calls.
func TestDisabledNeverBuildsClient(t *testing.T) {
	et, err := NewErrorTracker(ErrorTrackingOptions{Binary: "worker"})
	if err != nil || et != nil {
		t.Fatalf("disabled tracker = %v, %v", et, err)
	}
	SetErrorTracker(et)
	t.Cleanup(func() { SetErrorTracker(nil) })
	before := testutil.ToFloat64(panicsRecovered.WithLabelValues("game", EntrypointJob))
	RecoveredPanic(context.Background(), "game", EntrypointJob, "boom")
	ReportUnexpected(context.Background(), "game", EntrypointJob, errors.New("boom"))
	et.Flush(time.Second)
	if testutil.ToFloat64(panicsRecovered.WithLabelValues("game", EntrypointJob))-before != 1 {
		t.Error("panic not counted while error tracking is off")
	}
	if c := sentry.CurrentHub().Client(); c != nil && c.IsEnabled() {
		t.Error("a global Sentry client was built")
	}
}

func TestInvalidDSNDoesNotEchoIt(t *testing.T) {
	_, err := NewErrorTracker(ErrorTrackingOptions{DSN: "not a dsn with secret-key"})
	if err == nil || strings.Contains(err.Error(), "secret-key") {
		t.Fatalf("err = %v", err)
	}
}
