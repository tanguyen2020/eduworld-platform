package observability_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// F002-AC-24: server span per request, traceparent accepted and propagated, standard HTTP metrics
// without user_id/IP labels.
func TestServerSpanAndMetrics(t *testing.T) {
	var buf bytes.Buffer
	rec := tracetest.NewSpanRecorder()
	reg := prometheus.NewRegistry()
	obs, err := observability.New(observability.Options{Logger: newLogger(&buf), SpanProcessor: rec, Registerer: reg, Binary: "core-api"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(obs.Shutdown)

	var seen trace.SpanContext
	h := obs.HTTP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = trace.SpanContextFromContext(r.Context())
		observability.InfoFrom(r.Context()).SetRoute("/api/v1/lms/classes/{class_id}", "getClass", "learning")
		observability.InfoFrom(r.Context()).SetCode("NOT_FOUND")
		w.WriteHeader(http.StatusNotFound)
	}))
	const parent = "00-4bf92f3577b34da6a1c5c6a3e5a2f3e1-00f067aa0ba902b7-01"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/lms/classes/0193f0a2-7c1e-7000-8000-000000000001", nil)
	req.Header.Set("traceparent", parent)
	h.ServeHTTP(httptest.NewRecorder(), req)

	if seen.TraceID().String() != "4bf92f3577b34da6a1c5c6a3e5a2f3e1" {
		t.Fatalf("trace id not continued: %s", seen.TraceID())
	}
	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("spans = %d", len(spans))
	}
	s := spans[0]
	if s.SpanKind() != trace.SpanKindServer || s.Parent().SpanID().String() != "00f067aa0ba902b7" {
		t.Fatalf("span kind %v parent %s", s.SpanKind(), s.Parent().SpanID())
	}
	if !strings.Contains(s.Name(), "/api/v1/lms/classes/{class_id}") {
		t.Fatalf("span name %q must use the route pattern, not the raw path", s.Name())
	}

	n := testutil.ToFloat64(obs.RequestsCounter().WithLabelValues("learning", "/api/v1/lms/classes/{class_id}", "GET", "4xx", "NOT_FOUND"))
	if n != 1 {
		t.Fatalf("eduworld_http_server_requests_total = %v", n)
	}
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, mf := range mfs {
		found[mf.GetName()] = true
		for _, m := range mf.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "user_id" || l.GetName() == "ip" || l.GetName() == "tenant_id" {
					t.Fatalf("%s has forbidden label %s", mf.GetName(), l.GetName())
				}
				if strings.Contains(l.GetValue(), "0193f0a2-7c1e-7000-8000-000000000001") {
					t.Fatalf("%s label carries a raw path id", mf.GetName())
				}
			}
		}
	}
	for _, want := range []string{"eduworld_http_server_requests_total", "eduworld_http_server_duration_seconds"} {
		if !found[want] {
			t.Fatalf("metric %s not registered", want)
		}
	}
	if !strings.Contains(buf.String(), `"trace_id":"4bf92f3577b34da6a1c5c6a3e5a2f3e1"`) {
		t.Fatalf("access log has no trace_id: %s", buf.String())
	}
}

// Unmatched routes are labelled "unmatched" (bounded cardinality).
func TestUnmatchedRouteLabel(t *testing.T) {
	var buf bytes.Buffer
	obs, err := observability.New(observability.Options{Logger: newLogger(&buf), SpanProcessor: tracetest.NewSpanRecorder(),
		Registerer: prometheus.NewRegistry(), Binary: "core-api"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(obs.Shutdown)
	h := obs.HTTP(http.NotFoundHandler())
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/random/"+strings.Repeat("x", 40), nil))
	if n := testutil.ToFloat64(obs.RequestsCounter().WithLabelValues("", "unmatched", "GET", "4xx", "")); n != 1 {
		t.Fatalf("unmatched = %v", n)
	}
}
