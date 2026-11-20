// Package conn is the connection manager of the gateway (F090, REALTIME_ARCHITECTURE §2.1 ConnManager/Session):
// handshake on /realtime/v1 (subprotocol, Origin, ticket GETDEL), one effective connection per student across nodes,
// sessions (seq, send queue, heartbeat, resume, routing, rate limits), node-level channels, Valkey degradation and
// graceful drain. No business logic and no database (ADR-0074).
package conn

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/presence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/protocol"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/ratelimit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/registry"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/resume"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/revocation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/router"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/subs"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/ticket"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// Path is the WebSocket endpoint.
const Path = "/realtime/v1"

// Deps are the resources of the manager.
type Deps struct {
	Config  config.Config
	Valkey  valkey.Client
	NATS    *nats.Conn
	Audit   *audit.Publisher // nil: no audit (tests)
	Metrics *observability.Metrics
	Now     func() time.Time
}

// Manager accepts and owns the connections of one gateway node.
type Manager struct {
	cfg       config.Config
	vk        valkey.Client
	tickets   ticket.Consumer
	registry  registry.Registry
	resume    resume.Store
	router    router.Router
	hub       *subs.Hub
	audit     *audit.Publisher
	metrics   *observability.Metrics
	validator *protocol.Validator
	presence  *presence.Service
	clock     func() time.Time
	accept    *ratelimit.Bucket

	base     context.Context
	stop     context.CancelFunc
	mu       sync.Mutex
	conns    map[string]*Session
	draining atomic.Bool
	wg       sync.WaitGroup
}

// New builds the manager.
func New(d Deps) (*Manager, error) {
	if err := d.Config.Validate(); err != nil {
		return nil, err
	}
	if d.Valkey == nil {
		return nil, errors.New("conn: Valkey is required")
	}
	v, err := protocol.NewValidator()
	if err != nil {
		return nil, err
	}
	if d.Metrics == nil {
		d.Metrics = observability.New(nil, d.Config.NodeID)
	}
	base, stop := context.WithCancel(context.Background())
	m := &Manager{cfg: d.Config, vk: d.Valkey, tickets: ticket.Consumer{C: d.Valkey},
		registry: registry.Registry{C: d.Valkey, TTL: d.Config.PresenceTTL},
		resume:   resume.Store{C: d.Valkey, Window: d.Config.ResumeWindow, Max: d.Config.ResumeMax},
		router:   router.Router{NC: d.NATS, Timeout: d.Config.CmdTimeout}, audit: d.Audit, metrics: d.Metrics, validator: v,
		clock: d.Now, accept: ratelimit.NewBucket(d.Config.AcceptRate, d.Config.AcceptRate), base: base, stop: stop,
		conns: map[string]*Session{}}
	m.hub = subs.NewHub(d.Valkey, m.onHubState)
	// F091 US3: presence of the connected students (Valkey ew:gateway:presence:*, signal ew.rt.presence.<tenant>).
	m.presence = presence.New(d.Valkey, d.NATS, d.Config.PresenceTTL, d.Config.Timeout, m.now, func(ctx context.Context, user uuid.UUID) (bool, error) {
		h, err := m.registry.Holder(ctx, user)
		return h != nil, err
	})
	return m, nil
}

func (m *Manager) now() time.Time {
	if m.clock != nil {
		return m.clock()
	}
	return time.Now()
}

// Ready is the readiness check of the gateway: not draining and Valkey Pub/Sub up (F090-AC-27, AC-30).
func (m *Manager) Ready(context.Context) error {
	if m.draining.Load() {
		return errors.New("draining")
	}
	if !m.hub.Healthy() {
		return errors.New("valkey pubsub unavailable")
	}
	return nil
}

// Count returns the number of local connections.
func (m *Manager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.conns)
}

func (m *Manager) add(s *Session) {
	m.mu.Lock()
	m.conns[s.caller.ConnID] = s
	n := len(m.conns)
	m.mu.Unlock()
	m.metrics.Connections.WithLabelValues(m.cfg.NodeID).Set(float64(n))
}

func (m *Manager) remove(s *Session) {
	m.mu.Lock()
	if m.conns[s.caller.ConnID] == s {
		delete(m.conns, s.caller.ConnID)
	}
	n := len(m.conns)
	m.mu.Unlock()
	m.metrics.Connections.WithLabelValues(m.cfg.NodeID).Set(float64(n))
}

func (m *Manager) sessions() []*Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Session, 0, len(m.conns))
	for _, s := range m.conns {
		out = append(out, s)
	}
	return out
}

func (m *Manager) get(connID string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.conns[connID]
}

