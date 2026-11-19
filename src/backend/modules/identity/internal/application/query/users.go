package query

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// TenantUsers lists the members of the session tenant (listTenantUsers, tenant.user.read).
type TenantUsers struct {
	Model   read.TenantUserReader
	Cursors *page.Codec
}

// TenantUsersInput filters listTenantUsers.
type TenantUsersInput struct {
	ListInput
	Q, UserType, RoleCode string
	SchoolID              *uuid.UUID
}

// UserPage is UserPage of the spec.
type UserPage struct {
	Items []read.UserView `json:"items"`
	Page  page.Info       `json:"page"`
}

// List runs listTenantUsers; a reader limited to one school sees the members of that school only.
func (q TenantUsers) List(ctx context.Context, in TenantUsersInput) (out UserPage, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListTenantUsers")
	defer func() { done(err) }()
	p, err := authenticated(ctx)
	if err != nil {
		return out, err
	}
	if in.UserType != "" && in.UserType != "student" && in.UserType != "staff" {
		return out, problem.Invalid("user_type", problem.SubInvalid, "student or staff")
	}
	schools, all := SchoolsWith(p, authz.PermTenantUserRead)
	if !all && len(schools) > 0 && (in.SchoolID == nil || *in.SchoolID != schools[0]) {
		in.SchoolID = &schools[0]
	}
	if ctx, _, err = tenantRead(ctx, authz.PermTenantUserRead, in.SchoolID); err != nil {
		return out, err
	}
	after, err := Org{Cursors: q.Cursors}.cursor(in.Cursor)
	if err != nil {
		return out, err
	}
	f := read.TenantUserFilter{TenantID: *p.TenantID, Q: in.Q, UserType: in.UserType, SchoolID: in.SchoolID, RoleCode: in.RoleCode,
		Limit: page.Limit(in.Limit)}
	if after != nil {
		f.AfterID = &after.ID
	}
	rows, err := q.Model.TenantUsers(ctx, f)
	if err != nil {
		return out, err
	}
	return UserPage(pageOf(q.Cursors, rows, f.Limit, func(v read.UserView) (string, uuid.UUID) { return "", v.UserID })), nil
}
