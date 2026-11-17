package observability_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F003-AC-33: child span named after the use case, duration metric with module/kind/usecase/result, one
// log line with trace_id and no input payload.
func TestUseCase(t *testing.T) {
	var buf bytes.Buffer
	rec := tracetest.NewSpanRecorder()
	reg := prometheus.NewRegistry()
	obs, err := observability.New(observability.Options{Logger: newLogger(&buf), SpanProcessor: rec, Registerer: reg, Binary: "core-api"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(obs.Shutdown)
	prev := slog.Default()
	slog.SetDefault(obs.Logger())
	t.Cleanup(func() { slog.SetDefault(prev) })

	ctx, parent := otel.Tracer("test").Start(context.Background(), "POST /api/v1/sample/widgets")
	input := struct{ Name string }{Name: "secret-widget-name"}
	_ = input
	uctx, done := observability.UseCase(ctx, "sample.command.CreateWidget")
	done(nil)
	_, done2 := observability.UseCase(uctx, "sample.command.CreateWidget")
	done2(problem.Invalid("name", problem.SubMaxLength, "too long"))
	_, done3 := observability.UseCase(ctx, "sample.query.ListWidgets")
	done3(errors.New("db down"))
	parent.End()

	var child bool
	for _, s := range rec.Ended() {
		if s.Name() == "sample.command.CreateWidget" && s.Parent().SpanID() == parent.SpanContext().SpanID() {
			child = true
		}
	}
	if !child {
		t.Fatal("use case span is not a child of the request span")
	}
	for _, c := range []struct {
		kind, uc, result string
	}{{"command", "CreateWidget", "ok"}, {"command", "CreateWidget", "client_error"}, {"query", "ListWidgets", "server_error"}} {
		if n := testutil.CollectAndCount(observability.UseCaseDuration().WithLabelValues("sample", c.kind, c.uc, c.result).(prometheus.Histogram)); n != 1 {
			t.Fatalf("metric %v missing", c)
		}
	}
	out := buf.String()
	if strings.Count(out, `"msg":"usecase"`) != 3 || !strings.Contains(out, `"trace_id":"`+parent.SpanContext().TraceID().String()+`"`) {
		t.Fatalf("log lines: %s", out)
	}
	if strings.Contains(out, "secret-widget-name") || !strings.Contains(out, `"error.code":"VALIDATION_FAILED"`) {
		t.Fatalf("log content: %s", out)
	}
}
