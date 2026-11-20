// Package protocol is the WebSocket protocol eduworld.v1 of the gateway (AsyncAPI eduworld-realtime, REALTIME_
// ARCHITECTURE §3.1, §4): envelopes, close codes, error codes and the validator of client commands generated from
// the AsyncAPI (validate_gen.go, scripts/realtime/gen-routes.mjs).
package protocol

import (
	"encoding/json"
	"time"
)

// Subprotocol is the only accepted Sec-WebSocket-Protocol (REALTIME_ARCHITECTURE §10).
const Subprotocol = "eduworld.v1"

// Version is v of every envelope and protocol_version of the welcome.
const Version = 1

// Close codes (REALTIME_ARCHITECTURE §3.1, the owner of the list).
const (
	CloseServiceRestart = 1012 // node shutdown after system.server.draining
	CloseProtocol       = 4400
	CloseTicket         = 4401
	CloseRevoked        = 4403
	CloseTimeout        = 4408
	CloseReplaced       = 4409
	CloseSessionEnded   = 4410
	CloseRateLimited    = 4429
)

// Error codes of system.command.failed (AsyncAPI ErrorCode).
const (
	CodeUnknownType      = "UNKNOWN_TYPE"
	CodeValidationFailed = "VALIDATION_FAILED"
	CodeRateLimited      = "RATE_LIMITED"
	CodeUnavailable      = "UNAVAILABLE"
	CodeInternal         = "INTERNAL"
	CodeConflict         = "CONFLICT"
	CodeNotFound         = "NOT_FOUND"
)

// Server message types sent by the gateway itself.
const (
	TypeWelcome       = "system.session.welcome"
	TypePong          = "system.heartbeat.pong"
	TypePing          = "system.heartbeat.ping"
	TypeResume        = "system.session.resume"
	TypePresenceSet   = "presence.status.set" // handled by the gateway (F091-AC-22)
	TypeResumed       = "system.session.resumed"
	TypeResumeFailed  = "system.session.resume_failed"
	TypeAcked         = "system.command.acked"
	TypeFailed        = "system.command.failed"
	TypeDegraded      = "system.service.degraded"
	TypeDraining      = "system.server.draining"
	TypeSessionEnded  = "session.session.ended"
	TypeSessionReplay = "session.session.replaced"
)

// ClientEnvelope is a client command (AsyncAPI ClientEnvelope).
type ClientEnvelope struct {
	V       int             `json:"v"`
	Type    string          `json:"type"`
	ID      string          `json:"id"`
	Ts      int64           `json:"ts,omitempty"`
	Payload json.RawMessage `json:"payload"`
}

// ServerEnvelope is a server message (AsyncAPI ServerEnvelope); seq is assigned by the session.
type ServerEnvelope struct {
	V       int             `json:"v"`
	Type    string          `json:"type"`
	Seq     int64           `json:"seq"`
	Ref     string          `json:"ref,omitempty"`
	Ts      int64           `json:"ts"`
	Payload json.RawMessage `json:"payload"`
}

// Out is a server message waiting for its seq.
type Out struct {
	Type      string
	Ref       string
	Ts        int64
	Payload   json.RawMessage
	Droppable bool // dropped first when the send queue is full; never buffered for resume
	Transient bool // not buffered for resume (pong, welcome, resume answers, draining, degraded)
}

// NewOut builds a message with payload v at server time now.
func NewOut(typ string, v any, now time.Time) Out {
	b, err := json.Marshal(v)
	if err != nil || len(b) == 0 || b[0] != '{' {
		b = []byte("{}")
	}
	return Out{Type: typ, Ts: now.UnixMilli(), Payload: b}
}

// Encode returns the ServerEnvelope of o with seq.
func (o Out) Encode(seq int64) []byte {
	p := o.Payload
	if len(p) == 0 {
		p = json.RawMessage("{}")
	}
	b, _ := json.Marshal(ServerEnvelope{V: Version, Type: o.Type, Seq: seq, Ref: o.Ref, Ts: o.Ts, Payload: p})
	return b
}

// Failure is the payload of system.command.failed.
type Failure struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	RetryAfterMs int64  `json:"retry_after_ms,omitempty"`
}

// Failed builds system.command.failed for ref (empty when the frame had no usable id).
func Failed(ref, code, message string, retryAfter time.Duration, now time.Time) Out {
	if message == "" {
		message = code
	}
	o := NewOut(TypeFailed, Failure{Code: code, Message: message, RetryAfterMs: retryAfter.Milliseconds()}, now)
	o.Ref = ref
	return o
}

// Acked builds system.command.acked for ref with the result of the module.
func Acked(ref string, result json.RawMessage, now time.Time) Out {
	payload := map[string]json.RawMessage{}
	if len(result) > 0 && result[0] == '{' {
		payload["result"] = result
	}
	o := NewOut(TypeAcked, payload, now)
	o.Ref = ref
	return o
}
