package lookup_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/sis"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/contract"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/contractv1client"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/lookup"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/tests/fakesis"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

type secrets map[string]string // test double of port.SecretStore

func (s secrets) Get(_ context.Context, ref string) ([]byte, error) { return []byte(s[ref]), nil }

type env struct {
	adapter *lookup.Adapter
	conns   map[uuid.UUID]*write.ContractConnectionRow
}

func newEnv(t *testing.T, timeout time.Duration, concurrency int, servers ...*fakesis.Server) (*env, []uuid.UUID) {
	t.Helper()
	v, err := contract.New()
	if err != nil {
		t.Fatal(err)
	}
	e := &env{conns: map[uuid.UUID]*write.ContractConnectionRow{}}
	var ids []uuid.UUID
	for _, s := range servers {
		id := uuid.New()
		e.conns[id] = &write.ContractConnectionRow{ID: id, ProviderKind: sis.ProviderContractV1, SecretRef: "k",
			Config:       sis.ContractConfig{BaseURL: s.URL, AuthType: sis.AuthAPIKey},
			Capabilities: []string{sis.CapStudentLookup}, State: sis.ConnectionState{Status: sis.StatusActive}}
		ids = append(ids, id)
	}
	e.adapter = &lookup.Adapter{Timeout: timeout, Concurrency: concurrency, BreakerThreshold: 3, BreakerDelay: time.Minute,
		Providers: &contractv1client.Factory{Secrets: secrets{"k": "x"}, Validator: v},
		Load:      func(_ context.Context, id uuid.UUID) (*write.ContractConnectionRow, error) { return e.conns[id], nil }}
	return e, ids
}

// F017-AC-22, AC-23: one call within the timeout, no retry; not found; timeout and 5xx are temporary;
// unsupported connections are never called (TC-SIS-102, TC-SIS-104, TC-SIS-107).
func TestLookup(t *testing.T) {
	sisReqT := func(o string) float64 {
		return testutil.ToFloat64(observability.ExternalRequests().WithLabelValues("identity", observability.IntegrationSIS, o))
	}
	ok0, err0, to0 := sisReqT(observability.OutcomeOK), sisReqT(observability.OutcomeError), sisReqT(observability.OutcomeTimeout)
	s := fakesis.New() // MOCK
	defer s.Close()
	s.Lock()
	s.Lookups["hs.an.3a"] = fakesis.Student("HS-1", "TH-DEMO-001", "A1", "An", "hs.an.3a", "active")
	s.Extra[fakesis.OpLookup] = map[string]any{"parent_phone": "x"}
	s.Unlock()
	e, ids := newEnv(t, 200*time.Millisecond, 20, s)
	ctx := context.Background()
	rec, err := e.adapter.Lookup(ctx, ids[0], "TH-DEMO-001", "hs.an.3a")
	if err != nil || rec["student_external_id"] != "HS-1" || rec["parent_phone"] != nil || s.CallsOf(fakesis.OpLookup) != 1 {
		t.Fatalf("lookup = %v %v, %d calls", rec, err, s.CallsOf(fakesis.OpLookup))
	}
	if _, err := e.adapter.Lookup(ctx, ids[0], "TH-DEMO-001", "nobody"); !errors.Is(err, port.ErrSisLookupNotFound) {
		t.Fatalf("404: %v", err)
	}
	s.Lock()
	s.Fail[fakesis.OpLookup] = []int{503}
	s.Unlock()
	n := s.CallsOf(fakesis.OpLookup)
	if _, err := e.adapter.Lookup(ctx, ids[0], "TH-DEMO-001", "hs.an.3a"); err == nil || errors.Is(err, port.ErrSisLookupNotFound) ||
		s.CallsOf(fakesis.OpLookup) != n+1 {
		t.Fatalf("5xx must be a temporary error without retry: %v", err)
	}
	s.Lock()
	s.Delay[fakesis.OpLookup] = time.Second
	s.Unlock()
	start := time.Now()
	if _, err := e.adapter.Lookup(ctx, ids[0], "TH-DEMO-001", "hs.an.3a"); !errors.Is(err, port.ErrSisLookupTimeout) || time.Since(start) > 900*time.Millisecond {
		t.Fatalf("timeout: %v after %v", err, time.Since(start))
	}
	// MONITORING §3: found + not found are ok, the 503 an error, the slow SIS a timeout; unsupported lookups below make
	// no call and are not counted.
	if ok, e5, to := sisReqT(observability.OutcomeOK)-ok0, sisReqT(observability.OutcomeError)-err0, sisReqT(observability.OutcomeTimeout)-to0; ok != 2 || e5 != 1 || to != 1 {
		t.Fatalf("requests: ok += %v, error += %v, timeout += %v; want 2, 1, 1", ok, e5, to)
	}
	// Unsupported: no student_lookup capability, or not active → no network call.
	n = s.CallsOf(fakesis.OpLookup)
	e.conns[ids[0]].Capabilities = []string{sis.CapFullSync}
	if _, err := e.adapter.Lookup(ctx, ids[0], "TH-DEMO-001", "hs.an.3a"); !errors.Is(err, port.ErrSisLookupUnsupported) {
		t.Fatalf("no capability: %v", err)
	}
	e.conns[ids[0]].Capabilities = []string{sis.CapStudentLookup}
	e.conns[ids[0]].State.Status = sis.StatusPaused
	if _, err := e.adapter.Lookup(ctx, ids[0], "TH-DEMO-001", "hs.an.3a"); !errors.Is(err, port.ErrSisLookupUnsupported) || s.CallsOf(fakesis.OpLookup) != n {
		t.Fatalf("paused: %v, calls %d → %d", err, n, s.CallsOf(fakesis.OpLookup))
	}
	if ok := sisReqT(observability.OutcomeOK) - ok0; ok != 2 {
		t.Fatalf("unsupported lookups counted: ok += %v", ok)
	}
}

