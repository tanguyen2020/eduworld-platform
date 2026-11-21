package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Headers set by the gateway on every command (REALTIME_ARCHITECTURE §2.1). Handlers in core-api trust them only
// because the NATS permission to publish ew.cmd.> / ew.in.> belongs to the user of binary realtime alone
// (SECURITY_ARCHITECTURE §8.1); the tenant always comes from the ticket, never from client data (F090-AC-09).
const (
	HeaderTenantID      = "Ew-Tenant-Id"
	HeaderUserID        = "Ew-User-Id"
	HeaderGameSessionID = "Ew-Game-Session-Id"
	HeaderDeviceID      = "Ew-Device-Id"
	HeaderConnID        = "Ew-Conn-Id"
	HeaderNodeID        = "Ew-Node-Id"
	HeaderTraceparent   = "traceparent"
)

// CommandSubject is ew.cmd.<module>.<type> (request/reply to the owner module in core-api).
func CommandSubject(module, typ string) string { return "ew.cmd." + module + "." + typ }

// RoomInputSubject is ew.in.room.<room_id> (NATS core, fire-and-forget, room host of module competition).
func RoomInputSubject(room string) string { return "ew.in.room." + room }

// SessionHeartbeat is the internal command the gateway sends to module game every ws_heartbeat (F090-AC-12).
const (
	SessionHeartbeatModule = "game"
	SessionHeartbeatType   = "session.heartbeat"
)

// Caller is the authenticated identity of a connection, fixed at handshake from the ticket.
type Caller struct {
	TenantID      uuid.UUID
	UserID        uuid.UUID
	SchoolID      uuid.UUID
	GameSessionID uuid.UUID
	DeviceID      uuid.UUID
	ConnID        string
	NodeID        string
}

// Header returns the Ew-* headers of c plus traceparent of ctx.
func (c Caller) Header(ctx context.Context) nats.Header {
	h := nats.Header{}
	h.Set(HeaderTenantID, c.TenantID.String())
	h.Set(HeaderUserID, c.UserID.String())
	h.Set(HeaderGameSessionID, c.GameSessionID.String())
	h.Set(HeaderDeviceID, c.DeviceID.String())
	h.Set(HeaderConnID, c.ConnID)
	h.Set(HeaderNodeID, c.NodeID)
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(http.Header(h)))
	if h.Get(HeaderTraceparent) == "" {
		if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
			h.Set(HeaderTraceparent, fmt.Sprintf("00-%s-%s-%s", sc.TraceID(), sc.SpanID(), sc.TraceFlags()))
		}
	}
	return h
}

// ErrBadCaller is returned when a command lacks a valid Ew-* header.
var ErrBadCaller = errors.New("realtime: missing or invalid Ew-* header")

// CallerFrom reads the Ew-* headers of a command.
func CallerFrom(h nats.Header) (Caller, error) {
	id := func(k string) (uuid.UUID, error) {
		v, err := uuid.Parse(strings.TrimSpace(h.Get(k)))
		if err != nil || v == uuid.Nil {
			return uuid.Nil, fmt.Errorf("%w: %s", ErrBadCaller, k)
		}
		return v, nil
	}
	var c Caller
	var err error
	if c.TenantID, err = id(HeaderTenantID); err != nil {
		return c, err
	}
	if c.UserID, err = id(HeaderUserID); err != nil {
		return c, err
	}
	if c.GameSessionID, err = id(HeaderGameSessionID); err != nil {
		return c, err
	}
	if c.DeviceID, err = id(HeaderDeviceID); err != nil {
		return c, err
	}
	c.ConnID, c.NodeID = h.Get(HeaderConnID), h.Get(HeaderNodeID)
	if len(c.ConnID) > 64 || len(c.NodeID) > 64 {
		return c, ErrBadCaller
	}
	return c, nil
}

// Reply is the answer of a command handler (REALTIME_ARCHITECTURE §4.1).
type Reply struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result,omitempty"`
	Subscribe   []string        `json:"subscribe,omitempty"`
	Unsubscribe []string        `json:"unsubscribe,omitempty"`
	Problem     *ReplyProblem   `json:"problem,omitempty"`
}

// ReplyProblem is the error of a failed command (code of the AsyncAPI ErrorCode enum).
type ReplyProblem struct {
	Code         string `json:"code"`
	Retryable    bool   `json:"retryable"`
	RetryAfterMs int64  `json:"retry_after_ms,omitempty"`
	Message      string `json:"message,omitempty"`
	// Details are the stable details of the failure (CHAT_CONTENT_BLOCKED: category_code only — never a matched term).
	Details map[string]string `json:"details,omitempty"`
}

// OK is a successful reply with an optional result object.
func OK(result any) Reply {
	r := Reply{OK: true}
	if result != nil {
		if b, err := json.Marshal(result); err == nil {
			r.Result = b
		}
	}
	return r
}

// Fail is a failed reply.
func Fail(code string, retryable bool) Reply {
	return Reply{Problem: &ReplyProblem{Code: code, Retryable: retryable}}
}

// CommandHandler handles one command type; ctx carries the trace of the gateway.
type CommandHandler func(ctx context.Context, c Caller, payload json.RawMessage) Reply

// ServeCommand answers ew.cmd.<module>.<type> in queue group <module> until ctx ends (one replica of core-api
// handles each request). A request without valid Ew-* headers gets UNAUTHENTICATED and is not handled.
func ServeCommand(ctx context.Context, nc *nats.Conn, module, typ string, h CommandHandler, timeout time.Duration) error {
	if nc == nil {
		return errors.New("realtime: no NATS connection")
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	subject := CommandSubject(module, typ)
	sub, err := nc.QueueSubscribe(subject, module, func(m *nats.Msg) {
		reply := handle(ctx, m, h, timeout)
		b, err := json.Marshal(reply)
		if err != nil {
			b = []byte(`{"ok":false,"problem":{"code":"INTERNAL","retryable":true}}`)
		}
		if err := m.Respond(b); err != nil {
			slog.WarnContext(ctx, "realtime command reply failed", "subject", subject, "error", err)
		}
	})
	if err != nil {
		return fmt.Errorf("realtime: subscribe %s: %w", subject, err)
	}
	<-ctx.Done()
	_ = sub.Drain()
	return nil
}

func handle(parent context.Context, m *nats.Msg, h CommandHandler, timeout time.Duration) (r Reply) {
	defer func() {
		if p := recover(); p != nil {
			slog.ErrorContext(parent, "realtime command panic", "subject", m.Subject)
			r = Fail("INTERNAL", true)
		}
	}()
	c, err := CallerFrom(m.Header)
	if err != nil {
		return Fail("UNAUTHENTICATED", false)
	}
	ctx := otel.GetTextMapPropagator().Extract(context.WithoutCancel(parent), propagation.HeaderCarrier(http.Header(m.Header)))
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	payload := m.Data
	if len(payload) == 0 {
		payload = []byte("{}")
	}
	return h(ctx, c, payload)
}
