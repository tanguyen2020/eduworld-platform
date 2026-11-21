package scope

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// tenantsFake is a TenantDirectory test double (TEST DOUBLE — R17).
type tenantsFake []uuid.UUID

func (f tenantsFake) ActiveTenants(_ context.Context, ids []uuid.UUID) ([]uuid.UUID, error) {
	if len(ids) == 0 {
		return f, nil
	}
	var out []uuid.UUID
	for _, id := range ids {
		if slices.Contains(f, id) {
			out = append(out, id)
		}
	}
	return out, nil
}

func codeOf(err error) problem.Code {
	var pe *problem.Error
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

// F020-AC-11: tenant_ids from a tenant user is ignored; F020-AC-13: System Admin gets a CrossTenantScope of
// the requested tenants, or of every active tenant when none is requested.
func TestForRead(t *testing.T) {
	ctx := context.Background()
	dir := tenantsFake{authztest.TenantA, authztest.TenantB}
	staff := authztest.Staff(authztest.TenantA, "school_admin")
	sc, err := ForRead(ctx, staff, []uuid.UUID{authztest.TenantB}, dir)
	if err != nil || !slices.Equal(sc.TenantIDs(), []uuid.UUID{authztest.TenantA}) {
		t.Fatalf("tenant user: %v %v", sc, err)
	}
	sa := authztest.Platform("system_admin")
	sc, err = ForRead(ctx, sa, []uuid.UUID{authztest.TenantB}, dir)
	cs, ok := sc.(tenancy.CrossTenantScope)
	if err != nil || !ok || !slices.Equal(cs.TenantIDs(), []uuid.UUID{authztest.TenantB}) || cs.Reason() != tenancy.ReasonSystemAdminCrossTenant {
		t.Fatalf("system admin with tenant_ids: %v %v", sc, err)
	}
	sc, err = ForRead(ctx, sa, nil, dir)
	if err != nil || !slices.Equal(sc.TenantIDs(), []uuid.UUID(dir)) {
		t.Fatalf("system admin, all tenants: %v %v", sc, err)
	}
	// Platform staff without platform.cross_tenant.read (content_admin) cannot read across tenants.
	if _, err := ForRead(ctx, authztest.Platform("content_admin"), nil, dir); codeOf(err) != problem.PermissionDenied {
		t.Fatalf("content_admin: %v", err)
	}
	// A cross-tenant scope needs the portal client (no MFA since ADR-0094).
	d := sa.PrincipalData
	d.Channel = authz.ChannelGame
	if _, err := ForRead(ctx, authz.NewPrincipal(d), nil, dir); codeOf(err) != problem.PermissionDenied {
		t.Fatalf("game client: %v", err)
	}
}

// F020-AC-12: System Admin writing tenant content needs a target tenant; platform content uses PlatformScope
// (F020-AC-32); a tenant user never writes platform content.
func TestForWrite(t *testing.T) {
	sa := authztest.Platform("system_admin")
	if _, err := ForWrite(sa, contentmodel.OwnerTenant, nil); codeOf(err) != problem.ValidationFailed {
		t.Fatalf("tenant content without target: %v", err)
	}
	target := authztest.TenantA
	sc, err := ForWrite(sa, contentmodel.OwnerTenant, &target)
	if err != nil || !slices.Equal(sc.TenantIDs(), []uuid.UUID{target}) {
		t.Fatalf("tenant content with target: %v %v", sc, err)
	}
	sc, err = ForWrite(sa, contentmodel.OwnerPlatform, nil)
	if err != nil || !sc.PlatformOnly() {
		t.Fatalf("platform content: %v %v", sc, err)
	}
	if _, err := ForWrite(sa, contentmodel.OwnerPlatform, &target); codeOf(err) != problem.ValidationFailed {
		t.Fatalf("platform content with a tenant: %v", err)
	}
	staff := authztest.Staff(authztest.TenantA, "tenant_content_editor")
	if _, err := ForWrite(staff, contentmodel.OwnerPlatform, nil); codeOf(err) != problem.PermissionDenied {
		t.Fatalf("tenant user writing platform content: %v", err)
	}
	other := authztest.TenantB
	if _, err := ForWrite(staff, contentmodel.OwnerTenant, &other); codeOf(err) != problem.PermissionDenied {
		t.Fatalf("tenant user targeting another tenant: %v", err)
	}
	sc, err = ForWrite(staff, contentmodel.OwnerTenant, nil)
	if id, single := sc.SessionTenantID(); err != nil || !single || id != authztest.TenantA {
		t.Fatalf("tenant user: %v %v", sc, err)
	}
	if _, err := ForWrite(sa, "school", nil); codeOf(err) != problem.ValidationFailed {
		t.Fatalf("unknown owner scope: %v", err)
	}
}
