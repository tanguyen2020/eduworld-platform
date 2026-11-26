package observability

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace"
)

// Error tracking (F131 T036, F131-AC-17, MONITORING §9, ADR-0023): recovered panics and unexpected 5xx go to the
// self-hosted GlitchTip through the Sentry SDK, tagged service/module/version/trace_id/entrypoint. Nothing that
// can carry PII leaves the process: the SDK collects no user, cookie, header, body or query data
// (send_default_pii=false) and beforeSend strips the request (body, headers,
// cookies, query string), user, server name, breadcrumb data and every context except trace/runtime, and redacts
// messages (AGENTS R14). Without a DSN the tracker is nil and no Sentry client is ever built (no network).

// Entrypoints of eduworld_panics_recovered_total and of the entrypoint tag (MONITORING §3).
const (
	EntrypointHTTP     = "http"
	EntrypointNATS     = "nats"     // realtime command handlers (request/reply)
	EntrypointConsumer = "consumer" // JetStream event consumers
	EntrypointJob      = "job"      // saga steps, scheduled jobs
	EntrypointFacade   = "facade"   // in-process facade call (pkg/modcall)
)

var panicsRecovered = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "eduworld_panics_recovered_total",
	Help: "Panics caught by a recover middleware by module and entrypoint (the binary is the service label of the target).",
}, []string{"module", "entrypoint"})

// PanicsRecovered exposes eduworld_panics_recovered_total.
func PanicsRecovered() *prometheus.CounterVec { return panicsRecovered }

// ErrorTrackingOptions configure NewErrorTracker.
type ErrorTrackingOptions struct {
	DSN     string // from the Docker secret EW_ERROR_TRACKING_DSN_FILE; empty = disabled
	Binary  string // service tag
	Profile string // worker profile tag (empty for other binaries)
	Env     string // environment (EW_ENV)
	Version string // release and version tag (buildinfo)
	// Transport replaces the HTTP transport (tests only).
	Transport sentry.Transport
}

// ErrorTracker sends events to GlitchTip. A nil *ErrorTracker is the disabled tracker: every method is a no-op.
type ErrorTracker struct {
	client  *sentry.Client
	binary  string
	profile string
	version string
}

// NewErrorTracker returns nil (disabled, no client) when o.DSN is empty.
func NewErrorTracker(o ErrorTrackingOptions) (*ErrorTracker, error) {
	if o.DSN == "" {
		return nil, nil
	}
	c, err := sentry.NewClient(sentry.ClientOptions{
		Dsn:              o.DSN,
		Environment:      o.Env,
		Release:          o.Version,
		ServerName:       o.Binary,           // never the host name
		DataCollection:   noDataCollection(), // send_default_pii=false and stricter (SendDefaultPII is deprecated)
		AttachStacktrace: true,
		MaxBreadcrumbs:   -1, // breadcrumbs are never recorded
		BeforeSend:       beforeSend,
		BeforeBreadcrumb: func(*sentry.Breadcrumb, *sentry.BreadcrumbHint) *sentry.Breadcrumb { return nil },
		Transport:        o.Transport,
	})
	if err != nil {
		// the error of an invalid DSN echoes the DSN: report a fixed message only (R14)
		return nil, errors.New("error tracking: invalid EW_ERROR_TRACKING_DSN_FILE")
	}
	return &ErrorTracker{client: c, binary: o.Binary, profile: o.Profile, version: o.Version}, nil
}

// noDataCollection turns off every automatic collection of the SDK: user info, cookies, request/response headers,
// HTTP bodies and query parameters (the SDK API that replaces SendDefaultPII=false).
func noDataCollection() *sentry.DataCollection {
	off := func() *sentry.KeyValueCollectionBehavior {
		return &sentry.KeyValueCollectionBehavior{Mode: sentry.CollectionOff}
	}
	return &sentry.DataCollection{
		UserInfo:    sentry.Set(false),
		Cookies:     off(),
		HTTPHeaders: &sentry.HeaderCollectionConfig{Request: off(), Response: off()},
		HTTPBodies:  []sentry.BodyType{}, // empty, not nil: nil means every body type
		QueryParams: off(),
	}
}

