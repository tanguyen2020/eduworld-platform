package write

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// RoleInfo is a role visible in the scope (shared role, or custom role of the scope tenant) with its live
// permissions.
type RoleInfo struct {
	ID          uuid.UUID
	TenantID    *uuid.UUID
	Code        string
	ScopeKind   string // tenant | platform
	IsSystem    bool
	Permissions []string
}

// GrantTarget is the user a role is granted to.
type GrantTarget struct {
	UserType        string
	PrimaryTenantID *uuid.UUID
}

// UserRoleRow is an identity.user_roles row.
type UserRoleRow struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	RoleID      uuid.UUID
	RoleCode    string
	TenantID    *uuid.UUID
	ScopeType   string
	ScopeID     *uuid.UUID
	SchoolID    *uuid.UUID
	ValidFrom   time.Time
	ValidTo     *time.Time
	Source      string
	GrantedBy   uuid.UUID
	Training    *time.Time
	CreatedAt   time.Time
	SubjectCode *string
}

// UserRoleRepository persists user roles (F012 US5). Role and user reads see shared roles (tenant_id NULL)
// and the custom roles of the scope tenant only.
type UserRoleRepository interface {
	// Role returns a role visible to tenant (nil: platform); nil when unknown.
	Role(ctx context.Context, roleID uuid.UUID, tenant *uuid.UUID) (*RoleInfo, error)
	// Target returns the user; nil when unknown.
	Target(ctx context.Context, userID uuid.UUID) (*GrantTarget, error)
	// SchoolInTenant reports whether a live school belongs to the tenant.
	SchoolInTenant(ctx context.Context, schoolID, tenantID uuid.UUID) (bool, error)
	// Insert stores the grant; a trigger violation is problem 422, an identical active grant problem 409.
	Insert(ctx context.Context, r *UserRoleRow) error
	// Get returns a live user role of the scope (problem NotFound otherwise).
	Get(ctx context.Context, id uuid.UUID) (*UserRoleRow, error)
	// Revoke soft-deletes the user role.
	Revoke(ctx context.Context, id, by uuid.UUID, reason string) error
	// BumpUserVersion increments users.version (the Principal version, F012-AC-11).
	BumpUserVersion(ctx context.Context, userID uuid.UUID) error
}

// RoleRow is an identity.roles row with its live permissions.
type RoleRow struct {
	ID          uuid.UUID
	TenantID    *uuid.UUID
	Code        string
	NameVi      string
	NameEn      *string
	Description *string
	ScopeKind   string
	IsSystem    bool
	Permissions []string
	Version     int64
	UpdatedAt   time.Time
}

// RoleRepository persists roles and their permissions (F012 US4).
type RoleRepository interface {
	// PermissionScopes returns the scope kind of each known live permission code (unknown codes are absent).
	PermissionScopes(ctx context.Context, codes []string) (map[string]string, error)
	// Load returns a live role that is shared or belongs to tenant (nil tenant: any); problem NotFound otherwise.
	Load(ctx context.Context, id uuid.UUID, tenant *uuid.UUID) (*RoleRow, error)
	// Create inserts the role and its permissions (duplicate code: problem Conflict).
	Create(ctx context.Context, r *RoleRow, actor uuid.UUID) error
	// Update stores name/description and the permission changes when the version matches (problem
	// VersionMismatch otherwise) and returns the new version.
	Update(ctx context.Context, r *RoleRow, add, remove []string, actor uuid.UUID) (int64, error)
	// ActiveUserRoles counts valid, live user roles of the role.
	ActiveUserRoles(ctx context.Context, id uuid.UUID) (int, error)
	// SoftDelete deletes the role when the version matches.
	SoftDelete(ctx context.Context, id uuid.UUID, version int64, actor uuid.UUID, reason string) error
}
