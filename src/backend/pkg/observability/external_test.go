package observability_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/bulkhead"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
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

func requests(module, integration, outcome string) float64 {
	return testutil.ToFloat64(observability.ExternalRequests().WithLabelValues(module, integration, outcome))
}

var errAnswer = errors.New("not found")

func TestExternalOutcome(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{nil, observability.OutcomeOK},
		{errAnswer, observability.OutcomeOK},
		{fmt.Errorf("wrapped: %w", errAnswer), observability.OutcomeOK},
		{errors.New("status 500"), observability.OutcomeError},
		{circuitbreaker.ErrOpen, observability.OutcomeCircuitOpen},
		{bulkhead.ErrFull, observability.OutcomeBulkheadFull},
		{fmt.Errorf("s3: %w", context.DeadlineExceeded), observability.OutcomeTimeout},
		{context.Canceled, observability.OutcomeCanceled},
	}
	for _, c := range cases {
		if got := observability.ExternalOutcome(c.err, errAnswer); got != c.want {
			t.Errorf("ExternalOutcome(%v) = %q, want %q", c.err, got, c.want)
		}
	}
}

// ObserveCircuit pre-creates the request series of the pair (zero); ObserveRequest counts one call per outcome.
func TestObserveRequest(t *testing.T) {
	_ = observability.ObserveCircuit(circuitbreaker.NewBuilder[int](), "testreq", observability.IntegrationKeycloakAdmin).Build()
	n := testutil.CollectAndCount(observability.ExternalRequests(), "eduworld_external_client_requests_total")
	if n < 5 {
		t.Fatalf("series after construction = %d, want the 5 pre-created outcomes", n)
	}
	for _, o := range []string{observability.OutcomeOK, observability.OutcomeError, observability.OutcomeTimeout, observability.OutcomeCanceled, observability.OutcomeCircuitOpen} {
		if v := requests("testreq", observability.IntegrationKeycloakAdmin, o); v != 0 {
			t.Fatalf("pre-created %s = %v, want 0", o, v)
		}
	}
	observability.ObserveRequest("testreq", observability.IntegrationKeycloakAdmin, nil)
	observability.ObserveRequest("testreq", observability.IntegrationKeycloakAdmin, errAnswer, errAnswer)
	observability.ObserveRequest("testreq", observability.IntegrationKeycloakAdmin, errors.New("status 503"))
	if v := requests("testreq", observability.IntegrationKeycloakAdmin, observability.OutcomeOK); v != 2 {
		t.Fatalf("ok = %v, want 2", v)
	}
	if v := requests("testreq", observability.IntegrationKeycloakAdmin, observability.OutcomeError); v != 1 {
		t.Fatalf("error = %v, want 1", v)
	}
}

// Every retry of the policy is counted; the series exists (zero) before the first retry.
func TestObserveRetry(t *testing.T) {
	rp := observability.ObserveRetry(retrypolicy.NewBuilder[int](), "testretry", observability.IntegrationS3).
		WithMaxRetries(2).ReturnLastFailure().Build()
	c := observability.ExternalRetries().WithLabelValues("testretry", observability.IntegrationS3)
	if v := testutil.ToFloat64(c); v != 0 {
		t.Fatalf("retries after construction = %v, want 0", v)
	}
	_, _ = failsafe.With[int](rp).Get(func() (int, error) { return 0, errors.New("s3 down") })
	if v := testutil.ToFloat64(c); v != 2 {
		t.Fatalf("retries = %v, want 2 (max retries)", v)
	}
}

// A full bulkhead refuses at once and the refusal is counted; the series exist (zero) from construction.
func TestObserveBulkhead(t *testing.T) {
	bh := observability.ObserveBulkhead(bulkhead.NewBuilder[int](1), "testbh", observability.IntegrationSIS).Build()
	c := observability.ExternalBulkheadRejected().WithLabelValues("testbh", observability.IntegrationSIS)
	if v := testutil.ToFloat64(c); v != 0 {
		t.Fatalf("rejected after construction = %v, want 0", v)
	}
	if v := requests("testbh", observability.IntegrationSIS, observability.OutcomeBulkheadFull); v != 0 {
		t.Fatalf("bulkhead_full outcome after construction = %v, want 0", v)
	}
	exec := failsafe.With[int](bh)
	hold, release := make(chan struct{}), make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = exec.Get(func() (int, error) { close(hold); <-release; return 1, nil })
	}()
	<-hold
	_, err := exec.Get(func() (int, error) { return 1, nil })
	close(release)
	<-done
	if !errors.Is(err, bulkhead.ErrFull) {
		t.Fatalf("second call err = %v, want bulkhead.ErrFull", err)
	}
	if v := testutil.ToFloat64(c); v != 1 {
		t.Fatalf("rejected = %v, want 1", v)
	}
}