// Run keeps the Pub/Sub connection, the node-level channels (ew:rt:node:<id>:in, ew:rt:broadcast) and the node
// liveness until ctx ends, then drains (F090-AC-27).
func (m *Manager) Run(ctx context.Context) error {
	hubCtx, stopHub := context.WithCancel(context.WithoutCancel(ctx))
	hubDone := make(chan struct{})
	go func() { _ = m.hub.Run(hubCtx); close(hubDone) }()
	_ = m.hub.Subscribe(ctx, rt.NodeChannel(m.cfg.NodeID), nodeReceiver{m})
	_ = m.hub.Subscribe(ctx, rt.BroadcastChannel, broadcastReceiver{m})
	alive := time.NewTicker(3 * time.Second)
	defer alive.Stop()
	m.nodeAlive(ctx)
	for {
		select {
		case <-ctx.Done():
			m.Drain()
			stopHub()
			<-hubDone
			return nil
		case <-alive.C:
			m.nodeAlive(ctx)
		}
	}
}

func (m *Manager) nodeAlive(ctx context.Context) {
	ss := m.sessions()
	ids := make([]string, 0, len(ss))
	for _, s := range ss {
		ids = append(ids, s.caller.ConnID)
	}
	cctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	_ = m.registry.NodeAlive(cctx, m.cfg.NodeID, ids)
}

// Drain stops accepting, tells every client to reconnect elsewhere (system.server.draining with a random
// reconnect_after_ms) and closes the connections with 1012 spread over the drain window (F090-AC-27).
func (m *Manager) Drain() {
	if m.draining.Swap(true) {
		return
	}
	ss := m.sessions()
	now := m.now()
	window := m.cfg.DrainWindow
	for i, s := range ss {
		delay := time.Duration(0)
		if len(ss) > 1 && window > 0 {
			delay = window * time.Duration(i) / time.Duration(len(ss))
		}
		spread := window
		if spread <= 0 {
			spread = time.Second
		}
		o := protocol.NewOut(protocol.TypeDraining, map[string]any{"reconnect_after_ms": rand.Int64N(spread.Milliseconds() + 1),
			"reason": "deploy"}, now)
		o.Transient = true
		s.enqueue(o)
		s := s
		time.AfterFunc(delay, func() { s.closeAfterFlush(protocol.CloseServiceRestart, "draining") })
	}
	done := make(chan struct{})
	go func() { m.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(window + 5*time.Second):
		slog.Warn("realtime: drain timeout", "open", m.Count())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = m.registry.NodeGone(ctx, m.cfg.NodeID)
	m.stop()
}

// onHubState tells every connection that realtime delivery is degraded (F090-AC-30); recovery resubscribes in the
// hub and needs no message (the client goes back to WebSocket events when they arrive again).
func (m *Manager) onHubState(up bool) {
	if up {
		slog.Info("realtime: valkey pubsub up")
		return
	}
	slog.Warn("realtime: valkey pubsub down")
	now := m.now()
	for _, s := range m.sessions() {
		o := protocol.NewOut(protocol.TypeDegraded, map[string]any{"features": []string{"chat", "presence", "party", "matchmaking",
			"notification"}, "poll_interval_s": 10}, now)
		o.Transient = true
		s.enqueue(o)
	}
}

// nodeReceiver handles ew:rt:node:<id>:in: close a local connection replaced on another node (F090-AC-28).
type nodeReceiver struct{ m *Manager }

func (n nodeReceiver) Deliver(_ string, data []byte) {
	var e rt.Envelope
	if err := jsonUnmarshal(data, &e); err != nil || e.Validate() != nil || e.Target == nil || e.Target.ConnID == "" {
		return
	}
	if s := n.m.get(e.Target.ConnID); s != nil {
		if a := revocation.Decide(e, s.conn(), true); a.Close != 0 {
			s.closeNow(a.Close, closeReason(a.Close)) // 4409 without message (F090-AC-08)
		}
	}
}

// broadcastReceiver delivers ew:rt:broadcast to every local connection.
type broadcastReceiver struct{ m *Manager }

func (b broadcastReceiver) Deliver(ch string, data []byte) {
	for _, s := range b.m.sessions() {
		s.Deliver(ch, data)
	}
}

// ServeHTTP is the handshake of /realtime/v1 (F090-AC-05, AC-06, AC-07, AC-08, AC-29).
func (m *Manager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != Path {
		http.NotFound(w, r)
		return
	}
	if m.draining.Load() || !m.hub.Healthy() {
		w.Header().Set("Retry-After", "2")
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	q := r.URL.Query()
	if q.Has("access_token") || q.Has("token") || q.Has("jwt") || r.Header.Get("Authorization") != "" {
		m.metrics.TicketRejected.WithLabelValues("credential_in_request").Inc()
		http.Error(w, "use a realtime ticket", http.StatusBadRequest) // never a JWT in the URL (ADR-0008)
		return
	}
	if !offers(r.Header.Values("Sec-WebSocket-Protocol"), protocol.Subprotocol) {
		http.Error(w, "subprotocol eduworld.v1 required", http.StatusBadRequest)
		return
	}
	if o := r.Header.Get("Origin"); o != "" && !slices.Contains(m.cfg.AllowedOrigins, o) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{protocol.Subprotocol},
		InsecureSkipVerify: true, CompressionMode: websocket.CompressionDisabled}) // Origin checked above
	if err != nil {
		return
	}
	ws.SetReadLimit(int64(m.cfg.MaxFrame) * 4)
	now := m.now()
	if ok, wait := m.accept.Allow(now); !ok || m.Count() >= m.cfg.MaxConnections {
		if wait < time.Second {
			wait = time.Second
		}
		m.metrics.Closed.WithLabelValues("4429").Inc()
		_ = ws.Close(protocol.CloseRateLimited, "retry_after_ms="+strconv.FormatInt(wait.Milliseconds(), 10)) // F090-AC-29
		return
	}
	claims, err := m.tickets.Consume(r.Context(), q.Get("ticket"))
	if err != nil {
		reason := ticket.ReasonUnavailable
		var rej ticket.ErrRejected
		if errors.As(err, &rej) {
			reason = rej.Reason
		}
		m.metrics.TicketRejected.WithLabelValues(reason).Inc()
		m.metrics.Closed.WithLabelValues("4401").Inc()
		_ = ws.Close(protocol.CloseTicket, "ticket_invalid")
		return
	}
	m.serve(ws, claims)
}

