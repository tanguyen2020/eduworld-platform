package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/gameadmin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/gameconfig"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// GameConfigCommands are the write use cases of the competition configurations (command.GameConfigs).
type GameConfigCommands interface {
	Create(ctx context.Context, configType string, in command.ConfigDraft) (gameadmin.ConfigVersionView, error)
	UpdateDraft(ctx context.Context, configType string, configID uuid.UUID, no int, match string, in command.ConfigDraft) (gameadmin.ConfigVersionView, error)
	Transition(ctx context.Context, configType string, configID uuid.UUID, no int, match, action, note string) (gameadmin.ConfigVersionView, error)
	Publish(ctx context.Context, configType string, configID uuid.UUID, no int, match string, in command.PublishInput) (gameadmin.ReleaseView, error)
	Rollback(ctx context.Context, configType string, configID uuid.UUID, toVersion int, reason string) (gameadmin.ReleaseView, error)
	Unpublish(ctx context.Context, configType string, configID uuid.UUID, reason string) (gameadmin.ReleaseView, error)
}

// GameConfigQueries are the read use cases (query.GameConfigs).
type GameConfigQueries interface {
	List(ctx context.Context, configType, status, cursor string, limit int) (query.ConfigPage, error)
	Get(ctx context.Context, configType string, configID uuid.UUID, no int) (gameadmin.ConfigVersionView, error)
	Versions(ctx context.Context, configType string, configID uuid.UUID, cursor string, limit int) (query.VersionHistory, error)
	Diff(ctx context.Context, configType string, configID uuid.UUID, no, with int) (query.VersionDiff, error)
	Releases(ctx context.Context, configType string, configID uuid.UUID, cursor string, limit int) (query.ReleasePage, error)
}

// GameAdminHandlers serve the shared /game-admin/{config_type} operations for the competition config types (F108 US1):
// internal gateway only, portal staff with a platform role (route guards, ADR-0027, ADR-0094 — no MFA); the permission
// of the type is checked in each use case (R7, R20). X-Tenant-Id is ignored (platform configuration).
type GameAdminHandlers struct {
	Commands GameConfigCommands
	Queries  GameConfigQueries
}

func adminOp(id, method, pattern string, idem httpx.IdempotencyMode, status int) httpx.Operation {
	return httpx.Operation{ID: id, Module: "competition", Method: method, Pattern: pattern, Tenant: httpx.TenantNone, Idempotency: idem,
		SuccessStatus: status, Guard: httpx.GuardPortalStaff | httpx.GuardPlatformRole}
}

const (
	gaCfgPath  = "/game-admin/{config_type}"
	gaItemPath = "/game-admin/{config_type}/{config_id}"
	gaVerPath  = "/game-admin/{config_type}/{config_id}/versions/{version}"
)

// GameAdminOperations are the shared game-admin operations (x-service-routing game, competition, chat) module
// competition serves for its seven config types (F108-AC-01).
var GameAdminOperations = map[string]httpx.Operation{
	"listGameConfigs":             adminOp("listGameConfigs", http.MethodGet, gaCfgPath, httpx.IdempotencyNone, http.StatusOK),
	"createGameConfig":            adminOp("createGameConfig", http.MethodPost, gaCfgPath, httpx.IdempotencyRequired, http.StatusCreated),
	"getGameConfigVersion":        adminOp("getGameConfigVersion", http.MethodGet, gaVerPath, httpx.IdempotencyNone, http.StatusOK),
	"updateGameConfigDraft":       adminOp("updateGameConfigDraft", http.MethodPatch, gaVerPath, httpx.IdempotencyNone, http.StatusOK),
	"transitionGameConfigVersion": adminOp("transitionGameConfigVersion", http.MethodPost, gaVerPath+":transition", httpx.IdempotencyRequired, http.StatusOK),
	"listGameConfigVersions":      adminOp("listGameConfigVersions", http.MethodGet, gaItemPath+"/versions", httpx.IdempotencyNone, http.StatusOK),
	"diffGameConfigVersions":      adminOp("diffGameConfigVersions", http.MethodGet, gaVerPath+":diff", httpx.IdempotencyNone, http.StatusOK),
	"publishGameConfigVersion":    adminOp("publishGameConfigVersion", http.MethodPost, gaVerPath+":publish", httpx.IdempotencyRequired, http.StatusOK),
	"rollbackGameConfig":          adminOp("rollbackGameConfig", http.MethodPost, gaItemPath+":rollback", httpx.IdempotencyRequired, http.StatusOK),
	"unpublishGameConfig":         adminOp("unpublishGameConfig", http.MethodPost, gaItemPath+":unpublish", httpx.IdempotencyRequired, http.StatusOK),
	"listGameConfigReleases":      adminOp("listGameConfigReleases", http.MethodGet, gaItemPath+"/releases", httpx.IdempotencyNone, http.StatusOK),
}

