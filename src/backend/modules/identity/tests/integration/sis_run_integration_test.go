//go:build integration

package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	portwrite "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/contract"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/csvreader"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// sisFiles reads CSV files (name → content) the way the import job does.
func sisFiles(t *testing.T, files map[string]string) []command.SisFile {
	t.Helper()
	v, err := contract.New()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)
	var out []command.SisFile
	for name, body := range files {
		kind := csvreader.FileKinds[name]
		b, err := csvreader.Read(kind, strings.NewReader(body), v, now)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		f := command.SisFile{Kind: csvreader.EntityKind(kind)}
		for _, r := range b.Records {
			f.Records = append(f.Records, command.SisRecord{Row: r.Row, Data: r.Data})
		}
		for _, e := range b.Errors {
			f.Errors = append(f.Errors, portwrite.RecordError{Kind: csvreader.EntityKind(kind), Row: e.Row, Column: e.Column, Code: e.Code, Message: e.Message})
		}
		out = append(out, f)
	}
	return out
}

const (
	sisClasses = "class_external_id,school_external_id,academic_year,grade_level,name\n" +
		"C-3B,TH-A,2026-2027,3,3B\n" +
		"C-4A,TH-A,2026-2027,4,4A\n"
	sisStudents = "student_external_id,school_external_id,student_code,full_name,grade_level,status,login_username,teacher_name\n" +
		"HS-1,TH-A,001,Nguyễn Văn An,3,active,hs.sis.an,Cô Lan\n" +
		"HS-2,TH-A,002,Trần Bình,4,active,hs.sis.binh,Cô Lan\n" +
		"HS-3,TH-A,003,Lê Chi,3,active,hs.sis.chi,Cô Lan\n" +
		"HS-4,TH-A,004,Phạm Dũng,3,active,hs.taken,Cô Lan\n"
	sisEnrollments = "student_external_id,class_external_id,start_date,status\n" +
		"HS-1,C-3B,2026-09-05,active\n" +
		"HS-2,C-4A,2026-09-05,active\n" +
		"HS-9,C-3B,2026-09-05,active\n"
)

