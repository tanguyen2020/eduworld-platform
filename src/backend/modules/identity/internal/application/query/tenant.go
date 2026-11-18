// Package query holds the read-context use cases of module identity (ADR-0087): authorize explicitly,
// then read a read model; no write, no event, no UnitOfWork.
package query

import (
	"context"
	"strings"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/tenant"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// ListTenantsInput is the query of listTenants.
type ListTenantsInput struct {
	Q          string
	TenantType string
	Cursor     string
	Limit      int
}

// TenantPage is the response of listTenants (data + page).
type TenantPage struct {
	Data []dto.Tenant `json:"data"`
	Page page.Info    `json:"page"`
}

// ListTenantsHandler lists tenants for System Admin (platform.tenant.manage or platform.cross_tenant.read).
type ListTenantsHandler interface {
	Handle(ctx context.Context, in ListTenantsInput) (TenantPage, error)
}

// ListTenants implements ListTenantsHandler.
type ListTenants struct {
	Model   read.TenantReadModel
	Cursors *page.Codec
}

// Handle runs the query.
func (h ListTenants) Handle(ctx context.Context, in ListTenantsInput) (out TenantPage, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListTenants")
	defer func() { done(err) }()
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return out, problem.New(problem.Unauthenticated, "")
	}
	perm := authz.PermPlatformTenantManage
	if !p.Has(perm) && p.Has(authz.PermPlatformCrossTenantRead) {
		perm = authz.PermPlatformCrossTenantRead
	}
	if err = authz.Require(ctx, p, perm, authz.Resource{}); err != nil {
		return out, err
	}
	if !p.IsPlatform() {
		return out, problem.New(problem.PermissionDenied, "")
	}
	if in.TenantType != "" {
		if _, err := tenant.ParseType(in.TenantType); err != nil {
			return out, problem.Invalid("tenant_type", problem.SubInvalid, "must be school, unit or common")
		}
	}
	after, err := h.Cursors.Decode(in.Cursor)
	if err != nil {
		return out, problem.New(problem.BadRequest, "invalid cursor")
	}
	sc, err := authz.PlatformScope(p)
	if err != nil {
		return out, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	limit := page.Limit(in.Limit)
	rows, err := h.Model.ListTenants(tenancy.WithScope(ctx, sc), read.TenantFilter{Q: strings.TrimSpace(in.Q),
		TenantType: in.TenantType, After: after, Limit: limit})
	if err != nil {
		return out, err
	}
	out.Page = page.Info{Limit: limit}
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[limit-1]
		next := h.Cursors.Encode(page.Cursor{Key: last.Code, ID: last.TenantID})
		out.Page.HasMore, out.Page.NextCursor = true, &next
	}
	out.Data = rows
	if out.Data == nil {
		out.Data = []dto.Tenant{}
	}
	return out, nil
}
