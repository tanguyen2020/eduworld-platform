package command

import (
	"context"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/appscope"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	domainauthz "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// RoleRoute is /tenant/roles (custom roles of the session tenant) or /admin/roles (platform).
type RoleRoute int

// Role routes.
const (
	RoleRouteTenant RoleRoute = iota
	RoleRoutePlatform
)

// RoleInput is RoleInput of the spec.
type RoleInput struct {
	Route       RoleRoute
	RoleID      uuid.UUID // update/delete
	IfMatch     int64     // update/delete
	Code        string
	NameVi      string
	NameEn      *string
	Description *string
	ScopeKind   string // platform route only; tenant route is always tenant
	Permissions []string
	Reason      string
}

// Roles manages custom and shared roles (F012 US4: AC-16 … AC-19). System roles are read-only (trigger as
// layer 2); a tenant role never holds a platform permission (trigger as layer 2).
type Roles struct {
	UoW    write.UnitOfWork
	Repo   write.RoleRepository
	Events write.EventPublisher
	Audit  write.Auditor
}

var roleCodeRe = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)

// session returns the scope, principal and tenant of a role operation (AC-19).
func (h Roles) session(ctx context.Context, route RoleRoute) (context.Context, authz.Principal, *uuid.UUID, error) {
	p, err := principal(ctx)
	if err != nil {
		return ctx, p, nil, err
	}
	if route == RoleRoutePlatform {
		if err := authz.Require(ctx, p, authz.PermPlatformRoleWrite, authz.Resource{}); err != nil {
			return ctx, p, nil, err
		}
		if p.TenantID != nil { // a custom role of the X-Tenant-Id tenant
			sc, err := appscope.TargetTenant(p, *p.TenantID)
			if err != nil {
				return ctx, p, nil, err
			}
			t := *p.TenantID
			return tenancy.WithScope(ctx, sc), p, &t, nil
		}
		sc, err := authz.PlatformScope(p)
		if err != nil {
			return ctx, p, nil, problem.New(problem.PermissionDenied, "").Wrap(err)
		}
		return tenancy.WithScope(ctx, sc), p, nil, nil
	}
	if p.TenantID == nil {
		return ctx, p, nil, problem.New(problem.TenantHeaderRequired, "")
	}
	if err := authz.Require(ctx, p, authz.PermTenantRoleWrite, authz.Resource{TenantID: p.TenantID}); err != nil {
		return ctx, p, nil, err
	}
	sc, err := authz.ScopeFor(p)
	if err != nil {
		return ctx, p, nil, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	t := *p.TenantID
	return tenancy.WithScope(ctx, sc), p, &t, nil
}

// validate checks the input and the permission scopes (AC-16: PERMISSION_NOT_ALLOWED_FOR_SCOPE, audited).
func (h Roles) validate(ctx context.Context, in *RoleInput, kind string) error {
	pe := problem.New(problem.ValidationFailed, "")
	in.NameVi = strings.TrimSpace(in.NameVi)
	if in.NameEn == nil || strings.TrimSpace(*in.NameEn) == "" {
		vi := in.NameVi // name_en is required in the table; the English name defaults to the Vietnamese one
		in.NameEn = &vi
	}
	if n := utf8.RuneCountInString(in.NameVi); n < 1 || n > 255 {
		pe.AddField("name.vi", problem.SubInvalid, "1 to 255 characters")
	}
	if in.Description != nil && utf8.RuneCountInString(*in.Description) > 1000 {
		pe.AddField("description", problem.SubMaxLength, "at most 1000 characters")
	}
	if len(in.Permissions) == 0 || len(in.Permissions) > 500 {
		pe.AddField("permission_codes", problem.SubInvalid, "1 to 500 permissions")
	}
	if len(pe.Fields) > 0 {
		return pe
	}
	scopes, err := h.Repo.PermissionScopes(ctx, in.Permissions)
	if err != nil {
		return err
	}
	for i, c := range in.Permissions {
		k, known := scopes[c]
		switch {
		case !known:
			pe.AddField("permission_codes["+itoa(i)+"]", problem.SubInvalid, "unknown permission "+c)
		case kind == domainauthz.KindTenant && k == domainauthz.KindPlatform:
			pe.AddField("permission_codes["+itoa(i)+"]", "PERMISSION_NOT_ALLOWED_FOR_SCOPE", c+" is a platform permission")
		}
	}
	if len(pe.Fields) > 0 {
		authz.RecordDenied(ctx, "PERMISSION_NOT_ALLOWED_FOR_SCOPE", map[string]any{"operation": "role", "code": in.Code})
		return pe
	}
	return nil
}

// Create creates a custom (tenant) role or a shared role.
func (h Roles) Create(ctx context.Context, in RoleInput) (out write.RoleRow, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.CreateRole")
	defer func() { done(err) }()
	ctx, p, tenant, err := h.session(ctx, in.Route)
	if err != nil {
		return out, err
	}
	kind := domainauthz.KindTenant
	if in.Route == RoleRoutePlatform && tenant == nil && in.ScopeKind != "" {
		kind = in.ScopeKind
	}
	if kind != domainauthz.KindTenant && kind != domainauthz.KindPlatform {
		return out, problem.Invalid("role_scope_kind", problem.SubInvalid, "tenant or platform")
	}
	if !roleCodeRe.MatchString(in.Code) {
		return out, problem.Invalid("code", problem.SubInvalid, "must match ^[a-z][a-z0-9_]{1,63}$")
	}
	in.Permissions = uniqueSorted(in.Permissions)
	row := write.RoleRow{ID: ids.New(), TenantID: tenant, Code: in.Code, NameVi: in.NameVi, NameEn: in.NameEn, Description: in.Description,
		ScopeKind: kind, Permissions: in.Permissions}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.validate(ctx, &in, kind); err != nil {
			return err
		}
		row.NameVi, row.NameEn = in.NameVi, in.NameEn
		if err := h.Repo.Create(ctx, &row, p.UserID); err != nil {
			return err
		}
		return h.updated(ctx, row, "created", row.Permissions, nil, p.UserID, in.Reason)
	})
	return row, err
}

