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

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/gameadmin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gameconfig"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// GameAdminCommands are the write use cases of the Game Operations Console (command.GameAdmin).
type GameAdminCommands interface {
	Create(ctx context.Context, configType string, in command.Draft) (gameadmin.ConfigVersionView, error)
	CreateVersion(ctx context.Context, configType string, configID uuid.UUID, basedOn *int, title, changeNote *string) (gameadmin.ConfigVersionView, error)
	UpdateDraft(ctx context.Context, configType string, configID uuid.UUID, no int, match string, in command.Draft) (gameadmin.ConfigVersionView, error)
	Transition(ctx context.Context, configType string, configID uuid.UUID, no int, match, action, note string) (gameadmin.ConfigVersionView, error)
	Publish(ctx context.Context, configType string, configID uuid.UUID, no int, match string, publishAt *time.Time, changeNote string) (gameadmin.ReleaseView, error)
	CancelSchedule(ctx context.Context, configType string, configID, releaseID uuid.UUID, reason string) (gameadmin.ReleaseView, error)
	Rollback(ctx context.Context, configType string, configID uuid.UUID, toVersion int, reason string) (gameadmin.ReleaseView, error)
	Unpublish(ctx context.Context, configType string, configID uuid.UUID, reason string) (gameadmin.ReleaseView, error)
	CreatePreview(ctx context.Context, configType string, configID uuid.UUID, no int) (command.PreviewToken, error)
}

// GameAdminQueries are the read use cases of the console (query.GameAdmin).
type GameAdminQueries interface {
	List(ctx context.Context, configType string, in query.ConfigListInput) (query.ConfigPage, error)
	Get(ctx context.Context, configType string, configID uuid.UUID, no int) (gameadmin.ConfigVersionView, error)
	Versions(ctx context.Context, configType string, configID uuid.UUID, cursor string, limit int) (query.VersionHistory, error)
	Diff(ctx context.Context, configType string, configID uuid.UUID, no, with int) (query.VersionDiff, error)
	Releases(ctx context.Context, configType string, configID uuid.UUID, cursor string, limit int) (query.ReleasePage, error)
	Preview(ctx context.Context, token, configType string, configID uuid.UUID, no int) (query.PreviewSnapshot, error)
	MediaAsset(ctx context.Context, id uuid.UUID) (query.MediaView, error)
	ClientManifest(ctx context.Context) (query.ClientManifestView, error)
	Simulate(ctx context.Context, configType string, configID uuid.UUID, no int, in query.SimulationInput) (query.SimulationView, error)
	ConfigCounts(ctx context.Context) (query.ConfigCountsView, error)
}

// GameMediaCommands is presignGameMediaUpload (command.GameMedia).
type GameMediaCommands interface {
	Presign(ctx context.Context, in command.PresignInput) (command.PresignResult, error)
}

// GameAdminHandlers serve /game-admin/* and /game-media* of module game (F078): internal gateway only, portal staff with
// a platform role (route guards, ADR-0027, ADR-0094); the permission of the config type is checked in each use case (R7,
// R20). X-Tenant-Id is ignored (platform content, F078-AC-03).
type GameAdminHandlers struct {
	Commands GameAdminCommands
	Queries  GameAdminQueries
	Media    GameMediaCommands
}

var platformGuard = httpx.GuardPortalStaff | httpx.GuardPlatformRole

func adminOp(id, method, pattern string, idem httpx.IdempotencyMode, status int) httpx.Operation {
	return httpx.Operation{ID: id, Module: "game", Method: method, Pattern: pattern, Tenant: httpx.TenantNone, Idempotency: idem,
		SuccessStatus: status, Guard: platformGuard}
}

const (
	cfgPath  = "/game-admin/{config_type}"
	itemPath = "/game-admin/{config_type}/{config_id}"
	verPath  = "/game-admin/{config_type}/{config_id}/versions/{version}"
)