// F011-AC-01…AC-05, AC-10, AC-14, AC-15, AC-23 on PostgreSQL.
func TestSisImportRun(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	a := seedTenant(ctx, t, e.platform, "pgd-sis-a")
	b := seedTenant(ctx, t, e.platform, "pgd-sis-b")
	ta, school := uuid.MustParse(a.tenant), uuid.MustParse(a.school)
	sa := authztest.Ctx(ctx, authztest.Platform("system_admin"))
	conns := command.SisConnections{UoW: e.uow, Connections: write.SisConnectionRepo{}, Audit: e.deps.Audit}
	conn, err := conns.Create(sa, ta, "csv_import", "pilot")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conns.SetSchools(sa, conn.ID, []uuid.UUID{school}, "pilot"); err != nil {
		t.Fatal(err)
	}
	enrolled(ctx, t, e.platform, a, "hs.taken") // owner of the username of HS-4
	h := command.SisRun{UoW: e.uow, Savepoints: e.uow, Connections: write.SisConnectionRepo{}, Sync: write.SisSyncRepo{},
		Enrollments: write.EnrollmentRepo{}, Missing: write.SisSyncRepo{}, Events: e.deps.Events, Audit: e.deps.Audit, BatchSize: 2}
	admin := authztest.Ctx(ctx, authztest.Staff(ta, "school_admin"))
	users := func() int {
		return count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE username LIKE 'hs.sis.%'`)
	}
	events := func(typ string) int {
		return count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = $1`, typ)
	}

	// AC-01, AC-02: dry run; a file with a violation is rejected as a whole and the others are only diffed.
	files := sisFiles(t, map[string]string{"classes.csv": sisClasses, "students.csv": sisStudents, "enrollments.csv": sisEnrollments,
		"schools.csv": "school_external_id,school_code,name,status\nTH-A,A,,active\n"})
	dry, err := h.Handle(admin, command.RunImportInput{ConnectionID: conn.ID, SchoolID: school, Files: files, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if dry.Status != "partially_failed" || len(dry.Errors) == 0 || dry.Diff["student"]["create"] != 4 || dry.Diff["class"]["create"] != 2 ||
		dry.Diff["enrollment"]["create"] != 3 || dry.Diff["school"] != nil {
		t.Fatalf("dry run = %+v", dry)
	}
	if users() != 0 || count(ctx, t, e.platform, `SELECT count(*) FROM org.classes WHERE name IN ('3B', '4A')`) != 0 {
		t.Fatal("dry run changed business data")
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM sis.staging_records WHERE sync_run_id = $1`, dry.RunID); n != 9 {
		t.Fatalf("staging = %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM sis.sync_record_errors WHERE sync_run_id = $1 AND column_name = 'name'`, dry.RunID); n != 1 {
		t.Fatalf("record errors = %d", n)
	}
	// AC-03: columns outside the contract are never stored.
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM sis.staging_records WHERE payload ? 'teacher_name'`); n != 0 {
		t.Fatalf("ignored column stored %d times", n)
	}

	// AC-04, AC-10, AC-14, AC-15: apply in order school → class → student → enrollment.
	files = sisFiles(t, map[string]string{"classes.csv": sisClasses, "students.csv": sisStudents, "enrollments.csv": sisEnrollments})
	run, err := h.Handle(admin, command.RunImportInput{ConnectionID: conn.ID, SchoolID: school, Files: files})
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "succeeded" || run.Issues != 3 {
		t.Fatalf("apply = %+v", run)
	}
	if users() != 3 || count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE username = 'hs.taken'`) != 1 {
		t.Fatal("accounts: want 3 new, the taken username untouched (AC-10)")
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.users u JOIN identity.sis_identity_links l ON l.user_id = u.user_id
		WHERE u.username LIKE 'hs.sis.%' AND u.status = 'pending' AND l.status = 'active' AND l.tenant_id = $1`, a.tenant); n != 3 {
		t.Fatalf("pending users with identity link = %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM org.enrollments WHERE source = 'sis' AND status = 'active' AND tenant_id = $1
		AND class_id IN (SELECT class_id FROM org.classes WHERE name IN ('3B', '4A'))`, a.tenant); n != 2 {
		t.Fatalf("sis enrollments = %d", n)
	}
	for typ, want := range map[string]int{"duplicate_identifier": 1, "orphan_enrollment": 1, "missing_class": 1} {
		if n := count(ctx, t, e.platform, `SELECT count(*) FROM sis.reconciliation_issues WHERE issue_type = $1 AND tenant_id = $2`, typ, a.tenant); n != want {
			t.Fatalf("issues %s = %d", typ, n)
		}
	}
	for typ, want := range map[string]int{"student.provisioning_requested": 3, "class.synced": 2, "student.synced": 3,
		"sis.reconciliation_issue_raised": 3, "sis.sync_completed": 2} {
		if n := events(typ); n != want {
			t.Fatalf("events %s = %d, want %d", typ, n, want)
		}
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' = 'student.synced'`); n != 3 {
		t.Fatalf("student.synced audits = %d", n)
	}

	// AC-05: re-importing the same data changes nothing.
	again, err := h.Handle(admin, command.RunImportInput{ConnectionID: conn.ID, SchoolID: school, Files: files})
	if err != nil {
		t.Fatal(err)
	}
	if again.Diff["class"]["noop"] != 2 || again.Diff["student"]["noop"] != 3 || again.Diff["student"]["update"] != 0 ||
		again.Diff["enrollment"]["noop"] != 2 || users() != 3 || events("student.provisioning_requested") != 3 {
		t.Fatalf("re-import = %+v", again)
	}

	// AC-23: suspended → locked; graduated → deactivated + enrollment ended, without missing_class.
	status := sisFiles(t, map[string]string{"students.csv": "student_external_id,school_external_id,student_code,full_name,grade_level,status,login_username\n" +
		"HS-1,TH-A,001,Nguyễn Văn An,3,suspended,hs.sis.an\n" +
		"HS-2,TH-A,002,Trần Bình,4,graduated,hs.sis.binh\n"})
	st, err := h.Handle(admin, command.RunImportInput{ConnectionID: conn.ID, SchoolID: school, Files: status})
	if err != nil || st.Status != "succeeded" || st.Issues != 0 {
		t.Fatalf("status run = %+v %v", st, err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE (username = 'hs.sis.an' AND status = 'locked')
		OR (username = 'hs.sis.binh' AND status = 'deactivated')`); n != 2 {
		t.Fatalf("user statuses = %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM org.enrollments e JOIN identity.users u ON u.user_id = e.student_id
		WHERE u.username = 'hs.sis.binh' AND e.status = 'ended' AND e.tenant_id = $1`, a.tenant); n != 1 {
		t.Fatalf("ended enrollments = %d", n)
	}
	if events("user.status_changed") != 2 || events("user.access_revoked") != 2 {
		t.Fatalf("status events = %d/%d", events("user.status_changed"), events("user.access_revoked"))
	}

	// Authorization and scope: the school must be served by the connection; other tenants see nothing.
	if _, err := h.Handle(admin, command.RunImportInput{ConnectionID: conn.ID, SchoolID: uuid.MustParse(a.otherSchool), Files: files}); code(err) != problem.ValidationFailed {
		t.Fatalf("school not served: %v", err)
	}
	other := authztest.Ctx(ctx, authztest.Staff(uuid.MustParse(b.tenant), "school_admin"))
	if _, err := h.Handle(other, command.RunImportInput{ConnectionID: conn.ID, SchoolID: school, Files: files}); code(err) != problem.NotFound {
		t.Fatalf("other tenant: %v", err)
	}
	authztest.ExpectDenied(t, func() error {
		_, err := h.Handle(authztest.Ctx(ctx, authztest.Staff(ta, "academic_manager")), command.RunImportInput{ConnectionID: conn.ID, SchoolID: school, Files: files})
		return err
	}())
}

// F011-AC-24: students missing from full imports are marked missing_in_source, never deactivated; a
// student missing twice in a row gets a review; above 5% in two consecutive runs one school review.
func TestSisMissingInSource(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	a := seedTenant(ctx, t, e.platform, "pgd-miss")
	ta, school := uuid.MustParse(a.tenant), uuid.MustParse(a.school)
	sa := authztest.Ctx(ctx, authztest.Platform("system_admin"))
	conns := command.SisConnections{UoW: e.uow, Connections: write.SisConnectionRepo{}, Audit: e.deps.Audit}
	conn, err := conns.Create(sa, ta, "csv_import", "pilot")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conns.SetSchools(sa, conn.ID, []uuid.UUID{school}, "pilot"); err != nil {
		t.Fatal(err)
	}
	h := command.SisRun{UoW: e.uow, Savepoints: e.uow, Connections: write.SisConnectionRepo{}, Sync: write.SisSyncRepo{},
		Enrollments: write.EnrollmentRepo{}, Missing: write.SisSyncRepo{}, Events: e.deps.Events, Audit: e.deps.Audit}
	admin := authztest.Ctx(ctx, authztest.Staff(ta, "school_admin"))
	students := func(n int) string {
		var b strings.Builder
		b.WriteString("student_external_id,school_external_id,student_code,full_name,grade_level,status,login_username\n")
		for i := 1; i <= n; i++ {
			id := fmt.Sprintf("%02d", i)
			b.WriteString("HS-" + id + ",TH-A," + id + ",Hoc Sinh " + id + ",3,active,hs.m." + id + "\n")
		}
		return b.String()
	}
	importN := func(n int) command.RunImportResult {
		t.Helper()
		res, err := h.Handle(admin, command.RunImportInput{ConnectionID: conn.ID, SchoolID: school, Files: sisFiles(t, map[string]string{"students.csv": students(n)})})
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	reviews := func() int {
		return count(ctx, t, e.platform, `SELECT count(*) FROM sis.reconciliation_issues WHERE issue_type = 'deactivation_review' AND tenant_id = $1`, a.tenant)
	}
	missingRuns := func(ext string) int {
		return count(ctx, t, e.platform, `SELECT missing_runs FROM sis.synced_students WHERE student_external_id = $1 AND tenant_id = $2`, ext, a.tenant)
	}

	importN(21)
	if r := importN(20); r.Issues != 0 || missingRuns("HS-21") != 1 { // 1/21 < 5%: mark only
		t.Fatalf("first miss = %+v runs=%d", r, missingRuns("HS-21"))
	}
	if r := importN(20); r.Issues != 1 || reviews() != 1 || missingRuns("HS-21") != 2 {
		t.Fatalf("second miss = %+v reviews=%d", r, reviews())
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE username = 'hs.m.21' AND status = 'deactivated'`); n != 0 {
		t.Fatal("a missing student was deactivated")
	}
	importN(21) // back in the source, unchanged
	if missingRuns("HS-21") != 0 {
		t.Fatalf("missing_runs after return = %d", missingRuns("HS-21"))
	}
	if r := importN(18); r.Issues != 0 { // 3/21 > 5%, first run above: mark only
		t.Fatalf("first run above = %+v", r)
	}
	if r := importN(18); r.Issues != 1 || reviews() != 2 {
		t.Fatalf("second run above = %+v reviews=%d", r, reviews())
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM sis.reconciliation_issues WHERE issue_type = 'deactivation_review'
		AND entity_refs->>'scope' = 'school' AND entity_refs->>'missing_count' = '3' AND tenant_id = $1`, a.tenant); n != 1 {
		t.Fatalf("school review = %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE username LIKE 'hs.m.%' AND status = 'deactivated'`); n != 0 {
		t.Fatalf("deactivated = %d", n)
	}
}

// F011-AC-16: a student moving between two SIS connections of the same unit tenant keeps its User (link
// moved, old link superseded, enrollment transferred); a match in another tenant needs a confirmation.
func TestSisTransferBetweenConnections(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	a := seedTenant(ctx, t, e.platform, "pgd-trf-a")
	b := seedTenant(ctx, t, e.platform, "pgd-trf-b")
	sa := authztest.Ctx(ctx, authztest.Platform("system_admin"))
	conns := command.SisConnections{UoW: e.uow, Connections: write.SisConnectionRepo{}, Audit: e.deps.Audit}
	connect := func(tenant, school string) uuid.UUID {
		t.Helper()
		c, err := conns.Create(sa, uuid.MustParse(tenant), "csv_import", "pilot")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conns.SetSchools(sa, c.ID, []uuid.UUID{uuid.MustParse(school)}, "pilot"); err != nil {
			t.Fatal(err)
		}
		return c.ID
	}
	c1, c2, c3 := connect(a.tenant, a.school), connect(a.tenant, a.otherSchool), connect(b.tenant, b.school)
	h := command.SisRun{UoW: e.uow, Savepoints: e.uow, Connections: write.SisConnectionRepo{}, Sync: write.SisSyncRepo{},
		Enrollments: write.EnrollmentRepo{}, Missing: write.SisSyncRepo{}, Transfers: write.SisSyncRepo{}, Students: write.StudentRepo{},
		Events: e.deps.Events, Audit: e.deps.Audit, IDHashKey: []byte("test-only-key")}
	run := func(tenant, school string, conn uuid.UUID, files map[string]string) command.RunImportResult {
		t.Helper()
		ctx := authztest.Ctx(ctx, authztest.Staff(uuid.MustParse(tenant), "school_admin"))
		res, err := h.Handle(ctx, command.RunImportInput{ConnectionID: conn, SchoolID: uuid.MustParse(school), Files: sisFiles(t, files)})
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	student := func(ext, school, username string) string {
		return "student_external_id,school_external_id,student_code,full_name,grade_level,status,login_username,national_student_id\n" +
			ext + "," + school + ",001,Nguyen An,3,active," + username + ",0123 4567 8901\n"
	}
	files := func(ext, school, class, username string) map[string]string {
		return map[string]string{
			"classes.csv":     "class_external_id,school_external_id,academic_year,grade_level,name\n" + class + "," + school + ",2026-2027,3," + class + "\n",
			"students.csv":    student(ext, school, username),
			"enrollments.csv": "student_external_id,class_external_id,start_date,status\n" + ext + "," + class + ",2026-09-05,active\n",
		}
	}
	if r := run(a.tenant, a.school, c1, files("HS-1", "TH-A", "C-A3", "hs.t.an")); r.Issues != 0 {
		t.Fatalf("first school = %+v", r)
	}
	var an uuid.UUID
	if err := e.platform.QueryRowContext(ctx, `SELECT user_id FROM identity.users WHERE username = 'hs.t.an'`).Scan(&an); err != nil {
		t.Fatal(err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM sis.synced_students WHERE national_student_id_hash IS NOT NULL
		AND national_student_id_hash NOT LIKE '%0123%' AND tenant_id = $1`, a.tenant); n != 1 {
		t.Fatal("the national id must be stored as a keyed hash only")
	}

	// Same unit tenant, other SIS: link moved, user kept, enrollment transferred to the new school.
	if r := run(a.tenant, a.otherSchool, c2, files("HS-77", "TH-B", "C-B3", "hs.t.an.new")); r.Issues != 0 || r.Status != "succeeded" {
		t.Fatalf("second school = %+v", r)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE username = 'hs.t.an.new'`); n != 0 {
		t.Fatal("a second account was created")
	}
	for q, want := range map[string]int{
		`SELECT count(*) FROM identity.sis_identity_links WHERE user_id = $1 AND status = 'superseded' AND student_external_id = 'HS-1' AND superseded_at IS NOT NULL`: 1,
		`SELECT count(*) FROM identity.sis_identity_links WHERE user_id = $1 AND status = 'active' AND student_external_id = 'HS-77'`:                                  1,
		`SELECT count(*) FROM org.students WHERE student_id = $1 AND school_id = '` + a.otherSchool + `'`:                                                            1,
		`SELECT count(*) FROM org.enrollments WHERE student_id = $1 AND status = 'active' AND school_id = '` + a.otherSchool + `'`:                                    1,
		`SELECT count(*) FROM org.enrollments WHERE student_id = $1 AND status = 'transferred'`:                                                                      1,
	} {
		if n := count(ctx, t, e.platform, q, an); n != want {
			t.Fatalf("%s = %d", q, n)
		}
	}
	for typ, want := range map[string]int{"student.sis_link_superseded": 1, "student.transferred": 1} {
		if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = $1`, typ); n != want {
			t.Fatalf("%s = %d", typ, n)
		}
	}

	// Another tenant: no automatic move, transfer_confirmation, no account.
	if r := run(b.tenant, b.school, c3, files("HS-5", "TH-C", "C-C3", "hs.t.an.other")); r.Issues < 1 {
		t.Fatalf("other tenant = %+v", r)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM sis.reconciliation_issues WHERE issue_type = 'transfer_confirmation' AND tenant_id = $1`, b.tenant); n != 1 {
		t.Fatalf("transfer_confirmation = %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE username = 'hs.t.an.other'`); n != 0 {
		t.Fatal("an account was created for a cross-tenant match")
	}
}