// F017-AC-24 (TC-SIS-106): at most sis_login_sync_concurrency lookups per connection, a breaker per
// connection; a hanging SIS does not slow another connection.
func TestLookupBulkheadAndBreaker(t *testing.T) {
	slow, fast := fakesis.New(), fakesis.New() // MOCK
	defer slow.Close()
	defer fast.Close()
	for _, s := range []*fakesis.Server{slow, fast} {
		s.Lock()
		s.Lookups["hs.an.3a"] = fakesis.Student("HS-1", "TH-DEMO-001", "A1", "An", "hs.an.3a", "active")
		s.Unlock()
	}
	slow.Lock()
	slow.Delay[fakesis.OpLookup] = 300 * time.Millisecond
	slow.Unlock()
	e, ids := newEnv(t, 2*time.Second, 2, slow, fast)
	ctx := context.Background()
	sisReq := func(o string) float64 {
		return testutil.ToFloat64(observability.ExternalRequests().WithLabelValues("identity", observability.IntegrationSIS, o))
	}
	rejected := func() float64 {
		return testutil.ToFloat64(observability.ExternalBulkheadRejected().WithLabelValues("identity", observability.IntegrationSIS))
	}
	full0, rej0, open0 := sisReq(observability.OutcomeBulkheadFull), rejected(), sisReq(observability.OutcomeCircuitOpen)
	var wg sync.WaitGroup
	var throttled, ok atomic.Int32
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.adapter.Lookup(ctx, ids[0], "TH-DEMO-001", "hs.an.3a")
			switch {
			case errors.Is(err, lookup.ErrThrottled):
				throttled.Add(1)
			case err == nil:
				ok.Add(1)
			}
		}()
	}
	time.Sleep(50 * time.Millisecond)
	start := time.Now()
	if _, err := e.adapter.Lookup(ctx, ids[1], "TH-DEMO-001", "hs.an.3a"); err != nil || time.Since(start) > 200*time.Millisecond {
		t.Fatalf("the other connection is slowed: %v after %v", err, time.Since(start))
	}
	wg.Wait()
	if ok.Load() != 2 || throttled.Load() != 4 || slow.CallsOf(fakesis.OpLookup) != 2 {
		t.Fatalf("bulkhead: ok %d throttled %d calls %d (want 2/4/2)", ok.Load(), throttled.Load(), slow.CallsOf(fakesis.OpLookup))
	}
	// MONITORING §3: the 4 refused lookups in eduworld_external_client_bulkhead_rejected_total and as outcome bulkhead_full.
	if r, o := rejected()-rej0, sisReq(observability.OutcomeBulkheadFull)-full0; r != 4 || o != 4 {
		t.Fatalf("bulkhead metrics: rejected += %v, bulkhead_full += %v; want 4, 4", r, o)
	}
	// Breaker: 3 consecutive failures open the circuit of this connection only.
	slow.Lock()
	slow.Delay[fakesis.OpLookup] = 0
	slow.Fail[fakesis.OpLookup] = []int{500, 500, 500}
	slow.Unlock()
	for range 3 {
		_, _ = e.adapter.Lookup(ctx, ids[0], "TH-DEMO-001", "hs.an.3a")
	}
	calls := slow.CallsOf(fakesis.OpLookup)
	if _, err := e.adapter.Lookup(ctx, ids[0], "TH-DEMO-001", "hs.an.3a"); !errors.Is(err, lookup.ErrThrottled) || slow.CallsOf(fakesis.OpLookup) != calls {
		t.Fatalf("open circuit: %v", err)
	}
	if d := sisReq(observability.OutcomeCircuitOpen) - open0; d != 1 {
		t.Fatalf("circuit_open += %v, want 1", d)
	}
	if _, err := e.adapter.Lookup(ctx, ids[1], "TH-DEMO-001", "hs.an.3a"); err != nil {
		t.Fatalf("other connection: %v", err)
	}
}
