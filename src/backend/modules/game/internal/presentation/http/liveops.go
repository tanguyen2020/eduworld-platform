package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/liveops"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// LiveOpsQueries are the live-ops read use cases (query.LiveOps, F077).
type LiveOpsQueries interface {
	ListGameEvents(ctx context.Context, statuses []string, limit int, cursor string) (query.Page[query.EventSummaryView], error)
	GetGameEvent(ctx context.Context, event uuid.UUID) (query.EventDetailView, error)
	GetCommunityGoal(ctx context.Context, goal uuid.UUID) (query.GoalView, error)
	GetSeasonTrack(ctx context.Context) (query.SeasonTrackView, error)
	ListSeasonTrackHistory(ctx context.Context, limit int, cursor string) (query.Page[query.SeasonHistoryView], error)
	GetEventRunStats(ctx context.Context, event, run uuid.UUID) (map[string]any, error)
	ListEventTemplates(ctx context.Context, limit int, cursor string) (query.Page[query.TemplateView], error)
	ListSchoolEvents(ctx context.Context, status string, limit int, cursor string) (query.Page[map[string]any], error)
	GetSchoolEvent(ctx context.Context, event uuid.UUID) (map[string]any, error)
	GetSchoolEventProgress(ctx context.Context, event uuid.UUID, school *uuid.UUID) (query.SchoolEventProgressView, error)
}

// LiveOpsCommands are the live-ops write use cases (command.LiveOps, F077).
type LiveOpsCommands interface {
	JoinEvent(ctx context.Context, event uuid.UUID) (command.JoinResult, error)
	ClaimMilestone(ctx context.Context, event uuid.UUID, milestone string) (command.ClaimResult, error)
	ClaimTier(ctx context.Context, tier int) (command.ClaimResult, error)
	OperateRun(ctx context.Context, op command.RunOperation) (map[string]any, error)
	CreateSchoolEvent(ctx context.Context, in command.SchoolEventInput) (port.SchoolEventRow, error)
	UpdateSchoolEvent(ctx context.Context, event uuid.UUID, ifMatch int64, in command.SchoolEventInput) (port.SchoolEventRow, error)
	CancelSchoolEvent(ctx context.Context, event uuid.UUID, reason string) (port.SchoolEventRow, error)
}

// LiveOpsPlayerOperations are the player routes of F077 (F077-AC-12: game_player, staff tokens get 403
// GAME_ACCESS_STUDENT_ONLY).
var LiveOpsPlayerOperations = []string{"listGameEvents", "getGameEvent", "joinGameEvent", "claimEventMilestone", "getCommunityGoal",
	"getSeasonTrack", "claimSeasonTier", "listSeasonTrackHistory"}

// LiveOpsAdminOperations are the operator routes (internal gateway, portal client, platform role; game.event.operate or
// game.liveops.read — F077-AC-06).
var LiveOpsAdminOperations = []string{"suspendEventRun", "resumeEventRun", "cancelEventRun", "getEventRunStats"}

// LiveOpsSchoolOperations are the school event routes (/lms/*, portal staff, lms.school_event.write — F077-AC-25).
var LiveOpsSchoolOperations = []string{"listEventTemplates", "listSchoolEvents", "createSchoolEvent", "getSchoolEvent", "updateSchoolEvent",
	"cancelSchoolEvent", "getSchoolEventProgress"}

