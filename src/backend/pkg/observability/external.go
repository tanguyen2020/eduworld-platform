package observability

import (
	"context"
	"errors"
	"sync"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/bulkhead"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
	"github.com/prometheus/client_golang/prometheus"
)

// Integrations of eduworld_external_client_circuit_state (MONITORING §3 "Metric bổ sung", alert CircuitBreakerOpen of
// MONITORING §5): a closed list so an adapter can never create an unbounded series (AGENTS R14, cardinality).
const (
	IntegrationKeycloakAdmin = "keycloak_admin"
	IntegrationKeycloakToken = "keycloak_token"
	IntegrationSIS           = "sis"
	IntegrationS3            = "s3"
	IntegrationSMTP          = "smtp"
)

// Values of eduworld_external_client_circuit_state (API_DESIGN §12.3): 0 = closed, 1 = half-open, 2 = open.
const (
	CircuitClosed   = 0
	CircuitHalfOpen = 1
	CircuitOpen     = 2
)

// Values of the outcome label of eduworld_external_client_requests_total (MONITORING §3), a closed list. The outcome
// measures the availability of the integration: an answer of the external system that the adapter treats as a result
// (object not found, Keycloak 404/409, SIS student not found) is "ok"; any other failure is "error" unless it is one of
// the specific kinds below. Alert ObjectStorageDown counts every outcome other than "ok".
const (
	OutcomeOK           = "ok"
	OutcomeError        = "error"
	OutcomeTimeout      = "timeout"
	OutcomeCanceled     = "canceled"
	OutcomeCircuitOpen  = "circuit_open"
	OutcomeBulkheadFull = "bulkhead_full"
)

// requestOutcomes are pre-created as zero series for every (module, integration) that has a circuit breaker;
// bulkhead_full is pre-created by ObserveBulkhead (only an adapter with a bulkhead can produce it).
var requestOutcomes = []string{OutcomeOK, OutcomeError, OutcomeTimeout, OutcomeCanceled, OutcomeCircuitOpen}

var (
	externalCircuitState = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "eduworld_external_client_circuit_state",
		Help: "Worst circuit breaker state of the external integration adapters of a module (0 = closed, 1 = half-open, 2 = open).",
	}, []string{"module", "integration"})
	externalRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_external_client_requests_total",
		Help: "Logical calls of an external integration adapter (after retries) by outcome.",
	}, []string{"module", "integration", "outcome"})
	externalRetries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_external_client_retries_total",
		Help: "Retries of an idempotent call of an external integration adapter.",
	}, []string{"module", "integration"})
	externalBulkheadRejected = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_external_client_bulkhead_rejected_total",
		Help: "Calls of an external integration adapter refused because its bulkhead was full.",
	}, []string{"module", "integration"})
)

// circuits keeps the state of every breaker of one (module, integration): an adapter may hold several breakers (one per
// SIS connection, one per Keycloak client), and the gauge shows the worst of them so a closed breaker built later never
// hides an open one.
type circuits struct {
	mu     sync.Mutex
	states map[circuitKey][]int // per key: one state per breaker instance
}

type circuitKey struct{ module, integration string }

var allCircuits = &circuits{states: map[circuitKey][]int{}}

// add registers a new closed breaker and returns its index.
func (c *circuits) add(k circuitKey) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.states[k] = append(c.states[k], CircuitClosed)
	c.publish(k)
	return len(c.states[k]) - 1
}

func (c *circuits) set(k circuitKey, i, v int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.states[k][i] = v
	c.publish(k)
}

// publish sets the gauge to the worst state of k (caller holds mu).
func (c *circuits) publish(k circuitKey) {
	worst := CircuitClosed
	for _, s := range c.states[k] {
		worst = max(worst, s)
	}
	externalCircuitState.WithLabelValues(k.module, k.integration).Set(float64(worst))
}

// ExternalCollectors are the collectors of the external integration adapters, registered on the admin registry of
// every binary (pkg/bootstrap).
func ExternalCollectors() []prometheus.Collector {
	return []prometheus.Collector{externalCircuitState, externalRequests, externalRetries, externalBulkheadRejected}
}

