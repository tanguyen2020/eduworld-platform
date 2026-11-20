//go:build integration

// Package integration holds the integration tests of the realtime gateway (F090): real Valkey and NATS JetStream
// (testcontainers), gateway nodes on httptest servers, a headless WebSocket client (coder/websocket) and stub
// command handlers on NATS standing for the modules of core-api (TEST DOUBLES — R17: they answer like a module
// would, the gateway path under test is the real one).
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/valkey-io/valkey-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/conn"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/observability"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

type env struct {
	t       *testing.T
	vk      valkey.Client
	vkCtr   testcontainers.Container
	nc      *nats.Conn
	js      jetstream.JetStream
	natsURL string
	logs    *syncBuffer
}

// syncBuffer captures the logs of the gateway (F090-AC-07: no ticket, token or payload).
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func newEnv(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	ctr, addr := testkit.StartValkey(ctx, t)
	vk, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{addr}, DisableCache: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vk.Close)
	_, url := testkit.StartNATS(ctx, t)
	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	if err := audit.EnsureStream(ctx, js); err != nil {
		t.Fatal(err)
	}
	// Tracing on (no exporter) so traceparent travels in the NATS headers as in a binary with OTLP.
	otel.SetTracerProvider(sdktrace.NewTracerProvider())
	otel.SetTextMapPropagator(propagation.TraceContext{})
	logs := &syncBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &env{t: t, vk: vk, vkCtr: ctr, nc: nc, js: js, natsURL: url, logs: logs}
}

// node is one gateway node.
type node struct {
	id      string
	m       *conn.Manager
	srv     *httptest.Server
	metrics *observability.Metrics
	reg     *prometheus.Registry
	stop    context.CancelFunc
	done    chan struct{}
}

func (n *node) url() string { return "ws" + strings.TrimPrefix(n.srv.URL, "http") + conn.Path }

// testConfig is CONFIG_DEFAULTS shortened so the tests run in seconds.
func testConfig(id string) config.Config {
	c := config.Defaults(id)
	c.Heartbeat, c.Timeout, c.Ping = 15*time.Second, 45*time.Second, 20*time.Second
	c.CmdTimeout = 500 * time.Millisecond
	c.DrainWindow = 500 * time.Millisecond
	return c
}

func (e *env) node(id string, mut func(*config.Config)) *node {
	e.t.Helper()
	cfg := testConfig(id)
	if mut != nil {
		mut(&cfg)
	}
	reg := prometheus.NewRegistry()
	metrics := observability.New(reg, id)
	pub, err := audit.NewPublisher(e.js)
	if err != nil {
		e.t.Fatal(err)
	}
	m, err := conn.New(conn.Deps{Config: cfg, Valkey: e.vk, NATS: e.nc, Audit: pub, Metrics: metrics})
	if err != nil {
		e.t.Fatal(err)
	}
	ctx, stop := context.WithCancel(context.Background())
	n := &node{id: id, m: m, srv: httptest.NewServer(m), metrics: metrics, reg: reg, stop: stop, done: make(chan struct{})}
	go func() { _ = m.Run(ctx); close(n.done) }()
	e.t.Cleanup(func() {
		n.shutdown()
		n.srv.Close()
	})
	deadline := time.Now().Add(10 * time.Second)
	for m.Ready(ctx) != nil {
		if time.Now().After(deadline) {
			e.t.Fatal("gateway not ready")
		}
		time.Sleep(20 * time.Millisecond)
	}
	return n
}

func (n *node) shutdown() {
	n.stop()
	select {
	case <-n.done:
	case <-time.After(15 * time.Second):
	}
}

// student is the identity bound to a ticket.
type student struct {
	user, tenant, school, session, device uuid.UUID
}

func newStudent() student {
	return student{user: uuid.New(), tenant: uuid.New(), school: uuid.New(), session: uuid.New(), device: uuid.New()}
}

// ticket stores a ticket as module game does (SET NX EX of sha256, F090-AC-01) and returns it.
func (e *env) ticket(s student) string {
	e.t.Helper()
	tk := strings.ReplaceAll(uuid.NewString()+uuid.NewString(), "-", "")
	b, _ := json.Marshal(rt.TicketClaims{UserID: s.user, TenantID: s.tenant, SchoolID: s.school, GameSessionID: s.session,
		DeviceID: s.device, IssuedAt: time.Now().UnixMilli()})
	if err := e.vk.Do(context.Background(), e.vk.B().Set().Key(rt.TicketKey(rt.HashTicket(tk))).Value(string(b)).Nx().ExSeconds(60).Build()).Error(); err != nil {
		e.t.Fatal(err)
	}
	return tk
}

// publish sends an envelope on a channel as a module would.
func (e *env) publish(channel string, env rt.Envelope) {
	e.t.Helper()
	if err := (rt.Publisher{C: e.vk}).Publish(context.Background(), channel, env); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) message(typ string, payload map[string]any) rt.Envelope {
	e.t.Helper()
	m, err := rt.NewMessage(typ, payload, time.Now())
	if err != nil {
		e.t.Fatal(err)
	}
	return m
}

// numsub returns the number of Valkey subscribers of channel.
func (e *env) numsub(channel string) int64 {
	n, _ := e.vk.Do(context.Background(), e.vk.B().PubsubNumsub().Channel(channel).Build()).AsIntMap()
	return n[channel]
}