// liveOpsOperations are the F077 operations (method, path, X-Tenant-Id mode, Idempotency-Key, success status and guard
// of the spec; the contract test checks them against specs/openapi/eduworld-api.v1.yaml). /game-admin/ and /lms/ routes
// also get portal_staff (+ platform role for /game-admin/) from the route group.
var liveOpsOperations = []httpx.Operation{
	{ID: "listGameEvents", Module: "game", Method: http.MethodGet, Pattern: "/game/events", Tenant: httpx.TenantRequired,
		Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK, Guard: httpx.GuardGamePlayer},
	{ID: "getGameEvent", Module: "game", Method: http.MethodGet, Pattern: "/game/events/{event_id}", Tenant: httpx.TenantRequired,
		Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK, Guard: httpx.GuardGamePlayer},
	{ID: "joinGameEvent", Module: "game", Method: http.MethodPost, Pattern: "/game/events/{event_id}:join", Tenant: httpx.TenantRequired,
		Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusOK, Guard: httpx.GuardGamePlayer},
	{ID: "claimEventMilestone", Module: "game", Method: http.MethodPost, Pattern: "/game/events/{event_id}/milestones/{milestone_id}:claim", Tenant: httpx.TenantRequired,
		Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusOK, Guard: httpx.GuardGamePlayer},
	{ID: "getCommunityGoal", Module: "game", Method: http.MethodGet, Pattern: "/game/community-goals/{goal_id}", Tenant: httpx.TenantRequired,
		Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK, Guard: httpx.GuardGamePlayer},
	{ID: "getSeasonTrack", Module: "game", Method: http.MethodGet, Pattern: "/game/season-track", Tenant: httpx.TenantRequired,
		Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK, Guard: httpx.GuardGamePlayer},
	{ID: "claimSeasonTier", Module: "game", Method: http.MethodPost, Pattern: "/game/season-track/tiers/{tier}:claim", Tenant: httpx.TenantRequired,
		Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusOK, Guard: httpx.GuardGamePlayer},
	{ID: "listSeasonTrackHistory", Module: "game", Method: http.MethodGet, Pattern: "/game/season-track/history", Tenant: httpx.TenantRequired,
		Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK, Guard: httpx.GuardGamePlayer},
	{ID: "suspendEventRun", Module: "game", Method: http.MethodPost, Pattern: "/game-admin/events/{event_id}/runs/{run_id}:suspend",
		Tenant: httpx.TenantNone, Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusOK, Guard: httpx.GuardPortalStaff | httpx.GuardPlatformRole},
	{ID: "resumeEventRun", Module: "game", Method: http.MethodPost, Pattern: "/game-admin/events/{event_id}/runs/{run_id}:resume",
		Tenant: httpx.TenantNone, Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusOK, Guard: httpx.GuardPortalStaff | httpx.GuardPlatformRole},
	{ID: "cancelEventRun", Module: "game", Method: http.MethodPost, Pattern: "/game-admin/events/{event_id}/runs/{run_id}:cancel",
		Tenant: httpx.TenantNone, Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusOK, Guard: httpx.GuardPortalStaff | httpx.GuardPlatformRole},
	{ID: "getEventRunStats", Module: "game", Method: http.MethodGet, Pattern: "/game-admin/events/{event_id}/runs/{run_id}/stats",
		Tenant: httpx.TenantNone, Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK, Guard: httpx.GuardPortalStaff | httpx.GuardPlatformRole},
	{ID: "listEventTemplates", Module: "game", Method: http.MethodGet, Pattern: "/lms/event-templates", Tenant: httpx.TenantRequired,
		Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK, Guard: httpx.GuardPortalStaff},
	{ID: "listSchoolEvents", Module: "game", Method: http.MethodGet, Pattern: "/lms/school-events", Tenant: httpx.TenantOptional,
		Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK, Guard: httpx.GuardPortalStaff},
	{ID: "createSchoolEvent", Module: "game", Method: http.MethodPost, Pattern: "/lms/school-events", Tenant: httpx.TenantRequired,
		Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusCreated, Guard: httpx.GuardPortalStaff},
	{ID: "getSchoolEvent", Module: "game", Method: http.MethodGet, Pattern: "/lms/school-events/{event_id}", Tenant: httpx.TenantRequired,
		Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK, Guard: httpx.GuardPortalStaff},
	{ID: "updateSchoolEvent", Module: "game", Method: http.MethodPatch, Pattern: "/lms/school-events/{event_id}", Tenant: httpx.TenantRequired,
		Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusOK, Guard: httpx.GuardPortalStaff},
	{ID: "cancelSchoolEvent", Module: "game", Method: http.MethodPost, Pattern: "/lms/school-events/{event_id}:cancel", Tenant: httpx.TenantRequired,
		Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusOK, Guard: httpx.GuardPortalStaff},
	{ID: "getSchoolEventProgress", Module: "game", Method: http.MethodGet, Pattern: "/lms/school-events/{event_id}/progress", Tenant: httpx.TenantRequired,
		Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK, Guard: httpx.GuardPortalStaff},
}

