// Package http is the public HTTP presentation of module game (F014): thin handlers that decode the request, call
// one use case and encode the response; authentication, X-Tenant-Id, principal, guards (game_player for every player
// route, ADR-0031/R23), OpenAPI validation and idempotency are the pkg/httpx chain declared per operation.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gamesession"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// SessionHandler are the game session use cases (command.Sessions).
type SessionHandler interface {
	Start(ctx context.Context, in command.StartInput) (command.StartResult, error)
	End(ctx context.Context, id uuid.UUID) error
	Heartbeat(ctx context.Context, id, device uuid.UUID) error
}

// ClientConfigHandler is getClientConfig (query.ClientConfigSource).
type ClientConfigHandler interface {
	GetClientConfig(ctx context.Context, platform, version string) (query.ClientConfig, error)
}

// ManifestHandler is getContentManifest (query.GetContentManifest).
type ManifestHandler interface {
	Handle(ctx context.Context) (query.ContentManifest, error)
}

// LoginEventsHandler are the login history queries (query.LoginEvents).
type LoginEventsHandler interface {
	ForStudent(ctx context.Context, student uuid.UUID, in query.LoginEventsInput) (query.LoginEventsPage, error)
	CrossTenant(ctx context.Context, in query.LoginEventsInput) (query.LoginEventsPage, error)
}

// TicketHandler is createRealtimeTicket (command.RealtimeTickets, F090).
type TicketHandler interface {
	Issue(ctx context.Context, sessionID uuid.UUID) (command.TicketResult, error)
}

// Handlers are the F014 use cases served over HTTP.
type Handlers struct {
	Sessions     SessionHandler
	ClientConfig ClientConfigHandler
	Manifest     ManifestHandler
	LoginEvents  LoginEventsHandler
	Tickets      TicketHandler
	// Profiles and ProfileQueries are the F070 profile use cases.
	Profiles       ProfileCommands
	ProfileQueries ProfileQueries
	// World and WorldQueries are the F071 world use cases.
	World        WorldCommands
	WorldQueries WorldQueries
	// Quests and QuestQueries are the F072 quest use cases.
	Quests       QuestCommands
	QuestQueries QuestQueries
	// IPHasher hashes the client address for game.login_events.ip_hash (nil = no hash). The raw IP stops here.
	IPHasher port.IPHasher
}

func op(id, method, pattern string, tenant httpx.TenantMode, idem httpx.IdempotencyMode, status int, g httpx.Guard) httpx.Operation {
	return httpx.Operation{ID: id, Module: "game", Method: method, Pattern: pattern, Tenant: tenant, Idempotency: idem,
		SuccessStatus: status, Guard: g}
}

