package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// SwitchCommands are the write use cases of the moderation switches (command.SwitchAdmin, F116 US3).
type SwitchCommands interface {
	SetModerationSwitches(ctx context.Context, in command.SwitchInput) (command.SwitchResult, error)
	ApproveModerationSwitchChange(ctx context.Context, id uuid.UUID) (command.SwitchState, error)
}

// SwitchQueries are the read use cases of the moderation switches (query.Switches).
type SwitchQueries interface {
	ListModerationSwitches(ctx context.Context, limit int, cursor string) (query.SwitchOverview, error)
	ListModerationSwitchHistory(ctx context.Context, limit int, cursor string) (query.SwitchHistoryPage, error)
}

// SwitchHandlers serve the moderation switch operations (core-api, internal gateway: httpx adds GuardPortalStaff +
// GuardPlatformRole to /admin/ routes; chat.policy.toggle is required in each use case, R7).
type SwitchHandlers struct {
	Commands SwitchCommands
	Queries  SwitchQueries
}

// SwitchOperations are the F116 US3 operations (checked against the spec by the contract test).
var SwitchOperations = []string{"listModerationSwitches", "setModerationSwitches", "listModerationSwitchHistory",
	"approveModerationSwitchChange"}

func init() {
	for _, o := range []httpx.Operation{
		{ID: "listModerationSwitches", Module: "chat", Method: http.MethodGet, Pattern: "/admin/chat/moderation-switches",
			Tenant: httpx.TenantNone, SuccessStatus: http.StatusOK},
		{ID: "setModerationSwitches", Module: "chat", Method: http.MethodPut, Pattern: "/admin/chat/moderation-switches",
			Tenant: httpx.TenantOptional, Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusOK},
		{ID: "listModerationSwitchHistory", Module: "chat", Method: http.MethodGet, Pattern: "/admin/chat/moderation-switches/history",
			Tenant: httpx.TenantNone, SuccessStatus: http.StatusOK},
		{ID: "approveModerationSwitchChange", Module: "chat", Method: http.MethodPost,
			Pattern: "/admin/chat/moderation-switches/requests/{request_id}:approve", Tenant: httpx.TenantOptional,
			Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusOK},
	} {
		Operations[o.ID] = o
	}
}

// registerSwitches mounts the F116 US3 operations.
func registerSwitches(s *httpx.Server, h *SwitchHandlers) error {
	if h.Commands == nil || h.Queries == nil {
		return errors.New("chat: the switch handlers are required")
	}
	routes := map[string]http.HandlerFunc{
		"listModerationSwitches":        h.list,
		"setModerationSwitches":         h.set,
		"listModerationSwitchHistory":   h.history,
		"approveModerationSwitchChange": h.approve,
	}
	for _, id := range SwitchOperations {
		if err := s.Handle(Operations[id], routes[id]); err != nil {
			return err
		}
	}
	return nil
}

func (h SwitchHandlers) list(w http.ResponseWriter, r *http.Request) {
	res, err := h.Queries.ListModerationSwitches(r.Context(), limitOf(r), r.URL.Query().Get("cursor"))
	if err != nil {
		fail(w, r, err)
		return
	}
	tenants := make([]map[string]any, len(res.Tenants))
	for i, t := range res.Tenants {
		tenants[i] = t.Body()
	}
	writeJSON(w, http.StatusOK, map[string]any{"platform": res.Platform.Body(), "tenants": tenants, "page": res.Page})
}

func (h SwitchHandlers) set(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Scope      string          `json:"scope"`
		TenantID   *uuid.UUID      `json:"tenant_id"`
		EnabledAll *bool           `json:"enabled_all"`
		Steps      map[string]bool `json:"steps"`
		Reason     string          `json:"reason"`
		ExpiresAt  *time.Time      `json:"expires_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.EnabledAll == nil {
		fail(w, r, problem.Invalid("enabled_all", problem.SubInvalid, "required"))
		return
	}
	res, err := h.Commands.SetModerationSwitches(r.Context(), command.SwitchInput{Scope: b.Scope, TenantID: b.TenantID,
		EnabledAll: *b.EnabledAll, Steps: b.Steps, Reason: b.Reason, ExpiresAt: b.ExpiresAt})
	if err != nil {
		fail(w, r, err)
		return
	}
	if res.Request != nil {
		writeJSON(w, http.StatusAccepted, service.RequestBody(*res.Request))
		return
	}
	writeJSON(w, http.StatusOK, res.State.Body())
}

func (h SwitchHandlers) history(w http.ResponseWriter, r *http.Request) {
	p, err := h.Queries.ListModerationSwitchHistory(r.Context(), limitOf(r), r.URL.Query().Get("cursor"))
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(p.Data))
	for i, c := range p.Data {
		data[i] = service.ChangeBody(c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "page": p.Page})
}

func (h SwitchHandlers) approve(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "request_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	st, err := h.Commands.ApproveModerationSwitchChange(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, st.Body())
}