func init() {
	for _, o := range liveOpsOperations {
		Operations[o.ID] = o
	}
	PlayerOperations = append(PlayerOperations, LiveOpsPlayerOperations...)
}

func (h Handlers) liveOpsRoutes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"listGameEvents":         h.listGameEvents,
		"getGameEvent":           h.getGameEvent,
		"joinGameEvent":          h.joinGameEvent,
		"claimEventMilestone":    h.claimEventMilestone,
		"getCommunityGoal":       h.getCommunityGoal,
		"getSeasonTrack":         h.getSeasonTrack,
		"claimSeasonTier":        h.claimSeasonTier,
		"listSeasonTrackHistory": h.listSeasonTrackHistory,
		"suspendEventRun":        h.operateRun(liveops.ActionSuspend),
		"resumeEventRun":         h.operateRun(liveops.ActionResume),
		"cancelEventRun":         h.operateRun(liveops.ActionCancel),
		"getEventRunStats":       h.getEventRunStats,
		"listEventTemplates":     h.listEventTemplates,
		"listSchoolEvents":       h.listSchoolEvents,
		"createSchoolEvent":      h.createSchoolEvent,
		"getSchoolEvent":         h.getSchoolEvent,
		"updateSchoolEvent":      h.updateSchoolEvent,
		"cancelSchoolEvent":      h.cancelSchoolEvent,
		"getSchoolEventProgress": h.getSchoolEventProgress,
	}
}

func (h Handlers) listGameEvents(w http.ResponseWriter, r *http.Request) {
	limit, err := limitParam(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var statuses []string
	for _, v := range r.URL.Query()["status"] {
		for _, s := range strings.Split(v, ",") {
			if s = strings.TrimSpace(s); s != "" {
				statuses = append(statuses, s)
			}
		}
	}
	out, err := h.LiveOpsQueries.ListGameEvents(r.Context(), statuses, limit, r.URL.Query().Get("cursor"))
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, lang(r))
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) getGameEvent(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "event_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.LiveOpsQueries.GetGameEvent(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, lang(r))
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) joinGameEvent(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "event_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.LiveOpsCommands.JoinEvent(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) claimEventMilestone(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "event_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	m := chi.URLParam(r, "milestone_id")
	if m == "" || len(m) > 64 {
		fail(w, r, problem.Invalid("milestone_id", problem.SubInvalid, "1 to 64 characters"))
		return
	}
	out, err := h.LiveOpsCommands.ClaimMilestone(r.Context(), id, m)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out.View())
}

func (h Handlers) getCommunityGoal(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "goal_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.LiveOpsQueries.GetCommunityGoal(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) getSeasonTrack(w http.ResponseWriter, r *http.Request) {
	out, err := h.LiveOpsQueries.GetSeasonTrack(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, lang(r))
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) claimSeasonTier(w http.ResponseWriter, r *http.Request) {
	tier, err := strconv.Atoi(chi.URLParam(r, "tier"))
	if err != nil || tier < 1 {
		fail(w, r, problem.Invalid("tier", problem.SubInvalid, "a positive integer"))
		return
	}
	out, err := h.LiveOpsCommands.ClaimTier(r.Context(), tier)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out.View())
}

