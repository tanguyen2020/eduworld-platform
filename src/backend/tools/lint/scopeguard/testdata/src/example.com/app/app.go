package app

import (
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

func uses() {
	_ = authz.ScopeFor("t1")
	_ = tenancy.NewTenantScopeForAuthz("t1")      // want `tenancy.NewTenantScopeForAuthz is reserved for package authz`
	_ = tenancy.NewCrossTenantScopeForAuthz(nil)  // want `tenancy.NewCrossTenantScopeForAuthz is reserved for package authz`
	_ = tenancy.NewPlatformScopeForAuthz("job:x") // want `tenancy.NewPlatformScopeForAuthz is reserved for package authz`
	_ = tenancy.TenantScope{}                     // want `composite literal of tenancy.TenantScope`
	_ = &tenancy.PlatformScope{}                  // want `composite literal of tenancy.PlatformScope`
	var s tenancy.TenantScope                     // zero value via var is allowed (invalid scope, rejected at runtime)
	_ = s
	f := tenancy.NewTenantScopeForAuthz // want `tenancy.NewTenantScopeForAuthz is reserved for package authz`
	_ = f
}