// Update changes the names and permissions of a role (If-Match; reason required on the platform route).
func (h Roles) Update(ctx context.Context, in RoleInput) (out write.RoleRow, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.UpdateRole")
	defer func() { done(err) }()
	ctx, p, tenant, err := h.session(ctx, in.Route)
	if err != nil {
		return out, err
	}
	if in.Route == RoleRoutePlatform && utf8.RuneCountInString(strings.TrimSpace(in.Reason)) < 3 {
		return out, problem.Invalid("reason", problem.SubRequired, "required (at least 3 characters)")
	}
	in.Permissions = uniqueSorted(in.Permissions)
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		row, err := h.load(ctx, in, tenant)
		if err != nil {
			return err
		}
		if in.Code != "" && in.Code != row.Code {
			return problem.Invalid("code", problem.SubInvalid, "the code of a role cannot change")
		}
		if err := h.validate(ctx, &in, row.ScopeKind); err != nil {
			return err
		}
		if row.Version != in.IfMatch {
			return problem.New(problem.VersionMismatch, "")
		}
		add, remove := diff(row.Permissions, in.Permissions)
		row.NameVi, row.NameEn, row.Description = in.NameVi, in.NameEn, in.Description
		v, err := h.Repo.Update(ctx, row, add, remove, p.UserID)
		if err != nil {
			return err
		}
		row.Version, row.Permissions = v, in.Permissions
		out = *row
		return h.updated(ctx, *row, "updated", add, remove, p.UserID, in.Reason)
	})
	return out, err
}

// Delete soft-deletes a role without valid user roles (AC-17: 409 otherwise).
func (h Roles) Delete(ctx context.Context, in RoleInput) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.DeleteRole")
	defer func() { done(err) }()
	reason := strings.TrimSpace(in.Reason)
	if n := utf8.RuneCountInString(reason); n < 3 || n > 1000 {
		return problem.Invalid("reason", problem.SubInvalid, "3 to 1000 characters")
	}
	ctx, p, tenant, err := h.session(ctx, in.Route)
	if err != nil {
		return err
	}
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		row, err := h.load(ctx, in, tenant)
		if err != nil {
			return err
		}
		n, err := h.Repo.ActiveUserRoles(ctx, row.ID)
		if err != nil {
			return err
		}
		if n > 0 {
			return problem.New(problem.Conflict, "the role is still granted")
		}
		if row.Version != in.IfMatch {
			return problem.New(problem.VersionMismatch, "")
		}
		if err := h.Repo.SoftDelete(ctx, row.ID, row.Version, p.UserID, "deleted"); err != nil {
			return err
		}
		return h.updated(ctx, *row, "deleted", nil, row.Permissions, p.UserID, reason)
	})
}

// load returns the role for a change: system roles are read-only (403); a tenant user only changes the
// custom roles of its tenant (shared roles → 403, other tenants' roles → 404).
func (h Roles) load(ctx context.Context, in RoleInput, tenant *uuid.UUID) (*write.RoleRow, error) {
	row, err := h.Repo.Load(ctx, in.RoleID, tenant)
	if err != nil {
		return nil, err
	}
	if row.IsSystem {
		return nil, problem.New(problem.PermissionDenied, "system roles are read-only")
	}
	if in.Route == RoleRouteTenant && row.TenantID == nil {
		return nil, problem.New(problem.PermissionDenied, "shared roles are managed by the platform")
	}
	return row, nil
}

// updated publishes role.updated (AC-18) and the audit.
func (h Roles) updated(ctx context.Context, r write.RoleRow, op string, added, removed []string, actor uuid.UUID, reason string) error {
	payload := map[string]any{"role_id": r.ID.String(), "code": r.Code, "tenant_id": nil, "role_scope_kind": r.ScopeKind, "op": op,
		"version": r.Version, "permissions_added": nonNilList(added), "permissions_removed": nonNilList(removed), "actor_id": actor.String()}
	if r.TenantID != nil {
		payload["tenant_id"] = r.TenantID.String()
	}
	if err := h.Events.Publish(ctx, write.Event{Type: "role.updated", SubjectType: "role", SubjectID: r.ID.String(), TenantID: r.TenantID,
		Platform: r.TenantID == nil, Payload: payload}); err != nil {
		return err
	}
	return h.Audit.Record(ctx, audit.Event{Action: "role." + op, TargetType: "role", TargetID: r.ID.String(), Result: audit.ResultSuccess,
		Metadata: map[string]any{"code": r.Code, "permissions_added": nonNilList(added), "permissions_removed": nonNilList(removed), "reason": reason}})
}

func uniqueSorted(s []string) []string {
	out := append([]string(nil), s...)
	sort.Strings(out)
	return slices.Compact(out)
}

func diff(old, now []string) (add, remove []string) {
	for _, c := range now {
		if !slices.Contains(old, c) {
			add = append(add, c)
		}
	}
	for _, c := range old {
		if !slices.Contains(now, c) {
			remove = append(remove, c)
		}
	}
	return add, remove
}

func nonNilList(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
