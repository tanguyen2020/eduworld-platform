// Package fakert is a TEST STAND-IN of the realtime gateway, used only by the unit tests of loadbot (AGENTS R17): it
// accepts a WebSocket with subprotocol eduworld.v1 and ?ticket=, sends system.session.welcome, records every client
// envelope and answers through a test-supplied function (default: ack every command, pong every ping). It does not
// implement any rule of the real gateway (rate limits, validation, rooms) and must never be used to measure anything.
package fakert

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// In is a client envelope as received.
type In struct {
	V       int             `json:"v"`
	Type    string          `json:"type"`
	ID      string          `json:"id"`
	Ts      int64           `json:"ts"`
	Payload json.RawMessage `json:"payload"`
}

// Out is a server message to send (seq and ts are filled in).
type Out struct {
	Type    string
	Ref     string
	Payload any
}

// Server is the fake gateway.
type Server struct {
	*httptest.Server
	Ticket    string
	heartbeat int
	reply     func(in In) []Out
	mu        sync.Mutex
	got       []In
	conns     []*websocket.Conn
}

// Ack acknowledges commands and answers pings, the default behaviour.
func Ack(in In) []Out {
	if in.Type == "system.heartbeat.ping" {
		return []Out{{Type: "system.heartbeat.pong", Payload: map[string]any{"server_time": time.Now().UnixMilli()}}}
	}
	return []Out{{Type: "system.command.acked", Ref: in.ID, Payload: map[string]any{"result": map[string]any{}}}}
}

// New starts the fake gateway; ticket is the only accepted ticket.
func New(ticket string) *Server {
	s := &Server{Ticket: ticket, heartbeat: 15}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	return s
}

// URL is the realtime URL as createRealtimeTicket returns it.
func (s *Server) URL() string { return "ws" + s.Server.URL[len("http"):] + "/realtime/v1" }

// SetReply sets how client envelopes are answered (nil = Ack); call it before the client connects.
func (s *Server) SetReply(fn func(in In) []Out) {
	s.mu.Lock()
	s.reply = fn
	s.mu.Unlock()
}

// SetHeartbeat sets heartbeat_s of the welcome.
func (s *Server) SetHeartbeat(sec int) {
	s.mu.Lock()
	s.heartbeat = sec
	s.mu.Unlock()
}

// Received returns the client envelopes received so far.
func (s *Server) Received() []In {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]In(nil), s.got...)
}

// Push sends a server message to every connection (e.g. battle.question.shown).
func (s *Server) Push(o Out) {
	s.mu.Lock()
	conns := append([]*websocket.Conn(nil), s.conns...)
	s.mu.Unlock()
	for _, c := range conns {
		write(c, o, 0)
	}
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("ticket") != s.Ticket {
		http.Error(w, "ticket", http.StatusUnauthorized)
		return
	}
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{"eduworld.v1"}})
	if err != nil {
		return
	}
	if c.Subprotocol() != "eduworld.v1" {
		_ = c.Close(4400, "subprotocol")
		return
	}
	s.mu.Lock()
	s.conns = append(s.conns, c)
	hb := s.heartbeat
	s.mu.Unlock()
	var seq int64
	next := func() int64 { seq++; return seq }
	write(c, Out{Type: "system.session.welcome", Payload: map[string]any{"conn_id": "c1", "server_time": time.Now().UnixMilli(),
		"heartbeat_s": hb, "timeout_s": 45, "resume_token": "rt", "protocol_version": 1}}, next())
	for {
		_, b, err := c.Read(context.Background())
		if err != nil {
			return
		}
		var in In
		if json.Unmarshal(b, &in) != nil {
			_ = c.Close(4400, "bad message")
			return
		}
		s.mu.Lock()
		s.got = append(s.got, in)
		reply := s.reply
		s.mu.Unlock()
		if reply == nil {
			reply = Ack
		}
		for _, o := range reply(in) {
			write(c, o, next())
		}
	}
}

var writeMu sync.Mutex

func write(c *websocket.Conn, o Out, seq int64) {
	if seq == 0 {
		seq = time.Now().UnixNano() // pushed messages: monotonic enough for a stand-in
	}
	m := map[string]any{"v": 1, "type": o.Type, "seq": seq, "ts": time.Now().UnixMilli(), "payload": o.Payload}
	if o.Ref != "" {
		m["ref"] = o.Ref
	}
	b, _ := json.Marshal(m)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	writeMu.Lock()
	defer writeMu.Unlock()
	_ = c.Write(ctx, websocket.MessageText, b)
}