// RegisterGameAdmin mounts the competition game-admin operations on the shared routes (httpx.HandleRouted by config_type).
func RegisterGameAdmin(s *httpx.Server, h GameAdminHandlers) error {
	if h.Commands == nil || h.Queries == nil {
		return errors.New("competition: the game-admin handlers are required")
	}
	routes := map[string]http.HandlerFunc{"listGameConfigs": h.list, "createGameConfig": h.create, "getGameConfigVersion": h.get,
		"updateGameConfigDraft": h.update, "transitionGameConfigVersion": h.transition, "listGameConfigVersions": h.versions,
		"diffGameConfigVersions": h.diff, "publishGameConfigVersion": h.publish, "rollbackGameConfig": h.rollback,
		"unpublishGameConfig": h.unpublish, "listGameConfigReleases": h.releases}
	values := gameconfig.Names()
	for id, fn := range routes {
		if err := s.HandleRouted(GameAdminOperations[id], "config_type", values, fn); err != nil {
			return err
		}
	}
	return nil
}

func gaType(r *http.Request) string { return chi.URLParam(r, "config_type") }

func gaLimit(r *http.Request) int {
	n, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	return n
}

func gaBody(r *http.Request, v any) error {
	if r.Body == nil || r.ContentLength == 0 {
		return nil
	}
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return problem.New(problem.BadRequest, "invalid JSON body")
	}
	return nil
}

// gaItem reads config_id (and version) of the path; a malformed one is 404.
func gaItem(w http.ResponseWriter, r *http.Request, withVersion bool) (uuid.UUID, int, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "config_id"))
	if err != nil {
		fail(w, r, problem.New(problem.NotFound, ""))
		return uuid.Nil, 0, false
	}
	if !withVersion {
		return id, 0, true
	}
	n, err := strconv.Atoi(chi.URLParam(r, "version"))
	if err != nil || n < 1 {
		fail(w, r, problem.New(problem.NotFound, ""))
		return uuid.Nil, 0, false
	}
	return id, n, true
}

func writeConfigVersion(w http.ResponseWriter, status int, v gameadmin.ConfigVersionView) {
	if v.ETag != "" {
		w.Header().Set("ETag", v.ETag)
	}
	writeJSON(w, status, v)
}

type configDraftBody struct {
	Title            *string         `json:"title"`
	Payload          json.RawMessage `json:"payload"`
	ConfigID         *uuid.UUID      `json:"config_id"`
	BasedOnVersionID *uuid.UUID      `json:"based_on_version_id"`
}