// Operations of F014 (method, path, X-Tenant-Id mode, Idempotency-Key, success status and guard of the spec; the
// contract test checks them against specs/openapi/eduworld-api.v1.yaml). Player routes carry game_player; /tenant/
// and /admin/ get portal_staff (+ platform role for /admin/) from the route group.
var Operations = map[string]httpx.Operation{
	"createGameSession": op("createGameSession", http.MethodPost, "/game/sessions", httpx.TenantRequired, httpx.IdempotencyRequired,
		http.StatusCreated, httpx.GuardGamePlayer),
	"endGameSession": op("endGameSession", http.MethodPost, "/game/sessions/{game_session_id}:end", httpx.TenantRequired,
		httpx.IdempotencyNone, http.StatusNoContent, httpx.GuardGamePlayer),
	"heartbeatGameSession": op("heartbeatGameSession", http.MethodPost, "/game/sessions/{game_session_id}:heartbeat",
		httpx.TenantRequired, httpx.IdempotencyNone, http.StatusNoContent, httpx.GuardGamePlayer),
	"createRealtimeTicket": op("createRealtimeTicket", http.MethodPost, "/game/sessions/{game_session_id}/realtime-tickets",
		httpx.TenantRequired, httpx.IdempotencyNone, http.StatusCreated, httpx.GuardGamePlayer),
	"getContentManifest": op("getContentManifest", http.MethodGet, "/game/content-manifest", httpx.TenantRequired,
		httpx.IdempotencyNone, http.StatusOK, httpx.GuardGamePlayer),
	"getClientConfig": {ID: "getClientConfig", Module: "game", Method: http.MethodGet, Pattern: "/game/client-config", Public: true,
		Tenant: httpx.TenantNone, Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK},
	"listTenantStudentLoginEvents": op("listTenantStudentLoginEvents", http.MethodGet, "/tenant/students/{student_id}/login-events",
		httpx.TenantRequired, httpx.IdempotencyNone, http.StatusOK, httpx.GuardPortalStaff),
	"listLoginEventsCrossTenant": op("listLoginEventsCrossTenant", http.MethodGet, "/admin/login-events", httpx.TenantNone,
		httpx.IdempotencyNone, http.StatusOK, httpx.GuardPortalStaff),
	// F070 player profile (game_player, X-Tenant-Id; PUT routes are idempotent by nature, no Idempotency-Key in the spec).
	"getGameProfile": {ID: "getGameProfile", Module: "game", Method: http.MethodGet, Pattern: "/game/profile", Tenant: httpx.TenantRequired,
		Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK, Guard: httpx.GuardGamePlayer},
	"updateGameProfile": {ID: "updateGameProfile", Module: "game", Method: http.MethodPatch, Pattern: "/game/profile",
		Tenant: httpx.TenantRequired, Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK, Guard: httpx.GuardGamePlayer},
	"suggestNicknames": {ID: "suggestNicknames", Module: "game", Method: http.MethodGet, Pattern: "/game/profile/nickname-suggestions",
		Tenant: httpx.TenantRequired, Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK, Guard: httpx.GuardGamePlayer},
	"updateAvatar": {ID: "updateAvatar", Module: "game", Method: http.MethodPut, Pattern: "/game/profile/avatar",
		Tenant: httpx.TenantRequired, Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK, Guard: httpx.GuardGamePlayer},
	"listAvatarPresets": {ID: "listAvatarPresets", Module: "game", Method: http.MethodGet, Pattern: "/game/avatar-presets",
		Tenant: httpx.TenantRequired, Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK, Guard: httpx.GuardGamePlayer},
	"saveAvatarPreset": {ID: "saveAvatarPreset", Module: "game", Method: http.MethodPut, Pattern: "/game/avatar-presets/{slot}",
		Tenant: httpx.TenantRequired, Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK, Guard: httpx.GuardGamePlayer},
	"getCharacterCatalog": {ID: "getCharacterCatalog", Module: "game", Method: http.MethodGet, Pattern: "/game/character/catalog",
		Tenant: httpx.TenantRequired, Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK, Guard: httpx.GuardGamePlayer},
	// F071 world (game_player, X-Tenant-Id; startLevel, endLevelSession and reportDiscovery take an Idempotency-Key).
	"getWorldManifest": op("getWorldManifest", http.MethodGet, "/game/world/manifest", httpx.TenantRequired, httpx.IdempotencyNone,
		http.StatusOK, httpx.GuardGamePlayer),
	"listWorldAreas": op("listWorldAreas", http.MethodGet, "/game/world/areas", httpx.TenantRequired, httpx.IdempotencyNone,
		http.StatusOK, httpx.GuardGamePlayer),
	"enterWorldArea": op("enterWorldArea", http.MethodPost, "/game/world/areas/{area_id}:enter", httpx.TenantRequired,
		httpx.IdempotencyNone, http.StatusOK, httpx.GuardGamePlayer),
	"startLevel": op("startLevel", http.MethodPost, "/game/levels/{level_id}:start", httpx.TenantRequired, httpx.IdempotencyRequired,
		http.StatusCreated, httpx.GuardGamePlayer),
	"endLevelSession": op("endLevelSession", http.MethodPost, "/game/level-sessions/{level_session_id}:end", httpx.TenantRequired,
		httpx.IdempotencyRequired, http.StatusOK, httpx.GuardGamePlayer),
	"getNpcDialogue": op("getNpcDialogue", http.MethodGet, "/game/world/npcs/{npc_id}/dialogue", httpx.TenantRequired,
		httpx.IdempotencyNone, http.StatusOK, httpx.GuardGamePlayer),
	"getWorldExploration": op("getWorldExploration", http.MethodGet, "/game/world/exploration", httpx.TenantRequired,
		httpx.IdempotencyNone, http.StatusOK, httpx.GuardGamePlayer),
	"reportDiscovery": op("reportDiscovery", http.MethodPost, "/game/world/areas/{area_id}/discoveries/{interactable_id}:discover",
		httpx.TenantRequired, httpx.IdempotencyRequired, http.StatusOK, httpx.GuardGamePlayer),
	// F072 quests (game_player, X-Tenant-Id; accept, abandon, report and start take an Idempotency-Key).
	"listQuests": op("listQuests", http.MethodGet, "/game/quests", httpx.TenantRequired, httpx.IdempotencyNone, http.StatusOK, httpx.GuardGamePlayer),
	"getQuest": op("getQuest", http.MethodGet, "/game/quests/{quest_id}", httpx.TenantRequired, httpx.IdempotencyNone, http.StatusOK,
		httpx.GuardGamePlayer),
	"acceptQuest": op("acceptQuest", http.MethodPost, "/game/quests/{quest_id}:accept", httpx.TenantRequired, httpx.IdempotencyRequired,
		http.StatusOK, httpx.GuardGamePlayer),
	"abandonQuest": op("abandonQuest", http.MethodPost, "/game/quests/{quest_id}:abandon", httpx.TenantRequired, httpx.IdempotencyRequired,
		http.StatusOK, httpx.GuardGamePlayer),
	"reportQuestStep": op("reportQuestStep", http.MethodPost, "/game/quests/{quest_id}/steps/{step_id}:report", httpx.TenantRequired,
		httpx.IdempotencyRequired, http.StatusOK, httpx.GuardGamePlayer),
	"startQuestActivity": op("startQuestActivity", http.MethodPost, "/game/quests/{quest_id}/steps/{step_id}/activities:start",
		httpx.TenantRequired, httpx.IdempotencyRequired, http.StatusCreated, httpx.GuardGamePlayer),
}

