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
	// ActiveSchools limits the counted grants to these schools (reader with a grant limited to schools).
	ActiveSchools []uuid.UUID
	AfterID       *uuid.UUID
	Limit         int
}

// PermissionKey is the keyset position of the permission catalog (the last row of the previous page).
type PermissionKey struct {
	Group string
	Code  string
}

// RoleReader reads roles and the permission catalog copy.
type RoleReader interface {
	// Permissions is the live catalog copy ordered by (group_code, permission_code): one keyset page after after (nil =
	// first page) of limit rows, or the whole catalog when limit ≤ 0 (the tenant role editor, a bounded code catalog).
	Permissions(ctx context.Context, tenantOnly bool, after *PermissionKey, limit int) (items []PermissionView, catalogVersion string, err error)
	Roles(ctx context.Context, f RoleFilter) ([]RoleView, error)
	// Role returns one live role visible with f (problem NotFound otherwise).
	Role(ctx context.Context, id uuid.UUID, f RoleFilter) (RoleView, error)
}
