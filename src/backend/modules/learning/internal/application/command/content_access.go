package command

import (
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// RequireWritable applies the owner rule of a write on a content item (CONTENT_MODEL §4, F020-AC-18) after
// authz.Require: a tenant user may change only its own tenant's items — assigned platform content is
// read-only for it (403 PERMISSION_DENIED) and another tenant's item does not exist for it (404); platform
// staff change platform items, and tenant items only with the target tenant's scope (scope.ForWrite).
func RequireWritable(p authz.Principal, owner contentmodel.Owner) error {
	if p.TenantID != nil {
		switch {
		case owner.Platform():
			return problem.New(problem.PermissionDenied, "assigned platform content is read-only")
		case owner.TenantID == nil || *owner.TenantID != *p.TenantID:
			return problem.New(problem.NotFound, "")
		}
		return nil
	}
	if !p.IsPlatform() {
		return problem.New(problem.PermissionDenied, "")
	}
	return nil
}
