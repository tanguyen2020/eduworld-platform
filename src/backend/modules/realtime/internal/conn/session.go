package conn

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/presence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/protocol"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/ratelimit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/resume"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/revocation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/router"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// maxTopics bounds the topics a connection may hold (resume meta subs ≤ 64).
const maxTopics = 64

// Session is one authenticated connection (REALTIME_ARCHITECTURE §2.1): identity fixed by the ticket, outbound seq,
// bounded send queue, resume token, rate limiters, granted topics. It holds no business state.
type Session struct {
	m      *Manager
	ws     *websocket.Conn
	caller rt.Caller
	ctx    context.Context
	cancel context.CancelFunc
	q      *sendQueue

	mu      sync.Mutex
	seq     int64
	token   string
	topics  map[string]struct{}
	cleared bool // resume buffer invalidated: nothing more is buffered

	inflight  chan struct{}
	frames    *ratelimit.Bucket
	moves     *ratelimit.Bucket
	rateViol  ratelimit.Episodes
	protoErrs ratelimit.Events
	lastFrame atomic.Int64

	closeOnce  sync.Once
	finishOnce sync.Once
	closeCode  atomic.Int32
	done       chan struct{}
}

func (m *Manager) newSession(ws *websocket.Conn, c rt.Caller) *Session {
	ctx, cancel := context.WithCancel(m.base)
	s := &Session{m: m, ws: ws, caller: c, ctx: ctx, cancel: cancel, q: newSendQueue(m.cfg.SendQueue),
		topics: map[string]struct{}{}, inflight: make(chan struct{}, m.cfg.Inflight),
		frames: ratelimit.NewBucket(m.cfg.FrameRate, m.cfg.FrameBurst), moves: ratelimit.NewBucket(m.cfg.MoveRate, m.cfg.MoveRate),
		done: make(chan struct{})}
	s.lastFrame.Store(m.now().UnixNano())
	return s
}

func (s *Session) conn() revocation.Conn {
	return revocation.Conn{UserID: s.caller.UserID, GameSessionID: s.caller.GameSessionID, ConnID: s.caller.ConnID}
}

// enqueue assigns the next seq and queues o (F090-AC-13: seq increases for every server message).
func (s *Session) enqueue(o protocol.Out) {
	s.mu.Lock()
	s.seq++
	seq := s.seq
	buffer := !o.Droppable && !o.Transient && !s.cleared
	s.mu.Unlock()
	s.push(item{data: o.Encode(seq), seq: seq, typ: o.Type, ts: o.Ts, droppable: o.Droppable, buffer: buffer})
}

func (s *Session) push(it item) {
	dropped, hardFull := s.q.push(it, s.m.now())
	if dropped != nil {
		s.m.metrics.QueueDropped.WithLabelValues(dropped.typ).Inc()
	}
	if hardFull {
		s.closeNow(protocol.CloseTimeout, "send_queue_full") // F090-AC-23: the client resumes
	}
}

// closeAfterFlush closes with code once every queued frame is written, or after 2 s at the latest (≤ 5 s for
// revocation, F090-AC-24).
func (s *Session) closeAfterFlush(code int, reason string) {
	s.q.push(item{closeCode: code, closeReason: reason}, s.m.now())
	time.AfterFunc(2*time.Second, func() { s.closeNow(code, reason) })
}

// closeNow closes the connection with code (once).
func (s *Session) closeNow(code int, reason string) {
	s.closeOnce.Do(func() {
		s.closeCode.Store(int32(code))
		s.m.metrics.Closed.WithLabelValues(itoa(code)).Inc()
		// Cancel the session context only once the close frame is out: cancelling a Read/Write context of the
		// websocket would drop the connection without our close code.
		go func() {
			_ = s.ws.Close(websocket.StatusCode(code), reason)
			s.cancel()
		}()
	})
}

// clearResume invalidates the resume token (revocation, replacement by another device, session ended).
func (s *Session) clearResume() {
	s.mu.Lock()
	s.cleared = true
	token := s.token
	s.mu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(s.ctx), 2*time.Second)
		defer cancel()
		_ = s.m.resume.Delete(ctx, token)
	}()
}

