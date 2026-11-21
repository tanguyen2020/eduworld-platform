package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// WorldCommands are the world write use cases of a student (command.World, F071).
type WorldCommands interface {
	EnterArea(ctx context.Context, in command.EnterInput) (command.EnterResult, error)
	StartLevel(ctx context.Context, levelID, gameSessionID uuid.UUID) (command.LevelSessionView, error)
	EndLevel(ctx context.Context, id uuid.UUID, outcome string) (command.LevelSessionView, error)
	ReportDiscovery(ctx context.Context, areaID, interactableID string) (command.DiscoveryResult, error)
}

// WorldQueries are the world read use cases of a student (query.World, F071).
type WorldQueries interface {
	ListAreas(ctx context.Context) ([]query.AreaView, error)
	Manifest(ctx context.Context) (query.ManifestView, error)
	Exploration(ctx context.Context) (query.Exploration, error)
	NpcDialogue(ctx context.Context, npcID string) (query.Dialogue, error)
}

// WorldOperations are the player routes of F071 (F071-AC-06: staff tokens get 403 GAME_ACCESS_STUDENT_ONLY).
var WorldOperations = []string{"getWorldManifest", "listWorldAreas", "enterWorldArea", "startLevel", "endLevelSession", "getNpcDialogue",
	"getWorldExploration", "reportDiscovery"}

func (h Handlers) worldRoutes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"getWorldManifest":    h.getWorldManifest,
		"listWorldAreas":      h.listWorldAreas,
		"enterWorldArea":      h.enterWorldArea,
		"startLevel":          h.startLevel,
		"endLevelSession":     h.endLevelSession,
		"getNpcDialogue":      h.getNpcDialogue,
		"getWorldExploration": h.getWorldExploration,
		"reportDiscovery":     h.reportDiscovery,
	}
}

func (h Handlers) getWorldManifest(w http.ResponseWriter, r *http.Request) {
	m, err := h.WorldQueries.Manifest(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (h Handlers) listWorldAreas(w http.ResponseWriter, r *http.Request) {
	areas, err := h.WorldQueries.ListAreas(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": areas})
}

func (h Handlers) enterWorldArea(w http.ResponseWriter, r *http.Request) {
	var b struct {
		GameSessionID uuid.UUID `json:"game_session_id"`
		PortalID      string    `json:"portal_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.GameSessionID == uuid.Nil {
		fail(w, r, problem.Invalid("game_session_id", problem.SubRequired, "a UUID is required"))
		return
	}
	out, err := h.World.EnterArea(r.Context(), command.EnterInput{AreaID: chi.URLParam(r, "area_id"), GameSessionID: b.GameSessionID,
		PortalID: b.PortalID})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) startLevel(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "level_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		GameSessionID uuid.UUID `json:"game_session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.GameSessionID == uuid.Nil {
		fail(w, r, problem.Invalid("game_session_id", problem.SubRequired, "a UUID is required"))
		return
	}
	out, err := h.World.StartLevel(r.Context(), id, b.GameSessionID)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h Handlers) endLevelSession(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "level_session_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Outcome string `json:"outcome"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, r, problem.New(problem.BadRequest, "invalid JSON body"))
		return
	}
	out, err := h.World.EndLevel(r.Context(), id, b.Outcome)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) getNpcDialogue(w http.ResponseWriter, r *http.Request) {
	d, err := h.WorldQueries.NpcDialogue(r.Context(), chi.URLParam(r, "npc_id"))
	if err != nil {
		fail(w, r, err)
		return
	}
	w.Header().Set("ETag", `"`+d.ETag+`"`)
	w.Header().Set("Cache-Control", "private, no-cache")
	if inm := r.Header.Get("If-None-Match"); inm != "" && etagMatches(inm, d.ETag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	writeJSON(w, http.StatusOK, d.Payload)
}

func (h Handlers) getWorldExploration(w http.ResponseWriter, r *http.Request) {
	out, err := h.WorldQueries.Exploration(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) reportDiscovery(w http.ResponseWriter, r *http.Request) {
	out, err := h.World.ReportDiscovery(r.Context(), chi.URLParam(r, "area_id"), chi.URLParam(r, "interactable_id"))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
