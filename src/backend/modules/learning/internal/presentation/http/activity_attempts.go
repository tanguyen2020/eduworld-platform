package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// AttemptCommands are the write use cases of the attempt engine (command.Attempts, F030).
type AttemptCommands interface {
	StartActivity(ctx context.Context, in command.StartActivityInput) (dto.AttemptView, error)
	StartAdHoc(ctx context.Context, in command.StartAdHocInput) (dto.AttemptView, error)
	Respond(ctx context.Context, in command.RespondInput) (dto.ResponseResultView, error)
	Pause(ctx context.Context, id uuid.UUID) (dto.AttemptView, error)
	Submit(ctx context.Context, id uuid.UUID) (dto.AttemptView, error)
}

// AttemptQueries are the reads of the attempt engine (query.Attempts, F030).
type AttemptQueries interface {
	Get(ctx context.Context, id uuid.UUID) (dto.AttemptView, string, error)
	ListMine(ctx context.Context, in query.ListMineInput) (query.Page[dto.AttemptSummaryView], error)
	Review(ctx context.Context, id uuid.UUID) (dto.ReviewView, string, error)
	Activity(ctx context.Context, id uuid.UUID, acceptLanguage string) (dto.ActivityRenderView, error)
}

// player is an operation of the game client: the game_player guard (azp eduworld-game-backend + user_type student,
// R23, ADR-0031) and the session tenant.
func player(id, method, pattern string, idem httpx.IdempotencyMode, status int) httpx.Operation {
	o := op(id, method, pattern, httpx.TenantRequired, idem, status)
	o.Guard = httpx.GuardGamePlayer
	return o
}

const attemptsPath = "/learning/activity-attempts"

func init() {
	one := attemptsPath + "/{attempt_id}"
	for id, o := range map[string]httpx.Operation{
		"getLearningActivity":      player("getLearningActivity", http.MethodGet, "/learning/activities/{activity_id}", httpx.IdempotencyNone, 200),
		"startActivityAttempt":     player("startActivityAttempt", http.MethodPost, "/learning/activities/{activity_id}/attempts", httpx.IdempotencyRequired, 201),
		"startAdHocAttempt":        player("startAdHocAttempt", http.MethodPost, attemptsPath, httpx.IdempotencyRequired, 201),
		"listMyActivityAttempts":   player("listMyActivityAttempts", http.MethodGet, attemptsPath, httpx.IdempotencyNone, 200),
		"getActivityAttempt":       player("getActivityAttempt", http.MethodGet, one, httpx.IdempotencyNone, 200),
		"submitActivityResponse":   player("submitActivityResponse", http.MethodPost, one+"/responses", httpx.IdempotencyRequired, 200),
		"pauseActivityAttempt":     player("pauseActivityAttempt", http.MethodPost, one+":pause", httpx.IdempotencyRequired, 200),
		"submitActivityAttempt":    player("submitActivityAttempt", http.MethodPost, one+":submit", httpx.IdempotencyRequired, 200),
		"getActivityAttemptReview": player("getActivityAttemptReview", http.MethodGet, one+"/review", httpx.IdempotencyNone, 200),
	} {
		Operations[id] = o
	}
}

var attemptRoutes = map[string]func(Handlers, http.ResponseWriter, *http.Request){
	"getLearningActivity":      Handlers.getLearningActivity,
	"startActivityAttempt":     Handlers.startActivityAttempt,
	"startAdHocAttempt":        Handlers.startAdHocAttempt,
	"listMyActivityAttempts":   Handlers.listMyActivityAttempts,
	"getActivityAttempt":       Handlers.getActivityAttempt,
	"submitActivityResponse":   Handlers.submitActivityResponse,
	"pauseActivityAttempt":     Handlers.pauseActivityAttempt,
	"submitActivityAttempt":    Handlers.submitActivityAttempt,
	"getActivityAttemptReview": Handlers.getActivityAttemptReview,
}

func displayLang(w http.ResponseWriter, lang string) { contentLanguage(w, lang) }

// optionalBody decodes a body that may be empty.
func optionalBody(r *http.Request, v any) error {
	if r.ContentLength == 0 {
		return nil
	}
	if err := json.NewDecoder(r.Body).Decode(v); err != nil && err.Error() != "EOF" {
		return problem.New(problem.BadRequest, "invalid JSON body")
	}
	return nil
}

