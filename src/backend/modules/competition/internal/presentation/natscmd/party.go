package natscmd

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/view"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// Party Lobby command types served by module competition (x-routing module competition, F101).
const (
	TypeReadySet       = "party.member.ready_set"
	TypeActivitySelect = "party.activity.select"
	TypeSessionStart   = "party.session.start"
)

// Lobby is the application of the lobby commands (command.Parties): the same use cases as REST.
type Lobby interface {
	SetReady(ctx context.Context, partyID uuid.UUID, ready bool) error
	SelectActivity(ctx context.Context, partyID uuid.UUID, in command.SelectInput) (view.Party, error)
	StartPartySession(ctx context.Context, partyID uuid.UUID, in command.StartInput) (view.SessionStart, error)
}

// wsCodes maps the REST problem codes of the lobby to the realtime ErrorCode (AsyncAPI): a party of another tenant or
// school, or a party the caller is not a member of, is PERMISSION_DENIED without saying it exists (F101-AC-20).
var wsCodes = map[problem.Code]string{
	problem.PartyNotFound:            "PERMISSION_DENIED",
	problem.NotFound:                 "PERMISSION_DENIED",
	problem.VersionMismatch:          "CONFLICT",
	problem.InvalidStateTransition:   "CONFLICT",
	problem.Conflict:                 "CONFLICT",
	problem.PartyNotLeader:           "PARTY_NOT_LEADER",
	problem.PartyNotReady:            "PARTY_NOT_READY",
	problem.PartyActivityNotEligible: "PARTY_ACTIVITY_NOT_ELIGIBLE",
	problem.RateLimited:              "RATE_LIMITED",
	problem.ValidationFailed:         "VALIDATION_FAILED",
	problem.Unavailable:              "UNAVAILABLE",
	problem.GameAccessStudentOnly:    "GAME_ACCESS_STUDENT_ONLY",
	problem.Unauthenticated:          "UNAUTHENTICATED",
	problem.PermissionDenied:         "PERMISSION_DENIED",
}

// lobbyFailure maps a use case error to the reply (internal errors are logged without payload, R14).
func lobbyFailure(ctx context.Context, err error) rt.Reply {
	var pe *problem.Error
	if !errors.As(err, &pe) {
		slog.ErrorContext(ctx, "competition party command failed", "error", err)
		r := rt.Fail("UNAVAILABLE", true)
		r.Problem.RetryAfterMs = 1000
		return r
	}
	code, ok := wsCodes[pe.Code]
	if !ok {
		code = "INTERNAL"
	}
	r := rt.Fail(code, pe.Retryable)
	var ra command.RetryAfter
	if errors.As(err, &ra) {
		r.Problem.RetryAfterMs = max(ra.After, time.Second).Milliseconds()
	}
	return r
}

func invalid() rt.Reply { return rt.Fail("VALIDATION_FAILED", false) }

// ReadySet serves party.member.ready_set (F101-AC-01): the same use case as setPartyReady.
func ReadySet(h Lobby) rt.CommandHandler {
	return func(ctx context.Context, c rt.Caller, payload json.RawMessage) rt.Reply {
		var in struct {
			PartyID uuid.UUID `json:"party_id"`
			Ready   *bool     `json:"ready"`
		}
		if err := json.Unmarshal(payload, &in); err != nil || in.PartyID == uuid.Nil || in.Ready == nil {
			return invalid()
		}
		ctx, err := studentContext(ctx, c)
		if err != nil {
			return rt.Fail("UNAUTHENTICATED", false)
		}
		if err := h.SetReady(ctx, in.PartyID, *in.Ready); err != nil {
			return lobbyFailure(ctx, err)
		}
		return rt.OK(nil)
	}
}

// ActivitySelect serves party.activity.select (F101-AC-03, AC-04).
func ActivitySelect(h Lobby) rt.CommandHandler {
	return func(ctx context.Context, c rt.Caller, payload json.RawMessage) rt.Reply {
		var in struct {
			PartyID  uuid.UUID `json:"party_id"`
			GameMode string    `json:"game_mode"`
			Expected *int64    `json:"expected_version"`
			Activity struct {
				Kind string `json:"kind"`
				ID   string `json:"id"`
			} `json:"activity"`
		}
		if err := json.Unmarshal(payload, &in); err != nil || in.PartyID == uuid.Nil || in.Expected == nil || len(in.Activity.ID) > 64 ||
			len(in.GameMode) > 64 {
			return invalid()
		}
		ctx, err := studentContext(ctx, c)
		if err != nil {
			return rt.Fail("UNAUTHENTICATED", false)
		}
		v, err := h.SelectActivity(ctx, in.PartyID, command.SelectInput{Expected: *in.Expected,
			Selection: party.Selection{Type: in.Activity.Kind, Ref: in.Activity.ID, GameMode: in.GameMode}})
		if err != nil {
			return lobbyFailure(ctx, err)
		}
		return rt.OK(map[string]any{"party_id": in.PartyID, "state_version": v.Party.StateVersion})
	}
}

// SessionStart serves party.session.start (F101-AC-07…AC-10); the game session of the caller comes from the
// Ew-Game-Session-Id header.
func SessionStart(h Lobby) rt.CommandHandler {
	return func(ctx context.Context, c rt.Caller, payload json.RawMessage) rt.Reply {
		var in struct {
			PartyID  uuid.UUID `json:"party_id"`
			Expected *int64    `json:"expected_version"`
		}
		if err := json.Unmarshal(payload, &in); err != nil || in.PartyID == uuid.Nil || in.Expected == nil {
			return invalid()
		}
		ctx, err := studentContext(ctx, c)
		if err != nil {
			return rt.Fail("UNAUTHENTICATED", false)
		}
		session := c.GameSessionID
		res, err := h.StartPartySession(ctx, in.PartyID, command.StartInput{Expected: *in.Expected, GameSession: &session})
		if err != nil {
			return lobbyFailure(ctx, err)
		}
		return rt.OK(res.Started())
	}
}
