package write

import (
	"context"

	"github.com/google/uuid"
)

// NewStaff is a staff account created by hand in a tenant (F012 US6) or on the platform (F059-1).
type NewStaff struct {
	UserID      uuid.UUID
	Username    string // normalized
	Email       string // normalized (lower-case)
	DisplayName string
	Locale      string
	TenantID    *uuid.UUID // primary_tenant_id: the session tenant; nil = platform staff (ADR-0081)
	Issuer      string
	Subject     string // Keycloak user id
}

// StaffRow is a staff account of a tenant.
type StaffRow struct {
	UserID          uuid.UUID
	Username        string
	Email           *string
	DisplayName     string
	Locale          *string
	Status          string
	PrimaryTenantID *uuid.UUID
	KeycloakID      string // subject of the local identity ("" when none)
	Version         int64
}

// StaffRepository persists staff accounts.
type StaffRepository interface {
	// UsernameTaken / EmailTaken check every account, soft-deleted ones included (ADR-0069).
	UsernameTaken(ctx context.Context, username string) (bool, error)
	EmailTaken(ctx context.Context, email string, except *uuid.UUID) (bool, error)
	// CreateStaff inserts the user (active) and its local identity.
	CreateStaff(ctx context.Context, s NewStaff) error
	// Staff returns a live staff account whose home tenant is tenantID (problem NotFound otherwise).
	Staff(ctx context.Context, userID, tenantID uuid.UUID, issuer string) (*StaffRow, error)
	// PlatformStaff returns a live platform staff account (primary_tenant_id NULL; problem NotFound otherwise).
	PlatformStaff(ctx context.Context, userID uuid.UUID, issuer string) (*StaffRow, error)
	// UpdateStaff stores display name, locale, email and status when the version matches.
	UpdateStaff(ctx context.Context, s *StaffRow) error
}
