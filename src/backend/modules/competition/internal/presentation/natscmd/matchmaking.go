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

// Matchmaking command types served by module competition (AsyncAPI receiveMatchmakingCommands, F104).
const (
	TypeTicketCancel    = "matchmaking.ticket.cancel"
	TypeProposalAccept  = "matchmaking.proposal.accept"
	TypeProposalDecline = "matchmaking.proposal.decline"
)

// MatchmakingCommands is the application of the matchmaking commands (command.Matchmaking): the same use cases as REST.
type MatchmakingCommands interface {
	CancelTicket(ctx context.Context, id uuid.UUID) error
	AcceptProposal(ctx context.Context, id uuid.UUID) error
	DeclineProposal(ctx context.Context, id uuid.UUID) error
}

// mmCodes maps the problem codes of matchmaking to the realtime ErrorCode: a proposal that closed (declined, timed out)
// is PROPOSAL_EXPIRED; a ticket or proposal of someone else is NOT_FOUND (F104-AC-24).
var mmCodes = map[problem.Code]string{
	problem.NotFound:              "NOT_FOUND",
	problem.Conflict:              "PROPOSAL_EXPIRED",
	problem.PartyNotLeader:        "PARTY_NOT_LEADER",
	problem.Unavailable:           "UNAVAILABLE",
	problem.GameAccessStudentOnly: "GAME_ACCESS_STUDENT_ONLY",
	problem.Unauthenticated:       "UNAUTHENTICATED",
	problem.ValidationFailed:      "VALIDATION_FAILED",
}

func mmFailure(ctx context.Context, err error) rt.Reply {
	var pe *problem.Error
	if !errors.As(err, &pe) {
		slog.ErrorContext(ctx, "competition matchmaking command failed", "error", err)
		r := rt.Fail("UNAVAILABLE", true)
		r.Problem.RetryAfterMs = 1000
		return r
	}
	code, ok := mmCodes[pe.Code]
	if !ok {
		code = "INTERNAL"
	}
	return rt.Fail(code, pe.Retryable)
}

func idCommand(field string, fn func(ctx context.Context, id uuid.UUID) error) rt.CommandHandler {
	return func(ctx context.Context, c rt.Caller, payload json.RawMessage) rt.Reply {
		var in map[string]uuid.UUID
		if err := json.Unmarshal(payload, &in); err != nil || in[field] == uuid.Nil {
			return invalid()
		}
		ctx, err := studentContext(ctx, c)
		if err != nil {
			return rt.Fail("UNAUTHENTICATED", false)
		}
		if err := fn(ctx, in[field]); err != nil {
			return mmFailure(ctx, err)
		}
		return rt.OK(nil)
	}
}

// TicketCancel serves matchmaking.ticket.cancel (F104-AC-10).
func TicketCancel(h MatchmakingCommands) rt.CommandHandler {
	return idCommand("ticket_id", h.CancelTicket)
}

// ProposalAccept serves matchmaking.proposal.accept (F104-AC-05).
func ProposalAccept(h MatchmakingCommands) rt.CommandHandler {
	return idCommand("proposal_id", h.AcceptProposal)
}

// ProposalDecline serves matchmaking.proposal.decline (F104-AC-06).
func ProposalDecline(h MatchmakingCommands) rt.CommandHandler {
	return idCommand("proposal_id", h.DeclineProposal)
}
