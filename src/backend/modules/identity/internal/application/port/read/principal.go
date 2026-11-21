package read

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// PrincipalUser is the identity part of a Principal.
type PrincipalUser struct {
	UserID          uuid.UUID
	UserType        string // student | staff
	Status          string // pending | active | locked | deactivated | anonymized
	PrimaryTenantID *uuid.UUID
	Version         int64
}

// PrincipalGrantRow is one valid UserRole with its role and permissions.
type PrincipalGrantRow struct {
	RoleCode      string
	RoleScopeKind string // tenant | platform
	ScopeType     string // platform | tenant | school | subject
	ScopeID       *uuid.UUID
	SchoolID      *uuid.UUID
	Permissions   []string
}

// PrincipalMembership is the membership of a user in one tenant.
type PrincipalMembership struct {
	TenantType string
	SchoolIDs  []uuid.UUID // the school of an effective student (org.students — ADR-0095)
	GradeLevel *int
	// SIS source of the student (ADR-0046).
	SisConnectionID   *uuid.UUID
	StudentExternalID *string
}

// PrincipalReadModel reads what resolvePrincipal needs (F012-AC-11). Every method filters explicitly by
// user, tenant (or tenant_id IS NULL for platform rows), validity and deleted_at.
type PrincipalReadModel interface {
	// UserBySubject finds the user of an external identity (issuer, subject); nil when unknown.
	UserBySubject(ctx context.Context, issuer, subject string) (*PrincipalUser, error)
	// UserByID returns the user; nil when unknown.
	UserByID(ctx context.Context, userID uuid.UUID) (*PrincipalUser, error)
	// Membership returns the membership of the user in the tenant; nil when not a member.
	Membership(ctx context.Context, userID, tenantID uuid.UUID) (*PrincipalMembership, error)
	// Grants returns the valid user roles of the user in the tenant (tenantID nil: platform user roles).
	Grants(ctx context.Context, userID uuid.UUID, tenantID *uuid.UUID) ([]PrincipalGrantRow, error)
	// ActiveTenantType returns the type of an active tenant; nil when it does not exist or is disabled.
	ActiveTenantType(ctx context.Context, tenantID uuid.UUID) (*string, error)
}

// MemberRow is the result of checkMemberships for one user.
type MemberRow struct {
	UserID     uuid.UUID
	IsMember   bool
	UserType   string
	Status     string
	GradeLevel *int
}

// MembershipChecker checks many users against a tenant, optionally a school (checkMemberships).
type MembershipChecker interface {
	CheckMembers(ctx context.Context, tenantID uuid.UUID, userIDs []uuid.UUID, schoolID *uuid.UUID) ([]MemberRow, error)
}

// MyTenantRow is one tenant of the session user (listMyTenants).
type MyTenantRow struct {
	TenantID   uuid.UUID
	Code       string
	Name       string
	TenantType string
	Status     string
	RoleCodes  []string
}

// MeReadModel reads the session user's own data across tenants (the login path, ADR-0067).
type MeReadModel interface {
	// MyTenants returns the active tenants where the user is a member, with its valid tenant role codes,
	// and whether it holds a valid platform role.
	MyTenants(ctx context.Context, userID uuid.UUID) (rows []MyTenantRow, hasPlatformRole bool, err error)
	// DisplayName returns the display name of a live user ("" when unknown).
	DisplayName(ctx context.Context, userID uuid.UUID) (string, error)
}

// UserRoleFilter filters user roles (newest first, user_role_id < After).
type UserRoleFilter struct {
	TenantIDs []uuid.UUID // required: the tenants of the scope (nil with Platform)
	Platform  bool        // platform user roles (tenant_id NULL) instead
	SchoolIDs []uuid.UUID // nil = every school
	UserID    *uuid.UUID
	RoleID    *uuid.UUID
	ScopeType string
	ScopeID   *uuid.UUID
	AfterID   *uuid.UUID
	Limit     int
}

// UserRoleView is the UserRole schema.
type UserRoleView struct {
	UserRoleID uuid.UUID  `json:"user_role_id"`
	UserID     uuid.UUID  `json:"user_id"`
	RoleID     uuid.UUID  `json:"role_id"`
	RoleCode   string     `json:"role_code"`
	TenantID   *uuid.UUID `json:"tenant_id"`
	ScopeType  string     `json:"scope_type"`
	ScopeID    *uuid.UUID `json:"scope_id"`
	ValidFrom  time.Time  `json:"valid_from"`
	ValidTo    *time.Time `json:"valid_to"`
	Source     string     `json:"source"`
	GrantedBy  *uuid.UUID `json:"granted_by"`
	CreatedAt  time.Time  `json:"created_at"`
}

// UserRoleReader lists live user roles.
type UserRoleReader interface {
	UserRoles(ctx context.Context, f UserRoleFilter) ([]UserRoleView, error)
}
