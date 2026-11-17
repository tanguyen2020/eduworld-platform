package observability

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// RequestInfo collects per-request fields filled by later middleware (route, principal, problem code)
// and read by logs and metrics. Methods are safe on a nil receiver.
type RequestInfo struct {
	mu        sync.Mutex
	requestID string
	route     string
	operation string
	module    string
	code      string
	userID    string
	tenantID  string
}

type infoSnapshot struct{ requestID, route, operation, module, code, userID, tenantID string }

func (i *RequestInfo) snapshot() infoSnapshot {
	if i == nil {
		return infoSnapshot{}
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	return infoSnapshot{i.requestID, i.route, i.operation, i.module, i.code, i.userID, i.tenantID}
}

// SetRoute records the route pattern (never the raw path), operation id and owning module.
func (i *RequestInfo) SetRoute(route, operation, module string) {
	if i == nil {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	i.route, i.operation, i.module = route, operation, module
}

// SetPrincipal records internal ids (UUIDs) of the caller.
func (i *RequestInfo) SetPrincipal(userID, tenantID string) {
	if i == nil {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	i.userID, i.tenantID = userID, tenantID
}

// SetCode records the ProblemCode of an error response.
func (i *RequestInfo) SetCode(code string) {
	if i == nil {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	i.code = code
}

// RequestID returns the request id.
func (i *RequestInfo) RequestID() string { return i.snapshot().requestID }

type infoKey struct{}

// InfoFrom returns the RequestInfo of ctx (nil-safe when absent).
func InfoFrom(ctx context.Context) *RequestInfo {
	i, _ := ctx.Value(infoKey{}).(*RequestInfo)
	return i
}

// Options configures New.
type Options struct {
	Logger        *slog.Logger
	Binary        string
	SpanProcessor sdktrace.SpanProcessor // nil = no tracing export (spans still propagate)
	Sampler       sdktrace.Sampler       // nil = OTEL_TRACES_SAMPLER (SDK default: parent-based always-on)
	Registerer    prometheus.Registerer
}

// Obs owns the tracer provider and HTTP metrics of a binary.
type Obs struct {
	log      *slog.Logger
	tp       *sdktrace.TracerProvider
	tracer   trace.Tracer
	prop     propagation.TextMapPropagator
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// New builds tracing and metrics. Metric labels: module, route (pattern), method, status_class, code —
// never user_id, tenant_id or IP (MONITORING §2).
func New(o Options) (*Obs, error) {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	ob := &Obs{log: o.Logger, prop: propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})}
	if o.SpanProcessor != nil {
		res, _ := sdkresource.Merge(sdkresource.Default(), sdkresource.NewSchemaless(attribute.String("service.name", o.Binary)))
		opts := []sdktrace.TracerProviderOption{sdktrace.WithSpanProcessor(o.SpanProcessor), sdktrace.WithResource(res)}
		if o.Sampler != nil {
			opts = append(opts, sdktrace.WithSampler(o.Sampler)) // otherwise OTEL_TRACES_SAMPLER / parent-based always-on
		}
		ob.tp = sdktrace.NewTracerProvider(opts...)
		otel.SetTracerProvider(ob.tp)
		otel.SetTextMapPropagator(ob.prop)
		ob.tracer = ob.tp.Tracer("eduworld/httpx")
	} else {
		ob.tracer = noop.NewTracerProvider().Tracer("eduworld/httpx")
	}
	ob.requests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_http_server_requests_total", Help: "HTTP requests by route pattern, status class and problem code.",
	}, []string{"module", "route", "method", "status_class", "code"})
	ob.duration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "eduworld_http_server_duration_seconds", Help: "HTTP server latency by route pattern.",
		Buckets: []float64{.005, .01, .025, .05, .1, .15, .25, .4, .6, 1, 2.5, 5, 10},
	}, []string{"module", "route", "method", "status_class"})
	if o.Registerer != nil {
		for _, c := range []prometheus.Collector{ob.requests, ob.duration} {
			if err := o.Registerer.Register(c); err != nil {
				return nil, err
			}
		}
	}
	return ob, nil
}

// NewOTLPProcessor exports spans with OTLP/HTTP configured by the standard OTEL_* variables.
func NewOTLPProcessor(ctx context.Context) (sdktrace.SpanProcessor, error) {
	exp, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, err
	}
	return sdktrace.NewBatchSpanProcessor(exp), nil
}

// NewForTest returns an Obs with a no-op tracer and private metrics.
func NewForTest(log *slog.Logger) *Obs {
	o, _ := New(Options{Logger: log})
	return o
}

// Logger returns the logger.
func (o *Obs) Logger() *slog.Logger { return o.log }

// RequestsCounter exposes eduworld_http_server_requests_total.
func (o *Obs) RequestsCounter() *prometheus.CounterVec { return o.requests }

// Propagator returns the W3C propagator (also used for NATS headers).
func (o *Obs) Propagator() propagation.TextMapPropagator { return o.prop }

// Shutdown flushes spans.
func (o *Obs) Shutdown() {
	if o.tp != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = o.tp.Shutdown(ctx)
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// HTTP is the outermost middleware: request id, server span (continuing traceparent), metrics and one
// access log line. It never logs headers, query strings or bodies.
func (o *Obs) HTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rid := r.Header.Get("X-Request-Id")
		if _, err := uuid.Parse(rid); err != nil || len(rid) != 36 {
			rid = ids.New().String()
		}
		w.Header().Set("X-Request-Id", rid)
		info := &RequestInfo{requestID: rid}
		ctx := o.prop.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		ctx, span := o.tracer.Start(ctx, r.Method, trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(attribute.String("http.request.method", r.Method)))
		ctx = context.WithValue(ctx, infoKey{}, info)
		sw := &statusWriter{ResponseWriter: w}

		defer func() {
			if v := recover(); v != nil {
				span.SetStatus(codes.Error, "panic")
				span.End()
				panic(v)
			}
			status := sw.status
			if status == 0 {
				status = http.StatusOK
			}
			s := info.snapshot()
			route := s.route
			if route == "" {
				route = "unmatched"
			}
			class := strconv.Itoa(status/100) + "xx"
			elapsed := time.Since(start)
			o.requests.WithLabelValues(s.module, route, r.Method, class, s.code).Inc()
			o.duration.WithLabelValues(s.module, route, r.Method, class).Observe(elapsed.Seconds())

			span.SetName(r.Method + " " + route)
			span.SetAttributes(attribute.String("http.route", route), attribute.Int("http.response.status_code", status))
			if s.code != "" {
				span.SetAttributes(attribute.String("eduworld.problem.code", s.code))
			}
			if status >= 500 {
				span.SetStatus(codes.Error, s.code)
			}
			span.End()

			level := slog.LevelInfo
			if status >= 500 {
				level = slog.LevelError
			}
			attrs := []slog.Attr{slog.String("method", r.Method), slog.String("route", route), slog.Int("status", status),
				slog.Float64("duration_ms", float64(elapsed.Microseconds())/1000)}
			if s.operation != "" {
				attrs = append(attrs, slog.String("operation", s.operation))
			}
			if s.code != "" {
				attrs = append(attrs, slog.String("error.code", s.code))
			}
			o.log.LogAttrs(ctx, level, "http request", attrs...)
		}()
		next.ServeHTTP(sw, r.WithContext(ctx))
	})
}
