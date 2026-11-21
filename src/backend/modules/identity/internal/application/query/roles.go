package query

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Roles reads roles and the permission catalog (F012 US4).
type Roles struct {
	Model   read.RoleReader
	Cursors *page.Codec
}

// PermissionList is PermissionList of the spec.
type PermissionList struct {
	Items          []read.PermissionView `json:"items"`
	CatalogVersion string                `json:"catalog_version"`
}

// RolePage is RolePage of the spec.
type RolePage struct {
	Items []read.RoleView `json:"items"`
	Page  page.Info       `json:"page"`
}

// tenantRoleRead authorizes tenant.role.read (or tenant.role.write) in the session tenant. Roles have no school: a
// grant limited to schools (School Admin, F012 Clarifications 2026-11-22) is checked on one of its schools and the
// returned schools limit the counted grants to them (nil: every grant of the tenant).
func tenantRoleRead(ctx context.Context) (context.Context, authz.Principal, []uuid.UUID, error) {
	p, err := authenticated(ctx)
	if err != nil {
		return ctx, p, nil, err
	}
	perm := authz.PermTenantRoleRead
	if !p.Has(perm) {
		perm = authz.PermTenantRoleWrite
	}
	schools, all := SchoolsWith(p, perm)
	var school *uuid.UUID
	if all || len(schools) == 0 {
		schools = nil
	} else {
		school = &schools[0]
	}
	ctx, p, err = tenantRead(ctx, perm, school)
	return ctx, p, schools, err
}

func platformRoleRead(ctx context.Context) (authz.Principal, error) {
	p, err := authenticated(ctx)
	if err != nil {
		return p, err
	}
	return p, authz.Require(ctx, p, authz.PermPlatformRoleRead, authz.Resource{})
}

// ListTenantPermissions is listTenantPermissions: the tenant permissions of the catalog (never platform).
func (q Roles) ListTenantPermissions(ctx context.Context) (out PermissionList, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListTenantPermissions")
	defer func() { done(err) }()
	if ctx, _, _, err = tenantRoleRead(ctx); err != nil {
		return out, err
	}
	items, v, err := q.Model.Permissions(ctx, true)
	return PermissionList{Items: nonNilPerms(items), CatalogVersion: v}, err
}

// ListAllPermissions is listAllPermissions (platform.role.read).
func (q Roles) ListAllPermissions(ctx context.Context) (out PermissionList, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListAllPermissions")
	defer func() { done(err) }()
	if _, err = platformRoleRead(ctx); err != nil {
		return out, err
	}
	items, v, err := q.Model.Permissions(ctx, false)
	return PermissionList{Items: nonNilPerms(items), CatalogVersion: v}, err
}

func (q Roles) page(ctx context.Context, f read.RoleFilter, in ListInput) (out RolePage, err error) {
	after, err := Org{Cursors: q.Cursors}.cursor(in.Cursor)
	if err != nil {
		return out, err
	}
	if after != nil {
		f.AfterID = &after.ID
	}
	f.Limit = page.Limit(in.Limit)
	rows, err := q.Model.Roles(ctx, f)
	if err != nil {
		return out, err
	}
	pg := pageOf(q.Cursors, rows, f.Limit, func(v read.RoleView) (string, uuid.UUID) { return "", v.RoleID })
	return RolePage(pg), nil
}

// ListTenantRoles is listTenantRoles: shared tenant roles (system roles read-only) and the custom roles of
// the session tenant, with the valid grants counted in the tenant.
func (q Roles) ListTenantRoles(ctx context.Context, in ListInput) (out RolePage, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListTenantRoles")
	defer func() { done(err) }()
	ctx, p, schools, err := tenantRoleRead(ctx)
	if err != nil {
		return out, err
	}
	return q.page(ctx, read.RoleFilter{TenantIDs: []uuid.UUID{*p.TenantID}, ScopeKind: "tenant", ActiveIn: p.TenantID, ActiveSchools: schools}, in)
}

// GetTenantRole is getTenantRole (a platform role or another tenant's role is 404).
func (q Roles) GetTenantRole(ctx context.Context, id uuid.UUID) (out read.RoleView, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.GetTenantRole")
	defer func() { done(err) }()
	ctx, p, schools, err := tenantRoleRead(ctx)
	if err != nil {
		return out, err
	}
	return q.Model.Role(ctx, id, read.RoleFilter{TenantIDs: []uuid.UUID{*p.TenantID}, ScopeKind: "tenant", ActiveIn: p.TenantID, ActiveSchools: schools})
}

// ListPlatformRoles is listPlatformRoles: shared roles, plus the custom roles of tenant_ids
// (CrossTenantScope, audited).
func (q Roles) ListPlatformRoles(ctx context.Context, scopeKind string, in ListInput) (out RolePage, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListPlatformRoles")
	defer func() { done(err) }()
	if _, err = platformRoleRead(ctx); err != nil {
		return out, err
	}
	if scopeKind != "" && scopeKind != "tenant" && scopeKind != "platform" {
		return out, problem.Invalid("role_scope_kind", problem.SubInvalid, "tenant or platform")
	}
	f := read.RoleFilter{ScopeKind: scopeKind}
	if sc, ok := tenancy.ScopeFrom(ctx); ok {
		if cs, cross := sc.(tenancy.CrossTenantScope); cross {
			f.TenantIDs = cs.TenantIDs()
		}
	}
	return q.page(ctx, f, in)
}

// GetPlatformRole is getPlatformRole (any live role).
func (q Roles) GetPlatformRole(ctx context.Context, id uuid.UUID) (out read.RoleView, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.GetPlatformRole")
	defer func() { done(err) }()
	if _, err = platformRoleRead(ctx); err != nil {
		return out, err
	}
	return q.Model.Role(ctx, id, read.RoleFilter{AnyTenant: true})
}

func nonNilPerms(p []read.PermissionView) []read.PermissionView {
	if p == nil {
		return []read.PermissionView{}
	}
	return p
}
