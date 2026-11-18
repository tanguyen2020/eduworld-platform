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

// OrgMembershipReader lists, per tenant, the Organization and the Keycloak subjects that should be members
// (platform read for the reconciliation job, F010-AC-22).
type OrgMembershipReader interface {
	TenantOrgs(ctx context.Context) ([]TenantOrg, error)
	ExpectedMembers(ctx context.Context, tenantID uuid.UUID) (map[string]bool, error)
}

// SchoolFilter selects schools of the scope (the scope decides the visible tenants).
type SchoolFilter struct {
	TenantID     *uuid.UUID // restrict to one tenant (System Admin path); nil = every tenant of the scope
	Status       string
	ProvinceCode string
	Level        string
	Q            string
	After        *page.Cursor
	Limit        int
}

// ClassFilter selects classes of one school.
type ClassFilter struct {
	SchoolID       uuid.UUID
	AcademicYearID *uuid.UUID
	Grade          int
	After          *page.Cursor
	Limit          int // 0 = no paging (listClasses)
}

// OrgReadModel reads the catalog and the organization of the scope (keyset pages return Limit+1 rows).
type OrgReadModel interface {
	Countries(ctx context.Context) ([]dto.Country, error)
	Provinces(ctx context.Context, country, status string) ([]dto.Province, error)
	Schools(ctx context.Context, f SchoolFilter) ([]dto.School, error)
	School(ctx context.Context, id uuid.UUID) (dto.School, error)
	Classes(ctx context.Context, f ClassFilter) ([]dto.Class, error)
	UnassignedStudents(ctx context.Context, schoolID *uuid.UUID, after *page.Cursor, limit int) ([]dto.UnassignedStudent, error)
	StudentSummary(ctx context.Context, studentID uuid.UUID) (dto.StudentSummary, error)
}