// Deliver implements subs.Receiver for the channels of the connection (F090-AC-21, US4). Never blocks.
func (s *Session) Deliver(channel string, data []byte) {
	var e rt.Envelope
	if err := json.Unmarshal(data, &e); err != nil || e.Validate() != nil {
		slog.Debug("realtime: invalid pubsub envelope dropped", "channel_kind", channelKind(channel))
		return
	}
	s.DeliverEnvelope(channel, &e)
}

// DeliverEnvelope implements subs.EnvelopeReceiver (envelope decoded once per node message). Never blocks.
func (s *Session) DeliverEnvelope(channel string, ep *rt.Envelope) {
	e := *ep
	userChannel := channel == rt.UserChannel(s.caller.UserID)
	switch {
	case e.Kind == rt.KindBundle: // F091-AC-14: the viewer's own snapshot out of the node bundle
		if p := e.Bundle.Item(s.caller.UserID); p != nil {
			ts := e.Bundle.Ts
			if ts == 0 {
				ts = s.m.now().UnixMilli()
			}
			s.enqueue(protocol.Out{Type: e.Bundle.Type, Ts: ts, Payload: p, Droppable: e.Droppable})
		}
		return
	case e.Kind == rt.KindControl && e.Control.Action == rt.ActionPresenceUpdate:
		// F091-AC-23: area/room from the room host, recorded in presence only (never delivered, F091-AC-25).
		if userChannel && s.m.presence != nil {
			p := e.Control.Presence
			area, room := "", ""
			if p.AreaID != nil {
				area = *p.AreaID
			}
			if p.RoomID != nil {
				room = *p.RoomID
			}
			go s.presenceDo(func(ctx context.Context) error { return s.m.presence.SetArea(ctx, s.presenceCaller(), area, room) })
		}
		return
	}
	a := revocation.Decide(e, s.conn(), userChannel)
	if a.ClearResume {
		s.clearResume()
	}
	now := s.m.now()
	if a.EndedReason != "" {
		s.enqueue(protocol.NewOut(protocol.TypeSessionEnded, map[string]any{"game_session_id": s.caller.GameSessionID.String(),
			"reason": a.EndedReason, "ended_at": now.UnixMilli()}, now))
	}
	if a.Deliver && e.Message != nil {
		ts := e.Message.Ts
		if ts == 0 {
			ts = now.UnixMilli()
		}
		s.enqueue(protocol.Out{Type: e.Message.Type, Ts: ts, Payload: e.Message.Payload, Droppable: e.Droppable})
	}
	if a.Close != 0 {
		s.closeAfterFlush(a.Close, closeReason(a.Close))
	}
}

func closeReason(code int) string {
	switch code {
	case protocol.CloseRevoked:
		return "revoked"
	case protocol.CloseReplaced:
		return "replaced"
	case protocol.CloseSessionEnded:
		return "session_ended"
	}
	return ""
}

func channelKind(ch string) string {
	if i := strings.LastIndex(ch, ":"); i > 0 {
		return ch[:i]
	}
	return ch
}

// writeLoop writes queued frames; non-droppable ones are appended to the resume buffer first (research R3).
func (s *Session) writeLoop() {
	for {
		it, ok := s.q.pop(s.ctx)
		if !ok {
			return
		}
		if it.closeCode != 0 {
			s.closeNow(it.closeCode, it.closeReason)
			return
		}
		if it.buffer {
			s.appendResume(it)
		}
		ctx, cancel := context.WithTimeout(s.ctx, s.m.cfg.WriteTimeout)
		err := s.ws.Write(ctx, websocket.MessageText, it.data)
		cancel()
		if err != nil {
			if s.ctx.Err() == nil {
				s.closeNow(protocol.CloseTimeout, "write_failed")
			}
			return
		}
		s.m.metrics.Messages.WithLabelValues("out", it.typ).Inc()
		if it.ts > 0 {
			s.m.metrics.DeliveryLatency.Observe(time.Since(time.UnixMilli(it.ts)).Seconds())
		}
	}
}

func (s *Session) appendResume(it item) {
	s.mu.Lock()
	token, cleared := s.token, s.cleared
	s.mu.Unlock()
	if cleared || token == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(s.ctx), time.Second)
	defer cancel()
	if err := s.m.resume.Append(ctx, token, it.seq, it.data); err != nil {
		slog.Warn("realtime: resume append failed", "error", err)
	}
}