// PlayerOperations are the player routes of F014 (F014-AC-18: staff tokens get 403 GAME_ACCESS_STUDENT_ONLY).
var PlayerOperations = []string{"createGameSession", "endGameSession", "heartbeatGameSession", "getContentManifest",
	"createRealtimeTicket", "getGameProfile", "updateGameProfile", "suggestNicknames", "updateAvatar", "listAvatarPresets",
	"saveAvatarPreset", "getCharacterCatalog", "getWorldManifest", "listWorldAreas", "enterWorldArea", "startLevel", "endLevelSession",
	"getNpcDialogue", "getWorldExploration", "reportDiscovery", "listQuests", "getQuest", "acceptQuest", "abandonQuest", "reportQuestStep",
	"startQuestActivity"}

// Register mounts every F014 operation on the server.
func Register(s *httpx.Server, h Handlers) error {
	if h.Sessions == nil || h.ClientConfig == nil || h.Manifest == nil || h.LoginEvents == nil || h.Tickets == nil ||
		h.Profiles == nil || h.ProfileQueries == nil || h.World == nil || h.WorldQueries == nil || h.Quests == nil || h.QuestQueries == nil {
		return errors.New("game: every F014/F090/F070/F071/F072 handler is required")
	}
	routes := map[string]http.HandlerFunc{
		"createGameSession":            h.createGameSession,
		"endGameSession":               h.endGameSession,
		"heartbeatGameSession":         h.heartbeatGameSession,
		"getContentManifest":           h.getContentManifest,
		"createRealtimeTicket":         h.createRealtimeTicket,
		"getClientConfig":              h.getClientConfig,
		"listTenantStudentLoginEvents": h.listTenantStudentLoginEvents,
		"listLoginEventsCrossTenant":   h.listLoginEventsCrossTenant,
	}
	for id, fn := range h.worldRoutes() {
		routes[id] = fn
	}
	for id, fn := range h.questRoutes() {
		routes[id] = fn
	}
	for id, fn := range h.profileRoutes() {
		routes[id] = fn
	}
	for id, fn := range routes {
		o, ok := Operations[id]
		if !ok {
			return errors.New("game: no operation " + id)
		}
		if err := s.Handle(o, fn); err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, r *http.Request, err error) {
	var pe *problem.Error
	if errors.As(err, &pe) && pe.Code == problem.Maintenance {
		w.Header().Set("Retry-After", "60")
	}
	var ra command.RetryAfter
	if errors.As(err, &ra) {
		secs := int((ra.After + time.Second - 1) / time.Second)
		if secs < 1 {
			secs = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(secs))
	}
	httpx.WriteError(w, r, err)
}

func pathUUID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		return uuid.Nil, problem.Invalid(name, problem.SubInvalid, "must be a UUID")
	}
	return id, nil
}

