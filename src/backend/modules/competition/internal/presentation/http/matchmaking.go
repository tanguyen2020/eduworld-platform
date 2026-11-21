package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/view"
	mm "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/matchmaking"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// MatchmakingCommands are the F104 write use cases (command.Matchmaking).
type MatchmakingCommands interface {
	CreateTicket(ctx context.Context, in mm.Request) (view.Ticket, error)
	CancelTicket(ctx context.Context, id uuid.UUID) error
	AcceptProposal(ctx context.Context, id uuid.UUID) error
	DeclineProposal(ctx context.Context, id uuid.UUID) error
	StartBotMatch(ctx context.Context, ticket uuid.UUID) (map[string]any, error)
	CreateDuel(ctx context.Context, in command.DuelInput) (map[string]any, error)
	AcceptDuel(ctx context.Context, id uuid.UUID) (map[string]any, error)
	DeclineDuel(ctx context.Context, id uuid.UUID) error
}

// MatchmakingQueries are the F104 read use cases (query.Matchmaking).
type MatchmakingQueries interface {
	GetTicket(ctx context.Context, id uuid.UUID) (view.Ticket, error)
	ListCompetitionModes(ctx context.Context) ([]map[string]any, error)
}

// MatchmakingOperations of F104: every route is a player route (game_player + X-Tenant-Id, F104-AC-23); Idempotency-Key
// on createMatchmakingTicket, startBotMatch, createDuel and acceptDuel (F104-AC-25).
var MatchmakingOperations = map[string]httpx.Operation{
	"createMatchmakingTicket": op("createMatchmakingTicket", http.MethodPost, "/competition/matchmaking/tickets", httpx.IdempotencyRequired, http.StatusCreated),
	"getMatchmakingTicket":    op("getMatchmakingTicket", http.MethodGet, "/competition/matchmaking/tickets/{ticket_id}", httpx.IdempotencyNone, http.StatusOK),
	"cancelMatchmakingTicket": op("cancelMatchmakingTicket", http.MethodDelete, "/competition/matchmaking/tickets/{ticket_id}", httpx.IdempotencyNone, http.StatusNoContent),
	"startBotMatch":           op("startBotMatch", http.MethodPost, "/competition/matchmaking/tickets/{ticket_id}:bot-match", httpx.IdempotencyRequired, http.StatusCreated),
	"acceptMatchProposal":     op("acceptMatchProposal", http.MethodPost, "/competition/matches/{match_id}/proposal:accept", httpx.IdempotencyNone, http.StatusNoContent),
	"declineMatchProposal":    op("declineMatchProposal", http.MethodPost, "/competition/matches/{match_id}/proposal:decline", httpx.IdempotencyNone, http.StatusNoContent),
	"listCompetitionModes":    op("listCompetitionModes", http.MethodGet, "/competition/modes", httpx.IdempotencyNone, http.StatusOK),
	"createDuel":              op("createDuel", http.MethodPost, "/competition/duels", httpx.IdempotencyRequired, http.StatusCreated),
	"acceptDuel":              op("acceptDuel", http.MethodPost, "/competition/duels/{duel_id}:accept", httpx.IdempotencyRequired, http.StatusOK),
	"declineDuel":             op("declineDuel", http.MethodPost, "/competition/duels/{duel_id}:decline", httpx.IdempotencyNone, http.StatusNoContent),
}

// MatchmakingPlayerOperations are the F104 routes in registration order.
var MatchmakingPlayerOperations = []string{"createMatchmakingTicket", "getMatchmakingTicket", "cancelMatchmakingTicket", "startBotMatch",
	"acceptMatchProposal", "declineMatchProposal", "listCompetitionModes", "createDuel", "acceptDuel", "declineDuel"}

func init() {
	for id, o := range MatchmakingOperations {
		Operations[id] = o
	}
}

// registerMatchmaking mounts the F104 operations.
func registerMatchmaking(s *httpx.Server, c MatchmakingCommands, q MatchmakingQueries) error {
	routes := map[string]http.HandlerFunc{
		"createMatchmakingTicket": createTicket(c),
		"getMatchmakingTicket": idView("ticket_id", func(ctx context.Context, id uuid.UUID) (any, error) {
			v, err := q.GetTicket(ctx, id)
			return v.Body(), err
		}),
		"cancelMatchmakingTicket": idAction("ticket_id", c.CancelTicket),
		"startBotMatch":           idCreate("ticket_id", c.StartBotMatch),
		"acceptMatchProposal":     idAction("match_id", c.AcceptProposal),
		"declineMatchProposal":    idAction("match_id", c.DeclineProposal),
		"listCompetitionModes":    listModes(q),
		"createDuel":              createDuel(c),
		"acceptDuel": idView("duel_id", func(ctx context.Context, id uuid.UUID) (any, error) {
			return c.AcceptDuel(ctx, id)
		}),
		"declineDuel": idAction("duel_id", c.DeclineDuel),
	}
	for _, id := range MatchmakingPlayerOperations {
		if err := s.Handle(MatchmakingOperations[id], routes[id]); err != nil {
			return err
		}
	}
	return nil
}

func idAction(param string, fn func(ctx context.Context, id uuid.UUID) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathUUID(r, param)
		if err != nil {
			fail(w, r, err)
			return
		}
		if err := fn(r.Context(), id); err != nil {
			fail(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func idView(param string, fn func(ctx context.Context, id uuid.UUID) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathUUID(r, param)
		if err != nil {
			fail(w, r, err)
			return
		}
		v, err := fn(r.Context(), id)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, v)
	}
}

func idCreate(param string, fn func(ctx context.Context, id uuid.UUID) (map[string]any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathUUID(r, param)
		if err != nil {
			fail(w, r, err)
			return
		}
		v, err := fn(r.Context(), id)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, v)
	}
}

func createTicket(c MatchmakingCommands) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Mode            string     `json:"mode"`
			SubjectID       uuid.UUID  `json:"subject_id"`
			TopicID         *uuid.UUID `json:"topic_id"`
			PartyID         *uuid.UUID `json:"party_id"`
			MatchmakingType *string    `json:"matchmaking_type"`
			AllowFill       *bool      `json:"allow_fill"`
		}
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			fail(w, r, problem.Invalid("body", problem.SubInvalid, "must be a MatchmakingTicketCreate"))
			return
		}
		in := mm.Request{Mode: b.Mode, Type: mm.TypeSkill, SubjectID: b.SubjectID, TopicID: b.TopicID, PartyID: b.PartyID}
		if b.MatchmakingType != nil {
			in.Type = *b.MatchmakingType
		}
		if b.AllowFill != nil {
			in.AllowFill = *b.AllowFill
		}
		v, err := c.CreateTicket(r.Context(), in)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, v.Body())
	}
}

func listModes(q MatchmakingQueries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := q.ListCompetitionModes(r.Context())
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": data})
	}
}

func createDuel(c MatchmakingCommands) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			OpponentID uuid.UUID  `json:"opponent_id"`
			SubjectID  uuid.UUID  `json:"subject_id"`
			TopicID    *uuid.UUID `json:"topic_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			fail(w, r, problem.Invalid("body", problem.SubInvalid, "must be a duel request"))
			return
		}
		v, err := c.CreateDuel(r.Context(), command.DuelInput{OpponentID: b.OpponentID, SubjectID: b.SubjectID, TopicID: b.TopicID})
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, v)
	}
}