// ExternalCircuitState exposes eduworld_external_client_circuit_state (tests).
func ExternalCircuitState() *prometheus.GaugeVec { return externalCircuitState }

// ExternalRequests exposes eduworld_external_client_requests_total (tests).
func ExternalRequests() *prometheus.CounterVec { return externalRequests }

// ExternalRetries exposes eduworld_external_client_retries_total (tests).
func ExternalRetries() *prometheus.CounterVec { return externalRetries }

// ExternalBulkheadRejected exposes eduworld_external_client_bulkhead_rejected_total (tests).
func ExternalBulkheadRejected() *prometheus.CounterVec { return externalBulkheadRejected }

// CircuitValue maps a failsafe-go state to the gauge value.
func CircuitValue(s circuitbreaker.State) int {
	switch s {
	case circuitbreaker.OpenState:
		return CircuitOpen
	case circuitbreaker.HalfOpenState:
		return CircuitHalfOpen
	}
	return CircuitClosed
}

// ObserveCircuit makes the circuit breaker built by b report its state in
// eduworld_external_client_circuit_state{module, integration} (alert CircuitBreakerOpen, MONITORING §5): the series
// exists from construction (closed) and follows every state change; with several breakers for the same pair the gauge
// is the worst state. integration must be one of the Integration* constants. Call it once per built breaker. It also
// pre-creates the eduworld_external_client_requests_total series of the pair (counted by ObserveRequest).
func ObserveCircuit[R any](b circuitbreaker.Builder[R], module, integration string) circuitbreaker.Builder[R] {
	k := circuitKey{module: module, integration: integration}
	i := allCircuits.add(k)
	for _, o := range requestOutcomes {
		externalRequests.WithLabelValues(module, integration, o) // zero series from construction (rate() of the alerts)
	}
	return b.OnStateChanged(func(e circuitbreaker.StateChangedEvent) { allCircuits.set(k, i, CircuitValue(e.NewState)) })
}

// ExternalOutcome maps the final error of one logical call to the outcome label; an error matching one of answers is
// a valid answer of the external system and counts as "ok".
func ExternalOutcome(err error, answers ...error) string {
	switch {
	case err == nil:
		return OutcomeOK
	case errors.Is(err, circuitbreaker.ErrOpen):
		return OutcomeCircuitOpen
	case errors.Is(err, bulkhead.ErrFull):
		return OutcomeBulkheadFull
	case errors.Is(err, context.DeadlineExceeded):
		return OutcomeTimeout
	case errors.Is(err, context.Canceled):
		return OutcomeCanceled
	}
	for _, a := range answers {
		if errors.Is(err, a) {
			return OutcomeOK
		}
	}
	return OutcomeError
}

// ObserveRequest counts one logical call of an external adapter (once per call, after its retries) in
// eduworld_external_client_requests_total{module, integration, outcome}; err is the final error of the call and
// answers the errors that are a valid answer of the external system (see ExternalOutcome).
func ObserveRequest(module, integration string, err error, answers ...error) {
	externalRequests.WithLabelValues(module, integration, ExternalOutcome(err, answers...)).Inc()
}

// ObserveRetry makes the retry policy built by b count every retry in
// eduworld_external_client_retries_total{module, integration}; the series exists from construction (zero).
func ObserveRetry[R any](b retrypolicy.Builder[R], module, integration string) retrypolicy.Builder[R] {
	c := externalRetries.WithLabelValues(module, integration)
	return b.OnRetry(func(failsafe.ExecutionEvent[R]) { c.Inc() })
}

// ObserveBulkhead makes the bulkhead built by b count every refused call in
// eduworld_external_client_bulkhead_rejected_total{module, integration}; that series and the bulkhead_full outcome of
// eduworld_external_client_requests_total exist from construction (zero).
func ObserveBulkhead[R any](b bulkhead.Builder[R], module, integration string) bulkhead.Builder[R] {
	c := externalBulkheadRejected.WithLabelValues(module, integration)
	externalRequests.WithLabelValues(module, integration, OutcomeBulkheadFull)
	return b.OnFull(func(failsafe.ExecutionEvent[R]) { c.Inc() })
}
