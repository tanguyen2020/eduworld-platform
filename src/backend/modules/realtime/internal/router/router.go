// Package router forwards client commands to their owner module (F090 US3, REALTIME_ARCHITECTURE §4.1): NATS
// request/reply ew.cmd.<module>.<type> with the Ew-* headers of the connection, or NATS core ew.in.room.<room_id>
// for high-rate room input. The gateway decides nothing: permissions and topics come from the reply.
package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// Kind is the transport of a command type.
type Kind int

// Kinds of routes (routes_gen.go).
const (
	KindLocal Kind = iota + 1
	KindRequest
	KindRoomInput
)

// Route is the owner module and transport of a command type.
type Route struct {
	Module string
	Kind   Kind
}

// Lookup returns the route of type t.
func Lookup(t string) (Route, bool) {
	r, ok := Routes[t]
	return r, ok
}

// ErrUnavailable means the owner module did not answer in time (no responder, timeout, NATS down).
var ErrUnavailable = errors.New("router: module unavailable")

// Router sends commands over NATS.
type Router struct {
	NC      *nats.Conn
	Timeout time.Duration // nats_request_timeout
}

// Request sends payload to ew.cmd.<module>.<type> and decodes the reply.
func (r Router) Request(ctx context.Context, c rt.Caller, module, typ string, payload json.RawMessage) (rt.Reply, error) {
	if r.NC == nil {
		return rt.Reply{}, ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()
	m := nats.NewMsg(rt.CommandSubject(module, typ))
	m.Header = c.Header(ctx)
	m.Data = payload
	res, err := natsx.Request(ctx, r.NC, module, SubjectGroup(typ), m)
	if err != nil {
		return rt.Reply{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	var reply rt.Reply
	if err := json.Unmarshal(res.Data, &reply); err != nil {
		return rt.Reply{}, fmt.Errorf("router: reply of %s: %w", typ, err)
	}
	if !reply.OK && reply.Problem == nil {
		reply.Problem = &rt.ReplyProblem{Code: "INTERNAL", Retryable: true}
	}
	return reply, nil
}

// SubjectGroup is the subject_group label of eduworld_nats_request_* for a command type: its first segment
// ("battle.answer.submit" → "battle"), a closed list since the types come from the AsyncAPI (Routes).
func SubjectGroup(typ string) string {
	if i := strings.IndexByte(typ, '.'); i > 0 {
		return typ[:i]
	}
	if typ == "" {
		return "unknown"
	}
	return typ
}

// InitMetrics creates the eduworld_nats_request_errors_total series of every module reached by request/reply at 0
// (alert NatsRequestTimeouts needs them before the first failure).
func InitMetrics() {
	seen := map[string]bool{}
	for _, r := range Routes {
		if r.Kind == KindRequest && !seen[r.Module] {
			seen[r.Module] = true
			natsx.InitRequestMetrics(r.Module)
		}
	}
}

// RoomInput publishes payload to ew.in.room.<room> (fire-and-forget, no ack).
func (r Router) RoomInput(ctx context.Context, c rt.Caller, room string, payload json.RawMessage) error {
	if r.NC == nil {
		return ErrUnavailable
	}
	if !rt.ValidID(room) {
		return errors.New("router: invalid room id")
	}
	m := nats.NewMsg(rt.RoomInputSubject(room))
	m.Header = c.Header(ctx)
	m.Data = payload
	return r.NC.PublishMsg(m)
}

// Heartbeat sends ew.cmd.game.session.heartbeat for the session of c (F090-AC-12).
func (r Router) Heartbeat(ctx context.Context, c rt.Caller) (rt.Reply, error) {
	b, _ := json.Marshal(map[string]string{"game_session_id": c.GameSessionID.String()})
	return r.Request(ctx, c, rt.SessionHeartbeatModule, rt.SessionHeartbeatType, b)
}