func (h GameAdminHandlers) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("from") != "" || q.Get("to") != "" {
		fail(w, r, problem.Invalid("from", problem.SubInvalid, "the date window is only for config_type events"))
		return
	}
	out, err := h.Queries.List(r.Context(), gaType(r), q.Get("status"), q.Get("cursor"), gaLimit(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h GameAdminHandlers) create(w http.ResponseWriter, r *http.Request) {
	var b configDraftBody
	if err := gaBody(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Commands.Create(r.Context(), gaType(r), command.ConfigDraft{Title: b.Title, Payload: b.Payload, ConfigID: b.ConfigID,
		BasedOn: b.BasedOnVersionID})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeConfigVersion(w, http.StatusCreated, v)
}

func (h GameAdminHandlers) get(w http.ResponseWriter, r *http.Request) {
	id, n, ok := gaItem(w, r, true)
	if !ok {
		return
	}
	v, err := h.Queries.Get(r.Context(), gaType(r), id, n)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeConfigVersion(w, http.StatusOK, v)
}

func (h GameAdminHandlers) update(w http.ResponseWriter, r *http.Request) {
	id, n, ok := gaItem(w, r, true)
	if !ok {
		return
	}
	var b configDraftBody
	if err := gaBody(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Commands.UpdateDraft(r.Context(), gaType(r), id, n, r.Header.Get("If-Match"), command.ConfigDraft{Title: b.Title, Payload: b.Payload})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeConfigVersion(w, http.StatusOK, v)
}

func (h GameAdminHandlers) transition(w http.ResponseWriter, r *http.Request) {
	id, n, ok := gaItem(w, r, true)
	if !ok {
		return
	}
	var b struct {
		Action string `json:"action"`
		Note   string `json:"note"`
	}
	if err := gaBody(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Commands.Transition(r.Context(), gaType(r), id, n, r.Header.Get("If-Match"), b.Action, b.Note)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeConfigVersion(w, http.StatusOK, v)
}

func (h GameAdminHandlers) versions(w http.ResponseWriter, r *http.Request) {
	id, _, ok := gaItem(w, r, false)
	if !ok {
		return
	}
	out, err := h.Queries.Versions(r.Context(), gaType(r), id, r.URL.Query().Get("cursor"), gaLimit(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h GameAdminHandlers) diff(w http.ResponseWriter, r *http.Request) {
	id, n, ok := gaItem(w, r, true)
	if !ok {
		return
	}
	with, err := strconv.Atoi(r.URL.Query().Get("with"))
	if err != nil || with < 1 {
		fail(w, r, problem.Invalid("with", problem.SubInvalid, "a version number is required"))
		return
	}
	out, err := h.Queries.Diff(r.Context(), gaType(r), id, n, with)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h GameAdminHandlers) publish(w http.ResponseWriter, r *http.Request) {
	id, n, ok := gaItem(w, r, true)
	if !ok {
		return
	}
	var b struct {
		PublishAt     *time.Time `json:"publish_at"`
		EffectiveFrom *time.Time `json:"effective_from"`
		ChangeNote    string     `json:"change_note"`
	}
	if err := gaBody(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Commands.Publish(r.Context(), gaType(r), id, n, r.Header.Get("If-Match"),
		command.PublishInput{PublishAt: b.PublishAt, EffectiveFrom: b.EffectiveFrom, ChangeNote: b.ChangeNote})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h GameAdminHandlers) rollback(w http.ResponseWriter, r *http.Request) {
	id, _, ok := gaItem(w, r, false)
	if !ok {
		return
	}
	var b struct {
		ToVersion int    `json:"to_version"`
		Reason    string `json:"reason"`
	}
	if err := gaBody(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Commands.Rollback(r.Context(), gaType(r), id, b.ToVersion, b.Reason)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h GameAdminHandlers) unpublish(w http.ResponseWriter, r *http.Request) {
	id, _, ok := gaItem(w, r, false)
	if !ok {
		return
	}
	var b struct {
		Reason string `json:"reason"`
	}
	if err := gaBody(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Commands.Unpublish(r.Context(), gaType(r), id, b.Reason)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h GameAdminHandlers) releases(w http.ResponseWriter, r *http.Request) {
	id, _, ok := gaItem(w, r, false)
	if !ok {
		return
	}
	out, err := h.Queries.Releases(r.Context(), gaType(r), id, r.URL.Query().Get("cursor"), gaLimit(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
