package http

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

// PlatformUserHandlers are the Workspace D operations added for F059 (owner F012): platform staff accounts and the
// read-only integration overview; registered apart from Operations so the features evolve in separate files.
type PlatformUserHandlers struct {
	List         query.PlatformUsers
	Create       command.PlatformStaff
	Integrations query.Integrations
}

// PlatformUserOperations are the F059-1/F059-2 operations; the contract test checks them against the merged spec.
// /admin/* also gets platform_role from httpx (internal gateway, azp eduworld-portal, platform role, ADR-0027).
var PlatformUserOperations = map[string]httpx.Operation{
	"listPlatformUsers":      portalOp2("listPlatformUsers", http.MethodGet, "/admin/users", httpx.TenantNone, httpx.IdempotencyNone, 200),
	"createPlatformUser":     portalOp2("createPlatformUser", http.MethodPost, "/admin/users", httpx.TenantNone, httpx.IdempotencyRequired, 201),
	"getIntegrationOverview": portalOp2("getIntegrationOverview", http.MethodGet, "/admin/integrations", httpx.TenantNone, httpx.IdempotencyNone, 200),
}

// RegisterPlatformUsers mounts the F059-1/F059-2 operations.
func RegisterPlatformUsers(s *httpx.Server, h PlatformUserHandlers) error {
	routes := map[string]http.HandlerFunc{
		"listPlatformUsers":      h.listPlatformUsers,
		"createPlatformUser":     h.createPlatformUser,
		"getIntegrationOverview": h.getIntegrationOverview,
	}
	for id, fn := range routes {
		o, ok := PlatformUserOperations[id]
		if !ok {
			return errors.New("identity: no operation " + id)
		}
		if err := s.Handle(o, fn); err != nil {
			return err
		}
	}
	return nil
}

func (h PlatformUserHandlers) listPlatformUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	out, err := h.List.List(r.Context(), query.PlatformUsersInput{ListInput: list(r), Q: q.Get("q"), UserType: q.Get("user_type"),
		Status: q.Get("status"), RoleCode: q.Get("role_code"), TenantFilter: q.Get("tenant_ids") != ""})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h PlatformUserHandlers) createPlatformUser(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Username    string      `json:"username"`
		Email       string      `json:"email"`
		DisplayName string      `json:"display_name"`
		Locale      string      `json:"locale"`
		RoleIDs     []uuid.UUID `json:"initial_role_ids"`
		Reason      string      `json:"reason"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Create.Create(r.Context(), command.CreatePlatformStaffInput{Username: b.Username, Email: b.Email, DisplayName: b.DisplayName,
		Locale: b.Locale, RoleIDs: b.RoleIDs, Reason: b.Reason})
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, out.Version)
	writeJSON(w, http.StatusCreated, command.PlatformStaffView(out))
}

func (h PlatformUserHandlers) getIntegrationOverview(w http.ResponseWriter, r *http.Request) {
	out, err := h.Integrations.Overview(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
