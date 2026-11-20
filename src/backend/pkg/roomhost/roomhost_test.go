package roomhost_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/roomhost"
)

func TestTickRunsAtRate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 550*time.Millisecond)
	defer cancel()
	var n, observed atomic.Int32
	roomhost.Tick(ctx, 20, nil, func(time.Time) { n.Add(1) }, func(time.Duration) { observed.Add(1) })
	if got := n.Load(); got < 8 || got > 12 {
		t.Fatalf("%d ticks in 550 ms at 20 Hz", got)
	}
	if observed.Load() != n.Load() {
		t.Fatal("every tick is observed")
	}
}

func TestKeepLease(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	lost := make(chan struct{})
	go roomhost.KeepLease(ctx, 10*time.Millisecond, time.Second, func(context.Context) (bool, error) {
		return calls.Add(1) < 3, nil
	}, func() { close(lost) })
	select {
	case <-lost:
	case <-time.After(2 * time.Second):
		t.Fatal("lost lease not reported")
	}
	// transient errors are tolerated until ttl
	errs := make(chan struct{})
	go roomhost.KeepLease(ctx, 10*time.Millisecond, 100*time.Millisecond, func(context.Context) (bool, error) {
		return false, errors.New("valkey down")
	}, func() { close(errs) })
	start := time.Now()
	select {
	case <-errs:
		if time.Since(start) < 90*time.Millisecond {
			t.Fatal("lost before ttl on transient errors")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("never lost on a long outage")
	}
}
