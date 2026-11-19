package write

import (
	"context"

	"github.com/google/uuid"
)

// SeedAccount is an account found by username (platform staff provisioning and the dev seed).
type SeedAccount struct {
	UserID          uuid.UUID
	UserType        string
	Status          string
	PrimaryTenantID *uuid.UUID
	Subject         *string // live external identity of the requested issuer
}

// NewPlatformStaff is a platform staff account (user_type staff, no home tenant, ADR-0081) linked to an existing
// subject of the local realm.
type NewPlatformStaff struct {
	UserID      uuid.UUID
	Username    string // normalized
	Email       string // normalized (lower-case)
	DisplayName string
	Issuer      string
	Subject     string // Keycloak user id
}

// RoleGrantKey identifies an active grant: user, role, tenant (nil = platform), scope type and scope id.
type RoleGrantKey struct {
	UserID    uuid.UUID
	RoleID    uuid.UUID
	TenantID  *uuid.UUID
	ScopeType string
	ScopeID   *uuid.UUID
}

// PlatformStaffRepository is the persistence of command.PlatformStaff.
type PlatformStaffRepository interface {
	// AccountByUsername returns the live account with this username and its subject for issuer; nil when absent.
	AccountByUsername(ctx context.Context, username, issuer string) (*SeedAccount, error)
	// SystemAdminExists reports whether a live, valid system_admin grant exists.
	SystemAdminExists(ctx context.Context) (bool, error)
	// CreatePlatformStaff inserts the account and its external identity.
	CreatePlatformStaff(ctx context.Context, s NewPlatformStaff) error
	// HasGrant reports whether the grant is active (not revoked, not expired).
	HasGrant(ctx context.Context, k RoleGrantKey) (bool, error)
}

// SeedLookupRepository answers the idempotency questions of the dev seed (tenant, school, csv connection by
// their natural keys).
type SeedLookupRepository interface {
	// TenantByCode returns the id of the live tenant with the code; nil when absent.
	TenantByCode(ctx context.Context, code string) (*uuid.UUID, error)
	// SchoolByCode returns the id of the live school of the tenant with the code; nil when absent.
	SchoolByCode(ctx context.Context, tenantID uuid.UUID, code string) (*uuid.UUID, error)
	// CsvConnection returns the id of the live csv_import connection of the tenant; nil when absent.
	CsvConnection(ctx context.Context, tenantID uuid.UUID) (*uuid.UUID, error)
}

// SeedSchoolsRepository lists the schools of a connection in the scope of its tenant (no System Admin bypass, so a
// second run of the seed reads without writing cross_tenant.accessed).
type SeedSchoolsRepository interface {
	Schools(ctx context.Context, connectionID uuid.UUID) ([]SisConnectionSchool, error)
}
