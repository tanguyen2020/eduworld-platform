// Package read holds the read-context ports of module identity (ADR-0087): read models only read, in a
// READ ONLY session (replica allowed), never write, never publish events.
package read

import (
	"context"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
)

// TenantFilter selects tenants for listTenants.
type TenantFilter struct {
	Q          string // code or name contains (case-insensitive)
	TenantType string
	After      *page.Cursor
	Limit      int
}

// TenantReadModel lists tenants (platform.tenants, G) ordered by code then id.
type TenantReadModel interface {
	// ListTenants returns up to f.Limit+1 rows so the caller knows whether more exist.
	ListTenants(ctx context.Context, f TenantFilter) ([]dto.Tenant, error)
}
