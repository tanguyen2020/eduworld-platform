package loginsync

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// TEST DOUBLES (R17): the real state, lookup, applier and queue are covered by integration tests.

type uow struct{}

func (uow) Do(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

type fakeState struct {
	mu sync.Mutex
	st *write.LoginState
}

func (f *fakeState) LoginState(context.Context, uuid.UUID) (*write.LoginState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.st == nil {
		return nil, nil
	}
	c := *f.st
	return &c, nil
}

func (f *fakeState) LoginStateBySource(ctx context.Context, _ uuid.UUID, _ string) (*write.LoginState, error) {
	return f.LoginState(ctx, uuid.Nil)
}

type fakeLookup struct {
	calls, inFlight, peak atomic.Int32
	delay                 time.Duration
	err                   error
}

func (f *fakeLookup) Lookup(ctx context.Context, _ uuid.UUID, _, _ string) (map[string]any, error) {
	f.calls.Add(1)
	n := f.inFlight.Add(1)
	defer f.inFlight.Add(-1)
	for {
		p := f.peak.Load()
		if n <= p || f.peak.CompareAndSwap(p, n) {
			break
		}
	}
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if f.err != nil {
		return nil, f.err
	}
	return map[string]any{"student_external_id": "HS-1"}, nil
}

type fakeApply struct {
	state  *fakeState
	enroll bool
	calls  int
}

func (f *fakeApply) ApplyLookup(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, map[string]any) (command.RunImportResult, error) {
	f.state.mu.Lock()
	defer f.state.mu.Unlock()
	f.calls++
	if f.enroll {
		f.state.st.Placed = true
	}
	return command.RunImportResult{}, nil
}

type fakeJobs struct{ jobs []port.LoginRefreshJob }

func (f *fakeJobs) EnqueueLoginRefresh(_ context.Context, j port.LoginRefreshJob) error {
	f.jobs = append(f.jobs, j)
	return nil
}

type fakeMetrics struct {
	mu   sync.Mutex
	seen map[string]int
}

func (f *fakeMetrics) Observe(branch, result string, _ time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seen[branch+"/"+result]++
}

var now = time.Date(2026, 11, 20, 7, 0, 0, 0, time.UTC)

func fixture(st write.LoginState) (LoginSync, *fakeState, *fakeLookup, *fakeApply, *fakeJobs, *fakeMetrics) {
	fs := &fakeState{st: &st}
	fl, fa, fj, fm := &fakeLookup{}, &fakeApply{state: fs}, &fakeJobs{}, &fakeMetrics{seen: map[string]int{}}
	return LoginSync{UoW: uow{}, State: fs, Lookup: fl, Apply: fa, Jobs: fj, Metrics: fm, Guard: NewGuard(20, 5, 30*time.Second),
		Clock: func() time.Time { return now }}, fs, fl, fa, fj, fm
}

func synced(ago time.Duration) *time.Time {
	t := now.Add(-ago)
	return &t
}

// F011-AC-18: complete and fresh → no SIS call.
func TestBranchFresh(t *testing.T) {
	s, _, fl, _, fj, fm := fixture(write.LoginState{Placed: true, LastSyncedAt: synced(time.Hour), StudentLookup: true, SchoolExternalID: "TH"})
	res, err := s.Ensure(context.Background(), uuid.New())
	if err != nil || res.Branch != BranchFresh || fl.calls.Load() != 0 || len(fj.jobs) != 0 || fm.seen["fresh/ok"] != 1 {
		t.Fatalf("fresh = %+v %v", res, err)
	}
}

// F011-AC-21: complete but stale → login continues, one job queued (dedupe by Nats-Msg-Id in the stream).
func TestBranchBackground(t *testing.T) {
	conn := uuid.New()
	s, _, fl, _, fj, _ := fixture(write.LoginState{ConnectionID: conn, ExternalID: "HS-1", Placed: true, LastSyncedAt: synced(25 * time.Hour)})
	res, err := s.Ensure(context.Background(), uuid.New())
	if err != nil || res.Branch != BranchBackground || fl.calls.Load() != 0 || len(fj.jobs) != 1 || fj.jobs[0].ConnectionID != conn {
		t.Fatalf("background = %+v %v jobs=%v", res, err, fj.jobs)
	}
}

// F011-AC-20: a connection without student_lookup (csv_import) is never called; missing class → 403.
func TestBranchBlockingWithoutLookup(t *testing.T) {
	s, _, fl, _, _, _ := fixture(write.LoginState{StudentLookup: false, SchoolExternalID: "TH"})
	_, err := s.Ensure(context.Background(), uuid.New())
	var pe *problem.Error
	if !errors.As(err, &pe) || pe.Code != problem.NoActiveEnrollment || fl.calls.Load() != 0 {
		t.Fatalf("csv_import = %v calls=%d", err, fl.calls.Load())
	}
}

// F011-AC-19: blocking lookup, then the pipeline; with a class → ok, still no class → NO_ACTIVE_ENROLLMENT.
func TestBranchBlockingLookup(t *testing.T) {
	s, _, fl, fa, _, fm := fixture(write.LoginState{StudentLookup: true, SchoolExternalID: "TH"})
	fa.enroll = true
	res, err := s.Ensure(context.Background(), uuid.New())
	if err != nil || res.Outcome != ResultOK || fl.calls.Load() != 1 || fa.calls != 1 || fm.seen["blocking/ok"] != 1 {
		t.Fatalf("lookup ok = %+v %v", res, err)
	}
	s2, _, _, fa2, _, _ := fixture(write.LoginState{StudentLookup: true, SchoolExternalID: "TH"})
	res, err = s2.Ensure(context.Background(), uuid.New())
	var pe *problem.Error
	if !errors.As(err, &pe) || pe.Code != problem.NoActiveEnrollment || res.Outcome != ResultMissingPlacement || fa2.calls != 1 {
		t.Fatalf("still no class = %+v %v", res, err)
	}
}

// F011-AC-20: timeout → last synced data, NO_ACTIVE_ENROLLMENT with a retry message, no synchronous retry.
func TestBranchBlockingTimeout(t *testing.T) {
	s, _, fl, fa, _, _ := fixture(write.LoginState{StudentLookup: true, SchoolExternalID: "TH"})
	fl.delay, s.Timeout = time.Second, 20*time.Millisecond
	res, err := s.Ensure(context.Background(), uuid.New())
	var pe *problem.Error
	if !errors.As(err, &pe) || pe.Code != problem.NoActiveEnrollment || res.Outcome != ResultTimeout || fl.calls.Load() != 1 || fa.calls != 0 {
		t.Fatalf("timeout = %+v %v", res, err)
	}
}

// F011-AC-22: 200 concurrent blocking logins on one connection: at most sis_login_sync_concurrency lookups
// in flight, the rest throttled; the breaker opens after consecutive failures.
func TestGuardConcurrencyAndBreaker(t *testing.T) {
	s, _, fl, _, _, fm := fixture(write.LoginState{ConnectionID: uuid.New(), StudentLookup: true, SchoolExternalID: "TH"})
	fl.delay = 50 * time.Millisecond
	var wg sync.WaitGroup
	for range 200 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.Ensure(context.Background(), uuid.New())
		}()
	}
	wg.Wait()
	if p := fl.peak.Load(); p > 20 || p == 0 {
		t.Fatalf("peak concurrent lookups = %d", p)
	}
	if fm.seen["blocking/throttled"] == 0 || int(fl.calls.Load())+fm.seen["blocking/throttled"] != 200 {
		t.Fatalf("calls=%d throttled=%d", fl.calls.Load(), fm.seen["blocking/throttled"])
	}

	g := NewGuard(20, 3, 30*time.Second)
	conn := uuid.New()
	for range 3 {
		release, ok := g.Acquire(conn, now)
		if !ok {
			t.Fatal("closed circuit refused")
		}
		release(false, now)
	}
	if _, ok := g.Acquire(conn, now.Add(time.Second)); ok {
		t.Fatal("the circuit must be open after 3 failures")
	}
	if _, ok := g.Acquire(conn, now.Add(31*time.Second)); !ok {
		t.Fatal("the circuit must close after openFor")
	}
}

func TestNotASisStudent(t *testing.T) {
	s, fs, fl, _, _, _ := fixture(write.LoginState{})
	fs.st = nil
	if res, err := s.Ensure(context.Background(), uuid.New()); err != nil || res.Branch != BranchFresh || fl.calls.Load() != 0 {
		t.Fatalf("no link = %+v %v", res, err)
	}
}