func (h Handlers) getLearningActivity(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "activity_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.AttemptQueries.Activity(r.Context(), id, r.Header.Get("Accept-Language"))
	if err != nil {
		fail(w, r, err)
		return
	}
	displayLang(w, v.ContentLanguage)
	writeJSON(w, http.StatusOK, v)
}

func (h Handlers) startActivityAttempt(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "activity_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Mode             string     `json:"mode"`
		ContextType      string     `json:"context_type"`
		ContextID        *uuid.UUID `json:"context_id"`
		RecommendationID *uuid.UUID `json:"recommendation_id"`
	}
	if err := optionalBody(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Attempts.StartActivity(r.Context(), command.StartActivityInput{ActivityID: id, Mode: b.Mode, ContextType: b.ContextType,
		ContextID: b.ContextID, RecommendationID: b.RecommendationID, AcceptLanguage: r.Header.Get("Accept-Language")})
	if err != nil {
		fail(w, r, err)
		return
	}
	displayLang(w, langOf(v.DisplayLanguage))
	writeJSON(w, http.StatusCreated, v)
}

func langOf(locale string) string {
	if locale == "en-US" {
		return "en"
	}
	return "vi"
}

func (h Handlers) startAdHocAttempt(w http.ResponseWriter, r *http.Request) {
	var b command.StartAdHocInput
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	b.AcceptLanguage = r.Header.Get("Accept-Language")
	v, err := h.Attempts.StartAdHoc(r.Context(), b)
	if err != nil {
		fail(w, r, err)
		return
	}
	displayLang(w, langOf(v.DisplayLanguage))
	writeJSON(w, http.StatusCreated, v)
}

func (h Handlers) listMyActivityAttempts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	p, err := h.AttemptQueries.ListMine(r.Context(), query.ListMineInput{Mode: q.Get("mode"), Status: q.Get("status"),
		ActivityType: q.Get("activity_type"), Page: pageIn(r)})
	if err != nil {
		fail(w, r, err)
		return
	}
	displayLang(w, "vi")
	list(w, p, func(v dto.AttemptSummaryView) dto.AttemptSummaryView { return v })
}

func (h Handlers) getActivityAttempt(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "attempt_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, lang, err := h.AttemptQueries.Get(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	displayLang(w, lang)
	writeJSON(w, http.StatusOK, v)
}

func (h Handlers) submitActivityResponse(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "attempt_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	// Only the raw response, the item and the client elapsed time are read: score, is_correct, max_score or
	// duration_ms sent by the client are ignored (ADR-0016, F030-AC-09).
	var b struct {
		Kind            string          `json:"kind"`
		Position        int             `json:"position"`
		Response        json.RawMessage `json:"response"`
		ClientElapsedMS *int64          `json:"client_elapsed_ms"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	if b.Kind != "" && b.Kind != "question" {
		fail(w, r, problem.Invalid("/kind", problem.SubInvalid, "only question responses exist for quiz and interactive_exercise"))
		return
	}
	v, err := h.Attempts.Respond(r.Context(), command.RespondInput{AttemptID: id, Position: b.Position, Response: b.Response,
		ClientElapsedMS: b.ClientElapsedMS})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h Handlers) pauseActivityAttempt(w http.ResponseWriter, r *http.Request) {
	h.attemptAction(w, r, h.Attempts.Pause)
}

func (h Handlers) submitActivityAttempt(w http.ResponseWriter, r *http.Request) {
	h.attemptAction(w, r, h.Attempts.Submit)
}

func (h Handlers) attemptAction(w http.ResponseWriter, r *http.Request, fn func(context.Context, uuid.UUID) (dto.AttemptView, error)) {
	id, err := pathUUID(r, "attempt_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := fn(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	displayLang(w, langOf(v.DisplayLanguage))
	writeJSON(w, http.StatusOK, v)
}

func (h Handlers) getActivityAttemptReview(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "attempt_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, lang, err := h.AttemptQueries.Review(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	displayLang(w, lang)
	writeJSON(w, http.StatusOK, v)
}
