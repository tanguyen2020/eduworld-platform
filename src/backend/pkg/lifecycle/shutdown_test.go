package lifecycle_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/health"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/lifecycle"
)

type fakeNATS struct{ drained atomic.Bool }

func (f *fakeNATS) Drain() error { f.drained.Store(true); return nil }

// F003-AC-29: on shutdown readiness turns 503 first, the in-flight request completes, the server
// closes, NATS is drained and pools closed.
func TestAPIShutdown(t *testing.T) {
	hr := health.NewRegistry(buildinfo.Get("core-api", ""))
	adminLn, _ := net.Listen("tcp", "127.0.0.1:0")
	admin := &http.Server{Handler: hr.Handler(), ReadHeaderTimeout: time.Second}
	go func() { _ = admin.Serve(adminLn) }()

	started := make(chan struct{})
	release := make(chan struct{})
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	public := &http.Server{ReadHeaderTimeout: time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		_, _ = io.WriteString(w, "done")
	})}
	go func() { _ = public.Serve(ln) }()

	type result struct {
		status int
		body   string
		err    error
	}
	inflight := make(chan result, 1)
	go func() {
		resp, err := http.Get("http://" + ln.Addr().String() + "/slow")
		if err != nil {
			inflight <- result{err: err}
			return
		}
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		inflight <- result{status: resp.StatusCode, body: string(b)}
	}()
	<-started

	nc := &fakeNATS{}
	closed := atomic.Bool{}
	workerStopped := atomic.Bool{}
	g := lifecycle.NewGroup(context.Background())
	g.Go("consumer", func(ctx context.Context) error { <-ctx.Done(); workerStopped.Store(true); return nil })
	done := make(chan error, 1)
	go func() {
		done <- lifecycle.Plan{Readiness: hr, DrainDelay: 200 * time.Millisecond, Timeout: 5 * time.Second, Workers: g,
			Public: []*http.Server{public}, NATS: nc, Close: []func(){func() { closed.Store(true) }}}.Run()
	}()

	time.Sleep(50 * time.Millisecond)
	resp, err := http.Get("http://" + adminLn.Addr().String() + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("readyz during drain = %d", resp.StatusCode)
	}
	close(release)
	if r := <-inflight; r.err != nil || r.status != 200 || r.body != "done" {
		t.Fatalf("in-flight request: %+v", r)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !nc.drained.Load() || !closed.Load() || !workerStopped.Load() {
		t.Fatalf("drained=%v closed=%v worker=%v", nc.drained.Load(), closed.Load(), workerStopped.Load())
	}
	if _, err := http.Get("http://" + ln.Addr().String() + "/slow"); err == nil {
		t.Fatal("server still accepts requests after shutdown")
	}
	_ = admin.Close()
}

func TestWorkersTimeout(t *testing.T) {
	g := lifecycle.NewGroup(context.Background())
	g.Go("stuck", func(context.Context) error { time.Sleep(time.Second); return nil })
	if err := g.Stop(50 * time.Millisecond); err == nil {
		t.Fatal("stuck worker not reported")
	}
}
