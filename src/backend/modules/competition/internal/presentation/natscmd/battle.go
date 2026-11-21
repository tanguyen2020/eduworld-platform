package natscmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/battle"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/battlehost"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// Battle and co-op command types served by module competition (AsyncAPI receiveBattleCommands, receiveCoopCommands,
// F105). Identity and tenant come from the Ew-* headers only; a payload never carries a score, a time or a result
// (R5): unknown fields are refused.
const (
	TypeBattleAnswer = "battle.answer.submit"
	TypeBattleState  = "battle.state.request"
	TypeCoopAnswer   = "coop.answer.submit"
	TypeCoopAction   = "coop.action.perform"
)

// Rooms is the battle room host (battlehost.Host).
type Rooms interface {
	Answer(ctx context.Context, match uuid.UUID, c rt.Caller, in battlehost.AnswerIn) error
	State(ctx context.Context, match uuid.UUID, c rt.Caller) (json.RawMessage, error)
	Action(ctx context.Context, match uuid.UUID, c rt.Caller) error
}

// BattleMetrics counts the refused answers (eduworld_battle_answer_rejected_total{code}); nil = none.
type BattleMetrics interface {
	AnswerRejected(code string)
	Reconnect(result string)
}

// battleCodes maps the room errors to the realtime ErrorCode (F105-AC-03, AC-22, AC-27).
var battleCodes = []struct {
	err  error
	code string
}{
	{battle.ErrNotMember, "ROOM_NOT_MEMBER"},
	{battlehost.ErrWrongTenant, "PERMISSION_DENIED"},
	{battle.ErrLate, "ANSWER_LATE"},
	{battle.ErrAlreadySubmitted, "ANSWER_ALREADY_SUBMITTED"},
	{battle.ErrNotActive, "MATCH_NOT_ACTIVE"},
	{battle.ErrOut, "MATCH_NOT_ACTIVE"},
	{battlehost.ErrNoRoom, "MATCH_NOT_ACTIVE"},
	{battle.ErrNotOpen, "VALIDATION_FAILED"},
	{battle.ErrInvalidResponse, "VALIDATION_FAILED"},
	{battlehost.ErrNoActions, "VALIDATION_FAILED"},
}

func battleFailure(ctx context.Context, m BattleMetrics, err error) rt.Reply {
	for _, c := range battleCodes {
		if errors.Is(err, c.err) {
			if m != nil {
				m.AnswerRejected(c.code)
			}
			return rt.Fail(c.code, false)
		}
	}
	slog.ErrorContext(ctx, "competition battle command failed", "error", err) // never the response (R14)
	r := rt.Fail("UNAVAILABLE", true)
	r.Problem.RetryAfterMs = 1000
	return r
}

// strict decodes a payload refusing unknown fields and trailing data (VALIDATION_FAILED, F105-AC-27).
func strict(payload json.RawMessage, v any) bool {
	d := json.NewDecoder(bytes.NewReader(payload))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return false
	}
	return !d.More()
}

func validInstance(s string) bool { return s != "" && len(s) <= 64 }

// BattleAnswer serves battle.answer.submit (F105-AC-03, AC-06, AC-27): the room stamps the server time; the reply only
// acknowledges (the result comes with the reveal).
func BattleAnswer(rooms Rooms, m BattleMetrics) rt.CommandHandler {
	return func(ctx context.Context, c rt.Caller, payload json.RawMessage) rt.Reply {
		var in struct {
			MatchID         uuid.UUID       `json:"match_id"`
			Instance        string          `json:"question_instance_id"`
			Response        json.RawMessage `json:"response"`
			ClientElapsedMS *int64          `json:"client_elapsed_ms"` // reference only, never used to score
		}
		if !strict(payload, &in) || in.MatchID == uuid.Nil || !validInstance(in.Instance) || len(in.Response) == 0 {
			return battleFailure(ctx, m, battle.ErrInvalidResponse)
		}
		if err := rooms.Answer(ctx, in.MatchID, c, battlehost.AnswerIn{Instance: in.Instance, Response: in.Response}); err != nil {
			return battleFailure(ctx, m, err)
		}
		return rt.OK(map[string]any{"accepted": true})
	}
}

// BattleState serves battle.state.request (F105-AC-11, AC-14): (re)joins the room with the game session of the ticket,
// subscribes the connection to the room output and returns the snapshot.
func BattleState(rooms Rooms, m BattleMetrics) rt.CommandHandler {
	return func(ctx context.Context, c rt.Caller, payload json.RawMessage) rt.Reply {
		var in struct {
			MatchID uuid.UUID `json:"match_id"`
		}
		if !strict(payload, &in) || in.MatchID == uuid.Nil {
			return battleFailure(ctx, m, battle.ErrInvalidResponse)
		}
		snap, err := rooms.State(ctx, in.MatchID, c)
		if err != nil {
			if m != nil {
				m.Reconnect("refused")
			}
			return battleFailure(ctx, m, err)
		}
		if m != nil {
			m.Reconnect("ok")
		}
		r := rt.OK(nil)
		r.Result = snap
		r.Subscribe = []string{rt.RoomOutChannel(in.MatchID.String())}
		return r
	}
}

// CoopAnswer serves coop.answer.submit for a co-op challenge (F105-AC-18): the task is the question.
func CoopAnswer(rooms Rooms, m BattleMetrics) rt.CommandHandler {
	return func(ctx context.Context, c rt.Caller, payload json.RawMessage) rt.Reply {
		var in struct {
			RoomID   string          `json:"room_id"`
			TaskID   string          `json:"task_id"`
			Instance string          `json:"question_instance_id"`
			Response json.RawMessage `json:"response"`
		}
		if !strict(payload, &in) || !validInstance(in.Instance) || len(in.TaskID) > 64 || len(in.Response) == 0 {
			return battleFailure(ctx, m, battle.ErrInvalidResponse)
		}
		room, err := uuid.Parse(in.RoomID)
		if err != nil || len(in.RoomID) > 64 {
			return battleFailure(ctx, m, battle.ErrInvalidResponse)
		}
		if err := rooms.Answer(ctx, room, c, battlehost.AnswerIn{Instance: in.Instance, Response: in.Response}); err != nil {
			return battleFailure(ctx, m, err)
		}
		return rt.OK(map[string]any{"accepted": true})
	}
}

// CoopAction serves coop.action.perform: a co-op challenge of the battle engine has no puzzle actions (the co-op quests
// of module game do); a member gets VALIDATION_FAILED, anybody else ROOM_NOT_MEMBER.
func CoopAction(rooms Rooms, m BattleMetrics) rt.CommandHandler {
	return func(ctx context.Context, c rt.Caller, payload json.RawMessage) rt.Reply {
		var in struct {
			RoomID       string          `json:"room_id"`
			StateVersion int64           `json:"state_version"`
			ActionType   string          `json:"action_type"`
			Data         json.RawMessage `json:"data"`
		}
		if !strict(payload, &in) || len(in.ActionType) > 64 {
			return battleFailure(ctx, m, battle.ErrInvalidResponse)
		}
		room, err := uuid.Parse(in.RoomID)
		if err != nil {
			return battleFailure(ctx, m, battle.ErrInvalidResponse)
		}
		return battleFailure(ctx, m, rooms.Action(ctx, room, c))
	}
}
