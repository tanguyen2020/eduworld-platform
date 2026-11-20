package ratelimit_test

import (
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/ratelimit"
)

// F090-AC-22: 30/s burst 60 — the 61st immediate frame is refused with a wait; tokens come back at the rate.
func TestBucket(t *testing.T) {
	b := ratelimit.NewBucket(30, 60)
	now := time.Unix(1000, 0)
	for i := 0; i < 60; i++ {
		if ok, _ := b.Allow(now); !ok {
			t.Fatalf("frame %d refused", i)
		}
	}
	ok, wait := b.Allow(now)
	if ok || wait <= 0 || wait > time.Second/30+time.Millisecond {
		t.Fatalf("61st: %v %v", ok, wait)
	}
	if ok, _ := b.Allow(now.Add(100 * time.Millisecond)); !ok {
		t.Fatal("no token after 100 ms")
	}
}

// One episode per second, sliding minute.
func TestEpisodes(t *testing.T) {
	var e ratelimit.Episodes
	now := time.Unix(1000, 0)
	if n := e.Add(now); n != 1 {
		t.Fatal(n)
	}
	if n := e.Add(now.Add(100 * time.Millisecond)); n != 1 {
		t.Fatalf("same second: %d", n)
	}
	e.Add(now.Add(time.Second))
	e.Add(now.Add(2 * time.Second))
	if n := e.Add(now.Add(3 * time.Second)); n != 4 {
		t.Fatalf("4 seconds: %d", n)
	}
	if n := e.Add(now.Add(70 * time.Second)); n != 1 {
		t.Fatalf("after a minute: %d", n)
	}
}

// Every protocol error counts (F090-AC-20: repeated errors close 4400).
func TestEvents(t *testing.T) {
	var e ratelimit.Events
	now := time.Unix(1000, 0)
	for i := 1; i <= 4; i++ {
		if n := e.Add(now); n != i {
			t.Fatalf("event %d: %d", i, n)
		}
	}
	if n := e.Add(now.Add(61 * time.Second)); n != 1 {
		t.Fatalf("after a minute: %d", n)
	}
}
