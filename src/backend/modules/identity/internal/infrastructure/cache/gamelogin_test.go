package cache

import (
	"context"
	"errors"
	"testing"
	"time"
)

// downLimiter is a TEST DOUBLE of Valkey that always fails.
type downLimiter struct{}

func (downLimiter) Allow(context.Context, string, int, time.Duration) (bool, time.Duration, error) {
	return false, 0, errors.New("valkey down")
}

// F013 T024: with Valkey down the limit still applies, per key and per window, in the memory of the replica.
func TestFallbackLimiterWhenValkeyFails(t *testing.T) {
	now := time.Date(2026, 11, 19, 8, 0, 0, 0, time.UTC)
	l := NewFallbackLimiter(downLimiter{})
	l.now = func() time.Time { return now }
	ctx := context.Background()
	for i := range 3 {
		if ok, _, err := l.Allow(ctx, "d:1", 3, time.Minute); !ok || err != nil {
			t.Fatalf("request %d refused: %v", i, err)
		}
	}
	if ok, after, _ := l.Allow(ctx, "d:1", 3, time.Minute); ok || after <= 0 {
		t.Fatal("4th request allowed")
	}
	if ok, _, _ := l.Allow(ctx, "d:2", 3, time.Minute); !ok {
		t.Fatal("another key limited")
	}
	now = now.Add(time.Minute)
	if ok, _, _ := l.Allow(ctx, "d:1", 3, time.Minute); !ok {
		t.Fatal("new window refused")
	}
}
