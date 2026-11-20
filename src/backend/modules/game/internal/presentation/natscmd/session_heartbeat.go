// Package natscmd holds the NATS command handlers of module game called by the realtime gateway
// (ew.cmd.game.<type>, REALTIME_ARCHITECTURE §4.1, ADR-0074): thin adapters from the Ew-* headers and the payload to
// one use case, answering the reply of pkg/realtime.
package natscmd

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// Heartbeater is the use case behind ew.cmd.game.session.heartbeat (command.Sessions.RealtimeHeartbeat).
type Heartbeater interface {
	RealtimeHeartbeat(ctx context.Context, tenant, student, session, device uuid.UUID) error
}

// SessionHeartbeat handles ew.cmd.game.session.heartbeat (F090-AC-12): payload {game_session_id} must name the
// session bound to the connection (Ew-Game-Session-Id); the tenant is the one of the ticket (Ew-Tenant-Id).
func SessionHeartbeat(h Heartbeater) rt.CommandHandler {
	return func(ctx context.Context, c rt.Caller, payload json.RawMessage) rt.Reply {
		var in struct {
			GameSessionID string `json:"game_session_id"`
		}
		if err := json.Unmarshal(payload, &in); err != nil || len(in.GameSessionID) > 36 {
			return rt.Fail(string(problem.ValidationFailed), false)
		}
		if in.GameSessionID != "" && in.GameSessionID != c.GameSessionID.String() {
			return rt.Fail(string(problem.ValidationFailed), false)
		}
		if err := h.RealtimeHeartbeat(ctx, c.TenantID, c.UserID, c.GameSessionID, c.DeviceID); err != nil {
			return failure(ctx, err)
		}
		return rt.OK(nil)
	}
}

// failure maps a use case error to the reply problem (AsyncAPI ErrorCode); unknown errors are INTERNAL, retryable.
func failure(ctx context.Context, err error) rt.Reply {
	var pe *problem.Error
	if !errors.As(err, &pe) {
		slog.ErrorContext(ctx, "realtime command failed", "error", err)
		return rt.Fail("INTERNAL", true)
	}
	return rt.Fail(string(pe.Code), pe.Retryable)
}