func deviceID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(r.Header.Get("X-Device-Id")))
	if err != nil || id == uuid.Nil {
		return uuid.Nil, problem.Invalid("X-Device-Id", problem.SubInvalid, "must be a UUID")
	}
	return id, nil
}

// clientIP is the address of the client as seen by the server (the gateway's X-Forwarded-For is not trusted
// here: the deployment configures the real-IP middleware). It is only hashed, never kept.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (h Handlers) createGameSession(w http.ResponseWriter, r *http.Request) {
	device, err := deviceID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		ClientPlatform string  `json:"client_platform"`
		ClientVersion  string  `json:"client_version"`
		DeviceModel    *string `json:"device_model"`
		Takeover       bool    `json:"takeover"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, r, problem.New(problem.BadRequest, "invalid JSON body"))
		return
	}
	in := command.StartInput{DeviceID: device, Platform: b.ClientPlatform, ClientVersion: b.ClientVersion,
		DeviceModel: b.DeviceModel, Takeover: b.Takeover}
	if h.IPHasher != nil {
		in.IPHash = h.IPHasher.Hash(clientIP(r))
	}
	out, err := h.Sessions.Start(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out.Body())
}

func (h Handlers) endGameSession(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "game_session_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := h.Sessions.End(r.Context(), id); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) heartbeatGameSession(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "game_session_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	device, err := deviceID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := h.Sessions.Heartbeat(r.Context(), id, device); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) getClientConfig(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	c, err := h.ClientConfig.GetClientConfig(r.Context(), q.Get("platform"), q.Get("version"))
	if err != nil {
		fail(w, r, err)
		return
	}
	maintenance := map[string]any{"active": c.Maintenance.Active}
	if c.Maintenance.Message != "" {
		maintenance["message"] = c.Maintenance.Message
	}
	if c.Maintenance.Until != nil {
		maintenance["until"] = c.Maintenance.Until.UTC()
	}
	login := map[string]any{"remember_username_default": c.RememberUsernameDefault}
	if c.LoginURL != "" {
		login["login_url"] = c.LoginURL
	}
	w.Header().Set("Cache-Control", "no-cache")
	writeJSON(w, http.StatusOK, map[string]any{"min_client_version": c.MinClientVersion, "latest_client_version": c.LatestClientVersion,
		"update_required": c.UpdateRequired, "maintenance": maintenance, "login": login, "supported_locales": c.SupportedLocales})
}

// etagMatches implements If-None-Match (weak comparison, list or *).
func etagMatches(header, etag string) bool {
	for _, part := range strings.Split(header, ",") {
		p := strings.TrimSpace(part)
		if p == "*" || strings.Trim(strings.TrimPrefix(p, "W/"), `"`) == etag {
			return true
		}
	}
	return false
}

