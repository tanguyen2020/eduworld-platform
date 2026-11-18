package appscope

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

func code(err error) problem.Code {
	var pe *problem.Error
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

func TestTargetTenant(t *testing.T) {
	target := uuid.New()
	sa := authztest.Platform("system_admin")
	// No session tenant: cross-tenant scope of exactly the target (audited by UnitOfWork).
	sc, err := TargetTenant(sa, target)
	cs, ok := sc.(tenancy.CrossTenantScope)
	if err != nil || !ok || len(cs.TenantIDs()) != 1 || cs.TenantIDs()[0] != target {
		t.Fatalf("cross scope = %#v %v", sc, err)
	}
	// X-Tenant-Id resolved the System Admin into the target: its session scope.
	d := sa.PrincipalData
	d.TenantID = &target
	inTarget := authz.NewPrincipal(d)
	sc, err = TargetTenant(inTarget, target)
	if ts, ok := sc.(tenancy.TenantScope); err != nil || !ok || ts.TenantIDs()[0] != target {
		t.Fatalf("session scope = %#v %v", sc, err)
	}
	// Session tenant differs from the path tenant: 404 (no existence leak).
	if _, err := TargetTenant(inTarget, uuid.New()); code(err) != problem.NotFound {
		t.Fatalf("other target: %v", err)
	}
	// Platform staff without cross_tenant.read cannot get a cross-tenant scope.
	if _, err := TargetTenant(authztest.Platform("safety_moderator"), target); code(err) != problem.PermissionDenied {
		t.Fatalf("moderator: %v", err)
	}
}
