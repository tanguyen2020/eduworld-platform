package http

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
)

// F012 US4: roles and the permission catalog.
var roleOperations = map[string]func(Handlers) http.HandlerFunc{
	"listTenantPermissions": func(h Handlers) http.HandlerFunc { return h.listPermissions(true) },
	"listAllPermissions":    func(h Handlers) http.HandlerFunc { return h.listPermissions(false) },
	"listTenantRoles":       func(h Handlers) http.HandlerFunc { return h.listTenantRoles },
	"getTenantRole":         func(h Handlers) http.HandlerFunc { return h.getRole(command.RoleRouteTenant) },
	"createTenantRole":      func(h Handlers) http.HandlerFunc { return h.createRole(command.RoleRouteTenant) },
	"updateTenantRole":      func(h Handlers) http.HandlerFunc { return h.updateRole(command.RoleRouteTenant) },
	"deleteTenantRole":      func(h Handlers) http.HandlerFunc { return h.deleteRole(command.RoleRouteTenant) },
	"listPlatformRoles":     func(h Handlers) http.HandlerFunc { return h.listPlatformRoles },
	"getPlatformRole":       func(h Handlers) http.HandlerFunc { return h.getRole(command.RoleRoutePlatform) },
	"createPlatformRole":    func(h Handlers) http.HandlerFunc { return h.createRole(command.RoleRoutePlatform) },
	"updatePlatformRole":    func(h Handlers) http.HandlerFunc { return h.updateRole(command.RoleRoutePlatform) },
	"deletePlatformRole":    func(h Handlers) http.HandlerFunc { return h.deleteRole(command.RoleRoutePlatform) },
}

func (h Handlers) listPermissions(tenantOnly bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		listFn := h.RoleList.ListAllPermissions
		if tenantOnly {
			listFn = h.RoleList.ListTenantPermissions
		}
		out, err := listFn(r.Context())
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func (h Handlers) listTenantRoles(w http.ResponseWriter, r *http.Request) {
	out, err := h.RoleList.ListTenantRoles(r.Context(), list(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) listPlatformRoles(w http.ResponseWriter, r *http.Request) {
	out, err := h.RoleList.ListPlatformRoles(r.Context(), r.URL.Query().Get("role_scope_kind"), list(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) getRole(route command.RoleRoute) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathUUID(r, "role_id")
		if err != nil {
			fail(w, r, err)
			return
		}
		var out read.RoleView
		if route == command.RoleRouteTenant {
			out, err = h.RoleList.GetTenantRole(r.Context(), id)
		} else {
			out, err = h.RoleList.GetPlatformRole(r.Context(), id)
		}
		if err != nil {
			fail(w, r, err)
			return
		}
		etag(w, out.Version)
		writeJSON(w, http.StatusOK, out)
	}
}

type roleBody struct {
	Code string `json:"code"`
	Name struct {
		Vi string  `json:"vi"`
		En *string `json:"en"`
	} `json:"name"`
	Description *string  `json:"description"`
	ScopeKind   string   `json:"role_scope_kind"`
	Permissions []string `json:"permission_codes"`
	Reason      string   `json:"reason"`
}

func (b roleBody) input(route command.RoleRoute) command.RoleInput {
	return command.RoleInput{Route: route, Code: b.Code, NameVi: b.Name.Vi, NameEn: b.Name.En, Description: b.Description, ScopeKind: b.ScopeKind,
		Permissions: b.Permissions, Reason: b.Reason}
}

func roleView(r write.RoleRow) read.RoleView {
	perms := r.Permissions
	if perms == nil {
		perms = []string{}
	}
	return read.RoleView{RoleID: r.ID, Code: r.Code, Name: read.LocalizedName{Vi: r.NameVi, En: r.NameEn}, Description: r.Description,
		ScopeKind: r.ScopeKind, IsSystem: r.IsSystem, TenantID: r.TenantID, PermissionCodes: perms, Version: r.Version, UpdatedAt: r.UpdatedAt}
}

func (h Handlers) createRole(route command.RoleRoute) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var b roleBody
		if err := decode(r, &b); err != nil {
			fail(w, r, err)
			return
		}
		out, err := h.Roles.Create(r.Context(), b.input(route))
		if err != nil {
			fail(w, r, err)
			return
		}
		etag(w, out.Version)
		writeJSON(w, http.StatusCreated, roleView(out))
	}
}

func (h Handlers) updateRole(route command.RoleRoute) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, v, err := roleTarget(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		var b roleBody
		if err := decode(r, &b); err != nil {
			fail(w, r, err)
			return
		}
		in := b.input(route)
		in.RoleID, in.IfMatch = id, v
		out, err := h.Roles.Update(r.Context(), in)
		if err != nil {
			fail(w, r, err)
			return
		}
		etag(w, out.Version)
		writeJSON(w, http.StatusOK, roleView(out))
	}
}

func (h Handlers) deleteRole(route command.RoleRoute) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, v, err := roleTarget(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		if err := h.Roles.Delete(r.Context(), command.RoleInput{Route: route, RoleID: id, IfMatch: v, Reason: r.URL.Query().Get("reason")}); err != nil {
			fail(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func roleTarget(r *http.Request) (uuid.UUID, int64, error) {
	id, err := pathUUID(r, "role_id")
	if err != nil {
		return id, 0, err
	}
	v, err := ifMatch(r)
	return id, v, err
}
