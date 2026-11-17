package authz_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

func ptr(u uuid.UUID) *uuid.UUID { return &u }

func codeOf(err error) problem.Code {
	var pe *problem.Error
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

// F002-AC-19 (TC-TEN-001..003).
func TestRequire(t *testing.T) {
	ta, tb, s1, s2, math := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	schoolAdmin := authz.NewPrincipal(authz.PrincipalData{
		UserID: uuid.New(), UserType: authz.UserTypeStaff, TenantID: ptr(ta),
		Grants: []authz.Grant{{Role: "school_admin", SchoolID: ptr(s1), Permissions: []authz.Permission{authz.PermLmsAssignmentWrite}}},
	})
	subjectMgr := authz.NewPrincipal(authz.PrincipalData{
		UserID: uuid.New(), UserType: authz.UserTypeStaff, TenantID: ptr(ta),
		Grants: []authz.Grant{{Role: "academic_manager", SubjectID: ptr(math), Permissions: []authz.Permission{authz.PermLmsAssignmentWrite}}},
	})
	tenantAdmin := authz.NewPrincipal(authz.PrincipalData{
		UserID: uuid.New(), UserType: authz.UserTypeStaff, TenantID: ptr(ta),
		Grants: []authz.Grant{{Role: "tenant_admin", Permissions: []authz.Permission{authz.PermLmsAssignmentRead}}},
	})
	ctx := context.Background()
	cases := []struct {
		name string
		p    authz.Principal
		perm authz.Permission
		res  authz.Resource
		want problem.Code
	}{
		{"own school", schoolAdmin, authz.PermLmsAssignmentWrite, authz.Resource{TenantID: ptr(ta), SchoolID: ptr(s1)}, ""},
		{"other tenant is 404", schoolAdmin, authz.PermLmsAssignmentWrite, authz.Resource{TenantID: ptr(tb), SchoolID: ptr(s1)}, problem.NotFound},
		{"other school is 403", schoolAdmin, authz.PermLmsAssignmentWrite, authz.Resource{TenantID: ptr(ta), SchoolID: ptr(s2)}, problem.PermissionDenied},
		{"tenant-level resource with school grant is 403", schoolAdmin, authz.PermLmsAssignmentWrite, authz.Resource{TenantID: ptr(ta)}, problem.PermissionDenied},
		{"missing permission is 403", schoolAdmin, authz.PermLmsChallengeWrite, authz.Resource{TenantID: ptr(ta), SchoolID: ptr(s1)}, problem.PermissionDenied},
		{"subject match", subjectMgr, authz.PermLmsAssignmentWrite, authz.Resource{TenantID: ptr(ta), SchoolID: ptr(s2), SubjectID: ptr(math)}, ""},
		{"subject mismatch is 403", subjectMgr, authz.PermLmsAssignmentWrite, authz.Resource{TenantID: ptr(ta), SubjectID: ptr(uuid.New())}, problem.PermissionDenied},
		{"tenant grant covers schools", tenantAdmin, authz.PermLmsAssignmentRead, authz.Resource{TenantID: ptr(ta), SchoolID: ptr(s2)}, ""},
		{"tenant grant read only", tenantAdmin, authz.PermLmsAssignmentWrite, authz.Resource{TenantID: ptr(ta)}, problem.PermissionDenied},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := authz.Require(ctx, c.p, c.perm, c.res)
			if got := codeOf(err); got != c.want {
				t.Fatalf("code = %q, want %q (err %v)", got, c.want, err)
			}
			if c.want == "" && err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRequireSystemAdminCrossTenant(t *testing.T) {
	ta, tb := uuid.New(), uuid.New()
	sa := authz.NewPrincipal(authz.PrincipalData{
		UserID: uuid.New(), UserType: authz.UserTypeStaff, IsSystemAdmin: true, Channel: authz.ChannelPortal, MFA: true,
		Grants: []authz.Grant{{Role: "system_admin", Permissions: []authz.Permission{authz.PermPlatformCrossTenantRead, authz.PermLmsReportRead}}},
	})
	ctx := context.Background()
	// Without a cross-tenant scope a platform principal sees no tenant resource.
	if c := codeOf(authz.Require(ctx, sa, authz.PermLmsReportRead, authz.Resource{TenantID: ptr(ta)})); c != problem.NotFound {
		t.Fatalf("no scope: %q", c)
	}
	s, err := authz.CrossTenantScope(sa, []uuid.UUID{ta}, tenancy.ReasonSystemAdminCrossTenant)
	if err != nil {
		t.Fatal(err)
	}
	ctx = tenancy.WithScope(ctx, s)
	if err := authz.Require(ctx, sa, authz.PermLmsReportRead, authz.Resource{TenantID: ptr(ta)}); err != nil {
		t.Fatal(err)
	}
	if c := codeOf(authz.Require(ctx, sa, authz.PermLmsReportRead, authz.Resource{TenantID: ptr(tb)})); c != problem.NotFound {
		t.Fatalf("tenant outside scope: %q", c)
	}
}

func TestRequireAuditsDenial(t *testing.T) {
	var got []audit.Event
	authz.Configure(authz.Hooks{Recorder: audit.RecorderFunc(func(_ context.Context, e audit.Event) error {
		got = append(got, e)
		return nil
	})})
	t.Cleanup(func() { authz.Configure(authz.Hooks{}) })
	ta := uuid.New()
	p := authz.NewPrincipal(authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStaff, TenantID: ptr(ta)})
	_ = authz.Require(context.Background(), p, authz.PermLmsReportRead, authz.Resource{TenantID: ptr(ta)})
	if len(got) != 1 || got[0].Action != audit.ActionAuthzDenied || got[0].Result != audit.ResultDenied ||
		got[0].Metadata["permission"] != "lms.report.read" || got[0].Reason != string(problem.PermissionDenied) {
		t.Fatalf("audit = %+v", got)
	}
}
