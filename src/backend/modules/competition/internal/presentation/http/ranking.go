package http

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// RankingQueries are the F106 read use cases (query.Rankings). No route writes a rating, a point or a board: they come
// from server events only (R5, ADR-0016).
type RankingQueries interface {
	GetMyRatings(ctx context.Context, season *uuid.UUID, locale string) ([]map[string]any, error)
	ListSeasons(ctx context.Context, limit int, cursor string) (query.SeasonPage, error)
	GetSeason(ctx context.Context, id uuid.UUID) (map[string]any, error)
	ListLeaderboards(ctx context.Context) ([]map[string]any, error)
	GetLeaderboard(ctx context.Context, id uuid.UUID, p query.BoardParams) (query.PageBody, error)
	GetMyLeaderboardPosition(ctx context.Context, id uuid.UUID, p query.BoardParams) (query.PageBody, error)
	GetLeaderboardHistory(ctx context.Context, id uuid.UUID, p query.BoardParams) (query.HistoryPage, error)
}

func anyOp(id, pattern string) httpx.Operation {
	return httpx.Operation{ID: id, Module: "competition", Method: http.MethodGet, Pattern: pattern, Tenant: httpx.TenantRequired,
		Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK, Guard: httpx.GuardNone}
}

// RankingOperations of F106: getMyRatings and getMyLeaderboardPosition are player routes (game_player); seasons and
// leaderboards also serve school staff of the portal (lms.student.read) — the use cases check the channel and answer
// GAME_ACCESS_STUDENT_ONLY to staff with a game token.
var RankingOperations = map[string]httpx.Operation{
	"getMyRatings":             op("getMyRatings", http.MethodGet, "/competition/ratings/me", httpx.IdempotencyNone, http.StatusOK),
	"listSeasons":              anyOp("listSeasons", "/competition/seasons"),
	"getSeason":                anyOp("getSeason", "/competition/seasons/{season_id}"),
	"listLeaderboards":         anyOp("listLeaderboards", "/leaderboards"),
	"getLeaderboard":           anyOp("getLeaderboard", "/leaderboards/{leaderboard_id}"),
	"getMyLeaderboardPosition": op("getMyLeaderboardPosition", http.MethodGet, "/leaderboards/{leaderboard_id}/me", httpx.IdempotencyNone, http.StatusOK),
	"getLeaderboardHistory":    anyOp("getLeaderboardHistory", "/leaderboards/{leaderboard_id}/history"),
}

// RankingOperationIDs are the F106 routes in registration order.
var RankingOperationIDs = []string{"getMyRatings", "listSeasons", "getSeason", "listLeaderboards", "getLeaderboard",
	"getMyLeaderboardPosition", "getLeaderboardHistory"}

// StaffOperations are the routes that also serve portal staff (no game_player guard).
var StaffOperations = []string{"getMatch", "listSeasons", "getSeason", "listLeaderboards", "getLeaderboard", "getLeaderboardHistory"}

func init() {
	for id, o := range RankingOperations {
		Operations[id] = o
	}
}

// locale is the first language of Accept-Language (vi-VN by default).
func locale(r *http.Request) string {
	l := strings.TrimSpace(strings.Split(strings.Split(r.Header.Get("Accept-Language"), ",")[0], ";")[0])
	if l == "" || len(l) > 35 {
		return "vi-VN"
	}
	return l
}

func queryUUID(r *http.Request, name string) (*uuid.UUID, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return nil, nil
	}
	id, err := uuid.Parse(v)
	if err != nil {
		return nil, problem.Invalid(name, problem.SubInvalid, "must be a UUID")
	}
	return &id, nil
}

func queryLimit(r *http.Request) (int, error) {
	v := r.URL.Query().Get("limit")
	if v == "" {
		return 20, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > 100 {
		return 0, problem.Invalid("limit", problem.SubInvalid, "is between 1 and 100")
	}
	return n, nil
}

func boardParams(r *http.Request) (query.BoardParams, error) {
	var p query.BoardParams
	var err error
	if p.ScopeID, err = queryUUID(r, "scope_id"); err != nil {
		return p, err
	}
	if v := r.URL.Query().Get("grade_level"); v != "" {
		g, err := strconv.Atoi(v)
		if err != nil || g < 1 || g > 9 {
			return p, problem.Invalid("grade_level", problem.SubInvalid, "is between 1 and 9")
		}
		p.Grade = &g
	}
	p.Period = r.URL.Query().Get("period_key")
	if len(p.Period) > 64 {
		return p, problem.Invalid("period_key", problem.SubInvalid, "is at most 64 characters")
	}
	if p.Limit, err = queryLimit(r); err != nil {
		return p, err
	}
	p.Cursor = r.URL.Query().Get("cursor")
	return p, nil
}

func pageInfo(limit int, next string) map[string]any {
	p := map[string]any{"has_more": next != "", "limit": limit, "next_cursor": nil}
	if next != "" {
		p["next_cursor"] = next
	}
	return p
}

func registerRanking(s *httpx.Server, q RankingQueries) error {
	board := func(fn func(ctx context.Context, id uuid.UUID, p query.BoardParams) (query.PageBody, error)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			id, err := pathUUID(r, "leaderboard_id")
			if err != nil {
				fail(w, r, err)
				return
			}
			p, err := boardParams(r)
			if err != nil {
				fail(w, r, err)
				return
			}
			out, err := fn(r.Context(), id, p)
			if err != nil {
				fail(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		}
	}
	routes := map[string]http.HandlerFunc{
		"getMyRatings": func(w http.ResponseWriter, r *http.Request) {
			season, err := queryUUID(r, "season_id")
			if err != nil {
				fail(w, r, err)
				return
			}
			data, err := q.GetMyRatings(r.Context(), season, locale(r))
			if err != nil {
				fail(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"data": data})
		},
		"listSeasons": func(w http.ResponseWriter, r *http.Request) {
			limit, err := queryLimit(r)
			if err != nil {
				fail(w, r, err)
				return
			}
			page, err := q.ListSeasons(r.Context(), limit, r.URL.Query().Get("cursor"))
			if err != nil {
				fail(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"data": page.Data, "page": pageInfo(limit, page.Next)})
		},
		"getSeason": idView("season_id", func(ctx context.Context, id uuid.UUID) (any, error) { return q.GetSeason(ctx, id) }),
		"listLeaderboards": func(w http.ResponseWriter, r *http.Request) {
			data, err := q.ListLeaderboards(r.Context())
			if err != nil {
				fail(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"data": data})
		},
		"getLeaderboard":           board(q.GetLeaderboard),
		"getMyLeaderboardPosition": board(q.GetMyLeaderboardPosition),
		"getLeaderboardHistory": func(w http.ResponseWriter, r *http.Request) {
			id, err := pathUUID(r, "leaderboard_id")
			if err != nil {
				fail(w, r, err)
				return
			}
			p, err := boardParams(r)
			if err != nil {
				fail(w, r, err)
				return
			}
			page, err := q.GetLeaderboardHistory(r.Context(), id, p)
			if err != nil {
				fail(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"data": page.Data, "page": pageInfo(p.Limit, page.Next)})
		},
	}
	for _, id := range RankingOperationIDs {
		if err := s.Handle(RankingOperations[id], routes[id]); err != nil {
			return err
		}
	}
	return nil
}