func offers(values []string, proto string) bool {
	for _, v := range values {
		for _, p := range strings.Split(v, ",") {
			if strings.TrimSpace(p) == proto {
				return true
			}
		}
	}
	return false
}

// serve runs one authenticated connection until it ends.
func (m *Manager) serve(ws *websocket.Conn, c rt.TicketClaims) {
	caller := rt.Caller{TenantID: c.TenantID, UserID: c.UserID, SchoolID: c.SchoolID, GameSessionID: c.GameSessionID,
		DeviceID: c.DeviceID, ConnID: "c-" + uuid.NewString(), NodeID: m.cfg.NodeID}
	s := m.newSession(ws, caller)
	m.wg.Add(1)
	defer m.wg.Done()
	m.add(s)
	defer s.finish()
	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()
	prev, err := m.registry.Register(ctx, c.UserID, registry.Entry{Node: m.cfg.NodeID, ConnID: caller.ConnID,
		GameSessionID: c.GameSessionID.String()}, m.now())
	if err != nil {
		slog.Warn("realtime: register failed", "error", err)
		_ = ws.Close(websocket.StatusTryAgainLater, "unavailable")
		return
	}
	if prev != nil && prev.ConnID != caller.ConnID { // F090-AC-08: the later connection wins, the earlier closes 4409
		if prev.Node == m.cfg.NodeID {
			if old := m.get(prev.ConnID); old != nil {
				old.closeNow(protocol.CloseReplaced, "replaced")
			}
		} else {
			e := rt.Envelope{V: 1, Kind: rt.KindControl, Target: &rt.Target{ConnID: prev.ConnID},
				Control: &rt.Control{Action: "close", Code: rt.CloseReplaced, Reason: "replaced"}}
			if err := (rt.Publisher{C: m.vk}).Publish(ctx, rt.NodeChannel(prev.Node), e); err != nil {
				slog.Warn("realtime: cross-node replace failed", "error", err)
			}
		}
	}
	token, err := resume.NewToken(m.now())
	if err == nil {
		err = m.resume.Open(ctx, token, resume.Meta{UserID: c.UserID, GameSessionID: c.GameSessionID})
	}
	if err != nil {
		slog.Warn("realtime: resume buffer unavailable", "error", err)
	}
	s.mu.Lock()
	s.token = token
	s.mu.Unlock()
	now := m.now()
	welcome := protocol.NewOut(protocol.TypeWelcome, map[string]any{"conn_id": caller.ConnID, "user_id": c.UserID.String(),
		"game_session_id": c.GameSessionID.String(), "server_time": now.UnixMilli(), "heartbeat_s": int(m.cfg.Heartbeat / time.Second),
		"timeout_s": int(m.cfg.Timeout / time.Second), "resume_token": token, "resume_window_s": int(m.cfg.ResumeWindow / time.Second),
		"protocol_version": protocol.Version, "limits": map[string]any{"max_frame_bytes": m.cfg.MaxFrame, "max_move_hz": config.ClientMoveHz}}, now)
	welcome.Transient = true
	s.enqueue(welcome) // first message (seq 1), before any event of the user channel
	if err := m.hub.Subscribe(ctx, rt.UserChannel(c.UserID), s); err != nil {
		slog.Warn("realtime: user channel subscribe deferred", "error", err)
	}
	if err := m.presence.Connected(ctx, s.presenceCaller()); err != nil { // F091-AC-21
		slog.Warn("realtime: presence online failed", "error", err)
	}
	go s.writeLoop()
	go s.timers()
	s.readLoop()
}
