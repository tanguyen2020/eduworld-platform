//go:build integration

package integration

import (
	"context"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// F052-1 (owners F010, F011): the school-scoped public routes of Workspace C through the httpx chain —
// listTenantClassStudents, listSchoolReconciliationIssues, resolveSchoolReconciliationIssue: tenant isolation
// (tenant B → 404), School Admin limited to their school (another school → 404), missing permission → 403,
// only the ClassStudent fields (R16), Idempotency-Key replay and the resolution audit.
func TestSchoolScopedRoutes(t *testing.T) {
	ctx := context.Background()
	h := newHTTPEnv(ctx, t, newKCUsers())
	now := time.Now()
	a := seedTenant(ctx, t, h.platform, "pgd-f052-a")
	b := seedTenant(ctx, t, h.platform, "pgd-f052-b")
	ta, tb := uuid.MustParse(a.tenant), uuid.MustParse(b.tenant)
	schoolA1, schoolA2 := uuid.MustParse(a.school), uuid.MustParse(a.otherSchool)

	token := func(sub string, p authz.Principal) string {
		h.res.Set(sub, p.PrincipalData)
		return h.idp.Token(sub, httpxtest.PortalClient, now)
	}
	tenantAdminA := token("kc-f052-ta", authztest.Staff(ta, "tenant_admin"))
	schoolAdminA1 := token("kc-f052-sa1", authztest.Staff(ta, "school_admin", authztest.AtSchool(schoolA1)))
	schoolAdminA2 := token("kc-f052-sa2", authztest.Staff(ta, "school_admin", authztest.AtSchool(schoolA2)))
	tenantAdminB := token("kc-f052-tb", authztest.Staff(tb, "tenant_admin"))
	noRoleA := token("kc-f052-none", authztest.NoRole(ta))
	hdr := func(tenant string) map[string]string { return map[string]string{"X-Tenant-Id": tenant} }
	key := func(tenant string) map[string]string {
		return map[string]string{"X-Tenant-Id": tenant, "Idempotency-Key": ids.New().String()}
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := h.platform.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	// A CSV connection of school A1 and a run raising two issues (duplicate_identifier, orphan_enrollment).
	sa := authztest.Ctx(ctx, authztest.Platform("system_admin"))
	conns := command.SisConnections{UoW: h.uow, Connections: write.SisConnectionRepo{}, Audit: h.deps.Audit}
	conn, err := conns.Create(sa, ta, "csv_import", "pilot")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conns.SetSchools(sa, conn.ID, []uuid.UUID{schoolA1}, "pilot"); err != nil {
		t.Fatal(err)
	}
	enrolled(ctx, t, h.platform, a, "hs.f052.dup")
	run := command.SisRun{UoW: h.uow, Savepoints: h.uow, Connections: write.SisConnectionRepo{}, Sync: write.SisSyncRepo{},
		Enrollments: write.EnrollmentRepo{}, Missing: write.SisSyncRepo{}, Rules: write.SisSyncRepo{}, Events: h.deps.Events, Audit: h.deps.Audit}
	res, err := run.Handle(authztest.Ctx(ctx, authztest.Staff(ta, "school_admin")), command.RunImportInput{ConnectionID: conn.ID, SchoolID: schoolA1,
		Files: sisFiles(t, map[string]string{
			"classes.csv": "class_external_id,school_external_id,academic_year,grade_level,name\nC-3B,TH-A,2026-2027,3,3B\n",
			"students.csv": "student_external_id,school_external_id,student_code,full_name,grade_level,status,login_username\n" +
				"HS-1,TH-A,001,An,3,active,hs.f052.an\nHS-4,TH-A,004,Dung,3,active,hs.f052.dup\n",
			"enrollments.csv": "student_external_id,class_external_id,start_date,status\nHS-1,C-3B,2026-09-05,active\nHS-9,C-3B,2026-09-05,active\n",
		})})
	if err != nil || res.Issues != 2 {
		t.Fatalf("import = %+v %v", res, err)
	}
	// An import whose run raised them, an import of school A2 and still-unprocessed import of A1.
	importA1, importA2, pending := ids.New().String(), ids.New().String(), ids.New().String()
	for _, imp := range [][3]any{{importA1, a.school, res.RunID}, {importA2, a.otherSchool, nil}, {pending, a.school, nil}} {
		exec(`INSERT INTO sis.sis_imports (import_id, tenant_id, created_at, updated_at, school_id, requested_by, entity_kinds, status, sync_run_id, stats)
			VALUES ($1, $2, now(), now(), $3, $4, '["students"]', 'completed', $5, '{}')`, imp[0], a.tenant, imp[1], ids.New(), imp[2])
	}
	// One issue of school A2 and one of tenant B (no run).
	issueA2, issueB := ids.New().String(), ids.New().String()
	for _, iss := range [][3]string{{issueA2, a.tenant, a.otherSchool}, {issueB, b.tenant, b.school}} {
		exec(`INSERT INTO sis.reconciliation_issues (issue_id, tenant_id, created_at, updated_at, school_id, issue_type, entity_refs, status)
			VALUES ($1, $2, now(), now(), $3, 'missing_class', '{}', 'open')`, iss[0], iss[1], iss[2])
	}

	// ---------- listTenantClassStudents ----------
	authztest.Cover(t, "listTenantClassStudents", authztest.CrossTenant, authztest.RoleDenied)
	class3C := addClass(ctx, t, h.platform, a.tenant, a.school, "3C")
	tx, err := h.platform.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var roster []string
	for _, u := range []string{"hs.f052.r.an", "hs.f052.r.binh", "hs.f052.r.gone"} {
		id, err := insertStudent(ctx, tx, a, u)
		if err == nil {
			err = enroll(ctx, tx, a, id, a.school, class3C.String())
		}
		if err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
		roster = append(roster, id)
	}
	if _, err := insertStudent(ctx, tx, a, "hs.f052.r.free"); err != nil { // no enrollment: not in the roster
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	an, binh, gone := roster[0], roster[1], roster[2]
	exec(`UPDATE identity.users SET display_name = 'An Nguyen', last_login_at = now() - interval '3 days 1 hour' WHERE user_id = $1`, an)
	exec(`UPDATE identity.users SET display_name = 'Binh Tran' WHERE user_id = $1`, binh)
	exec(`UPDATE org.students SET student_code = 'HS0042' WHERE student_id = $1 AND tenant_id = $2`, binh, a.tenant)
	exec(`UPDATE identity.users SET deleted_at = now() WHERE user_id = $1`, gone) // soft-deleted: not in the roster
	exec(`INSERT INTO identity.sis_identity_links (link_id, tenant_id, created_at, updated_at, user_id, school_id, sis_connection_id, student_external_id, linked_at)
		VALUES ($1, $2, now(), now(), $3, $4, $5, 'HS-B', now())`, ids.New(), a.tenant, binh, a.school, conn.ID)

	students := "/tenant/schools/" + a.school + "/classes/" + class3C.String() + "/students"
	r := h.do(t, "GET", students, tenantAdminA, hdr(a.tenant), nil)
	if r.status != 200 {
		t.Fatalf("roster: %d %s", r.status, r.raw)
	}
	items := r.body["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("roster = %s", r.raw)
	}
	first, second := items[0].(map[string]any), items[1].(map[string]any)
	if first["student_id"] != an || first["display_name"] != "An Nguyen" || first["days_since_last_login"] != float64(3) ||
		first["last_login_at"] == nil || first["source"] != "manual" || first["status"] != "pending" {
		t.Fatalf("first = %v", first)
	}
	if second["student_id"] != binh || second["student_code"] != "HS0042" || second["source"] != "sis" ||
		second["last_login_at"] != nil || second["days_since_last_login"] != nil {
		t.Fatalf("second = %v", second)
	}
	// R16: exactly the ClassStudent fields, never the real name, birth date or contacts.
	var fields []string
	for k := range first {
		fields = append(fields, k)
	}
	sort.Strings(fields)
	if got := strings.Join(fields, ","); got != "days_since_last_login,display_name,last_login_at,source,status,student_code,student_id,username" {
		t.Fatalf("ClassStudent fields = %s", got)
	}
	if strings.Contains(r.raw, "Hoc Sinh") || strings.Contains(r.raw, "date_of_birth") || strings.Contains(r.raw, "email") {
		t.Fatalf("roster leaks student data: %s", r.raw)
	}
	// Keyset paging and search (display name, username, student code).
	p1 := h.do(t, "GET", students+"?limit=1", tenantAdminA, hdr(a.tenant), nil)
	pg := p1.body["page"].(map[string]any)
	if p1.status != 200 || pg["has_more"] != true {
		t.Fatalf("page 1: %d %s", p1.status, p1.raw)
	}
	p2 := h.do(t, "GET", students+"?limit=1&cursor="+url.QueryEscape(pg["next_cursor"].(string)), tenantAdminA, hdr(a.tenant), nil)
	if p2.status != 200 || len(p2.body["items"].([]any)) != 1 || p2.body["items"].([]any)[0].(map[string]any)["student_id"] != binh {
		t.Fatalf("page 2: %d %s", p2.status, p2.raw)
	}
	for _, q := range []string{"binh", "HS00", "f052.r.an"} {
		s := h.do(t, "GET", students+"?q="+url.QueryEscape(q), tenantAdminA, hdr(a.tenant), nil)
		if s.status != 200 || len(s.body["items"].([]any)) != 1 {
			t.Fatalf("q=%s: %d %s", q, s.status, s.raw)
		}
	}
	if s := h.do(t, "GET", students+"?q=%25%25", tenantAdminA, hdr(a.tenant), nil); s.status != 200 || len(s.body["items"].([]any)) != 0 {
		t.Fatalf("LIKE wildcards must be literal: %d %s", s.status, s.raw)
	}
	// School Admin of the school (lms.student.read) reads it; of another school → 404; tenant B → 404; no role → 403.
	if r := h.do(t, "GET", students, schoolAdminA1, hdr(a.tenant), nil); r.status != 200 || len(r.body["items"].([]any)) != 2 {
		t.Fatalf("school admin A1: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", students, schoolAdminA2, hdr(a.tenant), nil); r.status != 404 {
		t.Fatalf("school admin A2: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", students, tenantAdminB, hdr(b.tenant), nil); r.status != 404 {
		t.Fatalf("tenant B: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", students, noRoleA, hdr(a.tenant), nil); r.status != 403 {
		t.Fatalf("no role: %d %s", r.status, r.raw)
	}
	// A class of another school under this school's path → 404.
	if r := h.do(t, "GET", "/tenant/schools/"+a.school+"/classes/"+a.otherClass+"/students", tenantAdminA, hdr(a.tenant), nil); r.status != 404 {
		t.Fatalf("class of another school: %d %s", r.status, r.raw)
	}

	// ---------- listSchoolReconciliationIssues ----------
	authztest.Cover(t, "listSchoolReconciliationIssues", authztest.CrossTenant, authztest.RoleDenied)
	issues := "/schools/" + a.school + "/reconciliation-issues"
	r = h.do(t, "GET", issues+"?status=open", schoolAdminA1, hdr(a.tenant), nil) // tenant.sis.import, school-limited
	if r.status != 200 || len(r.body["data"].([]any)) != 2 || strings.Contains(r.raw, issueA2) || strings.Contains(r.raw, issueB) {
		t.Fatalf("school A1 issues: %d %s", r.status, r.raw)
	}
	var dupIssue, orphan string
	for _, it := range r.body["data"].([]any) {
		i := it.(map[string]any)
		switch i["issue_type"] {
		case "duplicate_identifier":
			dupIssue = i["issue_id"].(string)
			if _, ok := i["entity_refs"].(map[string]any)["user_id"]; ok {
				t.Fatal("the conflicting account is shown on the school route")
			}
		case "orphan_enrollment":
			orphan = i["issue_id"].(string)
		}
	}
	if dupIssue == "" || orphan == "" {
		t.Fatalf("issues = %s", r.raw)
	}
	if r := h.do(t, "GET", issues+"?issue_type=orphan_enrollment", tenantAdminA, hdr(a.tenant), nil); r.status != 200 || len(r.body["data"].([]any)) != 1 {
		t.Fatalf("by type: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", issues+"?import_id="+importA1, schoolAdminA1, hdr(a.tenant), nil); r.status != 200 || len(r.body["data"].([]any)) != 2 {
		t.Fatalf("by import: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", issues+"?import_id="+pending, schoolAdminA1, hdr(a.tenant), nil); r.status != 200 || len(r.body["data"].([]any)) != 0 {
		t.Fatalf("unprocessed import: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", issues+"?import_id="+importA2, tenantAdminA, hdr(a.tenant), nil); r.status != 404 {
		t.Fatalf("import of another school: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", "/schools/"+a.otherSchool+"/reconciliation-issues", tenantAdminA, hdr(a.tenant), nil); r.status != 200 ||
		len(r.body["data"].([]any)) != 1 || !strings.Contains(r.raw, issueA2) {
		t.Fatalf("tenant admin, school A2: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", issues, schoolAdminA2, hdr(a.tenant), nil); r.status != 404 {
		t.Fatalf("school admin A2 on A1: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", issues, tenantAdminB, hdr(b.tenant), nil); r.status != 404 {
		t.Fatalf("tenant B: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", "/schools/"+ids.New().String()+"/reconciliation-issues", tenantAdminA, hdr(a.tenant), nil); r.status != 404 {
		t.Fatalf("unknown school: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", issues, noRoleA, hdr(a.tenant), nil); r.status != 403 {
		t.Fatalf("no role: %d %s", r.status, r.raw)
	}

	// ---------- resolveSchoolReconciliationIssue ----------
	authztest.Cover(t, "resolveSchoolReconciliationIssue", authztest.CrossTenant, authztest.RoleDenied)
	resolve := func(school, issue string) string {
		return "/schools/" + school + "/reconciliation-issues/" + issue + ":resolve"
	}
	fixed := map[string]any{"resolution": "fixed_in_source", "reason": "Đã sửa lớp trong SIS"}
	if r := h.do(t, "POST", resolve(a.school, orphan), schoolAdminA1, hdr(a.tenant), fixed); r.status != 428 {
		t.Fatalf("no Idempotency-Key: %d %s", r.status, r.raw)
	}
	for name, body := range map[string]map[string]any{
		"same_person":  {"resolution": "same_person", "reason": "cùng một người"},
		"short reason": {"resolution": "ignore", "reason": "x"},
		"no reason":    {"resolution": "ignore"},
	} {
		if r := h.do(t, "POST", resolve(a.school, orphan), schoolAdminA1, key(a.tenant), body); r.status != 422 {
			t.Fatalf("%s: %d %s", name, r.status, r.raw)
		}
	}
	if r := h.do(t, "POST", resolve(a.school, orphan), schoolAdminA2, key(a.tenant), fixed); r.status != 404 {
		t.Fatalf("school admin A2 on A1 path: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "POST", resolve(a.otherSchool, orphan), schoolAdminA2, key(a.tenant), fixed); r.status != 404 {
		t.Fatalf("issue of A1 under the A2 path: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "POST", resolve(a.otherSchool, orphan), tenantAdminA, key(a.tenant), fixed); r.status != 404 {
		t.Fatalf("tenant admin, issue of another school: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "POST", resolve(a.school, orphan), tenantAdminB, key(b.tenant), fixed); r.status != 404 {
		t.Fatalf("tenant B: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "POST", resolve(a.school, orphan), noRoleA, key(a.tenant), fixed); r.status != 403 {
		t.Fatalf("no role: %d %s", r.status, r.raw)
	}
	k := key(a.tenant)
	done := h.do(t, "POST", resolve(a.school, orphan), schoolAdminA1, k, fixed)
	if done.status != 200 || done.body["status"] != "resolved" || done.body["resolution"] != "fixed_in_source" ||
		done.body["resolution_note"] != "Đã sửa lớp trong SIS" || done.body["resolved_by"] == nil || done.body["resolved_at"] == nil {
		t.Fatalf("resolve: %d %s", done.status, done.raw)
	}
	replay := h.do(t, "POST", resolve(a.school, orphan), schoolAdminA1, k, fixed)
	if replay.status != 200 || replay.header.Get("Idempotent-Replayed") != "true" || replay.body["resolved_at"] != done.body["resolved_at"] {
		t.Fatalf("replay: %d %v %s", replay.status, replay.header, replay.raw)
	}
	if r := h.do(t, "POST", resolve(a.school, orphan), schoolAdminA1, key(a.tenant), fixed); r.status != 409 {
		t.Fatalf("resolve twice: %d %s", r.status, r.raw)
	}
	ign := h.do(t, "POST", resolve(a.school, dupIssue), tenantAdminA, key(a.tenant), map[string]any{"resolution": "ignore", "reason": "  khác người  "})
	if ign.status != 200 || ign.body["status"] != "ignored" || ign.body["resolution_note"] != "khác người" {
		t.Fatalf("ignore: %d %s", ign.status, ign.raw)
	}
	if _, ok := ign.body["entity_refs"].(map[string]any)["user_id"]; ok {
		t.Fatal("the conflicting account is returned on the school route")
	}
	if n := count(ctx, t, h.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' = 'sis.reconciliation_issue.resolved'
		AND envelope->'payload'->'metadata'->>'route' = 'school'`); n != 2 {
		t.Fatalf("school resolution audits = %d", n)
	}
	if n := count(ctx, t, h.platform, `SELECT count(*) FROM sis.reconciliation_issues WHERE tenant_id = $1 AND status = 'open' AND deleted_at IS NULL`, a.tenant); n != 1 {
		t.Fatalf("open issues of A = %d (only the A2 issue)", n)
	}
}
