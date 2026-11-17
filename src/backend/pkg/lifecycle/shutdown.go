// Package lifecycle is the graceful shutdown of every binary (F003 US6, BACKEND_DESIGN §13,
// NFR-AVAIL-004): readiness off → drain delay → stop workers (consumers drain and ack what they
// finished) → shut down HTTP servers (in-flight requests complete) → drain NATS → close pools.
package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// DefaultTimeout is EW_HTTP_SHUTDOWN_TIMEOUT when unset.
const DefaultTimeout = 30 * time.Second

// Group runs background workers (relay, consumers, jobs) with a context cancelled by Stop.
type Group struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
	errs   []error
}

// NewGroup derives the workers' context from parent.
func NewGroup(parent context.Context) *Group {
	ctx, cancel := context.WithCancel(parent)
	return &Group{ctx: ctx, cancel: cancel}
}

// Go starts fn; fn must return when its context is cancelled after finishing in-flight work.
func (g *Group) Go(name string, fn func(ctx context.Context) error) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		if err := fn(g.ctx); err != nil && !errors.Is(err, context.Canceled) {
			g.mu.Lock()
			g.errs = append(g.errs, fmt.Errorf("%s: %w", name, err))
			g.mu.Unlock()
		}
	}()
}

// Stop cancels the workers and waits for them up to timeout.
func (g *Group) Stop(timeout time.Duration) error {
	g.cancel()
	done := make(chan struct{})
	go func() { g.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(timeout):
		return errors.New("lifecycle: workers did not stop in time")
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return errors.Join(g.errs...)
}

// Draining is the readiness switch (health.Registry).
type Draining interface{ SetDraining(bool) }

// Drainer is a NATS connection.
type Drainer interface{ Drain() error }

// Plan is the shutdown sequence of one binary.
type Plan struct {
	Readiness  Draining
	DrainDelay time.Duration // let the load balancer see 503 before closing (0 in tests)
	Timeout    time.Duration // EW_HTTP_SHUTDOWN_TIMEOUT
	Workers    *Group
	Public     []*http.Server // public API (and gateway-internal routes on the same server)
	Admin      *http.Server   // health/metrics: closed last
	NATS       Drainer
	Close      []func() // pools, clients
	Logger     *slog.Logger
}

// Run executes the plan; it always runs every step and returns the joined errors.
func (p Plan) Run() error {
	if p.Timeout <= 0 {
		p.Timeout = DefaultTimeout
	}
	log := p.Logger
	if log == nil {
		log = slog.Default()
	}
	var errs []error
	if p.Readiness != nil {
		p.Readiness.SetDraining(true)
	}
	if p.DrainDelay > 0 {
		time.Sleep(p.DrainDelay)
	}
	if p.Workers != nil {
		if err := p.Workers.Stop(p.Timeout); err != nil {
			errs = append(errs, err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), p.Timeout)
	defer cancel()
	for _, s := range p.Public {
		if s == nil {
			continue
		}
		if err := s.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("lifecycle: http shutdown: %w", err))
		}
	}
	if p.NATS != nil {
		if err := p.NATS.Drain(); err != nil {
			errs = append(errs, fmt.Errorf("lifecycle: nats drain: %w", err))
		}
	}
	for _, c := range p.Close {
		c()
	}
	if p.Admin != nil {
		_ = p.Admin.Shutdown(ctx)
	}
	err := errors.Join(errs...)
	if err != nil {
		log.Error("shutdown finished with errors", "error", err)
	} else {
		log.Info("shutdown complete")
	}
	return err
}
