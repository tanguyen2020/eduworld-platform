package http

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
)

// F012 US6: staff accounts of a tenant.
var userOperations = map[string]func(Handlers) http.HandlerFunc{
	"listTenantUsers":  func(h Handlers) http.HandlerFunc { return h.listTenantUsers },
	"createTenantUser": func(h Handlers) http.HandlerFunc { return h.createTenantUser },
	"updateTenantUser": func(h Handlers) http.HandlerFunc { return h.updateTenantUser },
}

func (h Handlers) listTenantUsers(w http.ResponseWriter, r *http.Request) {
	school, err := optUUID(r, "school_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	q := r.URL.Query()
	out, err := h.UserList.List(r.Context(), query.TenantUsersInput{ListInput: list(r), Q: q.Get("q"), UserType: q.Get("user_type"),
		RoleCode: q.Get("role_code"), SchoolID: school})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func staffView(s write.StaffRow) read.UserView {
	v := read.UserView{UserID: s.UserID, UserType: "staff", PrimaryTenantID: s.PrimaryTenantID, Status: s.Status, Username: s.Username,
		Email: s.Email, DisplayName: s.DisplayName, Source: "manual", UserRoles: []read.UserRoleView{}, Version: s.Version}
	if s.Locale != nil {
		v.Locale = *s.Locale
	}
	return v
}

func (h Handlers) createTenantUser(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Username    string `json:"username"`
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		UserType    string `json:"user_type"`
		Locale      string `json:"locale"`
		Roles       []struct {
			RoleID    uuid.UUID  `json:"role_id"`
			ScopeType string     `json:"scope_type"`
			ScopeID   *uuid.UUID `json:"scope_id"`
		} `json:"initial_user_roles"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	in := command.CreateStaffInput{Username: b.Username, Email: b.Email, DisplayName: b.DisplayName, Locale: b.Locale}
	for _, x := range b.Roles {
		in.Roles = append(in.Roles, command.InitialRole{RoleID: x.RoleID, ScopeType: x.ScopeType, ScopeID: x.ScopeID})
	}
	out, err := h.Staff.Create(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, out.Version)
	writeJSON(w, http.StatusCreated, staffView(out))
}

func (h Handlers) updateTenantUser(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "user_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Username    *string `json:"username"`
		Email       *string `json:"email"`
		DisplayName *string `json:"display_name"`
		Locale      *string `json:"locale"`
		Status      *string `json:"status"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Staff.Update(r.Context(), command.UpdateStaffInput{UserID: id, IfMatch: v, Username: b.Username, Email: b.Email,
		DisplayName: b.DisplayName, Locale: b.Locale, Status: b.Status})
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, out.Version)
	writeJSON(w, http.StatusOK, staffView(out))
}
