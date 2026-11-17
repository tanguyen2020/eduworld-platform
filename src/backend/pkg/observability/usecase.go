package observability

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

var usecaseDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "eduworld_usecase_duration_seconds",
	Help:    "Use case (command/query handler) latency by module, kind, use case and result.",
	Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
}, []string{"module", "kind", "usecase", "result"})

// UseCaseDuration exposes eduworld_usecase_duration_seconds (registered by New, tests).
func UseCaseDuration() *prometheus.HistogramVec { return usecaseDuration }

// Result classifies an error: ok, client_error (4xx ProblemCode) or server_error.
func Result(err error) string {
	if err == nil {
		return "ok"
	}
	var pe *problem.Error
	if errors.As(err, &pe) && problem.Status(pe.Code) < 500 {
		return "client_error"
	}
	return "server_error"
}

// UseCase opens the span of a command/query handler named "<module>.<kind>.<Name>" (kind = command or
// query, F003-AC-33) and returns the span context and a finish function to call with the handler's
// error: it records eduworld_usecase_duration_seconds and one log line with the result — never the
// input (AGENTS R14).
func UseCase(ctx context.Context, name string) (context.Context, func(error)) {
	module, kind, uc := "unknown", "unknown", name
	if parts := strings.SplitN(name, ".", 3); len(parts) == 3 {
		module, kind, uc = parts[0], parts[1], parts[2]
	}
	start := time.Now()
	ctx, span := otel.Tracer("eduworld/usecase").Start(ctx, name)
	span.SetAttributes(attribute.String("eduworld.module", module), attribute.String("eduworld.usecase.kind", kind))
	return ctx, func(err error) {
		result := Result(err)
		elapsed := time.Since(start)
		usecaseDuration.WithLabelValues(module, kind, uc, result).Observe(elapsed.Seconds())
		attrs := []slog.Attr{slog.String("usecase", name), slog.String("result", result),
			slog.Float64("duration_ms", float64(elapsed.Microseconds())/1000)}
		level := slog.LevelInfo
		if result == "server_error" {
			level = slog.LevelError
			span.SetStatus(codes.Error, "server_error")
		}
		if err != nil {
			code := problem.From(err, "").Code
			attrs = append(attrs, slog.String("error.code", string(code)))
			span.SetAttributes(attribute.String("eduworld.problem.code", string(code)))
		}
		span.SetAttributes(attribute.String("eduworld.usecase.result", result))
		span.End()
		slog.Default().LogAttrs(ctx, level, "usecase", attrs...)
	}
}
