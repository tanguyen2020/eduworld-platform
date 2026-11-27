package observability_test

import (
	"errors"
	"testing"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

func state(module, integration string) float64 {
	return testutil.ToFloat64(observability.ExternalCircuitState().WithLabelValues(module, integration))
}

// The breaker reports its state in eduworld_external_client_circuit_state{module,integration} (alert
// CircuitBreakerOpen): 0 from construction, 2 when open, 1 when half-open, back to 0 when closed.
func TestObserveCircuitFollowsState(t *testing.T) {
	cb := observability.ObserveCircuit(circuitbreaker.NewBuilder[int](), "testcb", observability.IntegrationS3).
		WithFailureThreshold(1).WithDelay(50 * time.Millisecond).Build()
	if v := state("testcb", observability.IntegrationS3); v != observability.CircuitClosed {
		t.Fatalf("gauge after construction = %v, want 0", v)
	}
	exec := failsafe.With[int](cb)
	_, _ = exec.Get(func() (int, error) { return 0, errors.New("s3 down") })
	if v := state("testcb", observability.IntegrationS3); v != observability.CircuitOpen {
		t.Fatalf("gauge after the failure = %v, want 2 (open)", v)
	}
	time.Sleep(80 * time.Millisecond)
	cb.HalfOpen()
	if v := state("testcb", observability.IntegrationS3); v != observability.CircuitHalfOpen {
		t.Fatalf("gauge half-open = %v, want 1", v)
	}
	cb.Close()
	if v := state("testcb", observability.IntegrationS3); v != observability.CircuitClosed {
		t.Fatalf("gauge after close = %v, want 0", v)
	}
}

// Several breakers of one pair (one per SIS connection): the gauge is the worst state, and a closed breaker built
// later does not hide an open one.
func TestObserveCircuitWorstOfSeveral(t *testing.T) {
	a := observability.ObserveCircuit(circuitbreaker.NewBuilder[int](), "testcb2", observability.IntegrationSIS).Build()
	a.Open()
	b := observability.ObserveCircuit(circuitbreaker.NewBuilder[int](), "testcb2", observability.IntegrationSIS).Build()
	if v := state("testcb2", observability.IntegrationSIS); v != observability.CircuitOpen {
		t.Fatalf("gauge with one open breaker = %v, want 2", v)
	}
	a.Close()
	b.HalfOpen()
	if v := state("testcb2", observability.IntegrationSIS); v != observability.CircuitHalfOpen {
		t.Fatalf("gauge = %v, want 1 (worst = half-open)", v)
	}
	b.Close()
	if v := state("testcb2", observability.IntegrationSIS); v != observability.CircuitClosed {
		t.Fatalf("gauge = %v, want 0", v)
	}
}

func TestCircuitValue(t *testing.T) {
	if observability.CircuitValue(circuitbreaker.ClosedState) != 0 || observability.CircuitValue(circuitbreaker.HalfOpenState) != 1 ||
		observability.CircuitValue(circuitbreaker.OpenState) != 2 {
		t.Fatal("CircuitValue does not follow 0 = closed, 1 = half-open, 2 = open")
	}
}
