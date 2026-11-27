package observability

import (
	"sync"

	"github.com/failsafe-go/failsafe-go/circuitbreaker"
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

var externalCircuitState = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "eduworld_external_client_circuit_state",
	Help: "Worst circuit breaker state of the external integration adapters of a module (0 = closed, 1 = half-open, 2 = open).",
}, []string{"module", "integration"})

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
	return []prometheus.Collector{externalCircuitState}
}

// ExternalCircuitState exposes eduworld_external_client_circuit_state (tests).
func ExternalCircuitState() *prometheus.GaugeVec { return externalCircuitState }

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
// is the worst state. integration must be one of the Integration* constants. Call it once per built breaker.
func ObserveCircuit[R any](b circuitbreaker.Builder[R], module, integration string) circuitbreaker.Builder[R] {
	k := circuitKey{module: module, integration: integration}
	i := allCircuits.add(k)
	return b.OnStateChanged(func(e circuitbreaker.StateChangedEvent) { allCircuits.set(k, i, CircuitValue(e.NewState)) })
}
