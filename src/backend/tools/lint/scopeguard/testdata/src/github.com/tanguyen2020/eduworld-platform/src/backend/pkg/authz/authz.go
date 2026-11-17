package authz

import "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"

func ScopeFor(id string) tenancy.TenantScope {
	if id == "" {
		return tenancy.TenantScope{}
	}
	return tenancy.NewTenantScopeForAuthz(id)
}
