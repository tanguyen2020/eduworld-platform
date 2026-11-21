//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// F010 T047 (F056-2): listSchoolAcademicYears — live academic years of the school newest first with empty terms;
// School Admin / Academic Manager (analytics.*) of the school; tenant admin without analytics.* → 403; another school of
// the grant scope or a school of tenant B → 404; soft-deleted years are not listed.
func TestListSchoolAcademicYears(t *testing.T) {
	ctx := context.Background()
	h := newHTTPEnv(ctx, t, newKCUsers())
	now := time.Now()
	a := seedTenant(ctx, t, h.platform, "pgd-t047-a")
	b := seedTenant(ctx, t, h.platform, "pgd-t047-b")
	ta, tb := uuid.MustParse(a.tenant), uuid.MustParse(b.tenant)
	schoolA1, schoolA2 := uuid.MustParse(a.school), uuid.MustParse(a.otherSchool)
	token := func(sub string, p authz.Principal) string {
		h.res.Set(sub, p.PrincipalData)
		return h.idp.Token(sub, httpxtest.PortalClient, false, now)
	}
	schoolAdminA1 := token("kc-t047-sa1", authztest.Staff(ta, "school_admin", authztest.AtSchool(schoolA1)))
	managerA := token("kc-t047-am", authztest.Staff(ta, "academic_manager"))
	tenantAdminA := token("kc-t047-ta", authztest.Staff(ta, "tenant_admin"))
	managerB := token("kc-t047-amb", authztest.Staff(tb, "academic_manager"))
	noRoleA := token("kc-t047-none", authztest.NoRole(ta))
	hdr := func(tenant string) map[string]string { return map[string]string{"X-Tenant-Id": tenant} }
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := h.platform.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	// seedTenant made 2026-2027 (current); add an older year and a soft-deleted one.
	exec(`INSERT INTO org.academic_years (academic_year_id, tenant_id, created_at, updated_at, school_id, name, start_date, end_date, is_current)
		VALUES ($1, $2, now(), now(), $3, '2025-2026', '2025-09-01', '2026-05-31', false)`, ids.New(), a.tenant, a.school)
	exec(`INSERT INTO org.academic_years (academic_year_id, tenant_id, created_at, updated_at, school_id, name, start_date, end_date, is_current, deleted_at)
		VALUES ($1, $2, now(), now(), $3, '2024-2025', '2024-09-01', '2025-05-31', false, now())`, ids.New(), a.tenant, a.school)

	authztest.Cover(t, "listSchoolAcademicYears", authztest.CrossTenant, authztest.RoleDenied)
	path := func(school string) string { return "/schools/" + school + "/academic-years" }
	for _, tok := range []string{schoolAdminA1, managerA} {
		r := h.do(t, "GET", path(a.school), tok, hdr(a.tenant), nil)
		if r.status != 200 {
			t.Fatalf("list: %d %s", r.status, r.raw)
		}
		data, _ := r.body["data"].([]any)
		if len(data) != 2 {
			t.Fatalf("years = %s", r.raw)
		}
		cur, old := data[0].(map[string]any), data[1].(map[string]any)
		if cur["academic_year_id"] != a.year || cur["name"] != "2026-2027" || cur["start_date"] != "2026-09-01" || cur["end_date"] != "2027-05-31" ||
			cur["is_current"] != true || old["name"] != "2025-2026" || old["is_current"] != false {
			t.Fatalf("years = %s", r.raw)
		}
		if terms, ok := cur["terms"].([]any); !ok || len(terms) != 0 {
			t.Fatalf("terms = %v", cur["terms"])
		}
		if len(cur) != 6 {
			t.Fatalf("fields beyond the contract: %v", cur)
		}
	}
	// Role denied: no analytics.* (Tenant Admin, no role) → 403.
	for _, tok := range []string{tenantAdminA, noRoleA} {
		if r := h.do(t, "GET", path(a.school), tok, hdr(a.tenant), nil); r.status != 403 {
			t.Fatalf("role denied: %d %s", r.status, r.raw)
		}
	}
	// Another school of the grant scope → 404; a school of tenant B → 404 (cross tenant), also for B's own manager on A.
	if r := h.do(t, "GET", path(schoolA2.String()), schoolAdminA1, hdr(a.tenant), nil); r.status != 404 {
		t.Fatalf("other school: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", path(b.school), managerA, hdr(a.tenant), nil); r.status != 404 {
		t.Fatalf("school of tenant B: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", path(a.school), managerB, hdr(b.tenant), nil); r.status != 404 {
		t.Fatalf("tenant B on school A: %d %s", r.status, r.raw)
	}
	// Soft-deleted school → 404.
	exec(`UPDATE org.schools SET deleted_at = now() WHERE school_id = $1 AND tenant_id = $2`, a.otherSchool, a.tenant)
	if r := h.do(t, "GET", path(a.otherSchool), managerA, hdr(a.tenant), nil); r.status != 404 {
		t.Fatalf("deleted school: %d %s", r.status, r.raw)
	}
}
