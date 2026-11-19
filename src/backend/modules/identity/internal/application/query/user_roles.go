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

// UserRoles lists user roles (listTenantUserRoles, listSchoolUserRoles, listAllUserRoles).
type UserRoles struct {
	Model   read.UserRoleReader
	Cursors *page.Codec
}

// UserRolesInput filters a list.
type UserRolesInput struct {
	ListInput
	UserID, RoleID, ScopeID *uuid.UUID
	ScopeType               string
}

// UserRolePage is UserRolePage of the spec.
type UserRolePage struct {
	Items []read.UserRoleView `json:"items"`
	Page  page.Info           `json:"page"`
}

func (q UserRoles) list(ctx context.Context, f read.UserRoleFilter, in UserRolesInput) (out UserRolePage, err error) {
	after, err := Org{Cursors: q.Cursors}.cursor(in.Cursor)
	if err != nil {
		return out, err
	}
	if after != nil {
		f.AfterID = &after.ID
	}
	f.UserID, f.RoleID, f.ScopeID, f.ScopeType = in.UserID, in.RoleID, in.ScopeID, in.ScopeType
	f.Limit = page.Limit(in.Limit)
	rows, err := q.Model.UserRoles(ctx, f)
	if err != nil {
		return out, err
	}
	pg := pageOf(q.Cursors, rows, f.Limit, func(v read.UserRoleView) (string, uuid.UUID) { return "", v.UserRoleID })
	return UserRolePage(pg), nil
}

// ListTenant is listTenantUserRoles (tenant.user.read; a school-scoped reader sees its schools only).
func (q UserRoles) ListTenant(ctx context.Context, in UserRolesInput) (out UserRolePage, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListTenantUserRoles")
	defer func() { done(err) }()
	p, err := authenticated(ctx)
	if err != nil {
		return out, err
	}
	schools, all := SchoolsWith(p, authz.PermTenantUserRead)
	var school *uuid.UUID
	if !all && len(schools) > 0 {
		school = &schools[0]
	}
	if ctx, _, err = tenantRead(ctx, authz.PermTenantUserRead, school); err != nil {
		return out, err
	}
	f := read.UserRoleFilter{TenantIDs: []uuid.UUID{*p.TenantID}}
	if !all {
		f.SchoolIDs = append([]uuid.UUID{}, schools...)
	}
	return q.list(ctx, f, in)
}

// ListSchool is listSchoolUserRoles (tenant.user_role.write on the school).
func (q UserRoles) ListSchool(ctx context.Context, school uuid.UUID, in UserRolesInput) (out UserRolePage, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListSchoolUserRoles")
	defer func() { done(err) }()
	ctx, p, err := tenantRead(ctx, authz.PermTenantUserRoleWrite, &school)
	if err != nil {
		return out, err
	}
	return q.list(ctx, read.UserRoleFilter{TenantIDs: []uuid.UUID{*p.TenantID}, SchoolIDs: []uuid.UUID{school}}, in)
}

// ListAll is listAllUserRoles: platform staff with platform.user_role.write; tenant_ids (CrossTenantScope,
// audited) lists tenant user roles, without tenant_ids the platform user roles.
func (q UserRoles) ListAll(ctx context.Context, in UserRolesInput) (out UserRolePage, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListAllUserRoles")
	defer func() { done(err) }()
	p, err := authenticated(ctx)
	if err != nil {
		return out, err
	}
	if err := authz.Require(ctx, p, authz.PermPlatformUserRoleWrite, authz.Resource{}); err != nil {
		return out, err
	}
	sc, ok := tenancy.ScopeFrom(ctx)
	if !ok {
		return out, problem.New(problem.PermissionDenied, "")
	}
	if cs, cross := sc.(tenancy.CrossTenantScope); cross {
		return q.list(ctx, read.UserRoleFilter{TenantIDs: cs.TenantIDs()}, in)
	}
	return q.list(ctx, read.UserRoleFilter{Platform: true}, in)
}
