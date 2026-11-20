// Package http is the public HTTP presentation of module chat (F095): thin handlers that decode the request, call one
// use case and encode the response; authentication, X-Tenant-Id, principal, the game_player guard (students only,
// ADR-0031/R23), OpenAPI validation and idempotency are the pkg/httpx chain declared per operation.
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

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Commands are the F095 write use cases (command.Social).
type Commands interface {
	SendFriendRequest(ctx context.Context, addressee uuid.UUID) (command.RequestResult, error)
	AcceptFriendRequest(ctx context.Context, id uuid.UUID) error
	RejectFriendRequest(ctx context.Context, id uuid.UUID) error
	CreateFriendCode(ctx context.Context) (command.CodeResult, error)
	RedeemFriendCode(ctx context.Context, code string) (command.RequestResult, error)
	RemoveFriend(ctx context.Context, other uuid.UUID) error
	BlockUser(ctx context.Context, other uuid.UUID) error
	UnblockUser(ctx context.Context, other uuid.UUID) error
	MuteUser(ctx context.Context, other uuid.UUID) error
	UnmuteUser(ctx context.Context, other uuid.UUID) error
}

// Queries are the F095 read use cases (query.Social).
type Queries interface {
	ListFriends(ctx context.Context, limit int, cursor string) (query.RelationPage, error)
	ListBlocks(ctx context.Context, limit int, cursor string) (query.RelationPage, error)
	ListMutes(ctx context.Context, limit int, cursor string) (query.RelationPage, error)
	ListFriendRequests(ctx context.Context, direction string) ([]query.Request, error)
}

// Handlers are the F095 use cases served over HTTP.
type Handlers struct {
	Commands Commands
	Queries  Queries
}

func op(id, method, pattern string, idem httpx.IdempotencyMode, status int) httpx.Operation {
	return httpx.Operation{ID: id, Module: "chat", Method: method, Pattern: pattern, Tenant: httpx.TenantRequired, Idempotency: idem,
		SuccessStatus: status, Guard: httpx.GuardGamePlayer}
}

// Operations of F095 (method, path, Idempotency-Key and success status of the spec; the contract test checks them
// against specs/openapi/eduworld-api.v1.yaml). Every route is a player route: game_player + X-Tenant-Id (F095-AC-10).
var Operations = map[string]httpx.Operation{
	"listFriends":         op("listFriends", http.MethodGet, "/social/friends", httpx.IdempotencyNone, http.StatusOK),
	"removeFriend":        op("removeFriend", http.MethodDelete, "/social/friends/{user_id}", httpx.IdempotencyNone, http.StatusNoContent),
	"listFriendRequests":  op("listFriendRequests", http.MethodGet, "/social/friend-requests", httpx.IdempotencyNone, http.StatusOK),
	"sendFriendRequest":   op("sendFriendRequest", http.MethodPost, "/social/friend-requests", httpx.IdempotencyRequired, http.StatusCreated),
	"acceptFriendRequest": op("acceptFriendRequest", http.MethodPost, "/social/friend-requests/{request_id}:accept", httpx.IdempotencyRequired, http.StatusNoContent),
	"rejectFriendRequest": op("rejectFriendRequest", http.MethodPost, "/social/friend-requests/{request_id}:reject", httpx.IdempotencyNone, http.StatusNoContent),
	"createFriendCode":    op("createFriendCode", http.MethodPost, "/social/friend-codes", httpx.IdempotencyRequired, http.StatusCreated),
	"redeemFriendCode":    op("redeemFriendCode", http.MethodPost, "/social/friend-codes:redeem", httpx.IdempotencyRequired, http.StatusCreated),
	"listBlocks":          op("listBlocks", http.MethodGet, "/social/blocks", httpx.IdempotencyNone, http.StatusOK),
	"blockUser":           op("blockUser", http.MethodPut, "/social/blocks/{user_id}", httpx.IdempotencyNone, http.StatusNoContent),
	"unblockUser":         op("unblockUser", http.MethodDelete, "/social/blocks/{user_id}", httpx.IdempotencyNone, http.StatusNoContent),
	"listMutes":           op("listMutes", http.MethodGet, "/social/mutes", httpx.IdempotencyNone, http.StatusOK),
	"muteUser":            op("muteUser", http.MethodPut, "/social/mutes/{user_id}", httpx.IdempotencyNone, http.StatusNoContent),
	"unmuteUser":          op("unmuteUser", http.MethodDelete, "/social/mutes/{user_id}", httpx.IdempotencyNone, http.StatusNoContent),
}

