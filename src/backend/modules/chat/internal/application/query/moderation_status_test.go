package query

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F122-AC-13 (Clarifications 2026-11-28): the banner status is tenant-wide, so the School Admin of ONE school of the
// session tenant (school.settings.read limited to that school, like the demo ad.truong.a) is authorized; staff without
// tenant.settings.read / school.settings.read and another tenant stay refused.
func TestStatusResourceAcceptsSchoolLimitedGrant(t *testing.T) {
	tenant, school := authztest.TenantA, uuid.New()
	ctx := context.Background()
	authorize := func(p authz.Principal, on uuid.UUID) error {
		perm := authz.PermSchoolSettingsRead
		if p.Has(authz.PermTenantSettingsRead) {
			perm = authz.PermTenantSettingsRead
		}
		return authz.Require(ctx, p, perm, statusResource(p, perm, on))
	}
	for name, p := range map[string]authz.Principal{
		"tenant_admin":              authztest.Staff(tenant, "tenant_admin"),
		"school_admin (tenant)":     authztest.Staff(tenant, "school_admin"),
		"school_admin (one school)": authztest.Staff(tenant, "school_admin", authztest.AtSchool(school)),
	} {
		if err := authorize(p, tenant); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := authorize(authztest.NoRole(tenant), tenant); codeOf(err) != problem.PermissionDenied {
		t.Errorf("no role: %v, want PERMISSION_DENIED", err)
	}
	// academic_manager holds neither permission (ADMIN_PORTAL §2.3): no banner call, refused here.
	if err := authorize(authztest.Staff(tenant, "academic_manager"), tenant); codeOf(err) != problem.PermissionDenied {
		t.Errorf("academic_manager: %v, want PERMISSION_DENIED", err)
	}
	limited := authztest.Staff(tenant, "school_admin", authztest.AtSchool(school))
	if err := authorize(limited, authztest.TenantB); codeOf(err) != problem.NotFound {
		t.Errorf("other tenant: %v, want NOT_FOUND", err)
	}
	// The resource keeps the grant limit (never widened to the whole tenant for another permission).
	if r := statusResource(limited, authz.PermSchoolSettingsRead, tenant); r.SchoolID == nil || *r.SchoolID != school {
		t.Errorf("resource of a school-limited grant: %+v", r)
	}
	if r := statusResource(authztest.Staff(tenant, "tenant_admin"), authz.PermTenantSettingsRead, tenant); r.SchoolID != nil || r.SubjectID != nil {
		t.Errorf("resource of a tenant-wide grant: %+v", r)
	}
}

func codeOf(err error) problem.Code {
	var pe *problem.Error
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}
