package read

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// TenantUserFilter selects the members of one tenant (students and staff), newest first.
type TenantUserFilter struct {
	TenantID uuid.UUID
	Q        string // username or display name contains
	UserType string
	SchoolID *uuid.UUID
	RoleCode string
	AfterID  *uuid.UUID
	Limit    int
}

// UserView is the User schema.
type UserView struct {
	UserID          uuid.UUID      `json:"user_id"`
	UserType        string         `json:"user_type"`
	PrimaryTenantID *uuid.UUID     `json:"primary_tenant_id"`
	Status          string         `json:"status"`
	Username        string         `json:"username"`
	Email           *string        `json:"email"`
	DisplayName     string         `json:"display_name,omitempty"`
	Locale          string         `json:"locale,omitempty"`
	Source          string         `json:"source"`
	LastLoginAt     *time.Time     `json:"last_login_at,omitempty"`
	TenantIDs       []uuid.UUID    `json:"tenant_ids,omitempty"` // listPlatformUsers with tenant_ids only (F059-1)
	UserRoles       []UserRoleView `json:"user_roles"`
	Version         int64          `json:"version"`
}

// TenantUserReader lists the members of a tenant with their valid roles in it.
type TenantUserReader interface {
	TenantUsers(ctx context.Context, f TenantUserFilter) ([]UserView, error)
}

// PlatformUserFilter selects accounts for System Admin (listPlatformUsers, F059-1). Without TenantIDs it lists the
// platform staff only (primary_tenant_id NULL); with TenantIDs the members of those tenants. There is no
// "every user of every tenant" read (AGENTS R8).
type PlatformUserFilter struct {
	TenantIDs []uuid.UUID
	Q         string // username, email or display name contains (case-insensitive, literal)
	UserType  string
	Status    string
	RoleCode  string // a valid user role of this code (platform roles, or tenant roles of TenantIDs)
	AfterID   *uuid.UUID
	Limit     int
}

// PlatformUserReader lists accounts for System Admin; UserView.TenantIDs holds the memberships within
// TenantIDs only, UserRoles is empty (listAllUserRoles?user_id= gives them).
type PlatformUserReader interface {
	PlatformUsers(ctx context.Context, f PlatformUserFilter) ([]UserView, error)
}
