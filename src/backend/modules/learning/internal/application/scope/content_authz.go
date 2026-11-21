package scope

import (
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
)

// tenantContent reports the permissions that act on tenant learning content as a whole: the content, its media and
// the curriculum it is tagged with have no school dimension (CONTENT_MODEL §4). Re-evaluation, curriculum writes
// and rollbacks are not in the list: they keep requiring a grant without a school limit.
func tenantContent(perm authz.Permission) bool {
	switch perm {
	case authz.PermLearningContentReEvaluate:
		return false
	case authz.PermMediaUpload, authz.PermCurriculumRead:
		return true
	}
	return strings.HasPrefix(string(perm), "learning_content.")
}

// RequireContent authorizes perm on learning content of the session tenant (tenant nil: content of the scope, e.g.
// a list). Tenant content has no school, so a grant of perm limited to a school of the session tenant covers it —
// School Admin authors, reviews and publishes the tenant's content (ADMIN_PORTAL §2.3/§2.4, CONTENT_MODEL §4; review
// tasks keep their school, F025). Every other case is authz.Require: 403 PERMISSION_DENIED + authz.denied, another
// tenant 404, subject-limited grants unchanged (F012 Clarifications 2026-11-21).
func RequireContent(ctx context.Context, p authz.Principal, perm authz.Permission, tenant *uuid.UUID) error {
	if tenantContent(perm) && p.Valid() && p.TenantID != nil && (tenant == nil || *tenant == *p.TenantID) {
		for _, g := range p.Grants {
			if g.SchoolID != nil && g.SubjectID == nil && slices.Contains(g.Permissions, perm) {
				return nil
			}
		}
	}
	return authz.Require(ctx, p, perm, authz.Resource{TenantID: tenant})
}
