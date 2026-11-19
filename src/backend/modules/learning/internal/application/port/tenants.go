package port

import (
	"context"

	"github.com/google/uuid"
)

// TenantDirectory reads the tenant projection ref.tenants (DATABASE_DESIGN §2.17): the module checks that a
// tenant exists and is active in its own projection, never by calling module identity (MULTITENANCY §4.1
// rule 7, AGENTS R30).
type TenantDirectory interface {
	// ActiveTenants returns the ids of ids that are active tenants, in the order of ids; with ids empty it
	// returns every active tenant ("all tenants" of System Admin, MULTITENANCY §4.1 rule 6).
	ActiveTenants(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, error)
}
