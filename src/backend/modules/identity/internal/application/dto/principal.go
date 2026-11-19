package dto

import "github.com/google/uuid"

// PrincipalGrant is one valid UserRole: its role, scope and permissions (F012-AC-11).
type PrincipalGrant struct {
	RoleCode      string
	RoleScopeKind string
	SchoolID      *uuid.UUID
	SubjectID     *uuid.UUID
	Permissions   []string
}

// PrincipalScope is a scope the user holds a role at.
type PrincipalScope struct {
	Type string // tenant | school | subject | platform
	ID   *uuid.UUID
}

// Principal is the authorization context of a user in a tenant (nil tenant: platform scope).
type Principal struct {
	UserID            uuid.UUID
	UserType          string
	Status            string
	TenantID          *uuid.UUID
	TenantType        *string
	Roles             []string
	Permissions       []string
	Scopes            []PrincipalScope
	Grants            []PrincipalGrant
	IsSystemAdmin     bool
	SchoolIDs         []uuid.UUID
	ClassIDs          []uuid.UUID
	GradeLevel        *int
	SisConnectionID   *uuid.UUID
	StudentExternalID *string
	Version           int
}
