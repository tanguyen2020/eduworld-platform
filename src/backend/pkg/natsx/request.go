package natsx

import (
	"context"
	"errors"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
)

// Reasons of eduworld_nats_request_errors_total (MONITORING §3 "Metric bổ sung"): a closed list.
const (
	ReasonTimeout      = "timeout"       // no reply before the deadline (handler slow, NATS paused or partitioned)
	ReasonNoResponders = "no_responders" // nobody subscribes the subject (no core-api handler for ew.cmd.<module>.>)
	ReasonUnavailable  = "unavailable"   // the connection is closed, draining or reconnecting (NATS cluster unreachable)
	ReasonError        = "error"         // any other failure
)

// NATS request/reply metrics of the realtime gateway → core-api commands (ADR-0074, alert NatsRequestTimeouts of
// MONITORING §5). Labels are bounded: module = owner module of the subject, subject_group = first segment of the
// command type (closed list of the AsyncAPI), reason = Reason*; never a user, tenant or payload (AGENTS R14).
var (
	requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "eduworld_nats_request_duration_seconds",
		Help:    "NATS request/reply latency (every request, answered or failed) by owner module and subject group.",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2, 5},
	}, []string{"module", "subject_group"})
	requestErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_nats_request_errors_total",
		Help: "NATS requests without a reply by owner module and reason (timeout, no_responders, unavailable, error).",
	}, []string{"module", "reason"})
)

// RequestCollectors are registered on the admin registry of every binary (pkg/bootstrap).
func RequestCollectors() []prometheus.Collector {
	return []prometheus.Collector{requestDuration, requestErrors}
}

// RequestDuration exposes eduworld_nats_request_duration_seconds (tests).
func RequestDuration() *prometheus.HistogramVec { return requestDuration }

// RequestErrors exposes eduworld_nats_request_errors_total (tests).
func RequestErrors() *prometheus.CounterVec { return requestErrors }

// InitRequestMetrics creates the series of module at 0 so the ratio of NatsRequestTimeouts has a denominator and a
// numerator before the first failure.
func InitRequestMetrics(module string) {
	for _, r := range []string{ReasonTimeout, ReasonNoResponders, ReasonUnavailable, ReasonError} {
		requestErrors.WithLabelValues(module, r)
	}
}

// ErrorReason classifies the error of a NATS request.
func ErrorReason(err error) string {
	switch {
	case errors.Is(err, nats.ErrNoResponders):
		return ReasonNoResponders
	case errors.Is(err, nats.ErrTimeout), errors.Is(err, context.DeadlineExceeded):
		return ReasonTimeout
	case errors.Is(err, nats.ErrConnectionClosed), errors.Is(err, nats.ErrConnectionDraining),
		errors.Is(err, nats.ErrConnectionReconnecting), errors.Is(err, nats.ErrDisconnected), errors.Is(err, nats.ErrNoServers),
		errors.Is(err, nats.ErrInvalidConnection):
		return ReasonUnavailable
	}
	return ReasonError
}

// Request sends m and waits for the reply within the deadline of ctx, recording
// eduworld_nats_request_duration_seconds{module, subject_group} for every request and
// eduworld_nats_request_errors_total{module, reason} for every failure. A request cancelled by its caller (context
// canceled, client gone) is not an error of NATS and is not counted as one.
func Request(ctx context.Context, nc *nats.Conn, module, subjectGroup string, m *nats.Msg) (*nats.Msg, error) {
	start := time.Now()
	res, err := nc.RequestMsgWithContext(ctx, m)
	requestDuration.WithLabelValues(module, subjectGroup).Observe(time.Since(start).Seconds())
	if err != nil && !errors.Is(err, context.Canceled) {
		requestErrors.WithLabelValues(module, ErrorReason(err)).Inc()
	}
	return res, err
}
