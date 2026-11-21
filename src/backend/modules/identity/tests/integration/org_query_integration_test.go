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
	// OQ F057-4: status filter, tenant isolation and the platform scope rules of listCatalogSchools.
	if _, err := e.platform.ExecContext(ctx, `UPDATE org.schools SET status = 'disabled' WHERE school_id = $1`, schoolA2); err != nil {
		t.Fatal(err)
	}
	if dis, err := q.ListCatalogSchools(adminA, query.CatalogSchoolsInput{Status: "disabled"}); err != nil || len(dis.Items) != 1 || dis.Items[0].SchoolID != schoolA2 {
		t.Fatalf("disabled schools of A = %+v %v", dis, err)
	}
	if act, err := q.ListCatalogSchools(adminA, query.CatalogSchoolsInput{Status: "active"}); err != nil || len(act.Items) != 1 || act.Items[0].SchoolID != schoolA1 {
		t.Fatalf("active schools of A = %+v %v", act, err)
	}
	adminB := authztest.Ctx(ctx, authztest.Staff(tb, "tenant_admin"))
	if bs, err := q.ListCatalogSchools(adminB, query.CatalogSchoolsInput{}); err != nil || len(bs.Items) != 2 || bs.Items[0].TenantID != tb || bs.Items[1].TenantID != tb {
		t.Fatalf("schools of B = %+v %v", bs, err)
	}
	if act, err := q.ListCatalogSchools(tenancy.WithScope(authz.WithPrincipal(ctx, sa), cross), query.CatalogSchoolsInput{Status: "active"}); err != nil || len(act.Items) != 3 {
		t.Fatalf("System Admin active schools of A and B = %d %v", len(act.Items), err)
	}
	if _, err := q.ListCatalogSchools(adminA, query.CatalogSchoolsInput{Status: "closed"}); code(err) != problem.ValidationFailed {
		t.Fatalf("unknown status: %v", err)
	}
	// Without tenant_ids the System Admin has a PlatformScope only → 403 (no listing of every tenant).
	authztest.ExpectDenied(t, func() error {
		_, err := q.ListCatalogSchools(authztest.Ctx(ctx, sa), query.CatalogSchoolsInput{})
		return err
	}())
	// A platform role without platform.cross_tenant.read is refused even with a cross-tenant scope.
	ca := authztest.Platform("content_admin")
	authztest.ExpectDenied(t, func() error {
		_, err := q.ListCatalogSchools(tenancy.WithScope(authz.WithPrincipal(ctx, ca), cross), query.CatalogSchoolsInput{})
		return err
	}())
	// A staff account without a session tenant and without a platform role (tenant user without X-Tenant-Id) → 403.
	noTenant := authz.NewPrincipal(authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStaff, Channel: authz.ChannelPortal})
	authztest.ExpectDenied(t, func() error {
		_, err := q.ListCatalogSchools(tenancy.WithScope(authz.WithPrincipal(ctx, noTenant), cross), query.CatalogSchoolsInput{})
		return err
	}())
	if _, err := e.platform.ExecContext(ctx, `UPDATE org.schools SET status = 'active' WHERE school_id = $1`, schoolA2); err != nil {
		t.Fatal(err)
	}

	// AC-18: a school-limited grant cannot list the classes of another school of the same tenant.
	schoolAdmin := authztest.Ctx(ctx, authztest.Staff(ta, "school_admin", authztest.AtSchool(schoolA1)))
	if cls, err := q.ListClasses(schoolAdmin, schoolA1, nil, 0); err != nil || len(cls) != 1 {
		t.Fatalf("own school classes = %+v %v", cls, err)
	}
	authztest.ExpectDenied(t, func() error { _, err := q.ListClasses(schoolAdmin, schoolA2, nil, 0); return err }())

	// F012 Clarifications 2026-11-21 (portal E2E): School Admin and Academic Manager read their schools; a
	// school-limited grant lists and opens only its schools; another tenant's school is never visible.
	if own, err := q.ListTenantSchools(schoolAdmin, "", query.ListInput{}); err != nil || len(own.Items) != 1 || own.Items[0].SchoolID != schoolA1 {
		t.Fatalf("school admin of A1 lists = %+v %v", own, err)
	}
	if s, err := q.GetTenantSchool(schoolAdmin, schoolA1); err != nil || s.SchoolID != schoolA1 {
		t.Fatalf("school admin of A1 opens A1 = %+v %v", s, err)
	}
	authztest.ExpectDenied(t, func() error { _, err := q.GetTenantSchool(schoolAdmin, schoolA2); return err }())
	schoolAdminB := authztest.Ctx(ctx, authztest.Staff(tb, "school_admin", authztest.AtSchool(schoolB)))
	if own, err := q.ListTenantSchools(schoolAdminB, "", query.ListInput{}); err != nil || len(own.Items) != 1 || own.Items[0].SchoolID != schoolB {
		t.Fatalf("school admin of B lists = %+v %v", own, err)
	}
	if _, err := q.GetTenantSchool(schoolAdminB, schoolA1); code(err) != problem.PermissionDenied && code(err) != problem.NotFound {
		t.Fatalf("school admin of B opens a school of A: %v (want 403/404)", err)
	}
	subjectAM := authztest.Ctx(ctx, authztest.Staff(ta, "academic_manager", authztest.ForSubject(uuid.New())))
	if am, err := q.ListTenantSchools(subjectAM, "", query.ListInput{}); err != nil || len(am.Items) != 2 || am.Items[0].TenantID != ta || am.Items[1].TenantID != ta {
		t.Fatalf("subject-scoped academic manager lists = %+v %v", am, err)
	}
	schoolAM := authztest.Ctx(ctx, authztest.Staff(ta, "academic_manager", authztest.AtSchool(schoolA2)))
	if am, err := q.ListTenantSchools(schoolAM, "", query.ListInput{}); err != nil || len(am.Items) != 1 || am.Items[0].SchoolID != schoolA2 {
		t.Fatalf("school-scoped academic manager lists = %+v %v", am, err)
	}
	editor := authztest.Ctx(ctx, authztest.Staff(ta, "tenant_content_editor"))
	authztest.ExpectDenied(t, func() error { _, err := q.ListTenantSchools(editor, "", query.ListInput{}); return err }())
	authztest.Cover(t, "listTenantSchools", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getTenantSchool", authztest.CrossTenant, authztest.RoleDenied)
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
