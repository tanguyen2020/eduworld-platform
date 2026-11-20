// Package ratelimit holds the per-connection limits of the gateway (F090-AC-22, AC-29, REALTIME_ARCHITECTURE §8,
// §8.1): a token bucket (ws_frame_rate, hub.player.move, ws_accept_rate) and a counter of violation episodes in a
// sliding minute (> 3 → close 4429; repeated protocol errors → 4400).
package ratelimit

import (
	"sync"
	"time"
)

// Bucket is a token bucket: rate tokens per second, capacity burst.
type Bucket struct {
	mu     sync.Mutex
	rate   float64
	burst  float64
	tokens float64
	last   time.Time
}

// NewBucket returns a full bucket.
func NewBucket(rate, burst int) *Bucket {
	if burst < rate {
		burst = rate
	}
	return &Bucket{rate: float64(rate), burst: float64(burst), tokens: float64(burst)}
}

// Allow takes one token at now; when empty it returns false and the wait until the next token.
func (b *Bucket) Allow(now time.Time) (bool, time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.last.IsZero() {
		b.tokens += now.Sub(b.last).Seconds() * b.rate
		if b.tokens > b.burst {
			b.tokens = b.burst
		}
	}
	b.last = now
	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	wait := time.Duration((1 - b.tokens) / b.rate * float64(time.Second))
	if wait < time.Millisecond {
		wait = time.Millisecond
	}
	return false, wait
}

// Episodes counts violation episodes (at most one per second) within the last minute.
type Episodes struct {
	mu   sync.Mutex
	secs []int64
}

// Add records a violation at now and returns the number of episodes in the last minute.
func (e *Episodes) Add(now time.Time) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := now.Unix()
	keep := e.secs[:0]
	for _, x := range e.secs {
		if s-x < 60 {
			keep = append(keep, x)
		}
	}
	e.secs = keep
	if len(e.secs) == 0 || e.secs[len(e.secs)-1] != s {
		e.secs = append(e.secs, s)
	}
	return len(e.secs)
}

// Events counts every event within the last minute (repeated protocol errors).
type Events struct {
	mu sync.Mutex
	at []time.Time
}

// Add records an event at now and returns the number of events in the last minute.
func (e *Events) Add(now time.Time) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	keep := e.at[:0]
	for _, x := range e.at {
		if now.Sub(x) < time.Minute {
			keep = append(keep, x)
		}
	}
	e.at = append(keep, now)
	return len(e.at)
}
