// Package scope chooses the data scope of the content use cases of module learning (MULTITENANCY §4.1 rules
// 5–6, ADR-0089 §3), shared by the write and read contexts. Handlers call it explicitly after authz.Require;
// scopes are only built by package authz.
package scope

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// ForRead returns the scope of a list or read of content:
//   - a tenant user reads its session tenant; tenant_ids sent by it is ignored (F020-AC-11);
//   - System Admin reads the requested tenants, or every active tenant of ref.tenants when none is requested,
//     through a CrossTenantScope (audited cross_tenant.accessed, mode read, when the session uses it,
//     F020-AC-13);
//   - nobody else has a content read scope (403).
func ForRead(ctx context.Context, p authz.Principal, requested []uuid.UUID, tenants port.TenantDirectory) (tenancy.Scope, error) {
	if !p.Valid() {
		return nil, problem.New(problem.Unauthenticated, "")
	}
	if p.TenantID != nil {
		sc, err := authz.ScopeFor(p)
		if err != nil {
			return nil, problem.New(problem.PermissionDenied, "").Wrap(err)
		}
		return sc, nil
	}
	if !p.IsPlatform() {
		return nil, problem.New(problem.PermissionDenied, "")
	}
	ids := requested
	if len(ids) == 0 {
		all, err := tenants.ActiveTenants(ctx, nil)
		if err != nil {
			return nil, err
		}
		if len(all) == 0 {
			return nil, problem.New(problem.NotFound, "no active tenant")
		}
		ids = all
	}
	sc, err := authz.CrossTenantScope(p, ids, tenancy.ReasonSystemAdminCrossTenant)
	if err != nil {
		authz.RecordDenied(ctx, string(problem.PermissionDenied), map[string]any{"why": "cross_tenant_scope"})
		return nil, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	return sc, nil
}

// ForWrite returns the scope of a content write with the given owner scope (AGENTS R20: a System Admin write
// always has one target tenant or the platform scope):
//   - a tenant user writes only tenant content of its session tenant (platform content → 403);
//   - platform staff writes platform content under PlatformScope (learning_platform, tenant_id NULL rows,
//     F020-AC-32) and tenant content only with a target tenant (missing → 422, F020-AC-12) through a
//     CrossTenantScope of exactly that tenant.
func ForWrite(p authz.Principal, owner contentmodel.OwnerScope, target *uuid.UUID) (tenancy.Scope, error) {
	if !p.Valid() {
		return nil, problem.New(problem.Unauthenticated, "")
	}
	if _, err := contentmodel.ParseOwnerScope(string(owner)); err != nil {
		return nil, problem.Invalid("owner_scope", problem.SubInvalid, "must be platform or tenant")
	}
	if p.TenantID != nil {
		if owner == contentmodel.OwnerPlatform || (target != nil && *target != *p.TenantID) {
			return nil, problem.New(problem.PermissionDenied, "")
		}
		sc, err := authz.ScopeFor(p)
		if err != nil {
			return nil, problem.New(problem.PermissionDenied, "").Wrap(err)
		}
		return sc, nil
	}
	if !p.IsPlatform() {
		return nil, problem.New(problem.PermissionDenied, "")
	}
	if owner == contentmodel.OwnerPlatform {
		if target != nil {
			return nil, problem.Invalid("tenant_id", problem.SubInvalid, "must be empty for platform content")
		}
		sc, err := authz.PlatformScope(p)
		if err != nil {
			return nil, problem.New(problem.PermissionDenied, "").Wrap(err)
		}
		return sc, nil
	}
	if target == nil || *target == uuid.Nil {
		return nil, problem.Invalid("tenant_id", problem.SubRequired, "a target tenant is required for tenant content")
	}
	sc, err := authz.CrossTenantScope(p, []uuid.UUID{*target}, tenancy.ReasonSystemAdminCrossTenant)
	if err != nil {
		return nil, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	return sc, nil
}