// timers runs the game heartbeat over NATS, WS ping control frames and the receive/send watchdog (US2).
func (s *Session) timers() {
	hb := time.NewTicker(s.m.cfg.Heartbeat)
	ping := time.NewTicker(s.m.cfg.Ping)
	tick := s.m.cfg.Timeout / 8
	if tick > time.Second {
		tick = time.Second
	}
	watch := time.NewTicker(tick)
	defer hb.Stop()
	defer ping.Stop()
	defer watch.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-hb.C:
			s.heartbeat()
		case <-ping.C:
			go func() { // F090-AC-10: WebSocket ping control frame every ws_ping
				ctx, cancel := context.WithTimeout(s.ctx, s.m.cfg.Ping)
				defer cancel()
				if s.ws.Ping(ctx) == nil {
					s.m.metrics.Pings.Inc()
				}
			}()
		case <-watch.C:
			now := s.m.now()
			if now.Sub(time.Unix(0, s.lastFrame.Load())) > s.m.cfg.Timeout {
				s.closeNow(protocol.CloseTimeout, "heartbeat_timeout") // F090-AC-11
				return
			}
			if s.q.stalled(now, s.m.cfg.SendStall) {
				s.closeNow(protocol.CloseTimeout, "send_stalled") // F090-AC-23
				return
			}
		}
	}
}

// heartbeat sends ew.cmd.game.session.heartbeat (F090-AC-12): an ended or replaced session closes 4409; a module
// that does not answer keeps the connection (NATS down must not cut realtime).
func (s *Session) heartbeat() {
	reply, err := s.m.router.Heartbeat(s.ctx, s.caller)
	if err == nil && !reply.OK && reply.Problem != nil &&
		(reply.Problem.Code == protocol.CodeConflict || reply.Problem.Code == protocol.CodeNotFound) {
		s.clearResume()
		s.closeAfterFlush(protocol.CloseReplaced, "session_ended")
		return
	}
	ctx, cancel := context.WithTimeout(s.ctx, time.Second)
	defer cancel()
	_ = s.m.registry.Refresh(ctx, s.caller.UserID, s.caller.ConnID)
	if s.m.presence != nil {
		_ = s.m.presence.Refresh(ctx, s.presenceCaller()) // presence_ttl (F091-AC-21)
	}
}

// readLoop reads client frames until the connection ends.
func (s *Session) readLoop() {
	for {
		typ, data, err := s.ws.Read(s.m.base) // ends with the connection (see closeNow)
		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusMessageTooBig {
				s.closeNow(protocol.CloseProtocol, "frame_too_large")
			}
			return
		}
		s.lastFrame.Store(s.m.now().UnixNano())
		if typ != websocket.MessageText {
			s.protocolError(&protocol.Rejection{Code: protocol.CodeValidationFailed})
			continue
		}
		s.handle(data)
	}
}

func (s *Session) handle(frame []byte) {
	now := s.m.now()
	if len(frame) > s.m.cfg.MaxFrame { // F090-AC-20: frame over ws_max_frame, not forwarded
		s.protocolError(&protocol.Rejection{Code: protocol.CodeValidationFailed})
		return
	}
	if ok, wait := s.frames.Allow(now); !ok {
		s.rateLimited(protocol.RefOf(frame), wait)
		return
	}
	env, rej := s.m.validator.Parse(frame)
	if rej != nil {
		s.protocolError(rej)
		return
	}
	s.m.metrics.Messages.WithLabelValues("in", env.Type).Inc()
	route, _ := router.Lookup(env.Type)
	switch route.Kind {
	case router.KindLocal:
		s.local(env)
	case router.KindRoomInput:
		if ok, wait := s.moves.Allow(now); !ok {
			s.rateLimited(env.ID, wait)
			return
		}
		var p struct {
			RoomID string `json:"room_id"`
		}
		if json.Unmarshal(env.Payload, &p) != nil || !rt.ValidID(p.RoomID) {
			s.protocolError(&protocol.Rejection{Code: protocol.CodeValidationFailed, Ref: env.ID})
			return
		}
		_ = s.m.router.RoomInput(s.ctx, s.caller, p.RoomID, env.Payload) // fire-and-forget, no ack (F090-AC-17)
	case router.KindRequest:
		s.command(env, route)
	}
}