func (e *env) waitNumsub(channel string, want int64) {
	e.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for e.numsub(channel) != want {
		if time.Now().After(deadline) {
			e.t.Fatalf("numsub %s = %d, want %d", channel, e.numsub(channel), want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// stub answers ew.cmd.<module>.<type> like a module handler and records the calls.
type stub struct {
	mu    sync.Mutex
	calls []*nats.Msg
}

func (s *stub) n() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func (s *stub) last() *nats.Msg {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls[len(s.calls)-1]
}

func (e *env) stub(module, typ string, reply func(m *nats.Msg) rt.Reply) *stub {
	e.t.Helper()
	st := &stub{}
	sub, err := e.nc.QueueSubscribe(rt.CommandSubject(module, typ), module, func(m *nats.Msg) {
		st.mu.Lock()
		st.calls = append(st.calls, m)
		st.mu.Unlock()
		b, _ := json.Marshal(reply(m))
		_ = m.Respond(b)
	})
	if err != nil {
		e.t.Fatal(err)
	}
	_ = e.nc.Flush()
	e.t.Cleanup(func() { _ = sub.Unsubscribe() })
	return st
}

// client is a headless WebSocket client.
type client struct {
	t     *testing.T
	ws    *websocket.Conn
	msgs  chan msg
	pause chan struct{} // closed = reading; nil channel = reading always
	mu    sync.Mutex
	err   error
	done  chan struct{}
}

type msg struct {
	V       int             `json:"v"`
	Type    string          `json:"type"`
	Seq     int64           `json:"seq"`
	Ref     string          `json:"ref"`
	Ts      int64           `json:"ts"`
	Payload json.RawMessage `json:"payload"`
}

func (m msg) field(k string) any {
	var p map[string]any
	_ = json.Unmarshal(m.Payload, &p)
	return p[k]
}

type dialOpts struct {
	noProtocol bool
	origin     string
	rawQuery   string
}

func dialRaw(t *testing.T, n *node, ticket string, o dialOpts) (*websocket.Conn, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	opts := &websocket.DialOptions{Subprotocols: []string{"eduworld.v1"}}
	if o.noProtocol {
		opts.Subprotocols = nil
	}
	if o.origin != "" {
		opts.HTTPHeader = map[string][]string{"Origin": {o.origin}}
	}
	u := n.url() + "?ticket=" + ticket
	if o.rawQuery != "" {
		u = n.url() + "?" + o.rawQuery
	}
	ws, resp, err := websocket.Dial(ctx, u, opts)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	return ws, err
}

func dial(t *testing.T, n *node, ticket string) *client {
	t.Helper()
	ws, err := dialRaw(t, n, ticket, dialOpts{})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	ws.SetReadLimit(1 << 20)
	c := &client{t: t, ws: ws, msgs: make(chan msg, 1024), done: make(chan struct{})}
	go c.read()
	t.Cleanup(func() { _ = ws.CloseNow() })
	return c
}

func (c *client) read() {
	defer close(c.done)
	for {
		c.mu.Lock()
		p := c.pause
		c.mu.Unlock()
		if p != nil {
			<-p
		}
		_, data, err := c.ws.Read(context.Background())
		if err != nil {
			c.mu.Lock()
			c.err = err
			c.mu.Unlock()
			return
		}
		var m msg
		_ = json.Unmarshal(data, &m)
		c.msgs <- m
	}
}

// stopReading simulates a network cut: frames stay in the socket, the server keeps sending.
func (c *client) stopReading() {
	c.mu.Lock()
	c.pause = make(chan struct{})
	c.mu.Unlock()
}

// next returns the next message of type typ (others are skipped).
func (c *client) next(typ string) msg {
	c.t.Helper()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case m := <-c.msgs:
			if m.Type == typ {
				return m
			}
		case <-c.done:
			select {
			case m := <-c.msgs:
				if m.Type == typ {
					return m
				}
				continue
			default:
			}
			c.t.Fatalf("connection closed (%v) before %s", c.closeErr(), typ)
		case <-timeout:
			c.t.Fatalf("no %s", typ)
		}
	}
}

// none asserts that no message of type typ arrives within d.
func (c *client) none(typ string, d time.Duration) {
	c.t.Helper()
	timeout := time.After(d)
	for {
		select {
		case m := <-c.msgs:
			if m.Type == typ {
				c.t.Fatalf("unexpected %s: %s", typ, m.Payload)
			}
		case <-timeout:
			return
		}
	}
}

func (c *client) closeErr() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// closed waits for the close of the server and returns its code.
func (c *client) closed(within time.Duration) websocket.StatusCode {
	c.t.Helper()
	select {
	case <-c.done:
	case <-time.After(within):
		c.t.Fatalf("not closed within %v", within)
	}
	return websocket.CloseStatus(c.closeErr())
}

func (c *client) send(typ, id string, payload any) {
	c.t.Helper()
	b, _ := json.Marshal(map[string]any{"v": 1, "type": typ, "id": id, "ts": time.Now().UnixMilli(), "payload": payload})
	c.sendRaw(b)
}

func (c *client) sendRaw(b []byte) {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.ws.Write(ctx, websocket.MessageText, b); err != nil {
		c.t.Fatalf("send: %v", err)
	}
}

func newID() string { return uuid.Must(uuid.NewV7()).String() }

// counter returns the value of a metric with labels on node n.
func (n *node) counter(name string, labels map[string]string) float64 {
	mfs, _ := n.reg.Gather()
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
	next:
		for _, m := range mf.GetMetric() {
			for _, lp := range m.GetLabel() {
				if v, ok := labels[lp.GetName()]; ok && v != lp.GetValue() {
					continue next
				}
			}
			if m.GetCounter() != nil {
				return m.GetCounter().GetValue()
			}
			return m.GetGauge().GetValue()
		}
	}
	return 0
}
