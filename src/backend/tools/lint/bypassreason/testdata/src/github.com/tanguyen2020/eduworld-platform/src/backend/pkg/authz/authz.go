package authz

import "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"

type Principal struct{}

func CrossTenantScope(p Principal, ids []string, reason tenancy.BypassReason) (tenancy.CrossTenantScope, error) {
	return tenancy.CrossTenantScope{}, nil
}