// PlayerOperations are the player routes of F095 (all of them).
var PlayerOperations = []string{"listFriends", "removeFriend", "listFriendRequests", "sendFriendRequest", "acceptFriendRequest",
	"rejectFriendRequest", "createFriendCode", "redeemFriendCode", "listBlocks", "blockUser", "unblockUser", "listMutes", "muteUser",
	"unmuteUser"}

// Register mounts every F095 operation on the server.
func Register(s *httpx.Server, h Handlers) error {
	if h.Commands == nil || h.Queries == nil {
		return errors.New("chat: the F095 handlers are required")
	}
	routes := map[string]http.HandlerFunc{
		"listFriends":         h.list(h.Queries.ListFriends, true),
		"removeFriend":        h.relation(h.Commands.RemoveFriend),
		"listFriendRequests":  h.listFriendRequests,
		"sendFriendRequest":   h.sendFriendRequest,
		"acceptFriendRequest": h.request(h.Commands.AcceptFriendRequest),
		"rejectFriendRequest": h.request(h.Commands.RejectFriendRequest),
		"createFriendCode":    h.createFriendCode,
		"redeemFriendCode":    h.redeemFriendCode,
		"listBlocks":          h.list(h.Queries.ListBlocks, false),
		"blockUser":           h.relation(h.Commands.BlockUser),
		"unblockUser":         h.relation(h.Commands.UnblockUser),
		"listMutes":           h.list(h.Queries.ListMutes, false),
		"muteUser":            h.relation(h.Commands.MuteUser),
		"unmuteUser":          h.relation(h.Commands.UnmuteUser),
	}
	for _, id := range PlayerOperations {
		if err := s.Handle(Operations[id], routes[id]); err != nil {
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

// card is a PlayerCard: user_id, nickname, level, avatar_config and (listFriends) presence only (R16).
func card(c query.Card) map[string]any {
	m := map[string]any{"user_id": c.UserID, "nickname": "", "level": c.Level}
	if c.Nickname != nil {
		m["nickname"] = *c.Nickname
	}
	if len(c.Avatar) > 0 {
		m["avatar_config"] = json.RawMessage(c.Avatar)
	}
	if c.Presence != "" {
		m["presence"] = c.Presence
	}
	return m
}

type lister func(ctx context.Context, limit int, cursor string) (query.RelationPage, error)

// list serves listFriends (PlayerCard with presence) and listBlocks / listMutes (PlayerRelation).
func (h Handlers) list(fn lister, friends bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		p, err := fn(r.Context(), n, r.URL.Query().Get("cursor"))
		if err != nil {
			fail(w, r, err)
			return
		}
		data := make([]map[string]any, len(p.Data))
		for i, rel := range p.Data {
			m := card(rel.Card)
			if friends {
				if _, ok := m["presence"]; !ok {
					m["presence"] = "offline"
				}
			} else {
				delete(m, "level")
				m["created_at"] = rel.CreatedAt.UTC()
			}
			data[i] = m
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": data, "page": p.Page})
	}
}

func (h Handlers) relation(fn func(ctx context.Context, other uuid.UUID) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathUUID(r, "user_id")
		if err != nil {
			fail(w, r, err)
			return
		}
		if err := fn(r.Context(), id); err != nil {
			fail(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h Handlers) request(fn func(ctx context.Context, id uuid.UUID) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathUUID(r, "request_id")
		if err != nil {
			fail(w, r, err)
			return
		}
		if err := fn(r.Context(), id); err != nil {
			fail(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h Handlers) sendFriendRequest(w http.ResponseWriter, r *http.Request) {
	var b struct {
		AddresseeID uuid.UUID `json:"addressee_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.AddresseeID == uuid.Nil {
		fail(w, r, problem.Invalid("addressee_id", problem.SubInvalid, "must be a UUID"))
		return
	}
	res, err := h.Commands.SendFriendRequest(r.Context(), b.AddresseeID)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, res.Body())
}

func (h Handlers) redeemFriendCode(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, r, problem.Invalid("code", problem.SubInvalid, "must match ^[A-Z0-9]{6,12}$"))
		return
	}
	res, err := h.Commands.RedeemFriendCode(r.Context(), b.Code)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, res.Body())
}

func (h Handlers) createFriendCode(w http.ResponseWriter, r *http.Request) {
	res, err := h.Commands.CreateFriendCode(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, res.Body())
}

func (h Handlers) listFriendRequests(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Queries.ListFriendRequests(r.Context(), r.URL.Query().Get("direction"))
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(rows))
	for i, q := range rows {
		data[i] = map[string]any{"request_id": q.ID, "status": q.Status, "created_at": q.CreatedAt.UTC(), "expires_at": q.ExpiresAt.UTC(),
			"requester": card(q.Requester), "addressee": card(q.Addressee)}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}
