package command

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// staffOf is the principal of a portal call: staff of the session tenant (System Admin with its X-Tenant-Id included).
func staffOf(ctx context.Context) (authz.Principal, error) {
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

// SchoolCovered reports whether a grant holding perm covers the school (the subject aside).
func SchoolCovered(p authz.Principal, perm authz.Permission, school uuid.UUID) bool {
	for _, g := range p.Grants {
		if slices.Contains(g.Permissions, perm) && (g.SchoolID == nil || *g.SchoolID == school) {
			return true
		}
	}
	return false
}

// AuthorizeLMS authorizes perm on a resource of a school and subject (UserRole school × subject, ADR-0072): without the
// permission 403 PERMISSION_DENIED; a school outside every grant 404 NOT_FOUND (no existence leak); a subject outside a
// subject-limited grant 403 PERMISSION_DENIED; denials are audited (authz.denied).
func AuthorizeLMS(ctx context.Context, p authz.Principal, perm authz.Permission, school uuid.UUID, subject *uuid.UUID) error {
	if !holdsPerm(p, perm) {
		return authz.Require(ctx, p, perm, authz.Resource{TenantID: p.TenantID, SchoolID: &school, SubjectID: subject})
	}
	if !SchoolCovered(p, perm, school) {
		_ = authz.Require(ctx, p, perm, authz.Resource{TenantID: p.TenantID, SchoolID: &school})
		return problem.New(problem.NotFound, "")
	}
	return authz.Require(ctx, p, perm, authz.Resource{TenantID: p.TenantID, SchoolID: &school, SubjectID: subject})
}
