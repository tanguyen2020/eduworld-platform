// Package subs maps Valkey Pub/Sub channels to the local receivers of a gateway node (F090-AC-19, AC-21, AC-30,
// REALTIME_ARCHITECTURE §2.1 "Subscriptions"): one dedicated Valkey connection per node, SUBSCRIBE when the first
// local receiver needs a channel and UNSUBSCRIBE when the last one leaves (reference count by node). When the
// connection breaks the hub reports degraded, reconnects with backoff and subscribes every channel again.
package subs

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/valkey-io/valkey-go"

	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// Receiver gets the messages of its channels; Deliver must not block (it runs on the Pub/Sub goroutine).
type Receiver interface {
	Deliver(channel string, data []byte)
}

// EnvelopeReceiver gets the envelope decoded once per node message (a room snapshot bundle goes to up to 40 local
// connections, F091-AC-14); invalid envelopes are dropped before any receiver sees them.
type EnvelopeReceiver interface {
	DeliverEnvelope(channel string, e *rt.Envelope)
}

// Hub is the per-node subscription table.
type Hub struct {
	C       valkey.Client
	OnState func(up bool) // called on every transition (degraded / recovered)

	mu      sync.Mutex
	topics  map[string]map[Receiver]struct{}
	dc      valkey.DedicatedClient
	release func()
	up      bool
}

// NewHub creates a hub over c.
func NewHub(c valkey.Client, onState func(up bool)) *Hub {
	return &Hub{C: c, OnState: onState, topics: map[string]map[Receiver]struct{}{}}
}

// Healthy reports a live Pub/Sub connection.
func (h *Hub) Healthy() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.up
}

// ErrDegraded is returned by Subscribe while the Pub/Sub connection is down (the channel is still recorded and
// subscribed again on recovery).
var ErrDegraded = errors.New("subs: Valkey Pub/Sub unavailable")

// Subscribe adds r to channel, subscribing in Valkey when r is the first local receiver.
func (h *Hub) Subscribe(ctx context.Context, channel string, r Receiver) error {
	h.mu.Lock()
	set, ok := h.topics[channel]
	if !ok {
		set = map[Receiver]struct{}{}
		h.topics[channel] = set
	}
	set[r] = struct{}{}
	dc, up := h.dc, h.up
	h.mu.Unlock()
	if ok {
		return nil
	}
	if !up || dc == nil {
		return ErrDegraded
	}
	return dc.Do(ctx, dc.B().Subscribe().Channel(channel).Build()).Error()
}

// Unsubscribe removes r from channel, unsubscribing in Valkey when no local receiver is left.
func (h *Hub) Unsubscribe(ctx context.Context, channel string, r Receiver) {
	h.mu.Lock()
	set, ok := h.topics[channel]
	if ok {
		delete(set, r)
	}
	last := ok && len(set) == 0
	if last {
		delete(h.topics, channel)
	}
	dc, up := h.dc, h.up
	h.mu.Unlock()
	if last && up && dc != nil {
		_ = dc.Do(ctx, dc.B().Unsubscribe().Channel(channel).Build()).Error()
	}
}

// Count returns the number of local receivers of channel (tests, metrics).
func (h *Hub) Count(channel string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.topics[channel])
}

func (h *Hub) deliver(m valkey.PubSubMessage) {
	h.mu.Lock()
	set := h.topics[m.Channel]
	rs := make([]Receiver, 0, len(set))
	for r := range set {
		rs = append(rs, r)
	}
	h.mu.Unlock()
	data := []byte(m.Message)
	var env *rt.Envelope
	decoded := false
	for _, r := range rs {
		er, ok := r.(EnvelopeReceiver)
		if !ok {
			r.Deliver(m.Channel, data)
			continue
		}
		if !decoded {
			decoded = true
			var e rt.Envelope
			if json.Unmarshal(data, &e) == nil && e.Validate() == nil {
				env = &e
			} else {
				slog.Debug("realtime: invalid pubsub envelope dropped")
			}
		}
		if env != nil {
			er.DeliverEnvelope(m.Channel, env)
		}
	}
}

func (h *Hub) setState(up bool) {
	h.mu.Lock()
	changed := h.up != up
	h.up = up
	h.mu.Unlock()
	if changed && h.OnState != nil {
		h.OnState(up)
	}
}

// Run keeps the dedicated Pub/Sub connection until ctx ends.
func (h *Hub) Run(ctx context.Context) error {
	backoff := 200 * time.Millisecond
	for ctx.Err() == nil {
		wait, err := h.connect(ctx)
		if err != nil {
			h.setState(false)
			slog.WarnContext(ctx, "realtime pubsub connect failed", "error", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(backoff):
			}
			if backoff < 5*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = 200 * time.Millisecond
		h.setState(true)
		select {
		case <-ctx.Done():
			h.close()
			return nil
		case err := <-wait:
			h.close()
			if err != nil { // a closed channel without error only means the hooks were reset: reconnect silently
				h.setState(false)
				slog.WarnContext(ctx, "realtime pubsub connection lost", "error", err)
			}
		}
	}
	return nil
}

func (h *Hub) connect(ctx context.Context) (<-chan error, error) {
	dc, release := h.C.Dedicate()
	wait := dc.SetPubSubHooks(valkey.PubSubHooks{OnMessage: h.deliver})
	h.mu.Lock()
	channels := make([]string, 0, len(h.topics))
	for c := range h.topics {
		channels = append(channels, c)
	}
	h.dc, h.release = dc, release
	h.mu.Unlock()
	// A PING proves the connection; then every recorded channel is subscribed again (F090-AC-30).
	cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := dc.Do(cctx, dc.B().Ping().Build()).Error(); err != nil {
		h.close()
		return nil, err
	}
	if len(channels) > 0 {
		if err := dc.Do(cctx, dc.B().Subscribe().Channel(channels...).Build()).Error(); err != nil {
			h.close()
			return nil, err
		}
	}
	return wait, nil
}

func (h *Hub) close() {
	h.mu.Lock()
	release := h.release
	h.dc, h.release = nil, nil
	h.mu.Unlock()
	if release != nil {
		release()
	}
}
