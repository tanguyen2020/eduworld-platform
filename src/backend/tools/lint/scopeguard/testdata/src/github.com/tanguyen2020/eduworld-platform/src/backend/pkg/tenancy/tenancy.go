package tenancy

type TenantScope struct{ id string }
type CrossTenantScope struct{ ids []string }
type PlatformScope struct{ actor string }

func NewTenantScopeForAuthz(id string) TenantScope              { return TenantScope{id: id} }
func NewCrossTenantScopeForAuthz(ids []string) CrossTenantScope { return CrossTenantScope{ids: ids} }
func NewPlatformScopeForAuthz(actor string) PlatformScope       { return PlatformScope{actor: actor} }
