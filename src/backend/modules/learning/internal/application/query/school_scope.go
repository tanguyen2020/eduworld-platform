package query

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// staffPrincipal is the principal of a portal read: staff of the session tenant.
func staffPrincipal(ctx context.Context) (authz.Principal, error) {
	p, err := principal(ctx)
	if err != nil {
		return p, err
	}
	if p.UserType == authz.UserTypeStudent {
		return p, problem.New(problem.PortalAccessStaffOnly, "")
	}
	if p.TenantID == nil {
		return p, problem.New(problem.TenantHeaderRequired, "")
	}
	return p, nil
}

func holdsIn(perms []authz.Permission, perm authz.Permission) bool {
	for _, x := range perms {
		if x == perm {
			return true
		}
	}
	return false
}

// authorizeSchool is command.AuthorizeLMS for the reads: 403 without the permission, 404 for a school outside every
// grant, 403 for a subject outside a subject-limited grant.
func authorizeSchool(ctx context.Context, p authz.Principal, perm authz.Permission, school uuid.UUID, subject *uuid.UUID) error {
	if !holds(p, perm) {
		return authz.Require(ctx, p, perm, authz.Resource{TenantID: p.TenantID, SchoolID: &school, SubjectID: subject})
	}
	if !coversSchool(p, perm, school) {
		_ = authz.Require(ctx, p, perm, authz.Resource{TenantID: p.TenantID, SchoolID: &school})
		return problem.New(problem.NotFound, "")
	}
	return authz.Require(ctx, p, perm, authz.Resource{TenantID: p.TenantID, SchoolID: &school, SubjectID: subject})
}
