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
	UserRoles       []UserRoleView `json:"user_roles"`
	Version         int64          `json:"version"`
}

// TenantUserReader lists the members of a tenant with their valid roles in it.
type TenantUserReader interface {
	TenantUsers(ctx context.Context, f TenantUserFilter) ([]UserView, error)
}
