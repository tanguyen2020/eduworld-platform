//go:build integration

package integration

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
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
	// A second student of the same school and grade (ADR-0095: every live student row places the student).
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
	// Owner rule 2026-11-23 (API_DESIGN §5): the catalog grids read one keyset page in SQL — page 2 continues page 1
	// with no duplicate and no gap, in code order (global catalog: the same rows for every tenant).
	var countries []string
	in := query.ListInput{Limit: 3}
	for i := 0; ; i++ {
		pg, err := q.ListCountries(anyUser, in)
		if err != nil || len(pg.Items) > 3 || i > 3 {
			t.Fatalf("countries page %d = %+v %v", i, pg, err)
		}
		for _, c := range pg.Items {
			countries = append(countries, c.CountryCode)
		}
		if !pg.Page.HasMore {
			break
		}
		in.Cursor = *pg.Page.NextCursor
	}
	if len(countries) != 4 || !sort.StringsAreSorted(countries) || len(dedupe(countries)) != 4 {
		t.Fatalf("countries = %v", countries)
	}
	var former []dto.Province
	pin := query.ListInput{Limit: 20}
	for i := 0; ; i++ {
		pg, err := q.ListProvinces(anyUser, "VN", "inactive", pin)
		if err != nil || len(pg.Items) > 20 || i > 5 {
			t.Fatalf("former provinces page %d = %+v %v", i, pg.Page, err)
		}
		former = append(former, pg.Items...)
		if !pg.Page.HasMore {
			break
		}
		pin.Cursor = *pg.Page.NextCursor
	}
	codes := make([]string, len(former))
	for i, p := range former {
		codes[i] = p.Code
		if p.SuccessorCode == nil {
			t.Fatalf("former province %s without successor (resolved for its page)", p.Code)
		}
	}
	if len(former) != 63 || len(dedupe(codes)) != 63 {
		t.Fatalf("former provinces = %d unique=%d", len(former), len(dedupe(codes)))
	}
	if _, err := q.ListProvinces(anyUser, "VN", "inactive", query.ListInput{Cursor: "garbage"}); code(err) != problem.BadRequest {
		t.Fatalf("garbage cursor: %v (want 400)", err)
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

	// AC-18 (ADR-0095): a school-limited grant lists the students of a grade of its school only; another school of the
	// same tenant → 404, a grade outside 1…9 → 422.
	schoolAdmin := authztest.Ctx(ctx, authztest.Staff(ta, "school_admin", authztest.AtSchool(schoolA1)))
	if gs, err := q.ListTenantGradeStudents(schoolAdmin, query.GradeStudentsInput{SchoolID: schoolA1, Grade: 3}); err != nil || len(gs.Items) != 2 {
		t.Fatalf("own school grade 3 = %+v %v", gs, err)
	}
	if gs, err := q.ListTenantGradeStudents(schoolAdmin, query.GradeStudentsInput{SchoolID: schoolA1, Grade: 4}); err != nil || len(gs.Items) != 0 {
		t.Fatalf("own school grade 4 = %+v %v", gs, err)
	}
	if _, err := q.ListTenantGradeStudents(schoolAdmin, query.GradeStudentsInput{SchoolID: schoolA2, Grade: 3}); code(err) != problem.NotFound {
		t.Fatalf("another school: %v (want 404)", err)
	}
	if _, err := q.ListTenantGradeStudents(schoolAdmin, query.GradeStudentsInput{SchoolID: schoolA1, Grade: 10}); code(err) != problem.ValidationFailed {
		t.Fatalf("grade 10: %v (want 422)", err)
	}

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
	// Tenant Admin (tenant.student.read): every school of the tenant; tenant B → 404 (R8).
	if gs, err := q.ListTenantGradeStudents(adminA, query.GradeStudentsInput{SchoolID: schoolA1, Grade: 3}); err != nil || len(gs.Items) != 2 ||
		gs.Items[0].GradeLevel != 3 {
		t.Fatalf("tenant admin grade 3 = %+v %v", gs, err)
	}
	if _, err := q.ListTenantGradeStudents(authztest.Ctx(ctx, authztest.Staff(tb, "tenant_admin")), query.GradeStudentsInput{SchoolID: schoolA1, Grade: 3}); code(err) != problem.NotFound {
		t.Fatalf("tenant B lists a school of A: %v (want 404)", err)
	}

	// AC-19: the student reads their own school and grade (no class — ADR-0095); others need school.people.read in scope.
	me := authz.NewPrincipal(authz.PrincipalData{UserID: student, UserType: authz.UserTypeStudent, TenantID: &ta, Channel: authz.ChannelGame})
	prof, err := q.GetMyStudentProfile(authztest.Ctx(ctx, me))
	if err != nil || prof.SchoolID != schoolA1 || prof.GradeLevel != 3 || prof.SchoolName == "" {
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

// dedupe returns the distinct values of xs (a page walk must not repeat a row).
func dedupe(xs []string) map[string]bool {
	out := make(map[string]bool, len(xs))
	for _, x := range xs {
		out[x] = true
	}
	return out
}