func (h Handlers) listSeasonTrackHistory(w http.ResponseWriter, r *http.Request) {
	limit, err := limitParam(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.LiveOpsQueries.ListSeasonTrackHistory(r.Context(), limit, r.URL.Query().Get("cursor"))
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, lang(r))
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) operateRun(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		event, err := pathUUID(r, "event_id")
		if err != nil {
			fail(w, r, err)
			return
		}
		run, err := pathUUID(r, "run_id")
		if err != nil {
			fail(w, r, err)
			return
		}
		var b struct {
			Reason string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			fail(w, r, problem.New(problem.BadRequest, "invalid JSON body"))
			return
		}
		out, err := h.LiveOpsCommands.OperateRun(r.Context(), command.RunOperation{EventID: event, RunID: run, Action: action, Reason: b.Reason})
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func (h Handlers) getEventRunStats(w http.ResponseWriter, r *http.Request) {
	event, err := pathUUID(r, "event_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	run, err := pathUUID(r, "run_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.LiveOpsQueries.GetEventRunStats(r.Context(), event, run)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) listEventTemplates(w http.ResponseWriter, r *http.Request) {
	limit, err := limitParam(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.LiveOpsQueries.ListEventTemplates(r.Context(), limit, r.URL.Query().Get("cursor"))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) listSchoolEvents(w http.ResponseWriter, r *http.Request) {
	limit, err := limitParam(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	q := r.URL.Query()
	status := q.Get("status")
	if status != "" && !slices.Contains(schoolEventStatuses, status) {
		fail(w, r, problem.Invalid("status", problem.SubInvalid, "not a SchoolEventStatus"))
		return
	}
	out, err := h.LiveOpsQueries.ListSchoolEvents(r.Context(), status, limit, q.Get("cursor"))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// schoolEventStatuses is the enum SchoolEventStatus of the contract (filter of listSchoolEvents).
var schoolEventStatuses = []string{"draft", "scheduled", "live", "suspended", "ended", "cancelled", "archived"}

// getSchoolEventProgress serves getSchoolEventProgress (F081-AC-10): optional school_id (UUID) of the school viewed.
func (h Handlers) getSchoolEventProgress(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "event_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var school *uuid.UUID
	if raw := r.URL.Query().Get("school_id"); raw != "" {
		s, err := uuid.Parse(raw)
		if err != nil {
			fail(w, r, problem.Invalid("school_id", problem.SubInvalid, "school_id must be a UUID"))
			return
		}
		school = &s
	}
	out, err := h.LiveOpsQueries.GetSchoolEventProgress(r.Context(), id, school)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func writeSchoolEvent(w http.ResponseWriter, status int, row port.SchoolEventRow) {
	w.Header().Set("ETag", `"`+port.ETag(row.Version)+`"`)
	writeJSON(w, status, port.SchoolEventView(row))
}

func readSchoolEvent(r *http.Request) (command.SchoolEventInput, error) {
	b, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return command.SchoolEventInput{}, problem.New(problem.BadRequest, "invalid JSON body")
	}
	return command.ParseSchoolEventInput(b)
}

func (h Handlers) createSchoolEvent(w http.ResponseWriter, r *http.Request) {
	in, err := readSchoolEvent(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	row, err := h.LiveOpsCommands.CreateSchoolEvent(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeSchoolEvent(w, http.StatusCreated, row)
}

func (h Handlers) getSchoolEvent(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "event_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.LiveOpsQueries.GetSchoolEvent(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	if v, ok := out["version"].(int64); ok {
		w.Header().Set("ETag", `"`+port.ETag(v)+`"`)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) updateSchoolEvent(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "event_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	tag := strings.Trim(strings.TrimPrefix(strings.TrimSpace(r.Header.Get("If-Match")), "W/"), `"`)
	version, err := strconv.ParseInt(tag, 10, 64)
	if err != nil {
		fail(w, r, problem.New(problem.VersionMismatch, "If-Match must be the ETag of the school event"))
		return
	}
	in, err := readSchoolEvent(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	row, err := h.LiveOpsCommands.UpdateSchoolEvent(r.Context(), id, version, in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeSchoolEvent(w, http.StatusOK, row)
}

func (h Handlers) cancelSchoolEvent(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "event_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, r, problem.New(problem.BadRequest, "invalid JSON body"))
		return
	}
	row, err := h.LiveOpsCommands.CancelSchoolEvent(r.Context(), id, b.Reason)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeSchoolEvent(w, http.StatusOK, row)
}
