// Package appscope chooses the data scope of System Admin operations on one target tenant (shared by the
// write and read contexts of module identity).
package appscope

import (
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// TargetTenant returns the scope of a platform operation on target ("a System Admin write always has one
// target tenant_id", AGENTS R20). With X-Tenant-Id the principal is resolved into that tenant: its session
// TenantScope is used (RLS applies) and another target is 404. Without a session tenant the operation uses
// a CrossTenantScope of exactly the target (ADR-0089 §3, audited as cross_tenant.accessed).
func TargetTenant(p authz.Principal, target uuid.UUID) (tenancy.Scope, error) {
	if p.TenantID != nil {
		if *p.TenantID != target {
			return nil, problem.New(problem.NotFound, "")
		}
		sc, err := authz.ScopeFor(p)
		if err != nil {
			return nil, problem.New(problem.PermissionDenied, "").Wrap(err)
		}
		return sc, nil
	}
	sc, err := authz.CrossTenantScope(p, []uuid.UUID{target}, tenancy.ReasonSystemAdminCrossTenant)
	if err != nil {
		return nil, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	return sc, nil
}
