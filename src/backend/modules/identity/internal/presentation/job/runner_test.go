package job

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestEvery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var runs atomic.Int32
	done := make(chan struct{})
	go func() {
		Every(ctx, "test", 5*time.Millisecond, func(context.Context) (int, error) {
			if runs.Add(1) == 2 {
				return 0, errors.New("transient")
			}
			return 1, nil
		})
		close(done)
	}()
	for runs.Load() < 4 {
		time.Sleep(2 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Every did not stop with its context")
	}
}
