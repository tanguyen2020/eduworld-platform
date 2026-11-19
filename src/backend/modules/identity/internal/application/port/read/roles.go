package read

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// LocalizedName is {vi, en}.
type LocalizedName struct {
	Vi string  `json:"vi"`
	En *string `json:"en,omitempty"`
}

// PermissionView is the Permission schema.
type PermissionView struct {
	Code      string        `json:"code"`
	ScopeKind string        `json:"scope_kind"`
	GroupCode string        `json:"group_code"`
	Name      LocalizedName `json:"name"`
}

// RoleView is the Role schema.
type RoleView struct {
	RoleID          uuid.UUID     `json:"role_id"`
	Code            string        `json:"code"`
	Name            LocalizedName `json:"name"`
	Description     *string       `json:"description,omitempty"`
	ScopeKind       string        `json:"role_scope_kind"`
	IsSystem        bool          `json:"is_system"`
	TenantID        *uuid.UUID    `json:"tenant_id"`
	PermissionCodes []string      `json:"permission_codes"`
	ActiveUserRoles int           `json:"active_user_role_count"`
	Version         int64         `json:"version"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

// RoleFilter selects roles: shared roles (tenant_id NULL, optionally of one kind) plus the custom roles of
// TenantIDs; ActiveIn counts the valid user roles inside one tenant (nil: every grant).
type RoleFilter struct {
	TenantIDs []uuid.UUID
	AnyTenant bool // every tenant (platform staff reading one role)
	ScopeKind string
	ActiveIn  *uuid.UUID
	AfterID   *uuid.UUID
	Limit     int
}

// RoleReader reads roles and the permission catalog copy.
type RoleReader interface {
	Permissions(ctx context.Context, tenantOnly bool) (items []PermissionView, catalogVersion string, err error)
	Roles(ctx context.Context, f RoleFilter) ([]RoleView, error)
	// Role returns one live role visible with f (problem NotFound otherwise).
	Role(ctx context.Context, id uuid.UUID, f RoleFilter) (RoleView, error)
}
