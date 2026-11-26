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

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/chatasset"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// AssetCommands are the Game Studio use cases of the sticker/GIF library (command.ChatAssets, F118-AC-17).
type AssetCommands interface {
	Create(ctx context.Context, typ, title string, payload json.RawMessage) (port.AssetVersion, error)
	UpdateDraft(ctx context.Context, typ string, id uuid.UUID, version int, title string, payload json.RawMessage) (port.AssetVersion, error)
	Transition(ctx context.Context, typ string, id uuid.UUID, version int, action string) (port.AssetVersion, error)
	Publish(ctx context.Context, typ string, id uuid.UUID, version int, note string) (port.AssetRelease, error)
	Presign(ctx context.Context, typ string, id uuid.UUID, file, contentType, sha string, size int) (string, string, time.Time, error)
	Rollback(ctx context.Context, typ string, id uuid.UUID, to int, reason string) (port.AssetRelease, error)
	Unpublish(ctx context.Context, typ string, id uuid.UUID, reason string) (port.AssetRelease, error)
}

// AssetQueries are the Game Studio reads (query.ChatAssets).
type AssetQueries interface {
	List(ctx context.Context, typ string, in query.ListInput) (query.AssetPage, error)
	Versions(ctx context.Context, typ string, id uuid.UUID, cursor string, limit int) (query.AssetVersionPage, error)
	Releases(ctx context.Context, typ string, id uuid.UUID, cursor string, limit int) (query.AssetReleasePage, error)
	Diff(ctx context.Context, typ string, id uuid.UUID, no, with int) ([]chatasset.Change, error)
	Get(ctx context.Context, typ string, id uuid.UUID, version int) (*port.AssetVersion, error)
	// PreviewURLs adds the signed preview URLs of the files to a response payload (F082-AC-07).
	PreviewURLs(ctx context.Context, t chatasset.Type, payload map[string]any)
}

// AssetHandlers serve the Game Studio sticker/GIF administration (F118 US3): internal gateway, portal staff with a
// platform role (httpx adds GuardPortalStaff + GuardPlatformRole on /game-admin/, no MFA — ADR-0094); the permission is
// checked in each use case (R7, R20). The shared /game-admin/{config_type} operations are served for config_type
// sticker-packs and gifs (x-service-routing: chat); any other type is 404 here until its owner module serves it.
type AssetHandlers struct {
	Commands AssetCommands
	Queries  AssetQueries
}

// AssetConfigTypes are the values of config_type module chat serves on the shared game-admin routes.
var AssetConfigTypes = []string{"sticker-packs", "gifs"}

// AssetOperations are the game-admin operations served by module chat.
var AssetOperations = []string{"listGameConfigs", "createGameConfig", "getGameConfigVersion", "updateGameConfigDraft",
	"transitionGameConfigVersion", "listGameConfigVersions", "publishGameConfigVersion", "presignChatAssetUpload",
	"diffGameConfigVersions", "rollbackGameConfig", "unpublishGameConfig", "listGameConfigReleases"}

func init() {
	for _, o := range []httpx.Operation{
		{ID: "listGameConfigs", Method: http.MethodGet, Pattern: "/game-admin/{config_type}", SuccessStatus: http.StatusOK},
		{ID: "createGameConfig", Method: http.MethodPost, Pattern: "/game-admin/{config_type}", SuccessStatus: http.StatusCreated,
			Idempotency: httpx.IdempotencyRequired},
		{ID: "getGameConfigVersion", Method: http.MethodGet, Pattern: "/game-admin/{config_type}/{config_id}/versions/{version}",
			SuccessStatus: http.StatusOK},
		{ID: "updateGameConfigDraft", Method: http.MethodPatch, Pattern: "/game-admin/{config_type}/{config_id}/versions/{version}",
			SuccessStatus: http.StatusOK},
		{ID: "transitionGameConfigVersion", Method: http.MethodPost, Pattern: "/game-admin/{config_type}/{config_id}/versions/{version}:transition",
			SuccessStatus: http.StatusOK, Idempotency: httpx.IdempotencyRequired},
		{ID: "listGameConfigVersions", Method: http.MethodGet, Pattern: "/game-admin/{config_type}/{config_id}/versions", SuccessStatus: http.StatusOK},
		{ID: "publishGameConfigVersion", Method: http.MethodPost, Pattern: "/game-admin/{config_type}/{config_id}/versions/{version}:publish",
			SuccessStatus: http.StatusOK, Idempotency: httpx.IdempotencyRequired},
		{ID: "presignChatAssetUpload", Method: http.MethodPost, Pattern: "/game-admin/{config_type}/{config_id}/assets:presign",
			SuccessStatus: http.StatusCreated, Idempotency: httpx.IdempotencyRequired},
		// F082: diff, rollback, unpublish and the release history of sticker-packs/gifs.
		{ID: "diffGameConfigVersions", Method: http.MethodGet, Pattern: "/game-admin/{config_type}/{config_id}/versions/{version}:diff",
			SuccessStatus: http.StatusOK},
		{ID: "rollbackGameConfig", Method: http.MethodPost, Pattern: "/game-admin/{config_type}/{config_id}:rollback", SuccessStatus: http.StatusOK,
			Idempotency: httpx.IdempotencyRequired},
		{ID: "unpublishGameConfig", Method: http.MethodPost, Pattern: "/game-admin/{config_type}/{config_id}:unpublish", SuccessStatus: http.StatusOK,
			Idempotency: httpx.IdempotencyRequired},
		{ID: "listGameConfigReleases", Method: http.MethodGet, Pattern: "/game-admin/{config_type}/{config_id}/releases", SuccessStatus: http.StatusOK},
	} {
		o.Module, o.Tenant = "chat", httpx.TenantNone
		Operations[o.ID] = o
	}
}