func (h Handlers) getContentManifest(w http.ResponseWriter, r *http.Request) {
	m, err := h.Manifest.Handle(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	w.Header().Set("ETag", `"`+m.ETag+`"`)
	w.Header().Set("Cache-Control", "private, no-cache")
	if inm := r.Header.Get("If-None-Match"); inm != "" && etagMatches(inm, m.ETag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	body := map[string]any{}
	if len(m.Entries) > 0 {
		if err := json.Unmarshal(m.Entries, &body); err != nil || body == nil {
			body = map[string]any{}
		}
	}
	body["tenant_id"], body["manifest_version"], body["etag"] = m.TenantID, m.ManifestVersion, m.ETag
	body["min_client_version"] = m.MinClientVersion
	if m.GeneratedAt != nil {
		body["generated_at"] = m.GeneratedAt.UTC()
	}
	if _, ok := body["items"]; !ok {
		body["items"] = []any{}
	}
	writeJSON(w, http.StatusOK, body)
}

func timeParam(r *http.Request, name string) (*time.Time, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return nil, problem.Invalid(name, problem.SubInvalid, "must be RFC 3339")
	}
	return &t, nil
}

func listInput(r *http.Request) (query.LoginEventsInput, error) {
	from, err := timeParam(r, "from")
	if err != nil {
		return query.LoginEventsInput{}, err
	}
	to, err := timeParam(r, "to")
	if err != nil {
		return query.LoginEventsInput{}, err
	}
	n, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	return query.LoginEventsInput{From: from, To: to, Cursor: r.URL.Query().Get("cursor"), Limit: n}, nil
}

// loginEvent is the LoginEvent of the contract: never device_id, ip_hash or an IP (F014-AC-11).
func loginEvent(e read.LoginEvent) map[string]any {
	m := map[string]any{"login_event_id": e.LoginEventID, "tenant_id": e.TenantID, "student_id": e.StudentID,
		"game_session_id": e.GameSessionID, "platform": e.Platform, "login_at": e.LoginAt.UTC(), "logout_at": e.LogoutAt,
		"end_reason": e.EndReason}
	if l := gamesession.DeviceLabel(e.DeviceModel); l != nil {
		m["device_label"] = *l
	}
	if e.ClientVersion != nil {
		m["client_version"] = *e.ClientVersion
	}
	return m
}

func writePage(w http.ResponseWriter, p query.LoginEventsPage) {
	data := make([]map[string]any, len(p.Data))
	for i, e := range p.Data {
		data[i] = loginEvent(e)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "page": p.Page})
}

func (h Handlers) listTenantStudentLoginEvents(w http.ResponseWriter, r *http.Request) {
	student, err := pathUUID(r, "student_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	in, err := listInput(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.LoginEvents.ForStudent(r.Context(), student, in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writePage(w, p)
}

func (h Handlers) listLoginEventsCrossTenant(w http.ResponseWriter, r *http.Request) {
	in, err := listInput(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	if v := r.URL.Query().Get("student_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			fail(w, r, problem.Invalid("student_id", problem.SubInvalid, "must be a UUID"))
			return
		}
		in.StudentID = &id
	}
	p, err := h.LoginEvents.CrossTenant(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writePage(w, p)
}

// createRealtimeTicket answers 201 RealtimeTicket (F090-AC-01); the ticket is never logged (R14).
func (h Handlers) createRealtimeTicket(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "game_session_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	t, err := h.Tickets.Issue(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, map[string]any{"ticket": t.Ticket, "expires_at": t.ExpiresAt.UTC(), "url": t.URL,
		"protocol_version": t.ProtocolVersion})
}
