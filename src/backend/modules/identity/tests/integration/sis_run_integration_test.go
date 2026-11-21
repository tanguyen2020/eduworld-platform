//go:build integration

package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	portwrite "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/contract"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/csvreader"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/metrics"
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
		if k, ok := csvreader.IgnoredFiles[name]; ok { // SIS contract 1.3.0: the run ignores the file with a warning
			f := command.SisFile{Kind: k}
			for i := 1; i < len(strings.Split(strings.TrimSpace(body), "\n")); i++ {
				f.Records = append(f.Records, command.SisRecord{Row: i + 1, Data: map[string]any{}})
			}
			out = append(out, f)
			continue
		}
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

// F011-AC-01…AC-05, AC-10, AC-14, AC-23 on PostgreSQL; SIS contract 1.3.0 (ADR-0095 D6): classes.csv and enrollments.csv
// are ignored with a warning, every student record places the student in its school and grade.
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
		Students: write.StudentRepo{}, Missing: write.SisSyncRepo{}, Rules: write.SisSyncRepo{}, Events: e.deps.Events, Audit: e.deps.Audit, BatchSize: 2,
		Observer: metrics.Runs{}}
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
	if dry.Status != "partially_failed" || len(dry.Errors) == 0 || dry.Diff["student"]["create"] != 4 || dry.Diff["class"] != nil ||
		dry.Diff["enrollment"] != nil || dry.Diff["school"] != nil {
		t.Fatalf("dry run = %+v", dry)
	}
	if users() != 0 || count(ctx, t, e.platform, `SELECT count(*) FROM org.classes WHERE name IN ('3B', '4A')`) != 0 {
		t.Fatal("dry run changed business data")
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM sis.staging_records WHERE sync_run_id = $1`, dry.RunID); n != 4 {
		t.Fatalf("staging = %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM sis.sync_record_errors WHERE sync_run_id = $1 AND column_name = 'name'`, dry.RunID); n != 1 {
		t.Fatalf("record errors = %d", n)
	}
	// AC-03: columns outside the contract are never stored.
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM sis.staging_records WHERE payload ? 'teacher_name'`); n != 0 {
		t.Fatalf("ignored column stored %d times", n)
	}

	// AC-04, AC-10, AC-14: apply in order school → student; the student record places the student (ADR-0095).
	files = sisFiles(t, map[string]string{"classes.csv": sisClasses, "students.csv": sisStudents, "enrollments.csv": sisEnrollments})
	run, err := h.Handle(admin, command.RunImportInput{ConnectionID: conn.ID, SchoolID: school, Files: files})
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "succeeded" || run.Issues != 1 {
		t.Fatalf("apply = %+v", run)
	}
	if testutil.ToFloat64(metrics.LastSuccess().WithLabelValues(conn.ID.String())) < float64(time.Now().Add(-time.Minute).Unix()) {
		t.Fatal("eduworld_sis_last_success_timestamp not set by a succeeded run")
	}
	if users() != 3 || count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE username = 'hs.taken'`) != 1 {
		t.Fatal("accounts: want 3 new, the taken username untouched (AC-10)")
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.users u JOIN identity.sis_identity_links l ON l.user_id = u.user_id
		WHERE u.username LIKE 'hs.sis.%' AND u.status = 'pending' AND l.status = 'active' AND l.tenant_id = $1`, a.tenant); n != 3 {
		t.Fatalf("pending users with identity link = %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM org.students s JOIN identity.users u ON u.user_id = s.student_id
		WHERE u.username LIKE 'hs.sis.%' AND s.status = 'active' AND s.school_id = $1 AND s.tenant_id = $2
		AND ((u.username = 'hs.sis.binh' AND s.grade_level = 4) OR (u.username <> 'hs.sis.binh' AND s.grade_level = 3))`, a.school, a.tenant); n != 3 {
		t.Fatalf("placed students = %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM org.enrollments WHERE tenant_id = $1`, a.tenant); n != 0 {
		t.Fatalf("enrollments written = %d (none since ADR-0095)", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM sis.sync_runs WHERE sync_run_id = $1 AND (stats->>'class_ignored')::int = 2
		AND (stats->>'enrollment_ignored')::int = 3`, run.RunID); n != 1 {
		t.Fatal("ignored class/enrollment records are counted in the run stats (warning)")
	}
	for typ, want := range map[string]int{"duplicate_identifier": 1, "orphan_enrollment": 0, "missing_class": 0} {
		if n := count(ctx, t, e.platform, `SELECT count(*) FROM sis.reconciliation_issues WHERE issue_type = $1 AND tenant_id = $2`, typ, a.tenant); n != want {
			t.Fatalf("issues %s = %d", typ, n)
		}
	}
	for typ, want := range map[string]int{"student.provisioning_requested": 3, "class.synced": 0, "enrollment.changed": 0,
		"student.synced": 3, "student.placement_changed": 3, "sis.reconciliation_issue_raised": 1, "sis.sync_completed": 2} {
		if n := events(typ); n != want {
			t.Fatalf("events %s = %d, want %d", typ, n, want)
		}
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' = 'student.synced'`); n != 3 {
		t.Fatalf("student.synced audits = %d", n)
	}
	// ADR-0095: student.placement_changed of a new SIS student — first placement (every field changed), source sis, the
	// current academic year of the school, version of the student row (schema-valid).
	placements := syncedPayloads(ctx, t, e, "student.placement_changed", a.tenant)
	if len(placements) != 3 {
		t.Fatalf("student.placement_changed payloads = %d", len(placements))
	}
	for _, p := range placements {
		if p["source"] != "sis" || p["school_id"] != a.school || p["status"] != "active" || p["previous_school_id"] != nil ||
			len(p["changed_fields"].([]any)) != 4 || p["academic_year_id"] != a.year || p["sync_run_id"] != run.RunID.String() {
			t.Fatalf("student.placement_changed = %v", p)
		}
	}

	// AC-05: re-importing the same data changes nothing.
	again, err := h.Handle(admin, command.RunImportInput{ConnectionID: conn.ID, SchoolID: school, Files: files})
	if err != nil {
		t.Fatal(err)
	}
	if again.Diff["student"]["noop"] != 3 || again.Diff["student"]["update"] != 0 || users() != 3 ||
		events("student.provisioning_requested") != 3 || events("student.placement_changed") != 3 {
		t.Fatalf("re-import = %+v", again)
	}

	// AC-23: suspended → locked (the student keeps its school and grade); graduated → deactivated + the student is no longer
	// effective (student.placement_changed status inactive).
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
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM org.students s JOIN identity.users u ON u.user_id = s.student_id
		WHERE ((u.username = 'hs.sis.binh' AND s.status = 'inactive') OR (u.username = 'hs.sis.an' AND s.status = 'active')) AND s.tenant_id = $1`,
		a.tenant); n != 2 {
		t.Fatalf("student statuses = %d", n)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'student.placement_changed'
		AND envelope->'payload'->>'status' = 'inactive' AND envelope->'payload'->>'previous_status' = 'active'`); n != 1 {
		t.Fatalf("inactive placements = %d", n)
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
		Students: write.StudentRepo{}, Missing: write.SisSyncRepo{}, Rules: write.SisSyncRepo{}, Events: e.deps.Events, Audit: e.deps.Audit}
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

	// T036: sis-staging-anonymizer clears payloads past sis_staging_retention only, without deleting rows.
	staged := count(ctx, t, e.platform, `SELECT count(*) FROM sis.staging_records`)
	anon := command.AnonymizeSisStaging{UoW: e.uow, Staging: write.SisSyncRepo{}, Batch: 50}
	if n, err := anon.RunOnce(ctx); err != nil || n != 0 {
		t.Fatalf("fresh payloads cleared: %d %v", n, err)
	}
	anon.Clock = func() time.Time { return time.Now().Add(command.SisStagingRetention + time.Hour) }
	if n, err := anon.RunOnce(ctx); err != nil || n != staged {
		t.Fatalf("anonymized = %d of %d: %v", n, staged, err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM sis.staging_records WHERE payload IS NULL`); n != staged {
		t.Fatalf("rows kept with null payload = %d of %d", n, staged)
	}
}

// F011-AC-16: a student moving between two SIS connections of the same unit tenant keeps its User (link
// moved, old link superseded, school moved — ADR-0095); a match in another tenant needs a confirmation.
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
		Missing: write.SisSyncRepo{}, Rules: write.SisSyncRepo{}, Transfers: write.SisSyncRepo{}, Students: write.StudentRepo{},
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
	files := func(ext, school, username string) map[string]string {
		return map[string]string{"students.csv": student(ext, school, username)}
	}
	if r := run(a.tenant, a.school, c1, files("HS-1", "TH-A", "hs.t.an")); r.Issues != 0 {
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

	// Same unit tenant, other SIS: link moved, user kept, student moved to the new school.
	if r := run(a.tenant, a.otherSchool, c2, files("HS-77", "TH-B", "hs.t.an.new")); r.Issues != 0 || r.Status != "succeeded" {
		t.Fatalf("second school = %+v", r)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE username = 'hs.t.an.new'`); n != 0 {
		t.Fatal("a second account was created")
	}
	for q, want := range map[string]int{
		`SELECT count(*) FROM identity.sis_identity_links WHERE user_id = $1 AND status = 'superseded' AND student_external_id = 'HS-1' AND superseded_at IS NOT NULL`: 1,
		`SELECT count(*) FROM identity.sis_identity_links WHERE user_id = $1 AND status = 'active' AND student_external_id = 'HS-77'`:                                  1,
		`SELECT count(*) FROM org.students WHERE student_id = $1 AND school_id = '` + a.otherSchool + `'`:                                                              1,
		`SELECT count(*) FROM identity.tenant_memberships_v WHERE user_id = $1 AND school_id = '` + a.otherSchool + `'`:                                                1,
		`SELECT count(*) FROM org.enrollments WHERE student_id = $1`:                                                                                                   0,
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
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'student.placement_changed'
		AND envelope->'payload'->>'student_id' = $1 AND envelope->'payload'->>'previous_school_id' = $2
		AND envelope->'payload'->>'school_id' = $3`, an.String(), a.school, a.otherSchool); n != 1 {
		t.Fatal("student.placement_changed of the school move")
	}

	// Another tenant: no automatic move, transfer_confirmation, no account.
	if r := run(b.tenant, b.school, c3, files("HS-5", "TH-C", "hs.t.an.other")); r.Issues < 1 {
		t.Fatalf("other tenant = %+v", r)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM sis.reconciliation_issues WHERE issue_type = 'transfer_confirmation' AND tenant_id = $1`, b.tenant); n != 1 {
		t.Fatalf("transfer_confirmation = %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE username = 'hs.t.an.other'`); n != 0 {
		t.Fatal("an account was created for a cross-tenant match")
	}
}

// F011-AC-14: the remaining types of the conflict matrix: possible_duplicate, school_province_missing, unmapped_school —
// each skips the record and raises an issue (class_school_mismatch, multi_school_enrollment are gone with ADR-0095).
func TestSisConflictMatrix(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	a := seedTenant(ctx, t, e.platform, "pgd-matrix")
	ta, sA, sB := uuid.MustParse(a.tenant), uuid.MustParse(a.school), uuid.MustParse(a.otherSchool)
	sa := authztest.Ctx(ctx, authztest.Platform("system_admin"))
	conns := command.SisConnections{UoW: e.uow, Connections: write.SisConnectionRepo{}, Audit: e.deps.Audit}
	conn, err := conns.Create(sa, ta, "csv_import", "pilot")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conns.SetSchools(sa, conn.ID, []uuid.UUID{sA, sB}, "pilot"); err != nil {
		t.Fatal(err)
	}
	h := command.SisRun{UoW: e.uow, Savepoints: e.uow, Connections: write.SisConnectionRepo{}, Sync: write.SisSyncRepo{},
		Missing: write.SisSyncRepo{}, Rules: write.SisSyncRepo{}, Transfers: write.SisSyncRepo{},
		Students: write.StudentRepo{}, Events: e.deps.Events, Audit: e.deps.Audit}
	admin := authztest.Ctx(ctx, authztest.Staff(ta, "school_admin"))
	run := func(school uuid.UUID, files map[string]string) command.RunImportResult {
		t.Helper()
		res, err := h.Handle(admin, command.RunImportInput{ConnectionID: conn.ID, SchoolID: school, Files: sisFiles(t, files)})
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	issues := func(typ string) int {
		return count(ctx, t, e.platform, `SELECT count(*) FROM sis.reconciliation_issues WHERE issue_type = $1 AND tenant_id = $2`, typ, a.tenant)
	}
	const stHeader = "student_external_id,school_external_id,student_code,full_name,grade_level,status,login_username\n"

	// School A: school and student.
	run(sA, map[string]string{
		"schools.csv":  "school_external_id,school_code,name,status,province_code\nTH-A,A,Truong A,active,vn.ha-noi\n",
		"students.csv": stHeader + "HS-1,TH-A,001,An,3,active,hs.mx.an\n",
	})
	// school_province_missing: an unknown province code.
	run(sA, map[string]string{"schools.csv": "school_external_id,school_code,name,status,province_code\nTH-A2,A2,Truong A2,active,vn.khong-co\n"})
	// possible_duplicate: a new source key with the student_code of HS-1 in the same school.
	run(sA, map[string]string{"students.csv": stHeader + "HS-99,TH-A,001,An Copy,3,active,hs.mx.copy\n"})
	// School B: unmapped_school (TH-A belongs to school A).
	run(sB, map[string]string{
		"students.csv": stHeader + "HS-2,TH-A,002,Binh,3,active,hs.mx.binh\nHS-3,TH-B,003,Chi,3,active,hs.mx.chi\n",
	})
	for typ, want := range map[string]int{"school_province_missing": 1, "possible_duplicate": 1, "unmapped_school": 1,
		"class_school_mismatch": 0, "multi_school_enrollment": 0} {
		if n := issues(typ); n != want {
			t.Fatalf("%s = %d, want %d", typ, n, want)
		}
	}
	// F011 T040: school.synced carries the version of the school (schema-valid).
	var schoolVersion float64
	if err := e.platform.QueryRowContext(ctx, `SELECT version FROM org.schools WHERE school_id = $1 AND tenant_id = $2`, sA, a.tenant).Scan(&schoolVersion); err != nil {
		t.Fatal(err)
	}
	if ps := syncedPayloads(ctx, t, e, "school.synced", a.tenant); len(ps) != 1 || ps[0]["source_version"] != schoolVersion {
		t.Fatalf("school.synced = %v (version %v)", ps, schoolVersion)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE username IN ('hs.mx.copy', 'hs.mx.binh')`); n != 0 {
		t.Fatalf("skipped records created %d accounts", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM org.students s JOIN identity.users u ON u.user_id = s.student_id
		WHERE u.username = 'hs.mx.an' AND s.status = 'active' AND s.school_id = $1`, a.school); n != 1 {
		t.Fatal("HS-1 stays in school A")
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'sis.reconciliation_issue_raised'`); n < 3 {
		t.Fatalf("issue events = %d", n)
	}

	// AC-25: a student cleaned up for inactivity (ADR-0032) is never restored by a sync — through its SIS link
	// or through its username — and cleaned_up_student_in_sis is raised.
	if _, err := e.platform.ExecContext(ctx, `UPDATE identity.users SET deleted_at = now(), delete_reason = 'inactive_cleanup' WHERE username = 'hs.mx.an'`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.platform.ExecContext(ctx, `UPDATE identity.users SET deleted_at = now(), delete_reason = 'inactive_cleanup' WHERE username = 'hs.mx.chi'`); err != nil {
		t.Fatal(err)
	}
	run(sA, map[string]string{"students.csv": stHeader + "HS-1,TH-A,001,An Moi,3,active,hs.mx.an\nHS-77,TH-A,077,Chi,3,active,hs.mx.chi\n"})
	if n := issues("cleaned_up_student_in_sis"); n != 2 {
		t.Fatalf("cleaned_up_student_in_sis = %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE username IN ('hs.mx.an', 'hs.mx.chi') AND deleted_at IS NOT NULL`); n != 2 {
		t.Fatalf("restored by a sync: %d still deleted", n)
	}
}