// registerAssets mounts the Game Studio operations of module chat.
func registerAssets(s *httpx.Server, h *AssetHandlers) error {
	if h.Commands == nil || h.Queries == nil {
		return errors.New("chat: the Game Studio asset handlers are required")
	}
	routes := map[string]http.HandlerFunc{"listGameConfigs": h.list, "createGameConfig": h.create, "getGameConfigVersion": h.get,
		"updateGameConfigDraft": h.update, "transitionGameConfigVersion": h.transition, "listGameConfigVersions": h.versions,
		"publishGameConfigVersion": h.publish, "presignChatAssetUpload": h.presign, "diffGameConfigVersions": h.diff,
		"rollbackGameConfig": h.rollback, "unpublishGameConfig": h.unpublish, "listGameConfigReleases": h.releases}
	for _, id := range AssetOperations {
		if id == "presignChatAssetUpload" {
			if err := s.Handle(Operations[id], routes[id]); err != nil {
				return err
			}
			continue
		}
		// Shared /game-admin/{config_type} operations: module chat serves its two config types; module game the game
		// types (F078) on the same route (httpx.HandleRouted, x-service-routing).
		if err := s.HandleRouted(Operations[id], "config_type", AssetConfigTypes, routes[id]); err != nil {
			return err
		}
	}
	return nil
}

func (h AssetHandlers) render(ctx context.Context, v port.AssetVersion) map[string]any {
	m := renderAsset(v)
	h.Queries.PreviewURLs(ctx, v.Type, m["payload"].(map[string]any))
	return m
}

func renderAsset(v port.AssetVersion) map[string]any {
	var payload map[string]any
	_ = json.Unmarshal(v.Payload, &payload)
	if payload == nil {
		payload = map[string]any{}
	}
	m := map[string]any{"config_type": string(v.Type), "config_id": v.ConfigID, "config_version_id": v.VersionID, "version": v.Version,
		"status": string(v.Status), "payload": payload, "author_id": v.AuthorID, "updated_at": v.UpdatedAt.UTC()}
	if v.Title != "" {
		m["title"] = v.Title
	}
	if v.ReviewerID != nil {
		m["reviewer_id"] = *v.ReviewerID
	}
	if v.PublishedAt != nil {
		m["published_at"] = v.PublishedAt.UTC()
	}
	return m
}

func configType(r *http.Request) string { return chi.URLParam(r, "config_type") }

func versionParam(r *http.Request) (int, error) {
	n, err := strconv.Atoi(chi.URLParam(r, "version"))
	if err != nil || n < 1 {
		return 0, problem.New(problem.NotFound, "")
	}
	return n, nil
}

type assetInput struct {
	Title   string          `json:"title"`
	Payload json.RawMessage `json:"payload"`
}

func (h AssetHandlers) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	// from/to (events calendar window) apply to config_type events of module game only (F079-AC-14); translation_status to
	// the game texts of module game (F082-AC-09).
	for _, k := range []string{"from", "to", "translation_status"} {
		if q.Get(k) != "" {
			fail(w, r, problem.Invalid(k, problem.SubInvalid, "filter of config types of module game only"))
			return
		}
	}
	in := query.ListInput{Status: q.Get("status"), Tag: q.Get("tag"), Cursor: q.Get("cursor")}
	in.Limit, _ = strconv.Atoi(q.Get("limit"))
	if a := q.Get("author_id"); a != "" {
		id, err := uuid.Parse(a)
		if err != nil {
			fail(w, r, problem.Invalid("author_id", problem.SubInvalid, "a uuid"))
			return
		}
		in.AuthorID = &id
	}
	out, err := h.Queries.List(r.Context(), configType(r), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(out.Rows))
	for i, v := range out.Rows {
		data[i] = h.render(r.Context(), v)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "page": map[string]any{"has_more": out.NextCursor != nil, "limit": out.Limit,
		"next_cursor": out.NextCursor}})
}

func (h AssetHandlers) create(w http.ResponseWriter, r *http.Request) {
	var b assetInput
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, r, problem.Invalid("payload", problem.SubInvalid, "invalid JSON"))
		return
	}
	v, err := h.Commands.Create(r.Context(), configType(r), b.Title, b.Payload)
	if err != nil {
		fail(w, r, err)
		return
	}
	w.Header().Set("ETag", `"`+strconv.Itoa(v.Version)+`"`)
	writeJSON(w, http.StatusCreated, h.render(r.Context(), v))
}

