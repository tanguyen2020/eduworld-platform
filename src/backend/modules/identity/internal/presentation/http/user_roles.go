package http

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
)

// F012 US5: user roles.
var userRoleOperations = map[string]func(Handlers) http.HandlerFunc{
	"listTenantUserRoles":    func(h Handlers) http.HandlerFunc { return h.listTenantUserRoles },
	"grantTenantUserRole":    func(h Handlers) http.HandlerFunc { return h.grantUserRole(command.RouteTenant) },
	"revokeTenantUserRole":   func(h Handlers) http.HandlerFunc { return h.revokeUserRole(command.RouteTenant) },
	"listSchoolUserRoles":    func(h Handlers) http.HandlerFunc { return h.listSchoolUserRoles },
	"grantSchoolUserRole":    func(h Handlers) http.HandlerFunc { return h.grantUserRole(command.RouteSchool) },
	"listAllUserRoles":       func(h Handlers) http.HandlerFunc { return h.listAllUserRoles },
	"grantPlatformUserRole":  func(h Handlers) http.HandlerFunc { return h.grantUserRole(command.RoutePlatform) },
	"revokePlatformUserRole": func(h Handlers) http.HandlerFunc { return h.revokeUserRole(command.RoutePlatform) },
}

func userRolesInput(r *http.Request) (query.UserRolesInput, error) {
	in := query.UserRolesInput{ListInput: list(r), ScopeType: r.URL.Query().Get("scope_type")}
	var err error
	if in.UserID, err = optUUID(r, "user_id"); err != nil {
		return in, err
	}
	if in.RoleID, err = optUUID(r, "role_id"); err != nil {
		return in, err
	}
	in.ScopeID, err = optUUID(r, "scope_id")
	return in, err
}

func (h Handlers) listTenantUserRoles(w http.ResponseWriter, r *http.Request) {
	in, err := userRolesInput(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.UserRoleList.ListTenant(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) listSchoolUserRoles(w http.ResponseWriter, r *http.Request) {
	school, err := pathUUID(r, "school_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	in, err := userRolesInput(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.UserRoleList.ListSchool(r.Context(), school, in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) listAllUserRoles(w http.ResponseWriter, r *http.Request) {
	in, err := userRolesInput(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.UserRoleList.ListAll(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

type grantBody struct {
	UserID    uuid.UUID  `json:"user_id"`
	RoleID    uuid.UUID  `json:"role_id"`
	ScopeType string     `json:"scope_type"`
	ScopeID   *uuid.UUID `json:"scope_id"`
	ValidFrom *time.Time `json:"valid_from"`
	ValidTo   *time.Time `json:"valid_to"`
	Reason    string     `json:"reason"`
	Training  *time.Time `json:"training_completed_at"`
}

func userRoleView(r write.UserRoleRow) read.UserRoleView {
	by := r.GrantedBy
	return read.UserRoleView{UserRoleID: r.ID, UserID: r.UserID, RoleID: r.RoleID, RoleCode: r.RoleCode, TenantID: r.TenantID, ScopeType: r.ScopeType,
		ScopeID: r.ScopeID, ValidFrom: r.ValidFrom, ValidTo: r.ValidTo, Source: r.Source, GrantedBy: &by, CreatedAt: r.CreatedAt}
}

func (h Handlers) grantUserRole(route command.UserRoleRoute) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		in := command.GrantUserRoleInput{Route: route}
		if route == command.RouteSchool {
			school, err := pathUUID(r, "school_id")
			if err != nil {
				fail(w, r, err)
				return
			}
			in.SchoolID = &school
		}
		var b grantBody
		if err := decode(r, &b); err != nil {
			fail(w, r, err)
			return
		}
		in.UserID, in.RoleID, in.ScopeType, in.ScopeID = b.UserID, b.RoleID, b.ScopeType, b.ScopeID
		in.ValidFrom, in.ValidTo, in.Reason, in.Training = b.ValidFrom, b.ValidTo, b.Reason, b.Training
		out, err := h.UserRoles.Grant(r.Context(), in)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, userRoleView(out))
	}
}

func (h Handlers) revokeUserRole(route command.UserRoleRoute) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathUUID(r, "user_role_id")
		if err != nil {
			fail(w, r, err)
			return
		}
		if err := h.UserRoles.Revoke(r.Context(), command.RevokeUserRoleInput{Route: route, UserRoleID: id, Reason: r.URL.Query().Get("reason")}); err != nil {
			fail(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
