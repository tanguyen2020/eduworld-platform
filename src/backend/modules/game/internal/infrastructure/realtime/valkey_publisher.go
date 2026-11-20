// Package realtime publishes server messages of module game to the realtime gateway through Valkey (F014 T017,
// REALTIME_ARCHITECTURE §3.1): PUBLISH ew:rt:user:<student_id> {type, payload}; the gateway (F090) wraps it in the
// ServerEnvelope of AsyncAPI eduworld-realtime and closes the connection holding the session (4409 / 4410 / 4403).
// Game clients never talk to Valkey (R25).
package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
)

// Channel is the Valkey channel of a user's connections.
func Channel(user uuid.UUID) string { return "ew:rt:user:" + user.String() }

// Message is what the gateway receives on ew:rt:user:<id>.
type Message struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
}

// ValkeyPublisher implements port.RealtimePublisher.
type ValkeyPublisher struct {
	C       valkey.Client
	Timeout time.Duration // per publish (default 2 s): a slow Valkey never blocks the caller for long
}

var _ port.RealtimePublisher = ValkeyPublisher{}

func (p ValkeyPublisher) publish(ctx context.Context, user uuid.UUID, m Message) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	if err := p.C.Do(ctx, p.C.B().Publish().Channel(Channel(user)).Message(string(b)).Build()).Error(); err != nil {
		return fmt.Errorf("realtime: publish %s: %w", m.Type, err)
	}
	return nil
}

// SessionReplaced implements port.RealtimePublisher (session.session.replaced, close 4409).
func (p ValkeyPublisher) SessionReplaced(ctx context.Context, student, session uuid.UUID, newPlatform string, at time.Time) error {
	return p.publish(ctx, student, Message{Type: "session.session.replaced", Payload: map[string]any{
		"game_session_id": session.String(), "replaced_at": at.UnixMilli(), "new_device_platform": newPlatform}})
}

// SessionEnded implements port.RealtimePublisher (session.session.ended: curfew, daily_limit → 4410 with reopen_at;
// else 4403).
func (p ValkeyPublisher) SessionEnded(ctx context.Context, student, session uuid.UUID, reason string, at time.Time, reopenAt *time.Time) error {
	payload := map[string]any{"game_session_id": session.String(), "reason": reason, "ended_at": at.UnixMilli()}
	if reopenAt != nil && (reason == "curfew" || reason == "daily_limit") {
		payload["reopen_at"] = reopenAt.UnixMilli()
	}
	return p.publish(ctx, student, Message{Type: "session.session.ended", Payload: payload})
}

// SessionEndingSoon implements port.RealtimePublisher (session.session.ending_soon: warning before the daily play
// limit ends the session, OQ-17).
func (p ValkeyPublisher) SessionEndingSoon(ctx context.Context, student, session uuid.UUID, reason string, endsAt time.Time) error {
	return p.publish(ctx, student, Message{Type: "session.session.ending_soon", Payload: map[string]any{
		"game_session_id": session.String(), "reason": reason, "ends_at": endsAt.UnixMilli()}})
}