func (h AssetHandlers) get(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "config_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	n, err := versionParam(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Queries.Get(r.Context(), configType(r), id, n)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, h.render(r.Context(), *v))
}

func (h AssetHandlers) update(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "config_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	n, err := versionParam(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var b assetInput
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, r, problem.Invalid("payload", problem.SubInvalid, "invalid JSON"))
		return
	}
	v, err := h.Commands.UpdateDraft(r.Context(), configType(r), id, n, b.Title, b.Payload)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, h.render(r.Context(), v))
}

func (h AssetHandlers) transition(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "config_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	n, err := versionParam(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, r, problem.Invalid("action", problem.SubInvalid, "invalid JSON"))
		return
	}
	v, err := h.Commands.Transition(r.Context(), configType(r), id, n, b.Action)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, h.render(r.Context(), v))
}

func (h AssetHandlers) versions(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "config_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := h.Queries.Versions(r.Context(), configType(r), id, r.URL.Query().Get("cursor"), limit)
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(out.Rows))
	for i, v := range out.Rows {
		row := map[string]any{"version": v.Version, "content_version_id": v.VersionID, "status": string(v.Status), "author_id": v.AuthorID,
			"is_current": out.Current != nil && *out.Current == v.Version}
		if v.ReviewerID != nil {
			row["reviewer_id"] = *v.ReviewerID
		}
		if v.PublishedBy != nil {
			row["published_by"] = *v.PublishedBy
		}
		if v.PublishedAt != nil {
			row["published_at"] = v.PublishedAt.UTC()
		}
		if v.Title != "" {
			row["change_note"] = v.Title
		}
		data[i] = row
	}
	body := map[string]any{"item_id": id, "data": data, "page": map[string]any{"has_more": out.NextCursor != nil, "limit": out.Limit,
		"next_cursor": out.NextCursor}}
	if out.Current != nil {
		body["current_version"] = *out.Current
	}
	writeJSON(w, http.StatusOK, body)
}

func renderRelease(rel port.AssetRelease) map[string]any {
	m := map[string]any{"release_id": rel.ReleaseID, "item_type": rel.Type.ItemType(), "item_id": rel.ConfigID, "action": rel.Action,
		"status": "applied", "released_by": rel.ActorID, "released_at": rel.At.UTC()}
	if rel.FromVersion != nil {
		m["from_version"] = *rel.FromVersion
	}
	if rel.ToVersion != nil {
		m["to_version"] = *rel.ToVersion
	}
	if rel.Reason != "" {
		m["reason"] = rel.Reason
	}
	return m
}

func (h AssetHandlers) releases(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "config_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := h.Queries.Releases(r.Context(), configType(r), id, r.URL.Query().Get("cursor"), limit)
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(out.Rows))
	for i, rel := range out.Rows {
		data[i] = renderRelease(rel)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "page": map[string]any{"has_more": out.NextCursor != nil, "limit": out.Limit,
		"next_cursor": out.NextCursor}})
}

func (h AssetHandlers) diff(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "config_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	n, err := versionParam(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	with, err := strconv.Atoi(r.URL.Query().Get("with"))
	if err != nil || with < 1 {
		fail(w, r, problem.Invalid("with", problem.SubInvalid, "a version number"))
		return
	}
	changes, err := h.Queries.Diff(r.Context(), configType(r), id, n, with)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"from_version": with, "to_version": n, "changes": changes})
}

func (h AssetHandlers) rollback(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "config_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		To     int    `json:"to_version"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, r, problem.Invalid("to_version", problem.SubInvalid, "invalid JSON"))
		return
	}
	rel, err := h.Commands.Rollback(r.Context(), configType(r), id, b.To, b.Reason)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, renderRelease(rel))
}

func (h AssetHandlers) unpublish(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "config_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, r, problem.Invalid("reason", problem.SubInvalid, "invalid JSON"))
		return
	}
	rel, err := h.Commands.Unpublish(r.Context(), configType(r), id, b.Reason)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, renderRelease(rel))
}

func (h AssetHandlers) publish(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "config_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	n, err := versionParam(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Note string `json:"change_note"`
	}
	if r.ContentLength != 0 {
		_ = json.NewDecoder(r.Body).Decode(&b)
	}
	rel, err := h.Commands.Publish(r.Context(), configType(r), id, n, b.Note)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, renderRelease(rel))
}

func (h AssetHandlers) presign(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "config_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		File        string `json:"file_name"`
		ContentType string `json:"content_type"`
		Size        int    `json:"size_bytes"`
		SHA256      string `json:"sha256"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, r, problem.Invalid("file_name", problem.SubInvalid, "invalid JSON"))
		return
	}
	key, url, exp, err := h.Commands.Presign(r.Context(), configType(r), id, b.File, b.ContentType, b.SHA256, b.Size)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"asset_key": key, "upload_url": url, "expires_at": exp.UTC()})
}