// rateLimited answers RATE_LIMITED; more than ws_frame_rate violations per minute close 4429 with an audit.
func (s *Session) rateLimited(ref string, wait time.Duration) {
	now := s.m.now()
	if wait < 100*time.Millisecond {
		wait = 100 * time.Millisecond
	}
	s.enqueue(protocol.Failed(ref, protocol.CodeRateLimited, "", wait, now))
	if s.rateViol.Add(now) > s.m.cfg.RateViolations {
		go func() {
			if err := s.m.audit.RateLimitClosed(context.WithoutCancel(s.ctx), s.caller); err != nil {
				slog.Warn("realtime: rate limit audit failed", "error", err)
			}
		}()
		s.closeAfterFlush(protocol.CloseRateLimited, "rate_limited")
	}
}

// protocolError answers VALIDATION_FAILED / UNKNOWN_TYPE; repeated errors close 4400 (F090-AC-20).
func (s *Session) protocolError(r *protocol.Rejection) {
	now := s.m.now()
	s.enqueue(protocol.Failed(r.Ref, r.Code, "", 0, now))
	s.m.metrics.CmdFailed.WithLabelValues("realtime", r.Code).Inc()
	if s.protoErrs.Add(now) > s.m.cfg.ProtocolErrors {
		s.closeAfterFlush(protocol.CloseProtocol, "protocol_error")
	}
}

func (s *Session) local(env protocol.ClientEnvelope) {
	now := s.m.now()
	switch env.Type {
	case protocol.TypePing:
		var p struct {
			ClientTime int64 `json:"client_time"`
		}
		_ = json.Unmarshal(env.Payload, &p)
		o := protocol.NewOut(protocol.TypePong, map[string]any{"client_time": p.ClientTime, "server_time": now.UnixMilli()}, now)
		o.Transient = true
		s.enqueue(o)
	case protocol.TypeResume:
		s.resume(env)
	case protocol.TypePresenceSet:
		s.presenceSet(env)
	default:
		s.enqueue(protocol.Failed(env.ID, protocol.CodeUnknownType, "", 0, now))
	}
}

// presenceSet applies presence.status.set at the gateway (F091-AC-22): the validator already refused values outside
// the enum; the service refuses them again (offline is set by the gateway only).
func (s *Session) presenceSet(env protocol.ClientEnvelope) {
	var p struct {
		Status   string `json:"status"`
		Activity string `json:"activity"`
		Hidden   *bool  `json:"hidden"`
	}
	if json.Unmarshal(env.Payload, &p) != nil {
		s.protocolError(&protocol.Rejection{Code: protocol.CodeValidationFailed, Ref: env.ID})
		return
	}
	if s.m.presence == nil {
		s.enqueue(protocol.Failed(env.ID, protocol.CodeUnavailable, "", time.Second, s.m.now()))
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
		defer cancel()
		err := s.m.presence.SetStatus(ctx, s.presenceCaller(), p.Status, p.Activity, p.Hidden)
		now := s.m.now()
		switch {
		case errors.Is(err, presence.ErrInvalid):
			s.protocolError(&protocol.Rejection{Code: protocol.CodeValidationFailed, Ref: env.ID})
		case err != nil:
			s.enqueue(protocol.Failed(env.ID, protocol.CodeUnavailable, "", time.Second, now))
		default:
			s.enqueue(protocol.Acked(env.ID, nil, now))
		}
	}()
}

func (s *Session) presenceCaller() presence.Caller {
	return presence.Caller{UserID: s.caller.UserID, TenantID: s.caller.TenantID, SchoolID: s.caller.SchoolID}
}

// presenceDo runs one presence write with its own deadline (presence is ephemeral: a failure is only logged).
func (s *Session) presenceDo(fn func(ctx context.Context) error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(s.ctx), 3*time.Second)
	defer cancel()
	if err := fn(ctx); err != nil {
		slog.Warn("realtime: presence update failed", "error", err)
	}
}

