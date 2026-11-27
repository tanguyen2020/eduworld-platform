// Package wsclient is the realtime connection of a load bot and of the anti-cheat runner (F132), following specs/asyncapi/eduworld-realtime.v1.yaml:
// one WebSocket on the ticket URL (?ticket=, subprotocol eduworld.v1), client envelopes {v, type, id, ts, payload},
// server envelopes {v, type, seq, ref, ts, payload}, heartbeat ping every heartbeat_s of system.session.welcome,
// commands answered by system.command.acked/failed with ref = id. Message payloads are never logged (AGENTS R14).
package wsclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/gameclient/ids"
)

// Subprotocol of the realtime gateway.
const Subprotocol = "eduworld.v1"

// ErrFailed is a command answered by system.command.failed (Code holds the stable code).
var ErrFailed = errors.New("wsclient: command failed")

// Envelope is a server message.
type Envelope struct {
	V       int             `json:"v"`
	Type    string          `json:"type"`
	Seq     int64           `json:"seq"`
	Ref     string          `json:"ref,omitempty"`
	Ts      int64           `json:"ts"`
	Payload json.RawMessage `json:"payload"`
}

// FailedError carries the code of system.command.failed.
type FailedError struct{ Code string }

func (e *FailedError) Error() string { return "wsclient: command failed: " + e.Code }
func (e *FailedError) Unwrap() error { return ErrFailed }

// Conn is one realtime connection.
type Conn struct {
	ws       *websocket.Conn
	mu       sync.Mutex
	pending  map[string]chan Envelope
	handlers sync.Map // type → func(Envelope)
	welcome  chan Envelope
	hello    Envelope // system.session.welcome of this connection (resume_token)
	lastSeq  atomic.Int64
	gaps     atomic.Int64
	received atomic.Int64
	sent     atomic.Int64
	done     chan struct{}
	err      error
	now      func() time.Time
}

// Dial opens the connection and waits for system.session.welcome; heartbeats start at the announced interval.
func Dial(ctx context.Context, rawURL, ticket string) (*Conn, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("ticket", ticket)
	u.RawQuery = q.Encode()
	ws, resp, err := websocket.Dial(ctx, u.String(), &websocket.DialOptions{Subprotocols: []string{Subprotocol}})
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("wsclient: dial: %w", err)
	}
	ws.SetReadLimit(1 << 20)
	c := &Conn{ws: ws, pending: map[string]chan Envelope{}, welcome: make(chan Envelope, 1), done: make(chan struct{}), now: time.Now}
	go c.readLoop()
	select {
	case w := <-c.welcome:
		c.hello = w
		var p struct {
			HeartbeatS int `json:"heartbeat_s"`
		}
		_ = json.Unmarshal(w.Payload, &p)
		if p.HeartbeatS <= 0 {
			p.HeartbeatS = 15
		}
		go c.heartbeat(time.Duration(p.HeartbeatS) * time.Second)
		return c, nil
	case <-c.done:
		return nil, fmt.Errorf("wsclient: closed before welcome: %w", c.err)
	case <-ctx.Done():
		_ = ws.Close(websocket.StatusNormalClosure, "")
		return nil, ctx.Err()
	}
}

// On registers the handler of a server message type (called on the read goroutine; keep it short).
func (c *Conn) On(typ string, fn func(Envelope)) { c.handlers.Store(typ, fn) }

// Send writes a client envelope and returns its id.
func (c *Conn) Send(ctx context.Context, typ string, payload any) (string, error) {
	id := ids.V7(c.now())
	return id, c.write(ctx, id, typ, payload)
}

// Request sends a command and waits for its ack (system.command.acked) or failure; it returns the round trip time.
func (c *Conn) Request(ctx context.Context, typ string, payload any, timeout time.Duration) (Envelope, time.Duration, error) {
	return c.RequestID(ctx, ids.V7(c.now()), typ, payload, timeout)
}

// SendID writes a client envelope with a caller-chosen id (replay scenarios of the anti-cheat runner, ANTI_CHEAT §4).
func (c *Conn) SendID(ctx context.Context, id, typ string, payload any) error {
	return c.write(ctx, id, typ, payload)
}

// RequestID is Request with a caller-chosen id: a replayed id must get the first reply back, never run twice
// (ANTI_CHEAT §4, AC-CMP-210).
func (c *Conn) RequestID(ctx context.Context, id, typ string, payload any, timeout time.Duration) (Envelope, time.Duration, error) {
	ch := make(chan Envelope, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()
	t0 := c.now()
	if err := c.write(ctx, id, typ, payload); err != nil {
		return Envelope{}, 0, err
	}
	tm := time.NewTimer(timeout)
	defer tm.Stop()
	select {
	case e := <-ch:
		rtt := c.now().Sub(t0)
		if e.Type == "system.command.failed" {
			var p struct {
				Code string `json:"code"`
			}
			_ = json.Unmarshal(e.Payload, &p)
			return e, rtt, &FailedError{Code: p.Code}
		}
		return e, rtt, nil
	case <-tm.C:
		return Envelope{}, 0, fmt.Errorf("wsclient: %s: timeout after %s", typ, timeout)
	case <-c.done:
		return Envelope{}, 0, fmt.Errorf("wsclient: closed: %w", c.err)
	case <-ctx.Done():
		return Envelope{}, 0, ctx.Err()
	}
}

func (c *Conn) write(ctx context.Context, id, typ string, payload any) error {
	if payload == nil {
		payload = map[string]any{}
	}
	b, err := json.Marshal(map[string]any{"v": 1, "type": typ, "id": id, "ts": c.now().UnixMilli(), "payload": payload})
	if err != nil {
		return err
	}
	if err := c.ws.Write(ctx, websocket.MessageText, b); err != nil {
		return fmt.Errorf("wsclient: write %s: %w", typ, err)
	}
	c.sent.Add(1)
	return nil
}

func (c *Conn) readLoop() {
	defer close(c.done)
	for {
		_, b, err := c.ws.Read(context.Background())
		if err != nil {
			c.err = err
			return
		}
		var e Envelope
		if json.Unmarshal(b, &e) != nil {
			continue
		}
		c.received.Add(1)
		if prev := c.lastSeq.Swap(e.Seq); prev != 0 && e.Seq > prev+1 {
			c.gaps.Add(1)
		}
		if e.Type == "system.session.welcome" {
			select {
			case c.welcome <- e:
			default:
			}
		}
		if e.Ref != "" && (e.Type == "system.command.acked" || e.Type == "system.command.failed") {
			c.mu.Lock()
			ch := c.pending[e.Ref]
			c.mu.Unlock()
			if ch != nil {
				ch <- e
			}
		}
		if fn, ok := c.handlers.Load(e.Type); ok {
			fn.(func(Envelope))(e)
		}
	}
}

func (c *Conn) heartbeat(every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-t.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, _ = c.Send(ctx, "system.heartbeat.ping", map[string]any{"client_time": c.now().UnixMilli()})
			cancel()
		}
	}
}

// Welcome is the system.session.welcome of the connection (resume_token, resume_window_s for system.session.resume).
func (c *Conn) Welcome() Envelope { return c.hello }

// Done is closed when the connection ends.
func (c *Conn) Done() <-chan struct{} { return c.done }

// Stats returns the received/sent message counts and the seq gaps seen.
func (c *Conn) Stats() (received, sent, gaps int64) {
	return c.received.Load(), c.sent.Load(), c.gaps.Load()
}

// Close closes the connection normally.
func (c *Conn) Close() error { return c.ws.Close(websocket.StatusNormalClosure, "bye") }