// GameAdminRoutedOperations are the shared /game-admin/{config_type} operations (x-service-routing: game, competition,
// chat) module game serves for its config types.
var GameAdminRoutedOperations = map[string]httpx.Operation{
	"listGameConfigs":             adminOp("listGameConfigs", http.MethodGet, cfgPath, httpx.IdempotencyNone, http.StatusOK),
	"createGameConfig":            adminOp("createGameConfig", http.MethodPost, cfgPath, httpx.IdempotencyRequired, http.StatusCreated),
	"getGameConfigVersion":        adminOp("getGameConfigVersion", http.MethodGet, verPath, httpx.IdempotencyNone, http.StatusOK),
	"updateGameConfigDraft":       adminOp("updateGameConfigDraft", http.MethodPatch, verPath, httpx.IdempotencyNone, http.StatusOK),
	"transitionGameConfigVersion": adminOp("transitionGameConfigVersion", http.MethodPost, verPath+":transition", httpx.IdempotencyRequired, http.StatusOK),
	"listGameConfigVersions":      adminOp("listGameConfigVersions", http.MethodGet, itemPath+"/versions", httpx.IdempotencyNone, http.StatusOK),
	"diffGameConfigVersions":      adminOp("diffGameConfigVersions", http.MethodGet, verPath+":diff", httpx.IdempotencyNone, http.StatusOK),
	"publishGameConfigVersion":    adminOp("publishGameConfigVersion", http.MethodPost, verPath+":publish", httpx.IdempotencyRequired, http.StatusOK),
	"rollbackGameConfig":          adminOp("rollbackGameConfig", http.MethodPost, itemPath+":rollback", httpx.IdempotencyRequired, http.StatusOK),
	"unpublishGameConfig":         adminOp("unpublishGameConfig", http.MethodPost, itemPath+":unpublish", httpx.IdempotencyRequired, http.StatusOK),
	"listGameConfigReleases":      adminOp("listGameConfigReleases", http.MethodGet, itemPath+"/releases", httpx.IdempotencyNone, http.StatusOK),
}

func init() {
	for _, o := range []httpx.Operation{
		adminOp("createGameConfigVersion", http.MethodPost, itemPath+"/versions", httpx.IdempotencyRequired, http.StatusCreated),
		adminOp("cancelScheduledGameConfigPublish", http.MethodPost, itemPath+"/releases/{release_id}:cancel", httpx.IdempotencyRequired, http.StatusOK),
		adminOp("createGamePreviewSession", http.MethodPost, "/game-admin/preview/sessions", httpx.IdempotencyRequired, http.StatusCreated),
		adminOp("getGamePreviewSnapshot", http.MethodGet, "/game-admin/preview/snapshot", httpx.IdempotencyNone, http.StatusOK),
		adminOp("presignGameMediaUpload", http.MethodPost, "/game-media:presign-upload", httpx.IdempotencyRequired, http.StatusCreated),
		adminOp("getGameMediaAsset", http.MethodGet, "/game-media/{asset_id}", httpx.IdempotencyNone, http.StatusOK),
		adminOp("getGameClientManifest", http.MethodGet, "/game-admin/client-manifests/current", httpx.IdempotencyNone, http.StatusOK),
		adminOp("getGameConfigCounts", http.MethodGet, "/game-admin/stats/config-counts", httpx.IdempotencyNone, http.StatusOK),
		adminOp("simulateGameConfig", http.MethodGet, verPath+":simulate", httpx.IdempotencyNone, http.StatusOK),
	} {
		Operations[o.ID] = o
	}
}

// GameAdminConfigTypes are the config_type values module game serves on the shared routes.
func GameAdminConfigTypes() []string {
	out := []string{}
	for _, t := range gameconfig.Types() {
		out = append(out, t.Name)
	}
	return out
}

// registerGameAdmin mounts the console operations.
func registerGameAdmin(s *httpx.Server, h GameAdminHandlers) error {
	if h.Commands == nil || h.Queries == nil || h.Media == nil {
		return errors.New("game: the Game Operations Console handlers are required")
	}
	routed := map[string]http.HandlerFunc{"listGameConfigs": h.list, "createGameConfig": h.create, "getGameConfigVersion": h.get,
		"updateGameConfigDraft": h.update, "transitionGameConfigVersion": h.transition, "listGameConfigVersions": h.versions,
		"diffGameConfigVersions": h.diff, "publishGameConfigVersion": h.publish, "rollbackGameConfig": h.rollback,
		"unpublishGameConfig": h.unpublish, "listGameConfigReleases": h.releases}
	values := GameAdminConfigTypes()
	for id, fn := range routed {
		if err := s.HandleRouted(GameAdminRoutedOperations[id], "config_type", values, fn); err != nil {
			return err
		}
	}
	plain := map[string]http.HandlerFunc{"createGameConfigVersion": h.createVersion, "cancelScheduledGameConfigPublish": h.cancel,
		"createGamePreviewSession": h.createPreview, "getGamePreviewSnapshot": h.preview, "presignGameMediaUpload": h.presign,
		"getGameMediaAsset": h.mediaAsset, "getGameClientManifest": h.clientManifest, "getGameConfigCounts": h.configCounts,
		"simulateGameConfig": h.simulate}
	for id, fn := range plain {
		if err := s.Handle(Operations[id], fn); err != nil {
			return err
		}
	}
	return nil
}

func cfgType(r *http.Request) string { return chi.URLParam(r, "config_type") }

func versionNo(r *http.Request) (int, error) {
	n, err := strconv.Atoi(chi.URLParam(r, "version"))
	if err != nil || n < 1 {
		return 0, problem.New(problem.NotFound, "")
	}
	return n, nil
}

