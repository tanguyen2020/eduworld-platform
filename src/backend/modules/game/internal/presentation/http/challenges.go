package http

import (
	"context"
	"net/http"
	"slices"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/challengerules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/challenge"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ChallengeCommands are the challenge write use cases of a student (command.Challenges, F076).
type ChallengeCommands interface {
	Start(ctx context.Context, challengeID uuid.UUID, difficulty string, partyID *uuid.UUID) (challengerules.AttemptView, error)
	Next(ctx context.Context, attemptID uuid.UUID) (command.NextView, error)
	Abandon(ctx context.Context, attemptID uuid.UUID) (challengerules.AttemptView, error)
}

// ChallengeQueries are the challenge read use cases of a student (query.Challenges, F076).
type ChallengeQueries interface {
	ListChallenges(ctx context.Context, in query.ChallengeListInput) (query.ChallengePage, error)
	GetChallenge(ctx context.Context, challengeID uuid.UUID) (query.ChallengeDetail, error)
	GetChallengeAttempt(ctx context.Context, attemptID uuid.UUID) (challengerules.AttemptView, error)
}

// ChallengeOperations are the player routes of F076 (F076-AC-03: game_player, staff tokens get 403
// GAME_ACCESS_STUDENT_ONLY; an X-Tenant-Id of another tenant gets 403 TENANT_MISMATCH).
var ChallengeOperations = []string{"listGameChallenges", "getGameChallenge", "startGameChallenge", "getGameChallengeAttempt",
	"nextChallengeActivity", "abandonGameChallengeAttempt"}

// challengeOperations are the F076 operations (method, path, X-Tenant-Id mode, Idempotency-Key, success status and guard
// of the spec; the contract test checks them against specs/openapi/eduworld-api.v1.yaml). No route takes a score, lives,
// a result or a completion from the client (F076-AC-16).
var challengeOperations = []httpx.Operation{
	op("listGameChallenges", http.MethodGet, "/game/challenges", httpx.TenantRequired, httpx.IdempotencyNone, http.StatusOK, httpx.GuardGamePlayer),
	op("getGameChallenge", http.MethodGet, "/game/challenges/{challenge_id}", httpx.TenantRequired, httpx.IdempotencyNone, http.StatusOK,
		httpx.GuardGamePlayer),
	op("startGameChallenge", http.MethodPost, "/game/challenges/{challenge_id}:start", httpx.TenantRequired, httpx.IdempotencyRequired,
		http.StatusCreated, httpx.GuardGamePlayer),
	op("getGameChallengeAttempt", http.MethodGet, "/game/challenge-attempts/{challenge_attempt_id}", httpx.TenantRequired,
		httpx.IdempotencyNone, http.StatusOK, httpx.GuardGamePlayer),
	op("nextChallengeActivity", http.MethodPost, "/game/challenge-attempts/{challenge_attempt_id}/activities:next", httpx.TenantRequired,
		httpx.IdempotencyRequired, http.StatusOK, httpx.GuardGamePlayer),
	op("abandonGameChallengeAttempt", http.MethodPost, "/game/challenge-attempts/{challenge_attempt_id}:abandon", httpx.TenantRequired,
		httpx.IdempotencyRequired, http.StatusOK, httpx.GuardGamePlayer),
}

func init() {
	for _, o := range challengeOperations {
		Operations[o.ID] = o
	}
	PlayerOperations = append(PlayerOperations, ChallengeOperations...)
}

func (h Handlers) challengeRoutes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"listGameChallenges":          h.listGameChallenges,
		"getGameChallenge":            h.getGameChallenge,
		"startGameChallenge":          h.startGameChallenge,
		"getGameChallengeAttempt":     h.getGameChallengeAttempt,
		"nextChallengeActivity":       h.nextChallengeActivity,
		"abandonGameChallengeAttempt": h.abandonGameChallengeAttempt,
	}
}

// listGameChallenges: query parameters outside their maxLength or enum are 422 VALIDATION_FAILED (F076-AC-21).
func (h Handlers) listGameChallenges(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, err := limitParam(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	in := query.ChallengeListInput{Type: q.Get("type"), AreaID: q.Get("area_id"), Difficulty: q.Get("difficulty"), Limit: limit,
		Cursor: q.Get("cursor")}
	switch {
	case len(in.AreaID) > 64:
		fail(w, r, problem.Invalid("area_id", problem.SubInvalid, "at most 64 characters"))
		return
	case len(in.Cursor) > 512:
		fail(w, r, problem.Invalid("cursor", problem.SubInvalid, "at most 512 characters"))
		return
	case in.Difficulty != "" && !slices.Contains(challenge.Levels, in.Difficulty):
		fail(w, r, problem.Invalid("difficulty", problem.SubInvalid, "easy, normal, hard, expert or legendary"))
		return
	}
	out, err := h.ChallengeQueries.ListChallenges(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, lang(r))
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) getGameChallenge(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "challenge_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.ChallengeQueries.GetChallenge(query.WithLanguage(r.Context(), lang(r)), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, lang(r))
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) startGameChallenge(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "challenge_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Difficulty string     `json:"difficulty"`
		PartyID    *uuid.UUID `json:"party_id"`
	}
	if err := optionalBody(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	if !slices.Contains(challenge.Levels, b.Difficulty) {
		fail(w, r, problem.Invalid("difficulty", problem.SubInvalid, "easy, normal, hard, expert or legendary"))
		return
	}
	out, err := h.Challenges.Start(r.Context(), id, b.Difficulty, b.PartyID)
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, lang(r))
	writeJSON(w, http.StatusCreated, out)
}

func (h Handlers) getGameChallengeAttempt(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "challenge_attempt_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.ChallengeQueries.GetChallengeAttempt(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, lang(r))
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) nextChallengeActivity(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "challenge_attempt_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Challenges.Next(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, lang(r))
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) abandonGameChallengeAttempt(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "challenge_attempt_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Challenges.Abandon(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, lang(r))
	writeJSON(w, http.StatusOK, out)
}
