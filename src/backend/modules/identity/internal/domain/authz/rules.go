// Package authz holds the role assignment rules of module identity (ADR-0066, ADR-0072, ADR-0081): which
// scopes a role kind accepts, who may hold portal roles, and the training rule of safety_moderator. The
// database triggers repeat the user and scope rules as a second layer.
package authz

import "errors"

// Role scope kinds and assignment scopes.
const (
	KindTenant   = "tenant"
	KindPlatform = "platform"

	ScopePlatform = "platform"
	ScopeTenant   = "tenant"
	ScopeSchool   = "school"
	ScopeSubject  = "subject"
)

// Errors of CheckGrant (mapped to 422 by the use case).
var (
	ErrScope           = errors.New("authz: scope not allowed for this role kind")
	ErrScopeID         = errors.New("authz: scope id required for school/subject scopes and forbidden otherwise")
	ErrStudentRole     = errors.New("authz: a student never holds a portal role")
	ErrPlatformStaff   = errors.New("authz: staff without home tenant hold platform roles only")
	ErrTenantStaff     = errors.New("authz: staff with a home tenant hold tenant roles only")
	ErrTrainingMissing = errors.New("authz: safety_moderator requires training_completed_at")
	ErrValidity        = errors.New("authz: valid_to must be after valid_from")
)

// Grant is a role assignment to validate.
type Grant struct {
	RoleCode      string
	RoleKind      string // tenant | platform
	ScopeType     string
	HasScopeID    bool
	UserType      string // student | staff
	UserHasHome   bool   // primary_tenant_id IS NOT NULL
	TrainingDone  bool   // training_completed_at given
	ValidityOrder bool   // valid_to is nil or after valid_from
}

// CheckGrant applies the assignment rules (F012-AC-20, AC-21, AC-24). There is no class scope (ADR-0072).
func CheckGrant(g Grant) error {
	switch g.RoleKind {
	case KindPlatform:
		if g.ScopeType != ScopePlatform {
			return ErrScope
		}
	case KindTenant:
		if g.ScopeType != ScopeTenant && g.ScopeType != ScopeSchool && g.ScopeType != ScopeSubject {
			return ErrScope
		}
	default:
		return ErrScope
	}
	needsID := g.ScopeType == ScopeSchool || g.ScopeType == ScopeSubject
	if needsID != g.HasScopeID {
		return ErrScopeID
	}
	if g.UserType != "staff" {
		return ErrStudentRole
	}
	if !g.UserHasHome && g.RoleKind != KindPlatform {
		return ErrPlatformStaff
	}
	if g.UserHasHome && g.RoleKind == KindPlatform {
		return ErrTenantStaff
	}
	if g.RoleCode == "safety_moderator" && !g.TrainingDone {
		return ErrTrainingMissing
	}
	if !g.ValidityOrder {
		return ErrValidity
	}
	return nil
}

// Exceeds reports the permissions of a role the granter does not hold itself (F012-AC-22: a tenant user
// cannot hand out more than it has).
func Exceeds(role []string, holder map[string]bool) []string {
	var out []string
	for _, p := range role {
		if !holder[p] {
			out = append(out, p)
		}
	}
	return out
}
