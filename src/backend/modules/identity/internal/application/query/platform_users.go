package query

import (
	"context"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/platformadmin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// PlatformUsers is listPlatformUsers (F059-1, owner F012): System Admin with platform.user.manage.
type PlatformUsers struct {
	Model   read.PlatformUserReader
	Cursors *page.Codec
}

// PlatformUsersInput filters listPlatformUsers; TenantFilter reports tenant_ids in the request.
type PlatformUsersInput struct {
	ListInput
	Q, UserType, Status, RoleCode string
	TenantFilter                  bool
}

var userStatuses = map[string]bool{"": true, "pending": true, "active": true, "locked": true, "deactivated": true}

// List runs listPlatformUsers. Without tenant_ids it lists the platform staff only; with tenant_ids the
// CrossTenantScope set by httpx (platform.cross_tenant.read + MFA) lists the members of those tenants and the
// read is audited as cross_tenant.accessed. A tenant session never reaches it (403).
func (q PlatformUsers) List(ctx context.Context, in PlatformUsersInput) (out UserPage, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListPlatformUsers")
	defer func() { done(err) }()
	p, err := authenticated(ctx)
	if err != nil {
		return out, err
	}
	if p.TenantID != nil {
		return out, problem.New(problem.PermissionDenied, "")
	}
	if err := authz.Require(ctx, p, authz.PermPlatformUserManage, authz.Resource{}); err != nil {
		return out, err
	}
	pe := problem.New(problem.ValidationFailed, "")
	if n := utf8.RuneCountInString(in.Q); in.Q != "" && (n < 2 || n > 100) {
		pe.AddField("q", problem.SubInvalid, "2 to 100 characters")
	}
	if in.UserType != "" && in.UserType != "student" && in.UserType != "staff" {
		pe.AddField("user_type", problem.SubInvalid, "student or staff")
	}
	if !userStatuses[in.Status] {
		pe.AddField("status", problem.SubInvalid, "pending, active, locked or deactivated")
	}
	if len(in.RoleCode) > 64 {
		pe.AddField("role_code", problem.SubMaxLength, "at most 64 characters")
	}
	if len(pe.Fields) > 0 {
		return out, pe
	}
	f := read.PlatformUserFilter{Q: in.Q, UserType: in.UserType, Status: in.Status, RoleCode: in.RoleCode, Limit: page.Limit(in.Limit)}
	if sc, ok := tenancy.ScopeFrom(ctx); ok {
		if cs, cross := sc.(tenancy.CrossTenantScope); cross {
			f.TenantIDs = cs.TenantIDs()
			if err := tenancy.RecordCrossTenantScope(tenancy.WithModule(ctx, platformadmin.ModuleIdentity), cs, tenancy.ModeRead); err != nil {
				return out, err
			}
		}
	}
	if in.TenantFilter && len(f.TenantIDs) == 0 {
		// tenant_ids without platform.cross_tenant.read: never fall back to another listing.
		authz.RecordDenied(ctx, string(problem.PermissionDenied), map[string]any{"why": "cross_tenant_scope"})
		return out, problem.New(problem.PermissionDenied, "")
	}
	after, err := Org{Cursors: q.Cursors}.cursor(in.Cursor)
	if err != nil {
		return out, err
	}
	if after != nil {
		f.AfterID = &after.ID
	}
	rows, err := q.Model.PlatformUsers(ctx, f)
	if err != nil {
		return out, err
	}
	return UserPage(pageOf(q.Cursors, rows, f.Limit, func(v read.UserView) (string, uuid.UUID) { return "", v.UserID })), nil
}
