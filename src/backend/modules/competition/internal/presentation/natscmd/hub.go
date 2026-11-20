// Package natscmd holds the NATS command handlers of module competition for the realtime gateway (F090 routing,
// REALTIME_ARCHITECTURE §4.1): identity and tenant come from the Ew-* headers only, never from the payload
// (F091-AC-08); the reply carries the topics the gateway subscribes the connection to.
package natscmd

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// Module and command types served by module competition for the hub (x-routing of the AsyncAPI).
const (
	Module           = "competition"
	TypeInstanceJoin = "hub.instance.join"
	TypePlayerEmote  = "hub.player.emote"
)

// Hub is the application of the hub commands.
type Hub interface {
	JoinHubInstance(ctx context.Context, c command.Caller, in command.JoinInput) (*command.Assigned, error)
	EmitEmote(ctx context.Context, c command.Caller, room, emote string) error
}

func caller(c rt.Caller) command.Caller {
	return command.Caller{TenantID: c.TenantID, UserID: c.UserID, GameSessionID: c.GameSessionID, NodeID: c.NodeID}
}

// failure maps an application error to the reply (internal errors are logged without payload, R14).
func failure(ctx context.Context, err error) rt.Reply {
	var e *command.Error
	if errors.As(err, &e) {
		r := rt.Fail(e.Code, e.Retryable)
		r.Problem.RetryAfterMs = e.RetryAfter.Milliseconds()
		return r
	}
	slog.ErrorContext(ctx, "competition hub command failed", "error", err)
	r := rt.Fail("UNAVAILABLE", true)
	r.Problem.RetryAfterMs = 1000
	return r
}

// InstanceJoin serves hub.instance.join: reply hub.instance.assigned as the ack result and subscribe the room output
// (F091-AC-01); unsubscribe the previous room when the student changed area.
func InstanceJoin(h Hub) rt.CommandHandler {
	return func(ctx context.Context, c rt.Caller, payload json.RawMessage) rt.Reply {
		var in struct {
			AreaID     string  `json:"area_id"`
			SpawnPoint string  `json:"spawn_point"`
			Follow     *string `json:"follow_user_id"`
		}
		if err := json.Unmarshal(payload, &in); err != nil {
			return rt.Fail(command.CodeValidation, false)
		}
		ji := command.JoinInput{AreaID: in.AreaID, SpawnPoint: in.SpawnPoint}
		if in.Follow != nil {
			id, err := uuid.Parse(*in.Follow)
			if err != nil || len(*in.Follow) > 36 {
				return rt.Fail(command.CodeValidation, false)
			}
			ji.Follow = &id
		}
		a, err := h.JoinHubInstance(ctx, caller(c), ji)
		if err != nil {
			return failure(ctx, err)
		}
		r := rt.OK(a)
		r.Subscribe = []string{rt.RoomOutChannel(a.RoomID)}
		if a.Previous != "" {
			r.Unsubscribe = []string{rt.RoomOutChannel(a.Previous)}
		}
		return r
	}
}

// PlayerEmote serves hub.player.emote (F091-AC-07, AC-18).
func PlayerEmote(h Hub) rt.CommandHandler {
	return func(ctx context.Context, c rt.Caller, payload json.RawMessage) rt.Reply {
		var in struct {
			RoomID  string `json:"room_id"`
			EmoteID string `json:"emote_id"`
		}
		if err := json.Unmarshal(payload, &in); err != nil || len(in.RoomID) > 64 || len(in.EmoteID) > 64 {
			return rt.Fail(command.CodeValidation, false)
		}
		if err := h.EmitEmote(ctx, caller(c), in.RoomID, in.EmoteID); err != nil {
			return failure(ctx, err)
		}
		return rt.OK(nil)
	}
}
