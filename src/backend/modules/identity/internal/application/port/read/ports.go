// Package read holds the read-context ports of module identity (ADR-0087): read models only read, in a
// READ ONLY session (replica allowed), never write, never publish events.
package read

import (
	"context"

	"github.com/google/uuid"

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

// TenantOrg is a tenant the user belongs to and its Keycloak Organization (nil until created).
type TenantOrg struct {
	TenantID uuid.UUID
	OrgID    *uuid.UUID
}

// UserMembership is what Keycloak membership must mirror for one user (F010 US4).
type UserMembership struct {
	KeycloakUserID string // subject of the local-realm identity; "" when the user has no Keycloak account
	Tenants        []TenantOrg
}

// MembershipReader reads a user's memberships across tenants (platform read, bypass platform_job).
type MembershipReader interface {
	MembershipOf(ctx context.Context, userID uuid.UUID) (UserMembership, error)
}