// storedResult is the dedupe value of a processed command.
type storedResult struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// command forwards a request to the owner module and answers exactly one ack or failed with ref = id
// (F090-AC-16..19).
func (s *Session) command(env protocol.ClientEnvelope, route router.Route) {
	select {
	case s.inflight <- struct{}{}:
	default: // ws_inflight_commands
		s.enqueue(protocol.Failed(env.ID, protocol.CodeRateLimited, "", time.Second, s.m.now()))
		return
	}
	go func() {
		defer func() { <-s.inflight }()
		// One span per command; traceparent travels in the NATS headers to core-api (REALTIME_ARCHITECTURE §12).
		ctx, span := otel.Tracer("eduworld/realtime").Start(s.ctx, "ws "+env.Type, trace.WithSpanKind(trace.SpanKindProducer),
			trace.WithAttributes(attribute.String("eduworld.realtime.type", env.Type), attribute.String("eduworld.module", route.Module)))
		defer span.End()
		prev, reserved, err := s.m.resume.Reserve(ctx, s.caller.UserID, env.ID)
		if err == nil && !reserved {
			now := s.m.now()
			var r storedResult
			if prev != nil && json.Unmarshal(prev, &r) == nil && r.Type != "" {
				s.enqueue(protocol.Out{Type: r.Type, Ref: env.ID, Ts: now.UnixMilli(), Payload: r.Payload})
			} else {
				s.enqueue(protocol.Failed(env.ID, protocol.CodeUnavailable, "command in progress", time.Second, now))
			}
			return
		}
		start := time.Now()
		reply, err := s.m.router.Request(ctx, s.caller, route.Module, env.Type, env.Payload)
		s.m.metrics.CmdLatency.WithLabelValues(route.Module, env.Type).Observe(time.Since(start).Seconds())
		now := s.m.now()
		var out protocol.Out
		switch {
		case err != nil:
			if reserved {
				_ = s.m.resume.Release(context.WithoutCancel(ctx), s.caller.UserID, env.ID)
			}
			s.m.metrics.CmdFailed.WithLabelValues(route.Module, protocol.CodeUnavailable).Inc()
			if !errors.Is(err, router.ErrUnavailable) {
				slog.Warn("realtime: command reply invalid", "type", env.Type, "error", err)
			}
			s.enqueue(protocol.Failed(env.ID, protocol.CodeUnavailable, "", time.Second, now)) // F090-AC-18
			return
		case reply.OK:
			s.applySubs(reply.Subscribe, reply.Unsubscribe) // F090-AC-19: before the ack
			out = protocol.Acked(env.ID, reply.Result, now)
		default:
			s.m.metrics.CmdFailed.WithLabelValues(route.Module, reply.Problem.Code).Inc()
			out = protocol.Failed(env.ID, reply.Problem.Code, reply.Problem.Message,
				time.Duration(reply.Problem.RetryAfterMs)*time.Millisecond, now)
		}
		if reserved {
			b, _ := json.Marshal(storedResult{Type: out.Type, Payload: out.Payload})
			if err := s.m.resume.Complete(context.WithoutCancel(ctx), s.caller.UserID, env.ID, b); err != nil {
				slog.Warn("realtime: command result not stored", "error", err)
			}
		}
		s.enqueue(out)
	}()
}

// applySubs applies the topics of a module reply. The gateway subscribes only channels a module granted, never a
// user channel or a node channel (F090-AC-19).
func (s *Session) applySubs(sub, unsub []string) {
	if len(sub) == 0 && len(unsub) == 0 {
		return
	}
	for _, t := range unsub {
		s.mu.Lock()
		_, had := s.topics[t]
		delete(s.topics, t)
		s.mu.Unlock()
		if had {
			s.m.hub.Unsubscribe(s.ctx, t, s)
		}
	}
	for _, t := range sub {
		kind, _ := rt.ParseTopic(t)
		if kind != rt.TopicParty && kind != rt.TopicRoom && kind != rt.TopicChat {
			s.m.metrics.SubscribeRefused.Inc()
			slog.Warn("realtime: topic refused", "channel_kind", channelKind(t))
			continue
		}
		s.mu.Lock()
		_, had := s.topics[t]
		full := len(s.topics) >= maxTopics
		if !had && !full {
			s.topics[t] = struct{}{}
		}
		s.mu.Unlock()
		if had || full {
			continue
		}
		if err := s.m.hub.Subscribe(s.ctx, t, s); err != nil {
			slog.Warn("realtime: subscribe deferred", "error", err) // recorded: subscribed again on recovery
		}
	}
	s.mu.Lock()
	token := s.token
	topics := make([]string, 0, len(s.topics))
	for t := range s.topics {
		topics = append(topics, t)
	}
	s.mu.Unlock()
	_ = s.m.resume.SaveSubs(context.WithoutCancel(s.ctx), token, topics)
}

