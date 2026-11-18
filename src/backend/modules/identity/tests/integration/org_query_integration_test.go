//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// F010-AC-06 (catalog), AC-10, AC-17, AC-18, AC-19 with the real read model and RLS.
func TestOrgQueries(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	a := seedTenant(ctx, t, e.platform, "pgd-qa")
	b := seedTenant(ctx, t, e.platform, "pgd-qb")
	student := enrolled(ctx, t, e.platform, a, "hs.q.a")
	// An unassigned student (no active enrollment) in tenant A.
	tx, _ := e.platform.BeginTx(ctx, nil)
	if _, err := insertStudent(ctx, tx, a, "hs.q.free"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	codec, _ := page.NewCodec([]byte(strings.Repeat("k", 32)))
	q := query.Org{Model: read.OrgModel{Session: e.read}, Cursors: codec}
	ta, tb := uuid.MustParse(a.tenant), uuid.MustParse(b.tenant)
	schoolA1, schoolA2, schoolB := uuid.MustParse(a.school), uuid.MustParse(a.otherSchool), uuid.MustParse(b.school)

	// Catalog: countries and provinces with successors.
	anyUser := authztest.Ctx(ctx, authztest.Staff(ta, "tenant_admin"))
	cs, err := q.ListCountries(anyUser)
	if err != nil || len(cs) != 4 {
		t.Fatalf("countries = %d %v", len(cs), err)
	}
	ps, err := q.ListProvinces(anyUser, "VN", "inactive")
	if err != nil || len(ps) != 63 || ps[0].SuccessorCode == nil {
		t.Fatalf("former provinces = %d %v", len(ps), err)
	}

	// AC-10/AC-17: tenant users see only their tenant; another tenant's school is 404.
	adminA := authztest.Ctx(ctx, authztest.Staff(ta, "tenant_admin"))
	sch, err := q.ListTenantSchools(adminA, "", query.ListInput{Limit: 1})
	if err != nil || len(sch.Items) != 1 || !sch.Page.HasMore {
		t.Fatalf("page 1 = %+v %v", sch, err)
	}
	sch2, err := q.ListTenantSchools(adminA, "", query.ListInput{Limit: 1, Cursor: *sch.Page.NextCursor})
	if err != nil || len(sch2.Items) != 1 || sch2.Items[0].TenantID != ta || sch2.Page.HasMore {
		t.Fatalf("page 2 = %+v %v", sch2, err)
	}
	if _, err := q.GetTenantSchool(adminA, schoolB); code(err) != problem.NotFound {
		t.Fatalf("school of tenant B: %v (want 404)", err)
	}
	cat, err := q.ListCatalogSchools(adminA, query.CatalogSchoolsInput{ProvinceCode: "vn.ha-noi"})
	if err != nil || len(cat.Items) != 2 {
		t.Fatalf("catalog schools of A = %+v %v", cat, err)
	}
	// System Admin with tenant_ids (CrossTenantScope from the request) sees both, and the read is audited.
	sa := authztest.Platform("system_admin")
	cross, _ := authz.CrossTenantScope(sa, []uuid.UUID{ta, tb}, tenancy.ReasonSystemAdminCrossTenant)
	all, err := q.ListCatalogSchools(tenancy.WithScope(authz.WithPrincipal(ctx, sa), cross), query.CatalogSchoolsInput{})
	if err != nil || len(all.Items) != 4 {
		t.Fatalf("System Admin catalog = %d %v", len(all.Items), err)
	}
	adm, err := q.ListAdminTenantSchools(authztest.Ctx(ctx, sa), tb, "active", query.ListInput{})
	if err != nil || len(adm.Items) != 2 || adm.Items[0].TenantID != tb {
		t.Fatalf("admin tenant schools = %+v %v", adm, err)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' = 'cross_tenant.accessed'`); n == 0 {
		t.Fatal("cross-tenant reads must be audited (F010-AC-10)")
	}

	// AC-18: a school-limited grant cannot list the classes of another school of the same tenant.
	schoolAdmin := authztest.Ctx(ctx, authztest.Staff(ta, "school_admin", authztest.AtSchool(schoolA1)))
	if cls, err := q.ListClasses(schoolAdmin, schoolA1, nil, 0); err != nil || len(cls) != 1 {
		t.Fatalf("own school classes = %+v %v", cls, err)
	}
	authztest.ExpectDenied(t, func() error { _, err := q.ListClasses(schoolAdmin, schoolA2, nil, 0); return err }())
	tcls, err := q.ListTenantSchoolClasses(adminA, query.ClassesInput{SchoolID: schoolA1, Grade: 3})
	if err != nil || len(tcls.Items) != 1 {
		t.Fatalf("tenant school classes = %+v %v", tcls, err)
	}

	un, err := q.ListUnassignedStudents(adminA, nil, query.ListInput{})
	if err != nil || len(un.Items) != 1 || un.Items[0].Reason != "missing_class" {
		t.Fatalf("unassigned = %+v %v", un, err)
	}

	// AC-19: the student reads their own school and class; others need school.people.read in scope.
	me := authz.NewPrincipal(authz.PrincipalData{UserID: student, UserType: authz.UserTypeStudent, TenantID: &ta, Channel: authz.ChannelGame})
	prof, err := q.GetMyStudentProfile(authztest.Ctx(ctx, me))
	if err != nil || prof.SchoolID != schoolA1 || prof.ClassName != "3A" || prof.SchoolName == "" {
		t.Fatalf("my profile = %+v %v", prof, err)
	}
	if _, err := q.GetStudentProfile(adminA, student); err != nil {
		t.Fatalf("tenant admin reads the student: %v", err)
	}
	if _, err := q.GetStudentProfile(authztest.Ctx(ctx, authztest.Staff(tb, "tenant_admin")), student); code(err) != problem.NotFound {
		t.Fatalf("tenant B reads a student of A: %v (want 404)", err)
	}
	authztest.ExpectDenied(t, func() error {
		_, err := q.GetStudentProfile(authztest.Ctx(ctx, authztest.Student(ta)), ids.New())
		return err
	}())
}