func limitOf(r *http.Request) int {
	n, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	return n
}

func decodeBody(r *http.Request, v any) error {
	if r.Body == nil || r.ContentLength == 0 {
		return nil
	}
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return problem.New(problem.BadRequest, "invalid JSON body")
	}
	return nil
}

type draftBody struct {
	Title   *string         `json:"title"`
	Payload json.RawMessage `json:"payload"`
}

func writeVersion(w http.ResponseWriter, status int, v gameadmin.ConfigVersionView) {
	if v.ETag != "" {
		w.Header().Set("ETag", v.ETag)
	}
	writeJSON(w, status, v)
}

func (h GameAdminHandlers) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	out, err := h.Queries.List(r.Context(), cfgType(r), query.ConfigListInput{Status: q.Get("status"), From: q.Get("from"), To: q.Get("to"),
		Cursor: q.Get("cursor"), Limit: limitOf(r)})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h GameAdminHandlers) create(w http.ResponseWriter, r *http.Request) {
	var b draftBody
	if err := decodeBody(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Commands.Create(r.Context(), cfgType(r), command.Draft{Title: b.Title, Payload: b.Payload})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeVersion(w, http.StatusCreated, v)
}

func (h GameAdminHandlers) createVersion(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "config_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		BasedOn    *int    `json:"based_on_version"`
		Title      *string `json:"title"`
		ChangeNote *string `json:"change_note"`
	}
	if err := decodeBody(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Commands.CreateVersion(r.Context(), cfgType(r), id, b.BasedOn, b.Title, b.ChangeNote)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeVersion(w, http.StatusCreated, v)
}

func (h GameAdminHandlers) item(w http.ResponseWriter, r *http.Request, withVersion bool) (uuid.UUID, int, bool) {
	id, err := pathUUID(r, "config_id")
	if err != nil {
		fail(w, r, problem.New(problem.NotFound, ""))
		return uuid.Nil, 0, false
	}
	if !withVersion {
		return id, 0, true
	}
	n, err := versionNo(r)
	if err != nil {
		fail(w, r, err)
		return uuid.Nil, 0, false
	}
	return id, n, true
}

func (h GameAdminHandlers) get(w http.ResponseWriter, r *http.Request) {
	id, n, ok := h.item(w, r, true)
	if !ok {
		return
	}
	v, err := h.Queries.Get(r.Context(), cfgType(r), id, n)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeVersion(w, http.StatusOK, v)
}

func (h GameAdminHandlers) update(w http.ResponseWriter, r *http.Request) {
	id, n, ok := h.item(w, r, true)
	if !ok {
		return
	}
	var b draftBody
	if err := decodeBody(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Commands.UpdateDraft(r.Context(), cfgType(r), id, n, r.Header.Get("If-Match"), command.Draft{Title: b.Title, Payload: b.Payload})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeVersion(w, http.StatusOK, v)
}

func (h GameAdminHandlers) transition(w http.ResponseWriter, r *http.Request) {
	id, n, ok := h.item(w, r, true)
	if !ok {
		return
	}
	var b struct {
		Action string `json:"action"`
		Note   string `json:"note"`
	}
	if err := decodeBody(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Commands.Transition(r.Context(), cfgType(r), id, n, r.Header.Get("If-Match"), b.Action, b.Note)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeVersion(w, http.StatusOK, v)
}

func (h GameAdminHandlers) versions(w http.ResponseWriter, r *http.Request) {
	id, _, ok := h.item(w, r, false)
	if !ok {
		return
	}
	out, err := h.Queries.Versions(r.Context(), cfgType(r), id, r.URL.Query().Get("cursor"), limitOf(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h GameAdminHandlers) diff(w http.ResponseWriter, r *http.Request) {
	id, n, ok := h.item(w, r, true)
	if !ok {
		return
	}
	with, err := strconv.Atoi(r.URL.Query().Get("with"))
	if err != nil || with < 1 {
		fail(w, r, problem.Invalid("with", problem.SubInvalid, "a version number is required"))
		return
	}
	out, err := h.Queries.Diff(r.Context(), cfgType(r), id, n, with)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h GameAdminHandlers) publish(w http.ResponseWriter, r *http.Request) {
	id, n, ok := h.item(w, r, true)
	if !ok {
		return
	}
	var b struct {
		PublishAt  *time.Time `json:"publish_at"`
		ChangeNote string     `json:"change_note"`
	}
	if err := decodeBody(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Commands.Publish(r.Context(), cfgType(r), id, n, r.Header.Get("If-Match"), b.PublishAt, b.ChangeNote)
	if err != nil {
		fail(w, r, err)
		return
	}
	status := http.StatusOK
	if out.Pending {
		status = http.StatusAccepted
	}
	writeJSON(w, status, out)
}

func (h GameAdminHandlers) cancel(w http.ResponseWriter, r *http.Request) {
	id, _, ok := h.item(w, r, false)
	if !ok {
		return
	}
	rid, err := pathUUID(r, "release_id")
	if err != nil {
		fail(w, r, problem.New(problem.NotFound, ""))
		return
	}
	var b struct {
		Reason string `json:"reason"`
	}
	if err := decodeBody(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Commands.CancelSchedule(r.Context(), cfgType(r), id, rid, b.Reason)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h GameAdminHandlers) rollback(w http.ResponseWriter, r *http.Request) {
	id, _, ok := h.item(w, r, false)
	if !ok {
		return
	}
	var b struct {
		ToVersion int    `json:"to_version"`
		Reason    string `json:"reason"`
	}
	if err := decodeBody(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Commands.Rollback(r.Context(), cfgType(r), id, b.ToVersion, b.Reason)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h GameAdminHandlers) unpublish(w http.ResponseWriter, r *http.Request) {
	id, _, ok := h.item(w, r, false)
	if !ok {
		return
	}
	var b struct {
		Reason string `json:"reason"`
	}
	if err := decodeBody(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Commands.Unpublish(r.Context(), cfgType(r), id, b.Reason)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h GameAdminHandlers) releases(w http.ResponseWriter, r *http.Request) {
	id, _, ok := h.item(w, r, false)
	if !ok {
		return
	}
	out, err := h.Queries.Releases(r.Context(), cfgType(r), id, r.URL.Query().Get("cursor"), limitOf(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h GameAdminHandlers) createPreview(w http.ResponseWriter, r *http.Request) {
	var b struct {
		ConfigType string    `json:"config_type"`
		ConfigID   uuid.UUID `json:"config_id"`
		Version    int       `json:"version"`
	}
	if err := decodeBody(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Commands.CreatePreview(r.Context(), b.ConfigType, b.ConfigID, b.Version)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h GameAdminHandlers) preview(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	id, err := uuid.Parse(q.Get("config_id"))
	if err != nil {
		fail(w, r, problem.Invalid("config_id", problem.SubInvalid, "must be a UUID"))
		return
	}
	n, err := strconv.Atoi(q.Get("version"))
	if err != nil || n < 1 {
		fail(w, r, problem.Invalid("version", problem.SubInvalid, "a version number is required"))
		return
	}
	out, err := h.Queries.Preview(r.Context(), r.Header.Get("X-Preview-Token"), q.Get("config_type"), id, n)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h GameAdminHandlers) presign(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Kind     string `json:"kind"`
		Filename string `json:"filename"`
		MIME     string `json:"mime_type"`
		Size     int64  `json:"size_bytes"`
		SHA256   string `json:"sha256"`
		License  string `json:"license"`
		AltText  string `json:"alt_text"`
	}
	if err := decodeBody(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Media.Presign(r.Context(), command.PresignInput{Kind: b.Kind, Filename: b.Filename, MIME: b.MIME, Size: b.Size, SHA256: b.SHA256,
		License: b.License, AltText: b.AltText})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h GameAdminHandlers) mediaAsset(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "asset_id")
	if err != nil {
		fail(w, r, problem.New(problem.NotFound, ""))
		return
	}
	out, err := h.Queries.MediaAsset(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h GameAdminHandlers) clientManifest(w http.ResponseWriter, r *http.Request) {
	out, err := h.Queries.ClientManifest(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h GameAdminHandlers) configCounts(w http.ResponseWriter, r *http.Request) {
	out, err := h.Queries.ConfigCounts(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// queryInt reads an integer query parameter (nil when absent; 422 naming it when not an integer).
func queryInt(r *http.Request, name string) (*int, error) {
	s := r.URL.Query().Get(name)
	if s == "" {
		return nil, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil, problem.Invalid(name, problem.SubInvalid, "an integer is required")
	}
	return &n, nil
}

func (h GameAdminHandlers) simulate(w http.ResponseWriter, r *http.Request) {
	id, n, ok := h.item(w, r, true)
	if !ok {
		return
	}
	in := query.SimulationInput{}
	minutes, err := queryInt(r, "minutes_per_day")
	if err != nil {
		fail(w, r, err)
		return
	}
	in.MinutesPerDay = minutes
	for _, p := range []struct {
		name string
		dst  *int
	}{{"days_per_week", &in.DaysPerWeek}, {"weeks", &in.Weeks}} {
		name, dst := p.name, p.dst
		v, err := queryInt(r, name)
		if err != nil {
			fail(w, r, err)
			return
		}
		if v == nil {
			fail(w, r, problem.Invalid(name, problem.SubRequired, "required"))
			return
		}
		*dst = *v
	}
	out, err := h.Queries.Simulate(r.Context(), cfgType(r), id, n, in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
