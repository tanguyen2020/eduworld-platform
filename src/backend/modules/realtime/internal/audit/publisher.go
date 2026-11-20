// Package audit publishes the audit events of the gateway (F090-AC-22, research R4): the binary realtime has no
// database and no outbox, so audit.recorded goes straight to JetStream ew.evt.realtime.audit.recorded (stream
// EVT_REALTIME, Nats-Msg-Id = event_id), consumed by the audit sink of module identity. At-most-once is accepted
// for rate-limit violations (not a business operation). Envelope and payload follow the common schemas.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// Module is the producer module of the gateway events.
const Module = "realtime"

// ActionRateLimitClosed is the audit action of a connection closed with 4429.
const ActionRateLimitClosed = "realtime.rate_limit_closed"

// Publisher publishes audit.recorded.
type Publisher struct {
	JS      jetstream.JetStream
	writer  *outbox.Writer
	Timeout time.Duration
}

// NewPublisher builds the publisher over js.
func NewPublisher(js jetstream.JetStream) (*Publisher, error) {
	w, err := outbox.NewWriter(Module, "gateway", nil)
	if err != nil {
		return nil, err
	}
	return &Publisher{JS: js, writer: w, Timeout: 2 * time.Second}, nil
}

// EnsureStream creates or updates EVT_REALTIME / DLQ_REALTIME (idempotent, at start-up).
func EnsureStream(ctx context.Context, js jetstream.JetStream) error {
	return natsx.EnsureStreams(ctx, js, Module, natsx.StreamOptions{})
}

// Envelope builds the audit.recorded envelope of a 4429 close of connection c.
func (p *Publisher) Envelope(ctx context.Context, c rt.Caller, reason string) (outbox.Envelope, error) {
	tenant := c.TenantID
	ctx = authz.WithPrincipal(ctx, authz.NewPrincipal(authz.PrincipalData{UserID: c.UserID, UserType: authz.UserTypeStudent,
		TenantID: &tenant}))
	actor := c.UserID.String()
	school := any(nil)
	if c.SchoolID != uuid.Nil {
		school = c.SchoolID.String()
	}
	return p.writer.Envelope(ctx, outbox.Message{Type: "audit.recorded", Subject: outbox.Subject{Type: "realtime_connection", ID: c.ConnID},
		TenantID: &tenant, Payload: map[string]any{"actor_id": actor, "actor_type": "user", "action": ActionRateLimitClosed,
			"target_type": "realtime_connection", "target_id": c.ConnID, "result": "denied", "reason": reason, "school_id": school,
			"metadata": map[string]any{"node_id": c.NodeID, "game_session_id": c.GameSessionID.String()}}})
}

// RateLimitClosed publishes the audit of a connection closed with 4429 (F090-AC-22).
func (p *Publisher) RateLimitClosed(ctx context.Context, c rt.Caller) error {
	if p == nil || p.JS == nil {
		return nil
	}
	e, err := p.Envelope(ctx, c, "RATE_LIMITED")
	if err != nil {
		return err
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), p.Timeout)
	defer cancel()
	m := nats.NewMsg(natsx.EventSubject(Module, e.EventType))
	m.Data = b
	m.Header.Set(jetstream.MsgIDHeader, e.EventID.String())
	if e.TenantID != nil {
		m.Header.Set(natsx.HeaderTenantID, e.TenantID.String())
	}
	if _, err := p.JS.PublishMsg(ctx, m); err != nil {
		return fmt.Errorf("audit: publish: %w", err)
	}
	return nil
}
