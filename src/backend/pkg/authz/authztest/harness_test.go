package authztest_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// A sample use case shows the intended pattern: Require first, then work.
func getAssignment(ctx context.Context, tenantOfRow uuid.UUID) error {
	p, _ := authz.PrincipalFrom(ctx)
	return authz.Require(ctx, p, authz.PermLmsAssignmentRead, authz.Resource{TenantID: &tenantOfRow})
}

func TestHarnessSample(t *testing.T) {
	authztest.Cover(t, "getAssignment", authztest.CrossTenant, authztest.RoleDenied)
	admin := authztest.Staff(authztest.TenantA, "school_admin")
	if err := getAssignment(authztest.Ctx(context.Background(), admin), authztest.TenantA); err != nil {
		t.Fatal(err)
	}
	authztest.ExpectNotFound(t, getAssignment(authztest.Ctx(context.Background(), admin), authztest.TenantB))
	authztest.ExpectDenied(t, getAssignment(authztest.Ctx(context.Background(), authztest.NoRole(authztest.TenantA)), authztest.TenantA))
	authztest.ExpectDenied(t, getAssignment(authztest.Ctx(context.Background(), authztest.Student(authztest.TenantA)), authztest.TenantA))
}

func TestFixtures(t *testing.T) {
	s := authztest.Staff(authztest.TenantA, "tenant_admin")
	if !s.Valid() || *s.TenantID != authztest.TenantA || !s.Has(authz.PermTenantUserRead) || s.Channel != authz.ChannelPortal {
		t.Fatalf("staff fixture %+v", s)
	}
	ctx := authztest.Ctx(context.Background(), s)
	if sc, ok := tenancy.ScopeFrom(ctx); !ok || sc.TenantIDs()[0] != authztest.TenantA {
		t.Fatal("tenant scope missing")
	}
	sa := authztest.Platform("system_admin")
	if !sa.IsSystemAdmin || !sa.MFA || !sa.HasPlatformRole() || sa.TenantID != nil {
		t.Fatalf("platform fixture %+v", sa)
	}
	if sc, ok := tenancy.ScopeFrom(authztest.Ctx(context.Background(), sa)); !ok || !sc.PlatformOnly() {
		t.Fatal("platform scope missing")
	}
	school := uuid.New()
	limited := authztest.Staff(authztest.TenantA, "school_admin", authztest.AtSchool(school))
	if err := authz.Require(context.Background(), limited, authz.PermLmsClassRead, authz.Resource{TenantID: &authztest.TenantA, SchoolID: &school}); err != nil {
		t.Fatal(err)
	}
	other := uuid.New()
	authztest.ExpectDenied(t, authz.Require(context.Background(), limited, authz.PermLmsClassRead, authz.Resource{TenantID: &authztest.TenantA, SchoolID: &other}))
}

type fakeTB struct {
	testing.TB
	failed bool
}

func (f *fakeTB) Helper()               {}
func (f *fakeTB) Fatalf(string, ...any) { f.failed = true }
func (f *fakeTB) Fatal(...any)          { f.failed = true }

func TestExpectHelpersFail(t *testing.T) {
	for name, run := range map[string]func(tb testing.TB){
		"ExpectNotFound on 403": func(tb testing.TB) { authztest.ExpectNotFound(tb, problem.New(problem.PermissionDenied, "")) },
		"ExpectDenied on nil":   func(tb testing.TB) { authztest.ExpectDenied(tb, nil) },
		"Cover without cases":   func(tb testing.TB) { authztest.Cover(tb, "x") },
	} {
		f := &fakeTB{TB: t}
		run(f)
		if !f.failed {
			t.Fatalf("%s did not fail", name)
		}
	}
}