// keptContexts are the event contexts that cannot carry PII.
var keptContexts = map[string]bool{"trace": true, "runtime": true}

// beforeSend is the last filter before an event leaves the process (F131-AC-17).
func beforeSend(e *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	e.Request = nil
	e.User = sentry.User{}
	e.ServerName = ""
	e.Modules = nil
	e.Message = RedactString(e.Message)
	for i := range e.Exception {
		e.Exception[i].Value = RedactString(e.Exception[i].Value)
	}
	for k := range e.Contexts {
		if !keptContexts[k] {
			delete(e.Contexts, k)
		}
	}
	for _, b := range e.Breadcrumbs {
		b.Data = nil
		b.Message = RedactString(b.Message)
	}
	for k, v := range e.Tags {
		e.Tags[k] = RedactString(v)
	}
	return e
}

// CapturePanic reports a recovered panic value v (level fatal, stack trace of the recovering goroutine).
func (t *ErrorTracker) CapturePanic(ctx context.Context, module, entrypoint string, v any) {
	if t == nil {
		return
	}
	err, ok := v.(error)
	if !ok {
		err = fmt.Errorf("panic: %v", v)
	}
	t.capture(ctx, module, entrypoint, err, sentry.LevelFatal)
}

// CaptureError reports an unexpected error (level error).
func (t *ErrorTracker) CaptureError(ctx context.Context, module, entrypoint string, err error) {
	if t == nil || err == nil {
		return
	}
	t.capture(ctx, module, entrypoint, err, sentry.LevelError)
}

func (t *ErrorTracker) capture(ctx context.Context, module, entrypoint string, err error, level sentry.Level) {
	scope := sentry.NewScope()
	tags := map[string]string{"service": t.binary, "module": module, "version": t.version, "entrypoint": entrypoint}
	if t.profile != "" {
		tags["profile"] = t.profile
	}
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		tags["trace_id"] = sc.TraceID().String()
		// the trace context of the event points at the OTel trace (Tempo), not a random Sentry trace
		scope.SetPropagationContext(sentry.PropagationContext{TraceID: sentry.TraceID(sc.TraceID()), SpanID: sentry.SpanID(sc.SpanID())})
	}
	scope.SetTags(tags)
	scope.SetLevel(level)
	ev := t.client.EventFromException(err, level)
	t.client.CaptureEvent(ev, &sentry.EventHint{OriginalException: err}, scope)
}

// Flush waits up to timeout for queued events (shutdown).
func (t *ErrorTracker) Flush(timeout time.Duration) {
	if t != nil {
		t.client.Flush(timeout)
	}
}

var current atomic.Pointer[ErrorTracker]

// SetErrorTracker installs the tracker of the binary (bootstrap); nil disables error tracking.
func SetErrorTracker(t *ErrorTracker) { current.Store(t) }

// RecoveredPanic counts a panic caught by a recover middleware and reports it. module may be empty for an
// HTTP request: the module of the matched route is used.
func RecoveredPanic(ctx context.Context, module, entrypoint string, v any) {
	if module == "" {
		module = InfoFrom(ctx).Module()
	}
	panicsRecovered.WithLabelValues(module, entrypoint).Inc()
	current.Load().CapturePanic(ctx, module, entrypoint, v)
}

// ReportUnexpected reports an unexpected error (an HTTP 500 INTERNAL) unless it was already reported as a panic.
func ReportUnexpected(ctx context.Context, module, entrypoint string, err error) {
	if err == nil || IsReported(err) {
		return
	}
	if module == "" {
		module = InfoFrom(ctx).Module()
	}
	current.Load().CaptureError(ctx, module, entrypoint, err)
}

type reportedError struct{ err error }

func (e reportedError) Error() string { return e.err.Error() }
func (e reportedError) Unwrap() error { return e.err }

// MarkReported marks err as already sent to error tracking (a recovered panic turned into an error), so the
// HTTP layer does not report the resulting 500 a second time.
func MarkReported(err error) error { return reportedError{err} }

// IsReported reports whether err (or an error it wraps) was marked by MarkReported.
func IsReported(err error) bool {
	var r reportedError
	return errors.As(err, &r)
}
