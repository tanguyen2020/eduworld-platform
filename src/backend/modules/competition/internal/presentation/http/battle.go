package http

import (
	"context"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// BattleQueries are the F105 read use cases (query.Battles). No route creates or changes a match result: results are
// written by the room host only (F105-AC-08).
type BattleQueries interface {
	GetMatch(ctx context.Context, id uuid.UUID) (map[string]any, error)
	ListMyMatches(ctx context.Context, mode string, limit int, cursor string) (query.MatchPage, error)
}

// BattleOperations of F105: listMyMatches is a player route (game_player); getMatch serves the participant (game) and
// the school staff (portal, API_DESIGN §9.7 "STU (participant), SA (school)") — the use case checks the channel and
// answers GAME_ACCESS_STUDENT_ONLY to staff with a game token (F105-AC-25).
var BattleOperations = map[string]httpx.Operation{
	"listMyMatches": op("listMyMatches", http.MethodGet, "/competition/matches", httpx.IdempotencyNone, http.StatusOK),
	"getMatch": {ID: "getMatch", Module: "competition", Method: http.MethodGet, Pattern: "/competition/matches/{match_id}",
		Tenant: httpx.TenantRequired, Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK, Guard: httpx.GuardNone},
}

// BattleOperationIDs are the F105 routes in registration order.
var BattleOperationIDs = []string{"listMyMatches", "getMatch"}

func init() {
	for id, o := range BattleOperations {
		Operations[id] = o
	}
}

func registerBattles(s *httpx.Server, q BattleQueries) error {
	routes := map[string]http.HandlerFunc{
		"getMatch": idView("match_id", func(ctx context.Context, id uuid.UUID) (any, error) { return q.GetMatch(ctx, id) }),
		"listMyMatches": func(w http.ResponseWriter, r *http.Request) {
			qs := r.URL.Query()
			limit := 20
			if v := qs.Get("limit"); v != "" {
				n, err := strconv.Atoi(v)
				if err != nil || n < 1 || n > 100 {
					fail(w, r, problem.Invalid("limit", problem.SubInvalid, "is between 1 and 100"))
					return
				}
				limit = n
			}
			page, err := q.ListMyMatches(r.Context(), qs.Get("mode"), limit, qs.Get("cursor"))
			if err != nil {
				fail(w, r, err)
				return
			}
			p := map[string]any{"has_more": page.Next != "", "limit": limit, "next_cursor": nil}
			if page.Next != "" {
				p["next_cursor"] = page.Next
			}
			writeJSON(w, http.StatusOK, map[string]any{"data": page.Data, "page": p})
		},
	}
	for _, id := range BattleOperationIDs {
		if err := s.Handle(BattleOperations[id], routes[id]); err != nil {
			return err
		}
	}
	return nil
}