// resume replays the buffer of a previous logical connection (F090-AC-14, AC-15).
func (s *Session) resume(env protocol.ClientEnvelope) {
	var p struct {
		ResumeToken string `json:"resume_token"`
		LastSeq     int64  `json:"last_seq"`
	}
	_ = json.Unmarshal(env.Payload, &p)
	now := s.m.now()
	fail := func(reason string) {
		s.m.metrics.Resume.WithLabelValues(reason).Inc()
		o := protocol.NewOut(protocol.TypeResumeFailed, map[string]string{"reason": reason}, now)
		o.Ref, o.Transient = env.ID, true
		s.enqueue(o)
	}
	meta, entries, err := s.m.resume.Load(s.ctx, p.ResumeToken)
	switch {
	case errors.Is(err, resume.ErrNotFound):
		if at, ok := resume.IssuedAt(p.ResumeToken); ok && now.Sub(at) >= s.m.cfg.ResumeWindow {
			fail("expired")
		} else {
			fail("unknown_token")
		}
		return
	case err != nil:
		fail("expired") // Valkey unavailable: the client resyncs over REST
		return
	case meta.UserID != s.caller.UserID || meta.GameSessionID != s.caller.GameSessionID:
		fail("unknown_token") // another student's or another session's buffer: nothing is disclosed
		return
	case meta.TrimmedSeq > p.LastSeq:
		fail("seq_gap")
		return
	}
	s.mu.Lock()
	replayed := 0
	for _, e := range entries {
		if e.Seq > p.LastSeq {
			s.q.push(item{data: e.Data, seq: e.Seq, typ: "replay"}, now)
			replayed++
		}
	}
	old := s.token
	s.token = p.ResumeToken
	if meta.LastSeq > s.seq {
		s.seq = meta.LastSeq
	}
	if p.LastSeq > s.seq {
		s.seq = p.LastSeq
	}
	s.mu.Unlock()
	if old != "" && old != p.ResumeToken {
		_ = s.m.resume.Delete(context.WithoutCancel(s.ctx), old)
	}
	s.applySubs(meta.Subs, nil)
	rooms := []string{}
	for _, t := range meta.Subs {
		if kind, id := rt.ParseTopic(t); kind == rt.TopicRoom {
			rooms = append(rooms, id)
		}
	}
	s.m.metrics.Resume.WithLabelValues("resumed").Inc()
	o := protocol.NewOut(protocol.TypeResumed, map[string]any{"replayed": replayed, "rejoined_rooms": rooms}, now)
	o.Ref, o.Transient = env.ID, true
	s.enqueue(o)
}

// finish releases everything of the connection (once): topics, registry entry, local table; queued non-droppable
// frames go to the resume buffer so a resume elsewhere loses nothing (F090-AC-27).
func (s *Session) finish() {
	s.finishOnce.Do(func() {
		s.cancel()
		ctx, cancel := context.WithTimeout(context.WithoutCancel(s.ctx), 3*time.Second)
		defer cancel()
		for _, it := range s.q.drain() {
			if it.buffer {
				s.appendResume(it)
			}
		}
		s.mu.Lock()
		topics := make([]string, 0, len(s.topics)+1)
		for t := range s.topics {
			topics = append(topics, t)
		}
		s.mu.Unlock()
		topics = append(topics, rt.UserChannel(s.caller.UserID))
		for _, t := range topics {
			s.m.hub.Unsubscribe(ctx, t, s)
		}
		_ = s.m.registry.Unregister(ctx, s.caller.UserID, s.caller.ConnID)
		if s.m.presence != nil { // F091-AC-21, AC-24: offline, or away then offline after ws_timeout
			if err := s.m.presence.Disconnected(ctx, s.presenceCaller(), int(s.closeCode.Load())); err != nil {
				slog.Warn("realtime: presence disconnect failed", "error", err)
			}
		}
		s.m.remove(s)
		close(s.done)
	})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
